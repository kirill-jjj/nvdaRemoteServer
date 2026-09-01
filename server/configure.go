package server

import (
	"context"
	"crypto/tls"
	"flag"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/caddyserver/certmagic"
)

var confFile string

var cert string

var key string

var logfile string

var loglevel int

var motd string

var motdAlwaysDisplay bool

var timeoutSecs float64

var pingTime int

var maxMsgLen int

var iface string

var iface6 string

var port int

var port6 int

var (
	domain    string
	acmeEmail string
	acmeCA    string
)

var includeTracebacks bool

var Servers []*Server

var (
	PID     int
	PID_STR string
	pidfile string
)

// The command line flag set, used to tell which parameters were given
// on the command line (they take priority over the configuration file).
var cliSet map[string]bool

func Configure() error {
	PID = os.Getpid()
	PID_STR = strconv.Itoa(PID)

	flag.CommandLine.SetOutput(os.Stdout)
	registerFlags()
	flag.Parse()

	cliSet = make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cliSet[f.Name] = true
	})

	applyConfigFile()
	log_init(logfile)
	Log(LOG_INFO, "Initializing configuration.")

	validateSettings()
	handleMotd()

	config, err := buildTLSConfig()
	if err != nil {
		return err
	}

	addrs := build_addresses()
	Servers = make([]*Server, len(addrs))
	for i, addr := range addrs {
		Servers[i] = NewWithTLSConfig(addr, config)
		Log(LOG_DEBUG, "Starting server listening on address "+addr)
	}

	return nil
}

// registerFlags registers all command line flags. The flags mirror
// the Python NVDARemoteServer exactly, both in name and behavior.
func registerFlags() {
	flag.StringVar(&confFile, "configfile", DEFAULT_CONF_FILE, "Path to a configuration file in the Python NVDARemoteServer format (option=value pairs). If the file does not exist, or can't be read, default or command line values are used.")
	flag.StringVar(&cert, "certfile", DEFAULT_CERT_FILE, "SSL certificate file to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&key, "keyfile", DEFAULT_KEY_FILE, "SSL key to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&domain, "domain", DEFAULT_DOMAIN, "Domain name for automatic TLS certificate management via Let's Encrypt / CertMagic.")
	flag.StringVar(&acmeEmail, "acme_email", DEFAULT_ACME_EMAIL, "Email address for ACME registration.")
	flag.StringVar(&acmeCA, "acme_ca", DEFAULT_ACME_CA, "Custom ACME CA URL (optional).")
	flag.StringVar(&pidfile, "pidfile", DEFAULT_PID_FILE, "Create a PID file when the server has successfully started.")
	flag.IntVar(&loglevel, "loglevel", DEFAULT_LOG_LEVEL, "Choose what log level you wish to use. Any value below -1 will be ignored.")
	flag.StringVar(&logfile, "logfile", DEFAULT_LOG_FILE, "Choose what log file you wish to use in addition to logging output to the console. If the file can't be created or open for writing, the program will fall back to console logging only.")
	flag.StringVar(&motd, "motd", DEFAULT_MOTD, "Display a message of the day for the server.")
	flag.BoolVar(&motdAlwaysDisplay, "motd_force_display", DEFAULT_MOTD_ALWAYS_DISPLAY, "Force the message of the day to be displayed upon each connection to the server, even if it hasn't changed.")
	flag.BoolVar(&includeTracebacks, "includeTracebacks", false, "Accepted for compatibility with the Python NVDARemoteServer. This Go server has no tracebacks, so the option has no effect.")
	flag.Float64Var(&timeoutSecs, "timeout", DEFAULT_TIMEOUT_SECS, "Maximum time, in seconds, a client can be connected without negotiating a TLS connection before an exception is raised. Values below 1.0 are reset to the default.")
	flag.IntVar(&pingTime, "ping_time", DEFAULT_PING_TIME, "Interval, in seconds, at which the server pings all connected clients. Values below 30 are reset to the default.")
	flag.IntVar(&maxMsgLen, "allowedMessageLength", DEFAULT_MAX_MSG_LEN, "Maximum allowed length, in characters, of incoming client messages. 0 means no limit. Clients sending longer messages are disconnected.")
	flag.StringVar(&iface, "interface", DEFAULT_INTERFACE, "IPv4 interface the server will listen on. This does not affect IPv6 interfaces. An empty value means all IPv4 interfaces.")
	flag.StringVar(&iface6, "interface6", DEFAULT_INTERFACE6, "IPv6 interface the server will listen on. This does not affect IPv4 interfaces. An empty value means all IPv6 interfaces.")
	flag.IntVar(&port, "port", DEFAULT_PORT, "TCP port the server will listen on for IPv4 connections. The port must be between 1 and 65535.")
	flag.IntVar(&port6, "port6", DEFAULT_PORT6, "TCP port the server will listen on for IPv6 connections. By default, uses the value specified in --port. The port must be between 1 and 65535.")
}

// applyConfigFile reads the configuration file if it exists, applying
// values to global variables. Command line flags take priority.
func applyConfigFile() {
	if confFile != "" {
		opts, err := conf_read_python(confFile)
		if err != nil {
			conf_print_missing(confFile, err)
		} else {
			conf_apply(opts, cliSet)
		}
	} else {
		conf_print_missing("", nil)
	}
}

// validateSettings resets invalid configuration values to defaults,
// mirroring the Python server's behavior of silently ignoring bad values.
func validateSettings() {
	if timeoutSecs < 1.0 {
		timeoutSecs = DEFAULT_TIMEOUT_SECS
		Log(LOG_INFO, "Timeout is less than 1.0 seconds, resetting to "+strconv.FormatFloat(DEFAULT_TIMEOUT_SECS, 'f', -1, 64))
	}
	if pingTime < 30 {
		pingTime = DEFAULT_PING_TIME
		Log(LOG_INFO, "Ping time is less than 30 seconds, resetting to "+strconv.Itoa(DEFAULT_PING_TIME))
	}
	if port < 1 || port > 65535 {
		port = DEFAULT_PORT
		Log(LOG_INFO, "Invalid port value, resetting to "+strconv.Itoa(DEFAULT_PORT))
	}
	if port6 < 1 || port6 > 65535 {
		port6 = port
		Log(LOG_INFO, "Invalid port6 value, resetting to "+strconv.Itoa(port))
	}
	if loglevel < LOG_SILENT {
		loglevel = LOG_SILENT
		Log(LOG_INFO, "Log level is less than silent log value, resetting to "+strconv.Itoa(LOG_SILENT))
	}
	if loglevel > LOG_PROTOCOL {
		loglevel = LOG_PROTOCOL
		Log(LOG_INFO, "Log level is greater than protocol log value, resetting to "+strconv.Itoa(LOG_PROTOCOL))
	}
}

// handleMotd processes the message of the day configuration,
// including the protocol logging warning.
func handleMotd() {
	if loglevel == LOG_PROTOCOL {
		Log(LOG_INFO, "Protocol logging is enabled. The server message of the day will be set to display always, and if unset, will have a value added to it that will alert all users connecting that protocol logging is enabled.")
		protocollogmotd := "WARNING!\nAll server information is being logged, including the protocol being used. This server is running in an insecure mode for production."
		if motd == "" {
			motd = protocollogmotd
		} else {
			motd = protocollogmotd + "\n" + motd
		}
		motdAlwaysDisplay = true
	}
	if motd != DEFAULT_MOTD {
		logstr := "The server will display the following message of the day:\r\n" + motd
		if motdAlwaysDisplay != DEFAULT_MOTD_ALWAYS_DISPLAY {
			logstr += "\r\nThe server will tell each client to display this message of the day upon each connection."
		}
		Log(LOG_DEBUG, logstr)
	}
	if motd == DEFAULT_MOTD && motdAlwaysDisplay == DEFAULT_MOTD_ALWAYS_DISPLAY {
		Log(LOG_INFO, "The server has been told to always display a message of the day, but no message of the day has been set. The -motd_force_display parameter will be reset to false.")
		motdAlwaysDisplay = false
	}
}

// buildTLSConfig creates the TLS configuration from certificate files,
// CertMagic ACME, or self-signed generation.
func buildTLSConfig() (*tls.Config, error) {
	generate := false
	if cert != DEFAULT_CERT_FILE && !fileExists(cert) {
		Log(LOG_INFO, "The certificate file at "+cert+" does not exist.")
		generate = true
	}
	if key != DEFAULT_KEY_FILE && !fileExists(key) {
		Log(LOG_INFO, "The key file at "+key+" does not exist.")
		generate = true
	}
	if cert == DEFAULT_CERT_FILE || key == DEFAULT_KEY_FILE {
		generate = true
	}

	if domain != "" {
		return buildCertMagicConfig()
	}
	if generate {
		return buildSelfSignedConfig()
	}
	return buildExplicitCertConfig()
}

// buildCertMagicConfig creates a TLS config via CertMagic ACME.
func buildCertMagicConfig() (*tls.Config, error) {
	domains := strings.Split(domain, ",")
	for i := range domains {
		domains[i] = strings.TrimSpace(domains[i])
	}

	Log(LOG_INFO, "Configuring CertMagic ACME for domain(s): "+strings.Join(domains, ", "))

	if acmeEmail != "" {
		certmagic.DefaultACME.Email = acmeEmail
	}
	certmagic.DefaultACME.Agreed = true
	if acmeCA != "" {
		certmagic.DefaultACME.CA = acmeCA
	}

	magic := certmagic.NewDefault()
	if err := magic.ManageSync(context.Background(), domains); err != nil {
		Log_error("CertMagic error managing domain certificates:\r\n" + err.Error())
		return nil, err
	}

	Log(LOG_INFO, "CertMagic successfully obtained/loaded TLS certificate.")
	config := magic.TLSConfig()
	// TLS 1.2 is the minimum: NVDA Remote addon (Python) bundled with
	// NVDA uses ssl.SSLContext() which defaults to PROTOCOL_TLS. On
	// Python 3.7-3.9 (used by NVDA 2019.3-2023.1), TLS 1.3 is not
	// available, so requiring TLS 1.3 breaks compatibility.
	config.MinVersion = tls.VersionTLS12
	return config, nil
}

// buildSelfSignedConfig generates a self-signed certificate in memory.
func buildSelfSignedConfig() (*tls.Config, error) {
	Log(LOG_DEBUG, "Attempting to generate self-signed SSL certificate.")
	config, err := gen_cert()
	if err != nil {
		Log_error("Unable to generate self-signed certificate.\r\n" + err.Error() + "\r\nUnable to start server.")
		return nil, err
	}
	Log(LOG_DEBUG, "SSL certificate generated.")
	config.MinVersion = tls.VersionTLS12
	return config, nil
}

// buildExplicitCertConfig loads certificate files specified by the user.
func buildExplicitCertConfig() (*tls.Config, error) {
	certPair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		Log_error("Error loading certificate and key files.\r\n" + err.Error() + "\r\nUnable to start server.")
		return nil, err
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{certPair},
		// TLS 1.2 minimum for NVDA Remote addon compatibility.
		MinVersion: tls.VersionTLS12,
	}
	return config, nil
}

// build_addresses returns the listen addresses for the configured
// IPv4/IPv6 interfaces and ports.
func build_addresses() []string {
	v4 := net.JoinHostPort(iface, strconv.Itoa(port))
	v6 := net.JoinHostPort(iface6, strconv.Itoa(port6))
	if v4 == v6 {
		// Both defaults: a single dual-stack wildcard address covers
		// IPv4 and IPv6, which is what the Python server achieves with
		// its IPv6 socket.
		return []string{v4}
	}
	return []string{v4, v6}
}

func Start() int {
	num := 0
	var err error

	for i := range Servers {
		err = Servers[i].Listen()
		if err != nil {
			Log_error("Unable to listen on address " + Servers[i].address + ".\r\n" + err.Error())
			Servers[i] = nil
			continue
		}
		num++
	}
	if num == 0 {
		Servers = nil
		return num
	}

	Log(LOG_DEBUG, "Number of servers started: "+strconv.Itoa(num))
	return num
}

// Launch_fail is kept for compatibility with the removed -launch flag;
// it is never triggered now, because the server always launches.
func Launch_fail() {}

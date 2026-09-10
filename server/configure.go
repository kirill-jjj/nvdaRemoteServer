package server

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
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
	pidStr  string
	pidfile string
)

// The command line flag set, used to tell which parameters were given
// on the command line (they take priority over the configuration file).
var cliSet map[string]bool

func Configure() error {
	PID = os.Getpid()
	pidStr = strconv.Itoa(PID)

	flag.CommandLine.SetOutput(os.Stdout)
	registerFlags()
	flag.Parse()

	cliSet = make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cliSet[f.Name] = true
	})

	applyConfigFile()
	log_init(logfile)
	Log(LogInfo, "initializing configuration")

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
		Log(LogDebug, "starting server on address", "address", addr)
	}

	return nil
}

// registerFlags registers all command line flags. The flags mirror
// the Python NVDARemoteServer exactly, both in name and behavior.
func registerFlags() {
	flag.StringVar(&confFile, "configfile", defaultConfFile, "Path to a configuration file in the Python NVDARemoteServer format (option=value pairs). If the file does not exist, or can't be read, default or command line values are used.")
	flag.StringVar(&cert, "certfile", defaultCertFile, "SSL certificate file to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&key, "keyfile", defaultKeyFile, "SSL key to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&domain, "domain", defaultDomain, "Domain name for automatic TLS certificate management via Let's Encrypt / CertMagic.")
	flag.StringVar(&acmeEmail, "acme_email", defaultAcmeEmail, "Email address for ACME registration.")
	flag.StringVar(&acmeCA, "acme_ca", defaultAcmeCa, "Custom ACME CA URL (optional).")
	flag.StringVar(&pidfile, "pidfile", DefaultPIDFile, "Create a PID file when the server has successfully started.")
	flag.IntVar(&loglevel, "loglevel", defaultLogLevel, "Choose what log level you wish to use. Any value below -1 will be ignored.")
	flag.StringVar(&logfile, "logfile", defaultLogFile, "Choose what log file you wish to use in addition to logging output to the console. If the file can't be created or open for writing, the program will fall back to console logging only.")
	flag.StringVar(&motd, "motd", defaultMotd, "Display a message of the day for the server.")
	flag.BoolVar(&motdAlwaysDisplay, "motd_force_display", defaultMotdAlwaysDisplay, "Force the message of the day to be displayed upon each connection to the server, even if it hasn't changed.")
	flag.BoolVar(&includeTracebacks, "includeTracebacks", false, "Accepted for compatibility with the Python NVDARemoteServer. This Go server has no tracebacks, so the option has no effect.")
	flag.Float64Var(&timeoutSecs, "timeout", defaultTimeoutSecs, "Maximum time, in seconds, a client can be connected without negotiating a TLS connection before an exception is raised. Values below 1.0 are reset to the default.")
	flag.IntVar(&pingTime, "ping_time", defaultPingTime, "Interval, in seconds, at which the server pings all connected clients. Values below 30 are reset to the default.")
	flag.IntVar(&maxMsgLen, "allowedMessageLength", defaultMaxMsgLen, "Maximum allowed length, in characters, of incoming client messages. 0 means no limit. Clients sending longer messages are disconnected.")
	flag.StringVar(&iface, "interface", defaultInterface, "IPv4 interface the server will listen on. This does not affect IPv6 interfaces. An empty value means all IPv4 interfaces.")
	flag.StringVar(&iface6, "interface6", defaultInterface6, "IPv6 interface the server will listen on. This does not affect IPv4 interfaces. An empty value means all IPv6 interfaces.")
	flag.IntVar(&port, "port", defaultPort, "TCP port the server will listen on for IPv4 connections. The port must be between 1 and 65535.")
	flag.IntVar(&port6, "port6", defaultPort6, "TCP port the server will listen on for IPv6 connections. By default, uses the value specified in --port. The port must be between 1 and 65535.")
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
		timeoutSecs = defaultTimeoutSecs
		Log(LogInfo, "timeout reset to default", "value", defaultTimeoutSecs)
	}
	if pingTime < 30 {
		pingTime = defaultPingTime
		Log(LogInfo, "ping_time reset to default", "value", defaultPingTime)
	}
	if port < 1 || port > 65535 {
		port = defaultPort
		Log(LogInfo, "port reset to default", "value", defaultPort)
	}
	if port6 < 1 || port6 > 65535 {
		port6 = port
		Log(LogInfo, "port6 reset to port value", "value", port)
	}
	if loglevel < LogSilent {
		loglevel = LogSilent
		Log(LogInfo, "loglevel reset to silent", "value", LogSilent)
	}
	if loglevel > LogProtocol {
		loglevel = LogProtocol
		Log(LogInfo, "loglevel reset to protocol", "value", LogProtocol)
	}
}

// handleMotd processes the message of the day configuration,
// including the protocol logging warning.
func handleMotd() {
	if loglevel == LogProtocol {
		Log(LogInfo, "protocol logging enabled")
		protocollogmotd := "WARNING!\nAll server information is being logged, including the protocol being used. This server is running in an insecure mode for production."
		if motd == "" {
			motd = protocollogmotd
		} else {
			motd = protocollogmotd + "\n" + motd
		}
		motdAlwaysDisplay = true
	}
	if motd != defaultMotd {
		Log(LogDebug, "MOTD configured", "motd", motd, "force_display", motdAlwaysDisplay)
	}
	if motd == defaultMotd && motdAlwaysDisplay == defaultMotdAlwaysDisplay {
		Log(LogInfo, "MOTD force_display reset to false (no MOTD set)")
		motdAlwaysDisplay = false
	}
}

// buildTLSConfig creates the TLS configuration from certificate files,
// CertMagic ACME, or self-signed generation.
func buildTLSConfig() (*tls.Config, error) {
	generate := false
	if cert != defaultCertFile && !fileExists(cert) {
		Log(LogInfo, "certificate file does not exist", "file", cert)
		generate = true
	}
	if key != defaultKeyFile && !fileExists(key) {
		Log(LogInfo, "key file does not exist", "file", key)
		generate = true
	}
	if cert == defaultCertFile || key == defaultKeyFile {
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

	Log(LogInfo, "configuring CertMagic ACME", "domains", strings.Join(domains, ", "))

	if acmeEmail != "" {
		certmagic.DefaultACME.Email = acmeEmail
	}
	certmagic.DefaultACME.Agreed = true
	if acmeCA != "" {
		certmagic.DefaultACME.CA = acmeCA
	}

	magic := certmagic.NewDefault()
	if err := magic.ManageSync(context.Background(), domains); err != nil {
		LogError("CertMagic error", "error", err)
		return nil, fmt.Errorf("certmagic ManageSync for %s: %w", strings.Join(domains, ","), err)
	}

	Log(LogInfo, "CertMagic certificate obtained")
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
	Log(LogDebug, "generating self-signed certificate")
	config, err := gen_cert()
	if err != nil {
		LogError("unable to generate self-signed certificate", "error", err)
		return nil, err
	}
	Log(LogDebug, "self-signed certificate generated")
	config.MinVersion = tls.VersionTLS12
	return config, nil
}

// buildExplicitCertConfig loads certificate files specified by the user.
func buildExplicitCertConfig() (*tls.Config, error) {
	certPair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		LogError("error loading certificate files", "error", err)
		return nil, fmt.Errorf("loading X509 key pair (%s, %s): %w", cert, key, err)
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
			LogError("unable to listen on address", "address", Servers[i].address, "error", err)
			Servers[i] = nil
			continue
		}
		num++
	}
	if num == 0 {
		Servers = nil
		return num
	}

	Log(LogDebug, "servers started", "count", num)
	return num
}

// Launch_fail is kept for compatibility with the removed -launch flag;
// it is never triggered now, because the server always launches.
func Launch_fail() {}

package server

import (
	"crypto/tls"
	"flag"
	"net"
	"os"
	"strconv"
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

	// The command line parameters mirror the Python NVDARemoteServer
	// exactly, both in name and in priority over the configuration file.
	flag.StringVar(&confFile, "configfile", DEFAULT_CONF_FILE, "Path to a configuration file in the Python NVDARemoteServer format (option=value pairs). If the file does not exist, or can't be read, default or command line values are used.")
	flag.StringVar(&cert, "certfile", DEFAULT_CERT_FILE, "SSL certificate file to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&key, "keyfile", DEFAULT_KEY_FILE, "SSL key to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
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

	flag.Parse()

	// Record which parameters were set on the command line so they take
	// priority over the configuration file.
	cliSet = make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cliSet[f.Name] = true
	})

	// Read the configuration file, exactly like the Python server: the
	// file is read if it exists, otherwise default or command line
	// values are used.
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

	log_init(logfile)

	Log(LOG_INFO, "Initializing configuration.")

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

	if !default_motd(motd) {
		logstr := "The server will display the following message of the day:\r\n" + motd
		if default_motd_always_display(motdAlwaysDisplay) {
			logstr += "\r\nThe server will tell each client to display this message of the day upon each connection."
		}
		Log(LOG_DEBUG, logstr)
	}

	if default_motd(motd) && !default_motd_always_display(motdAlwaysDisplay) {
		Log(LOG_INFO, "The server has been told to always display a message of the day, but no message of the day has been set. The -motd_force_display parameter will be reset to false.")
		motdAlwaysDisplay = false
	}

	// Build the TLS configuration. If explicit certificate files are
	// given they are loaded; otherwise a self-signed certificate is
	// generated in memory, so the server works out of the box.
	generate := false
	var config *tls.Config
	var err error

	if !default_cert_file(cert) && !fileExists(cert) {
		Log(LOG_INFO, "The certificate file at "+cert+" does not exist.")
		generate = true
	}
	if !default_key_file(key) && !fileExists(key) {
		Log(LOG_INFO, "The key file at "+key+" does not exist.")
		generate = true
	}
	if default_cert_file(cert) || default_key_file(key) {
		generate = true
	}

	if generate {
		Log(LOG_DEBUG, "Attempting to generate self-signed SSL certificate.")
		config, err = gen_cert()
		if err != nil {
			Log_error("Unable to generate self-signed certificate.\r\n" + err.Error() + "\r\nUnable to start server.")
			return err
		}
		Log(LOG_DEBUG, "SSL certificate generated.")
	} else {
		cert, cerr := tls.LoadX509KeyPair(cert, key)
		if cerr != nil {
			Log_error("Error loading certificate and key files.\r\n" + cerr.Error() + "\r\nUnable to start server.")
			return cerr
		}
		config = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	config.MinVersion = tls.VersionTLS12

	// Build the listen addresses from the IPv4 and IPv6 interface and
	// port options, like the Python server does. When both interfaces
	// and both ports are at their defaults, a single dual-stack
	// wildcard address is used.
	addrs := build_addresses()

	Servers = make([]*Server, len(addrs))
	for i, addr := range addrs {
		Servers[i] = NewWithTLSConfig(addr, config)
		Log(LOG_DEBUG, "Starting server listening on address "+addr)
	}

	return nil
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

package server

import (
	"crypto/tls"
	"errors"
	"flag"
	"net"
	"os"
	"strconv"
)

var confFile string

var (
	genConfFile string
	genConfDir  bool
)

var confRead bool

var addresses AddressList

var cert string

var key string

var gencertfile string

var logfile string

var loglevel int

var motd string

var motdAlwaysDisplay bool

var sendOrigin bool

var timeoutSecs float64

var pingTime int

var maxMsgLen int

var iface string

var iface6 string

var port int

var port6 int

var createDir bool

var Launch bool

var Servers []*Server

var (
	PID     int
	PID_STR string
	pidfile string
)

func Configure() error {
	PID = os.Getpid()
	PID_STR = strconv.Itoa(PID)

	flag.CommandLine.SetOutput(os.Stdout)

	flag.BoolVar(&createDir, "create", DEFAULT_CREATE_DIR, "Create directories upon any operation involving files being written to, or the working directory being changed.")

	flag.StringVar(&confFile, "conf-file", DEFAULT_CONF_FILE, "Path to a configuration file. If the configuration file does not exist, or there is an error reading the configuration file, the program will fall back to command line parameters.")

	flag.StringVar(&genConfFile, "gen-conf-file", DEFAULT_GEN_CONF_FILE, "Path to a configuration file to generate from command line parameters. If the configuration file can't be generated, an error message will be logged.")
	flag.BoolVar(&genConfDir, "gen-conf-dir", DEFAULT_GEN_CONF_DIR, "Whether or not to generate a configuration directory for the user. If the configuration directory and file can't be generated, an error message will be logged.")

	flag.BoolVar(&confRead, "conf-read", DEFAULT_CONF_READ, "Whether or not to read a configuration file. If a configuration file will not be read or searched for, the program will warn you. If you set a configuration file parameter, it will be reset to its default value.")

	flag.StringVar(&cert, "cert-file", DEFAULT_CERT_FILE, "SSL certificate file to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&key, "key-file", DEFAULT_KEY_FILE, "SSL key to use for the server's TLS connection, must point to an existing file. If this is empty, the server will automatically generate its own self-signed certificate.")
	flag.StringVar(&gencertfile, "gen-cert-file", DEFAULT_GEN_CERT_FILE, "Generate a certificate file from the self-generated, self-signed SSL certificate. This file will only be created if you aren't loading your own certificate key files. The file will encode the key and certificate, packaging them both in a single .pem file.")

	flag.StringVar(&pidfile, "pid-file", DEFAULT_PID_FILE, "Create a PID file when the server has successfully started.")

	flag.Var(&addresses, "address", "Address the server will listen on in the format ip:port, such as \"0.0.0.0:6837\", \":6837\", \"[::]:6837\". The port must be between 1 and 65536. You can declare this parameter more than once for multiple listen addresses.")

	flag.IntVar(&loglevel, "log-level", DEFAULT_LOG_LEVEL, "Choose what log level you wish to use. Any value below -1 will be ignored.")
	flag.StringVar(&logfile, "log-file", DEFAULT_LOG_FILE, "Choose what log file you wish to use in addition to logging output to the console. If the file can't be created or open for writing, the program will fall back to console logging only.")

	flag.StringVar(&motd, "motd", DEFAULT_MOTD, "Display a message of the day for the server.")
	flag.BoolVar(&motdAlwaysDisplay, "motd-always-display", DEFAULT_MOTD_ALWAYS_DISPLAY, "Force the message of the day to be displayed upon each connection to the server, even if it hasn't changed.")

	flag.BoolVar(&sendOrigin, "send-origin", DEFAULT_SEND_ORIGIN, "Send an origin message from every message received by a client.")

	flag.Float64Var(&timeoutSecs, "timeout", DEFAULT_TIMEOUT_SECS, "Maximum time, in seconds, a client can be connected without negotiating a TLS connection before an exception is raised. Values below 1.0 are reset to the default.")

	flag.IntVar(&pingTime, "ping-time", DEFAULT_PING_TIME, "Interval, in seconds, at which the server pings all connected clients. Values below 30 are reset to the default.")

	flag.IntVar(&maxMsgLen, "max-message-length", DEFAULT_MAX_MSG_LEN, "Maximum allowed length, in characters, of incoming client messages. 0 means no limit. Clients sending longer messages are disconnected.")

	flag.StringVar(&iface, "interface", DEFAULT_INTERFACE, "IPv4 interface the server will listen on. This does not affect IPv6 interfaces. An empty value means all IPv4 interfaces.")

	flag.StringVar(&iface6, "interface6", DEFAULT_INTERFACE6, "IPv6 interface the server will listen on. This does not affect IPv4 interfaces. An empty value means all IPv6 interfaces.")

	flag.IntVar(&port, "port", DEFAULT_PORT, "TCP port the server will listen on for IPv4 connections. The port must be between 1 and 65535.")

	flag.IntVar(&port6, "port6", DEFAULT_PORT6, "TCP port the server will listen on for IPv6 connections. By default, uses the value specified in --port. The port must be between 1 and 65535.")

	flag.BoolVar(&Launch, "launch", DEFAULT_LAUNCH, "Launch the server.")

	flag.Parse()

	if len(addresses) == 0 {
		addresses = make(AddressList, 1)
		addresses[0] = DEFAULT_ADDRESS
	}

	c := cfg_default()
	cfg_err := c.Setup()

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

	// If no -address has been specified, build the listen addresses from
	// the -interface/-interface6 and -port/-port6 options, mirroring the
	// Python server. -address always takes priority.
	if default_addresses(addresses) && (!default_interface(iface) || !default_interface6(iface6) || !default_port(port) || !default_port6(port6)) {
		addresses = make(AddressList, 0, 2)
		if !default_interface(iface) || !default_port(port) {
			addr := ""
			if iface != "" {
				addr = iface
			}
			addr = net.JoinHostPort(addr, strconv.Itoa(port))
			addresses = append(addresses, addr)
		}
		if !default_interface6(iface6) || !default_port6(port6) {
			addr := ""
			if iface6 != "" {
				addr = iface6
			}
			addr = net.JoinHostPort(addr, strconv.Itoa(port6))
			addresses = append(addresses, addr)
		}
		if len(addresses) == 0 {
			addresses = make(AddressList, 1)
			addresses[0] = DEFAULT_ADDRESS
		}
	}

	c.LogWrite()

	if c.panicString != "" {
		Log_close()
		os.Exit(2)
	}
	if cfg_err != nil {
		Log_close()
		os.Exit(1)
	}

	defer PanicHandle.Catch()

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
			Launch_fail()
			return err
		}
		Log(LOG_DEBUG, "SSL certificate generated.")
	} else {
		if gencertfile != "" {
			Log(LOG_INFO, "The server has not generated its own self-signed certificate, and the -gen-certfile parameter is set to "+gencertfile+". This parameter will be ignored.")
		}
		cert, cerr := tls.LoadX509KeyPair(cert, key)
		if cerr != nil {
			Log_error("Error loading certificate and key files.\r\n" + cerr.Error() + "\r\nUnable to start server.")
			Launch_fail()
			return cerr
		}
		config = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	config.MinVersion = tls.VersionTLS12

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
		Log(LOG_INFO, "The server has been told to always display a message of the day, but no message of the day has been set. The -motd-always-display parameter will be reset to false.")
		motdAlwaysDisplay = false
	}

	if !sendOrigin {
		Log(LOG_INFO, "The server is configured to send no origin message to other clients, which may improve performance slightly, but impact the useability of your server when the origin field is required.")
	}

	if !Launch {
		Log(LOG_INFO, "The server will not be launched. Shutting down.")
		return errors.New("Server launch parameter set to false.")
	}

	Servers = make([]*Server, len(addresses))
	for i, addr := range addresses {
		Servers[i] = NewWithTLSConfig(addr, config)
		Log(LOG_DEBUG, "Starting server listening on address "+addr)
	}

	return nil
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
	go signals_init()
	return num
}

func Launch_fail() {
	if !Launch {
		os.Exit(1)
	}
}

func gen_conf_check() bool {
	return (!default_gen_conf_file(genConfFile) || !default_gen_conf_dir(genConfDir))
}

package server

import (
	"os"
	"runtime"
)

// pathSeparator is the OS path separator, exposed for file-path
// helpers in this package.
var pathSeparator = string(os.PathSeparator)

// Configuration defaults mirror the Python NVDARemoteServer.
var (
	defaultConfFile string = ""
)

var (
	defaultCertFile string = ""
	defaultKeyFile  string = ""
)

var (
	defaultDomain    string = ""
	defaultAcmeEmail string = ""
	defaultAcmeCa    string = ""
)

var defaultLogFile string = ""

// Log levels use iota so adding a new level requires zero value changes.
const (
	LogSilent     = iota - 1 // -1
	LogInfo                  //  0
	LogConnection            //  1
	LogChannel               //  2
	LogDebug                 //  3
	LogProtocol              //  4
)

var defaultLogLevel int = LogChannel

var (
	defaultMotd              string = ""
	defaultMotdAlwaysDisplay bool   = false
)

var DefaultPIDFile string = ""

// Options ported from the Python server: TLS handshake timeout, client
// ping interval (300 seconds, as in Python), maximum incoming message
// length and separate IPv4/IPv6 interfaces and ports.
var (
	defaultTimeoutSecs float64 = 5.0
	defaultPingTime    int     = 300
	defaultMaxMsgLen   int     = 0
)

var (
	defaultInterface  string = ""
	defaultInterface6 string = ""
	defaultPort       int    = 6837
	defaultPort6      int    = 6837
)

func init() {
	switch runtime.GOOS {
	case "linux":
		defaultConfFile = "/etc/NVDARemoteServer.conf"
		defaultCertFile = "/usr/share/NVDARemoteServer/server.pem"
		defaultLogFile = "/var/log/NVDARemoteServer/NVDARemoteServer.log"
		DefaultPIDFile = "/run/NVDARemoteServer/NVDARemoteServer.pid"
	case "darwin":
		defaultConfFile = "/etc/NVDARemoteServer.conf"
		defaultCertFile = "/usr/share/NVDARemoteServer/server.pem"
		defaultLogFile = "/var/log/NVDARemoteServer/NVDARemoteServer.log"
		DefaultPIDFile = "/var/run/NVDARemoteServer.pid"
	default:
		// Windows and other systems: empty defaults, like Python.
	}
}

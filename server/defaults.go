package server

import (
	"os"
	"runtime"
)

var PS string = string(os.PathSeparator)

// Configuration file defaults mirror the Python NVDARemoteServer.
var (
	DEFAULT_CONF_FILE string = ""
	DEFAULT_CONF_NAME string = "NVDARemoteServer.conf"
)

var (
	DEFAULT_CERT_FILE string = ""
	DEFAULT_KEY_FILE  string = ""
)

var DEFAULT_DOMAIN     string = ""
var DEFAULT_ACME_EMAIL string = ""
var DEFAULT_ACME_CA    string = ""

var DEFAULT_LOG_FILE string = ""

// The Python server defaults to log level 2.
var DEFAULT_LOG_LEVEL int = 2

const (
	LOG_SILENT     int = -1
	LOG_INFO       int = 0
	LOG_CONNECTION int = 1
	LOG_CHANNEL    int = 2
	LOG_DEBUG      int = 3
	LOG_PROTOCOL   int = 4
)

var (
	DEFAULT_MOTD                string = ""
	DEFAULT_MOTD_ALWAYS_DISPLAY bool   = false
)

var DEFAULT_PID_FILE string = ""

// Options ported from the Python server: TLS handshake timeout, client
// ping interval (300 seconds, as in Python), maximum incoming message
// length and separate IPv4/IPv6 interfaces and ports.
var (
	DEFAULT_TIMEOUT_SECS float64 = 5.0
	DEFAULT_PING_TIME    int     = 300
	DEFAULT_MAX_MSG_LEN  int     = 0
)

var (
	DEFAULT_INTERFACE  string = ""
	DEFAULT_INTERFACE6 string = ""
	DEFAULT_PORT       int    = 6837
	DEFAULT_PORT6      int    = 6837
)

func init() {
	switch runtime.GOOS {
	case "linux":
		// Same defaults as the Python server on Linux.
		DEFAULT_CONF_FILE = "/etc/NVDARemoteServer.conf"
		DEFAULT_CERT_FILE = "/usr/share/NVDARemoteServer/server.pem"
		DEFAULT_LOG_FILE = "/var/log/NVDARemoteServer/NVDARemoteServer.log"
		DEFAULT_PID_FILE = "/run/NVDARemoteServer/NVDARemoteServer.pid"
	case "darwin":
		DEFAULT_CONF_FILE = "/etc/NVDARemoteServer.conf"
		DEFAULT_CERT_FILE = "/usr/share/NVDARemoteServer/server.pem"
		DEFAULT_LOG_FILE = "/var/log/NVDARemoteServer/NVDARemoteServer.log"
		DEFAULT_PID_FILE = "/var/run/NVDARemoteServer.pid"
	default:
		// Windows and other systems: empty defaults, like Python.
	}
}

func default_cert_file(p string) bool {
	return (p == DEFAULT_CERT_FILE)
}

func default_key_file(p string) bool {
	return (p == DEFAULT_KEY_FILE)
}

func default_motd(p string) bool {
	return (p == DEFAULT_MOTD)
}

func default_motd_always_display(p bool) bool {
	return (p == DEFAULT_MOTD_ALWAYS_DISPLAY)
}

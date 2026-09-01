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

var (
	DEFAULT_DOMAIN     string = ""
	DEFAULT_ACME_EMAIL string = ""
	DEFAULT_ACME_CA    string = ""
)

var DEFAULT_LOG_FILE string = ""

// Log levels use iota so adding a new level requires zero value changes.
const (
	LOG_SILENT     = iota - 1 // -1
	LOG_INFO                  //  0
	LOG_CONNECTION            //  1
	LOG_CHANNEL               //  2
	LOG_DEBUG                 //  3
	LOG_PROTOCOL              //  4
)

var DEFAULT_LOG_LEVEL int = LOG_CHANNEL

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

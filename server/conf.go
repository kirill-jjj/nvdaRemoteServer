package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// conf_read_python parses a configuration file in the exact format of
// the Python NVDARemoteServer: one option=value pair per line, UTF-8
// encoded, lines starting with '#' (or blank lines) ignored. Values
// may contain '=' characters (only the first one separates the option
// name from the value). Unknown options are ignored, like the Python
// server does.
func conf_read_python(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	opts := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		opts[strings.TrimSpace(parts[0])] = parts[1]
	}
	return opts, nil
}

// conf_apply applies the values read from a Python-format configuration
// file to the global configuration variables. Command line parameters
// (tracked in cliSet) always take priority over the configuration file,
// exactly like the Python server, where command line arguments override
// the configuration file. Invalid values for a given option are
// silently ignored (the previous value is kept), also like Python.
func conf_apply(opts map[string]string, cliSet map[string]bool) {
	if v, ok := opts["interface"]; ok && !cliSet["interface"] {
		iface = v
	}
	if v, ok := opts["interface6"]; ok && !cliSet["interface6"] {
		iface6 = v
	}
	if v, ok := opts["logfile"]; ok && !cliSet["logfile"] {
		logfile = v
	}
	if v, ok := opts["pidfile"]; ok && !cliSet["pidfile"] {
		pidfile = v
	}
	if v, ok := opts["keyfile"]; ok && !cliSet["keyfile"] {
		key = v
	}
	if v, ok := opts["certfile"]; ok && !cliSet["certfile"] {
		cert = v
	}
	if v, ok := opts["motd"]; ok && !cliSet["motd"] {
		motd = v
	}
	if v, ok := opts["motd_force_display"]; ok && !cliSet["motd_force_display"] {
		if n, err := strconv.Atoi(v); err == nil {
			motdAlwaysDisplay = n != 0
		}
	}
	// includeTracebacks exists in the Python server; this Go server has
	// no tracebacks, so the option is accepted and ignored.
	// (The value is not validated here: conf_apply runs before the
	// logger is initialized, and the Python server ignores bad values.)
	if _, ok := opts["includeTracebacks"]; ok && !cliSet["includeTracebacks"] {
		// no-op, accepted for Python configuration compatibility
	}
	if v, ok := opts["port"]; ok && !cliSet["port"] {
		if n, err := strconv.Atoi(v); err == nil {
			port = n
		}
	}
	if v, ok := opts["port6"]; ok && !cliSet["port6"] {
		if n, err := strconv.Atoi(v); err == nil {
			port6 = n
		}
	}
	if v, ok := opts["loglevel"]; ok && !cliSet["loglevel"] {
		if n, err := strconv.Atoi(v); err == nil {
			loglevel = n
		}
	}
	if v, ok := opts["allowedMessageLength"]; ok && !cliSet["allowedMessageLength"] {
		if n, err := strconv.Atoi(v); err == nil {
			maxMsgLen = n
		}
	}
	if v, ok := opts["ping_time"]; ok && !cliSet["ping_time"] {
		if n, err := strconv.Atoi(v); err == nil {
			pingTime = n
		}
	}
	if v, ok := opts["timeout"]; ok && !cliSet["timeout"] {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			timeoutSecs = f
		}
	}
}

// conf_print_missing mirrors the Python server's message printed to the
// console when the configuration file can't be read.
func conf_print_missing(path string, err error) {
	msg := "Can't open the configuration file, using default or commandline values"
	if path != "" {
		msg += ": " + path
	}
	if err != nil {
		msg += " (" + err.Error() + ")"
	}
	fmt.Println(msg)
}

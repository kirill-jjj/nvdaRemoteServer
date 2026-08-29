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
	applyStrings(opts, cliSet)
	applyNumbers(opts, cliSet)
	applyFloats(opts, cliSet)
}

// applyStringOpt copies an option into a global string variable unless
// the same option was set on the command line.
func applyStringOpt(opts map[string]string, cliSet map[string]bool, name string, target *string) {
	if v, ok := opts[name]; ok && !cliSet[name] {
		*target = v
	}
}

// applyIntOpt copies an option into a global int variable unless the
// same option was set on the command line. Invalid integers are
// silently ignored, mirroring the Python server.
func applyIntOpt(opts map[string]string, cliSet map[string]bool, name string, target *int) {
	if v, ok := opts[name]; ok && !cliSet[name] {
		if n, err := strconv.Atoi(v); err == nil {
			*target = n
		}
	}
}

func applyStrings(opts map[string]string, cliSet map[string]bool) {
	applyStringOpt(opts, cliSet, "interface", &iface)
	applyStringOpt(opts, cliSet, "interface6", &iface6)
	applyStringOpt(opts, cliSet, "logfile", &logfile)
	applyStringOpt(opts, cliSet, "pidfile", &pidfile)
	applyStringOpt(opts, cliSet, "keyfile", &key)
	applyStringOpt(opts, cliSet, "certfile", &cert)
	applyStringOpt(opts, cliSet, "domain", &domain)
	applyStringOpt(opts, cliSet, "acme_email", &acmeEmail)
	applyStringOpt(opts, cliSet, "acme_ca", &acmeCA)
	applyStringOpt(opts, cliSet, "motd", &motd)
	if v, ok := opts["motd_force_display"]; ok && !cliSet["motd_force_display"] {
		if n, err := strconv.Atoi(v); err == nil {
			motdAlwaysDisplay = n != 0
		}
	}
	// includeTracebacks exists in the Python server; this Go server has
	// no tracebacks, so the option is accepted and ignored.
	// (The value is not validated here: conf_apply runs before the
	// logger is initialized, and the Python server ignores bad values.)
}

func applyNumbers(opts map[string]string, cliSet map[string]bool) {
	applyIntOpt(opts, cliSet, "port", &port)
	applyIntOpt(opts, cliSet, "port6", &port6)
	applyIntOpt(opts, cliSet, "loglevel", &loglevel)
	applyIntOpt(opts, cliSet, "allowedMessageLength", &maxMsgLen)
	applyIntOpt(opts, cliSet, "ping_time", &pingTime)
}

func applyFloats(opts map[string]string, cliSet map[string]bool) {
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

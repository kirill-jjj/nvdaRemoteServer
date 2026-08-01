//go:build windows

package main

import (
	"fmt"
	"os"
)

// daemonCmd is a stub for Windows: the daemon control commands
// (start/stop/restart/kill/status) are Unix-only, mirroring the Python
// server, which also does not support them on Windows.
func daemonCmd(cmd string, rest []string) {
	fmt.Fprintln(os.Stderr, "The daemon commands are not supported on Windows. Run the server directly.")
	os.Exit(2)
}

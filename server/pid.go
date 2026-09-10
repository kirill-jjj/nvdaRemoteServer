package server

import (
	"os"
)

func PidfileSet() {
	if pidfile == "" {
		return
	}
	Log(LogDebug, "writing process ID to PID file", "file", pidfile)
	err := file_rewrite(pidfile, []byte(pidStr))
	if err != nil {
		Log(LogDebug, "failed to write PID file", "file", pidfile, "error", err)
		pidfile = ""
	}
	Log(LogDebug, "PID file written", "file", pidfile)
}

func PidfileClear() {
	if pidfile == "" {
		return
	}
	Log(LogDebug, "removing PID file", "file", pidfile)
	err := os.Remove(pidfile)
	if err != nil {
		Log(LogDebug, "failed to remove PID file", "file", pidfile, "error", err)
		pidfile = ""
		return
	}
	pidfile = ""
	Log(LogDebug, "PID file removed", "file", pidfile)
}

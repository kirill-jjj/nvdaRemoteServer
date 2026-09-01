package server

import (
	"os"
)

func PidfileSet() {
	if pidfile == "" {
		return
	}
	Log(LOG_DEBUG, "writing process ID to PID file", "file", pidfile)
	err := file_rewrite(pidfile, []byte(PID_STR))
	if err != nil {
		Log(LOG_DEBUG, "failed to write PID file", "file", pidfile, "error", err)
		pidfile = ""
	}
	Log(LOG_DEBUG, "PID file written", "file", pidfile)
}

func PidfileClear() {
	if pidfile == "" {
		return
	}
	Log(LOG_DEBUG, "removing PID file", "file", pidfile)
	err := os.Remove(pidfile)
	if err != nil {
		Log(LOG_DEBUG, "failed to remove PID file", "file", pidfile, "error", err)
		pidfile = ""
		return
	}
	pidfile = ""
	Log(LOG_DEBUG, "PID file removed", "file", pidfile)
}

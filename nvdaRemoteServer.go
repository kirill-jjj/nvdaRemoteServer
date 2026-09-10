package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/kirill-jjj/nvdaRemoteServer/server"
)

var Version string = "development"

func main() {
	Version = strings.TrimPrefix(versionSetter(), "v")
	args()

	defer server.Log_close()
	err := server.Configure()
	if err != nil {
		server.Log_close()
		os.Exit(1)
	}
	num := server.Start()
	if num == 0 {
		server.Log_error("No servers started. Shutting down.")
		server.Log_close()
		os.Exit(1)
	}
	defer server.CatchPanic()
	server.PidfileSet()
	server.Log(server.LOG_INFO, "server started", "pid", server.PID, "version", Version)
	wait()
	if server.Mctx.Err() != nil {
		server.Log(server.LOG_INFO, "Shutdown signal received, stopping servers.")
	}
	server.Shutdown()
	server.Log(server.LOG_INFO, "Server shutdown complete.")
}

func wait() {
	var wg sync.WaitGroup
	for _, s := range server.Servers {
		if s == nil {
			continue
		}
		wg.Add(1)
		go func(sv *server.Server) {
			sv.Wait()
			wg.Done()
		}(s)
	}
	wg.Wait()
}

func args() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(Version)
		os.Exit(0)
	case "buildinfo":
		fmt.Println(buildInfo())
		os.Exit(0)
	case "start", "stop", "restart", "kill", "status":
		// Daemon control commands, ported from the Python server.
		// Unix-only: on Windows daemonCmd is a stub (daemon_windows.go).
		daemonCmd(os.Args[1], os.Args[2:])
		os.Exit(0)
	default:
		return
	}
}

func versionSetter() string {
	i, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}
	m := i.Main
	if m.Sum != "" {
		return m.Version
	}
	return Version
}

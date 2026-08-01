package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/kirill-jjj/nvdaRemoteServer/server"
)

var Version string = "development"

func main() {
	Version = strings.TrimPrefix(versionSetter(), "v")
	args()

	defer Log_close()
	err := Configure()
	if err != nil {
		Log_close()
		os.Exit(1)
	}
	num := Start()
	if num == 0 {
		Log_error("No servers started. Shutting down.")
		Log_close()
		os.Exit(1)
	}
	defer PanicHandle.Catch()
	PidfileSet()
	Log(LOG_INFO, "Server started. Running under PID "+PID_STR+". Server version "+Version)
	wait()
	Shutdown()
	Log(LOG_INFO, "Server shutdown complete.")
}

func wait() {
	var wg sync.WaitGroup
	for _, s := range Servers {
		if s == nil {
			continue
		}
		wg.Add(1)
		go func(sv *Server) {
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
		if runtime.GOOS == "windows" {
			fmt.Fprintln(os.Stderr, "The daemon commands are not supported on Windows. Run the server directly.")
			os.Exit(2)
		}
		daemonCmd(os.Args[1], os.Args[2:])
		os.Exit(0)
	default:
		return
	}
}

// findPidFile returns the PID file path from the given command line
// arguments, or the server's default PID file if not specified.
func findPidFile(args []string) string {
	for i, a := range args {
		if a == "-pidfile" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, "-pidfile=") {
			return strings.TrimPrefix(a, "-pidfile=")
		}
	}
	return DEFAULT_PID_FILE
}

func readPid(path string) (int, error) {
	d, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(d)))
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func daemonStatus(pidfile string) (int, bool) {
	pid, err := readPid(pidfile)
	if err != nil {
		return 0, false
	}
	return pid, pidAlive(pid)
}

func daemonCmd(cmd string, rest []string) {
	pidfile := findPidFile(rest)
	pid, running := daemonStatus(pidfile)

	switch cmd {
	case "status":
		if running {
			fmt.Printf("Server is running with pid %d.\n", pid)
		} else {
			fmt.Println("Server is not running.")
		}
	case "stop":
		if !running {
			fmt.Println("Server is not running.")
			return
		}
		fmt.Printf("Stopping server with pid %d...\n", pid)
		_ = syscall.Kill(pid, syscall.SIGTERM)
		for i := 0; i < 50; i++ {
			if !pidAlive(pid) {
				fmt.Println("Server stopped.")
				_ = os.Remove(pidfile)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		fmt.Fprintln(os.Stderr, "Server did not stop in time. Use the kill command to force it.")
		os.Exit(1)
	case "kill":
		if !running {
			fmt.Println("Server is not running.")
			return
		}
		fmt.Printf("Killing server with pid %d...\n", pid)
		_ = syscall.Kill(pid, syscall.SIGKILL)
		for i := 0; i < 50; i++ {
			if !pidAlive(pid) {
				fmt.Println("Server killed.")
				_ = os.Remove(pidfile)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		fmt.Fprintln(os.Stderr, "Server could not be killed.")
		os.Exit(1)
	case "restart":
		if running {
			fmt.Printf("Stopping server with pid %d...\n", pid)
			_ = syscall.Kill(pid, syscall.SIGTERM)
			for i := 0; i < 50; i++ {
				if !pidAlive(pid) {
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if pidAlive(pid) {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
			_ = os.Remove(pidfile)
		}
		daemonStart(rest, pidfile)
	case "start":
		if running {
			fmt.Fprintf(os.Stderr, "Server is already running with pid %d.\n", pid)
			os.Exit(1)
		}
		daemonStart(rest, pidfile)
	}
}

// daemonStart launches the server binary itself in the background,
// detached from the current terminal, mirroring the Python server's
// start command.
func daemonStart(rest []string, pidfile string) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Unable to determine executable path: "+err.Error())
		os.Exit(1)
	}
	childArgs := make([]string, 0, len(rest)+2)
	hasPidFile := false
	for _, a := range rest {
		if a == "-pidfile" || strings.HasPrefix(a, "-pidfile=") {
			hasPidFile = true
		}
		childArgs = append(childArgs, a)
	}
	if !hasPidFile {
		childArgs = append(childArgs, "-pidfile", pidfile)
	}
	cmd := exec.Command(exe, childArgs...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Unable to start the server: "+err.Error())
		os.Exit(1)
	}
	// Write the child PID immediately so status works right away; the
	// server rewrites it once it has fully started.
	_ = os.WriteFile(pidfile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	fmt.Printf("Server started with pid %d.\n", cmd.Process.Pid)
	_ = cmd.Process.Release()
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

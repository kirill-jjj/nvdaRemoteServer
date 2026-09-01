package server

import (
	"fmt"
	"os"
	"runtime/debug"
)

func Shutdown() {
	PidfileClear()
}

// CatchPanic is a deferred function that catches panics, logs them
// with a stack trace, and exits. This replaces the third-party
// panichandler package with standard Go idioms: recover() + debug.Stack().
//
// Usage in main():
//
//	defer CatchPanic()
func CatchPanic() {
	if r := recover(); r != nil {
		fmt.Fprintf(os.Stderr, "PANIC: %v\n%s\n", r, debug.Stack())
		Shutdown()
		os.Exit(2)
	}
}

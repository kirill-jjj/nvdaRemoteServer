package server

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Custom log levels that map to the Python server's logging behavior.
// slog uses higher numbers for more severe levels, but our logging
// convention uses higher numbers for MORE verbose output:
//
//	LOG_SILENT   (-1) → nothing logged
//	LOG_INFO     ( 0) → server start/stop, configuration
//	LOG_CONNECTION(1) → client connect/disconnect
//	LOG_CHANNEL  ( 2) → channel join/leave
//	LOG_DEBUG    ( 3) → detailed debugging
//	LOG_PROTOCOL ( 4) → raw protocol data (most verbose)
//
// slog levels (higher = more severe): Debug=-4, Info=0, Warn=4, Error=8
// We invert our levels so that the user's loglevel threshold works:
// messages with level <= loglevel are shown, which maps cleanly to
// slog's "level > threshold → suppress" model when we negate.
var logLevelMap = [6]slog.Level{
	slog.LevelError + 2, // LOG_SILENT (-1) → slog 10 (nothing passes)
	slog.LevelInfo,      // LOG_INFO       (0) → slog 0
	2,                   // LOG_CONNECTION (1) → slog 2
	4,                   // LOG_CHANNEL    (2) → slog 4
	8,                   // LOG_DEBUG      (3) → slog 8
	10,                  // LOG_PROTOCOL   (4) → slog 10
}

// toSlogLevel converts our int log levels (from defaults.go) to slog.Level.
func toSlogLevel(level int) slog.Level {
	if level < LOG_SILENT || level > LOG_PROTOCOL {
		return slog.LevelError + 2
	}
	return logLevelMap[level+1]
}

var (
	logger    *slog.Logger
	log_level slog.Level
)

// Log logs a message at the specified level with optional key-value pairs.
//
// Example:
//
//	Log(LOG_CONNECTION, "client connected", "id", 42, "ip", "127.0.0.1")
func Log(level int, msg string, args ...any) {
	if slog.Level(level) > log_level {
		return
	}
	logger.Log(context.Background(), toSlogLevel(level), msg, args...)
}

// Log_error logs an error message. Convenience wrapper for backward compat.
func Log_error(msg string, args ...any) {
	logger.Error(msg, args...)
}

// log_init initializes the slog logger with the specified output.
// If file is empty, logs go to stdout only.
// If file is specified, logs go to both the file and stdout.
func log_init(file string) {
	log_level = toSlogLevel(loglevel)

	opts := &slog.HandlerOptions{
		Level: log_level,
	}

	if file == "" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
		return
	}

	file = fullPath(file)
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		log_init("")
		Log_error("unable to open log file for writing", "file", file, "error", err)
		return
	}

	multiWriter := io.MultiWriter(os.Stdout, f)
	logger = slog.New(slog.NewTextHandler(multiWriter, opts))
	log_file = f
}

// log_file holds the log file handle for cleanup.
var log_file *os.File

// Log_close closes the log file if open.
func Log_close() {
	if log_file != nil {
		_ = log_file.Sync()
		_ = log_file.Close()
		log_file = nil
	}
}

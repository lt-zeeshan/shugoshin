// Package logger provides a lightweight file-based logger for Shugoshin.
// It is designed for use inside Claude Code hook handlers which must always
// exit 0 — the logger never panics or crashes.
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const logFileName = "debug.log"

var (
	mu   sync.Mutex
	file *os.File
)

// Init opens baseDir/debug.log in append mode and configures the package-level
// logger to write there. Safe to call multiple times; subsequent calls after a
// successful open are no-ops. If baseDir is empty or the file cannot be opened,
// logging becomes a no-op for the lifetime of the process.
func Init(baseDir string) {
	mu.Lock()
	defer mu.Unlock()

	if file != nil {
		return // already initialised
	}
	if baseDir == "" {
		return // no-op mode
	}

	f, err := os.OpenFile(
		filepath.Join(baseDir, logFileName),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return // no-op mode — never crash
	}

	file = f
}

// Close closes the underlying log file. Safe to call if Init was never called
// or if Init failed. Idempotent.
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if file == nil {
		return
	}
	_ = file.Close()
	file = nil
}

// Debug writes a DEBUG-level message to the log file.
func Debug(format string, args ...any) {
	write("DEBUG", format, args...)
}

// Info writes an INFO-level message to the log file.
func Info(format string, args ...any) {
	write("INFO", format, args...)
}

// Error writes an ERROR-level message to the log file.
func Error(format string, args ...any) {
	write("ERROR", format, args...)
}

// write is the common output path. It formats the log line before acquiring
// the lock so that fmt.Sprintf is not done under contention, then writes the
// line atomically under the lock. Emits lines in the format:
//
//	2006-01-02T15:04:05.000 [LEVEL] message
//
// Any I/O failure is silently discarded to protect hook stability.
func write(level, format string, args ...any) {
	ts := time.Now().Format("2006-01-02T15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s\n", ts, level, msg)

	mu.Lock()
	f := file
	if f != nil {
		_, _ = f.WriteString(line)
	}
	mu.Unlock()
}

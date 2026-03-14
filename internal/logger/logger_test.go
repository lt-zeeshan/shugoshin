package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reset tears down any open file handle so each test starts from a clean slate.
func reset(t *testing.T) {
	t.Helper()
	Close()
}

func TestInit(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     func(t *testing.T) string
		wantFileCreated bool
	}{
		{
			name: "creates log file in existing directory",
			baseDir: func(t *testing.T) string {
				t.Helper()
				return t.TempDir()
			},
			wantFileCreated: true,
		},
		{
			name: "empty baseDir is a no-op",
			baseDir: func(t *testing.T) string {
				t.Helper()
				return ""
			},
			wantFileCreated: false,
		},
		{
			name: "non-existent baseDir is a no-op",
			baseDir: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "does", "not", "exist")
			},
			wantFileCreated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset(t)
			t.Cleanup(func() { reset(t) })

			dir := tt.baseDir(t)
			Init(dir)

			if dir == "" {
				return // nothing to check
			}

			logPath := filepath.Join(dir, logFileName)
			_, err := os.Stat(logPath)
			exists := err == nil

			if exists != tt.wantFileCreated {
				t.Errorf("log file exists = %v, want %v", exists, tt.wantFileCreated)
			}
		})
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { reset(t) })

	Init(dir)
	Init(dir) // second call must not open a second file or panic

	Info("idempotent test")

	data, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 log line, got %d: %v", len(lines), lines)
	}
}

func TestLevelFormats(t *testing.T) {
	tests := []struct {
		name      string
		logFn     func(string, ...any)
		wantLevel string
	}{
		{name: "Debug", logFn: Debug, wantLevel: "[DEBUG]"},
		{name: "Info", logFn: Info, wantLevel: "[INFO]"},
		{name: "Error", logFn: Error, wantLevel: "[ERROR]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset(t)
			t.Cleanup(func() { reset(t) })

			dir := t.TempDir()
			Init(dir)

			tt.logFn("hello %s", "world")

			data, err := os.ReadFile(filepath.Join(dir, logFileName))
			if err != nil {
				t.Fatalf("reading log: %v", err)
			}

			line := strings.TrimSpace(string(data))

			if !strings.Contains(line, tt.wantLevel) {
				t.Errorf("line %q does not contain level %q", line, tt.wantLevel)
			}
			if !strings.Contains(line, "hello world") {
				t.Errorf("line %q does not contain message", line)
			}
			// Timestamp check: starts with a 4-digit year.
			if len(line) < 4 || line[0] < '2' || line[4] != '-' {
				t.Errorf("line %q does not look like a timestamped entry", line)
			}
		})
	}
}

func TestNoOpWhenNotInitialised(t *testing.T) {
	reset(t)
	t.Cleanup(func() { reset(t) })

	// None of these should panic.
	Debug("should be silently discarded")
	Info("should be silently discarded")
	Error("should be silently discarded")
}

func TestCloseIdempotent(t *testing.T) {
	dir := t.TempDir()

	Init(dir)
	Close()
	Close() // second Close must not panic
}

func TestCloseWithoutInit(t *testing.T) {
	reset(t)
	Close() // must not panic when never initialised
}

func TestWriteAfterClose(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { reset(t) })

	Init(dir)
	Close()

	// Writes after Close must be silent no-ops, not panics.
	Info("after close")

	data, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if strings.TrimSpace(string(data)) != "" {
		t.Errorf("expected empty log after Close, got: %q", string(data))
	}
}

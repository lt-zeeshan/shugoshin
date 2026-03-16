package tracking

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func makeStateDir(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "state"), 0o755); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	return base
}

func TestWriteMarker(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string // returns baseDir
		status    *AnalysisStatus
		wantErr   bool
		checkFile bool
	}{
		{
			name:  "creates file with correct JSON content",
			setup: makeStateDir,
			status: &AnalysisStatus{
				PID:       12345,
				StartTime: 1700000000,
				FileCount: 42,
				Backend:   "claude",
				SessionID: "abc123",
			},
			wantErr:   false,
			checkFile: true,
		},
		{
			name: "missing state dir returns error",
			setup: func(t *testing.T) string {
				t.Helper()
				return t.TempDir() // no state/ subdirectory
			},
			status: &AnalysisStatus{
				PID:       1,
				SessionID: "nosession",
			},
			wantErr:   true,
			checkFile: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := tc.setup(t)
			err := WriteMarker(baseDir, tc.status)
			if (err != nil) != tc.wantErr {
				t.Fatalf("WriteMarker() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.checkFile {
				return
			}
			path := markerPath(baseDir, tc.status.SessionID)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("marker file not readable: %v", err)
			}
			var got AnalysisStatus
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("marker file contains invalid JSON: %v", err)
			}
			if got != *tc.status {
				t.Errorf("got %+v, want %+v", got, *tc.status)
			}
		})
	}
}

func TestRemoveMarker(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		pre       func(t *testing.T, baseDir string) // optional setup before Remove
	}{
		{
			name:      "deletes existing marker",
			sessionID: "session-exists",
			pre: func(t *testing.T, baseDir string) {
				t.Helper()
				s := &AnalysisStatus{PID: 1, SessionID: "session-exists"}
				if err := WriteMarker(baseDir, s); err != nil {
					t.Fatalf("WriteMarker: %v", err)
				}
			},
		},
		{
			name:      "non-existent marker does not panic",
			sessionID: "session-missing",
			pre:       nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := makeStateDir(t)
			if tc.pre != nil {
				tc.pre(t, baseDir)
			}
			// Must not panic.
			RemoveMarker(baseDir, tc.sessionID)

			path := markerPath(baseDir, tc.sessionID)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("expected marker file to be absent after RemoveMarker, stat err: %v", err)
			}
		})
	}
}

func TestListActive(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, baseDir string)
		wantCount int
		wantNil   bool
	}{
		{
			name:      "empty state dir returns nil slice",
			setup:     nil,
			wantCount: 0,
			wantNil:   true,
		},
		{
			name: "marker with current PID appears in results",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				s := &AnalysisStatus{
					PID:       os.Getpid(),
					StartTime: 1700000000,
					FileCount: 1,
					Backend:   "test",
					SessionID: "live-session",
				}
				if err := WriteMarker(baseDir, s); err != nil {
					t.Fatalf("WriteMarker: %v", err)
				}
			},
			wantCount: 1,
			wantNil:   false,
		},
		{
			name: "marker with PID 0 is cleaned up and not returned",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				s := &AnalysisStatus{
					PID:       0,
					SessionID: "dead-session",
				}
				if err := WriteMarker(baseDir, s); err != nil {
					t.Fatalf("WriteMarker: %v", err)
				}
			},
			wantCount: 0,
			wantNil:   true,
		},
		{
			name: "corrupt JSON marker is silently removed",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				path := filepath.Join(baseDir, "state", ".analysing-corrupt.json")
				if err := os.WriteFile(path, []byte("not valid json {{{{"), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			},
			wantCount: 0,
			wantNil:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseDir := makeStateDir(t)
			if tc.setup != nil {
				tc.setup(t, baseDir)
			}
			got := ListActive(baseDir)
			if tc.wantNil && got != nil {
				t.Errorf("expected nil slice, got %v", got)
			}
			if len(got) != tc.wantCount {
				t.Errorf("got %d active entries, want %d", len(got), tc.wantCount)
			}
			// For stale/corrupt cases: verify the marker file was actually removed.
			if tc.wantCount == 0 && !tc.wantNil == false {
				pattern := filepath.Join(baseDir, "state", ".analysing-*.json")
				remaining, _ := filepath.Glob(pattern)
				if len(remaining) != 0 {
					t.Errorf("expected stale marker files to be removed, found: %v", remaining)
				}
			}
		})
	}
}

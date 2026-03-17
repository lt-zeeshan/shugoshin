package state_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lt-zeeshan/shugoshin/internal/state"
	"github.com/lt-zeeshan/shugoshin/internal/types"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(baseDir string) // optional pre-test setup
		sessionID string
		want      *types.SessionState
		wantErr   bool
	}{
		{
			name:      "file does not exist returns default state with session ID",
			sessionID: "abc-123",
			want: &types.SessionState{
				SessionID: "abc-123",
			},
		},
		{
			name:      "file exists returns correct state",
			sessionID: "xyz-456",
			setup: func(baseDir string) {
				dir := filepath.Join(baseDir, "state")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				s := &types.SessionState{
					SessionID:      "xyz-456",
					Cwd:            "/home/user/project",
					CurrentIntent:  "add tests",
					CurrentChanges: []string{"main.go", "main_test.go"},
					ResponseIndex:  3,
				}
				data, _ := json.Marshal(s)
				if err := os.WriteFile(filepath.Join(dir, "xyz-456.json"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: &types.SessionState{
				SessionID:      "xyz-456",
				Cwd:            "/home/user/project",
				CurrentIntent:  "add tests",
				CurrentChanges: []string{"main.go", "main_test.go"},
				ResponseIndex:  3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseDir := t.TempDir()
			if tt.setup != nil {
				tt.setup(baseDir)
			}

			got, err := state.Load(baseDir, tt.sessionID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.SessionID != tt.want.SessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.want.SessionID)
			}
			if got.Cwd != tt.want.Cwd {
				t.Errorf("Cwd = %q, want %q", got.Cwd, tt.want.Cwd)
			}
			if got.CurrentIntent != tt.want.CurrentIntent {
				t.Errorf("CurrentIntent = %q, want %q", got.CurrentIntent, tt.want.CurrentIntent)
			}
			if len(got.CurrentChanges) != len(tt.want.CurrentChanges) {
				t.Fatalf("CurrentChanges len = %d, want %d", len(got.CurrentChanges), len(tt.want.CurrentChanges))
			}
			for i, v := range tt.want.CurrentChanges {
				if got.CurrentChanges[i] != v {
					t.Errorf("CurrentChanges[%d] = %q, want %q", i, got.CurrentChanges[i], v)
				}
			}
			if got.ResponseIndex != tt.want.ResponseIndex {
				t.Errorf("ResponseIndex = %d, want %d", got.ResponseIndex, tt.want.ResponseIndex)
			}
		})
	}
}

func TestSave(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		state   *types.SessionState
		wantErr bool
	}{
		{
			name: "creates parent dirs and writes valid JSON",
			state: &types.SessionState{
				SessionID:      "sess-save-1",
				Cwd:            "/tmp/work",
				CurrentIntent:  "refactor handler",
				CurrentChanges: []string{"handler.go"},
				ResponseIndex:  0,
			},
		},
		{
			name: "save and load roundtrip",
			state: &types.SessionState{
				SessionID:      "sess-roundtrip",
				Cwd:            "/srv/app",
				CurrentIntent:  "fix bug",
				CurrentChanges: []string{"a.go", "b.go", "c.go"},
				ResponseIndex:  7,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseDir := t.TempDir()

			if err := state.Save(baseDir, tt.state); (err != nil) != tt.wantErr {
				t.Fatalf("Save() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// Verify the file exists and contains valid JSON.
			path := filepath.Join(baseDir, "state", tt.state.SessionID+".json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading saved file: %v", err)
			}
			var got types.SessionState
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshaling saved JSON: %v", err)
			}
			if got.SessionID != tt.state.SessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.state.SessionID)
			}
		})
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	original := &types.SessionState{
		SessionID:      "roundtrip-99",
		Cwd:            "/code/myapp",
		CurrentIntent:  "add feature",
		CurrentChanges: []string{"feature.go", "feature_test.go"},
		ResponseIndex:  5,
	}

	if err := state.Save(baseDir, original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := state.Load(baseDir, original.SessionID)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, original.SessionID)
	}
	if loaded.Cwd != original.Cwd {
		t.Errorf("Cwd = %q, want %q", loaded.Cwd, original.Cwd)
	}
	if loaded.CurrentIntent != original.CurrentIntent {
		t.Errorf("CurrentIntent = %q, want %q", loaded.CurrentIntent, original.CurrentIntent)
	}
	if len(loaded.CurrentChanges) != len(original.CurrentChanges) {
		t.Fatalf("CurrentChanges len = %d, want %d", len(loaded.CurrentChanges), len(original.CurrentChanges))
	}
	for i, v := range original.CurrentChanges {
		if loaded.CurrentChanges[i] != v {
			t.Errorf("CurrentChanges[%d] = %q, want %q", i, loaded.CurrentChanges[i], v)
		}
	}
	if loaded.ResponseIndex != original.ResponseIndex {
		t.Errorf("ResponseIndex = %d, want %d", loaded.ResponseIndex, original.ResponseIndex)
	}
}

func TestClearResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		initial           *types.SessionState
		wantIntent        string
		wantChanges       []string
		wantResponseIndex int
		wantSessionID     string
		wantCwd           string
	}{
		{
			name: "clears changes, preserves intent, and bumps ResponseIndex",
			initial: &types.SessionState{
				SessionID:      "clear-1",
				Cwd:            "/work",
				CurrentIntent:  "some intent",
				CurrentChanges: []string{"foo.go", "bar.go"},
				ResponseIndex:  2,
			},
			wantIntent:        "some intent",
			wantChanges:       nil,
			wantResponseIndex: 3,
			wantSessionID:     "clear-1",
			wantCwd:           "/work",
		},
		{
			name: "preserves SessionID, Cwd, and intent",
			initial: &types.SessionState{
				SessionID:      "preserve-session",
				Cwd:            "/preserve/cwd",
				CurrentIntent:  "another intent",
				CurrentChanges: []string{"x.go"},
				ResponseIndex:  0,
			},
			wantIntent:        "another intent",
			wantChanges:       nil,
			wantResponseIndex: 1,
			wantSessionID:     "preserve-session",
			wantCwd:           "/preserve/cwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseDir := t.TempDir()

			// Persist initial state first so the file exists.
			if err := state.Save(baseDir, tt.initial); err != nil {
				t.Fatalf("Save() setup error: %v", err)
			}

			if err := state.ClearResponse(baseDir, tt.initial); err != nil {
				t.Fatalf("ClearResponse() error: %v", err)
			}

			// Reload from disk to confirm persistence.
			got, err := state.Load(baseDir, tt.initial.SessionID)
			if err != nil {
				t.Fatalf("Load() after ClearResponse error: %v", err)
			}

			if got.CurrentIntent != tt.wantIntent {
				t.Errorf("CurrentIntent = %q, want %q", got.CurrentIntent, tt.wantIntent)
			}
			if len(got.CurrentChanges) != 0 {
				t.Errorf("CurrentChanges = %v, want nil/empty", got.CurrentChanges)
			}
			if got.ResponseIndex != tt.wantResponseIndex {
				t.Errorf("ResponseIndex = %d, want %d", got.ResponseIndex, tt.wantResponseIndex)
			}
			if got.SessionID != tt.wantSessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.wantSessionID)
			}
			if got.Cwd != tt.wantCwd {
				t.Errorf("Cwd = %q, want %q", got.Cwd, tt.wantCwd)
			}
		})
	}
}

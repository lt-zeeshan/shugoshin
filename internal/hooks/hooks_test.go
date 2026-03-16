package hooks

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zeeshans/shugoshin/internal/analyser"
	"github.com/zeeshans/shugoshin/internal/state"
	"github.com/zeeshans/shugoshin/internal/types"
)

// mockAnalyser is a test double for analyser.Analyser that returns a fixed verdict.
type mockAnalyser struct {
	verdict *types.Verdict
	called  bool
}

func (m *mockAnalyser) Analyse(_ context.Context, _ string, _ map[string]string, _ string) (*types.Verdict, error) {
	m.called = true
	return m.verdict, nil
}

func (m *mockAnalyser) Name() string { return "mock" }

// newBaseDir creates a temporary .shugoshin directory and returns its path.
func newBaseDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, ".shugoshin")
	for _, sub := range []string{"state", "reports", "schemas"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0o755); err != nil {
			t.Fatalf("creating %s: %v", sub, err)
		}
	}
	// Write schema so stop hook can sync it.
	_ = os.WriteFile(filepath.Join(baseDir, "schemas", "verdict.json"), analyser.VerdictSchema, 0o644)
	return baseDir
}

// payloadJSON marshals a HookPayload to a JSON reader.
func payloadJSON(t *testing.T, p types.HookPayload) *strings.Reader {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return strings.NewReader(string(data))
}

// ---------- HandleSubmit ----------

func TestHandleSubmit(t *testing.T) {
	tests := []struct {
		name          string
		payload       types.HookPayload
		wantIntent    string
		wantSessionID string
	}{
		{
			name: "valid payload records intent",
			payload: types.HookPayload{
				SessionID: "sess-1",
				Prompt:    "refactor auth module",
				Cwd:       "", // will be filled in per-test
			},
			wantIntent:    "refactor auth module",
			wantSessionID: "sess-1",
		},
		{
			name: "missing prompt creates state with empty intent",
			payload: types.HookPayload{
				SessionID: "sess-2",
				Prompt:    "",
			},
			wantIntent:    "",
			wantSessionID: "sess-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := newBaseDir(t)
			cwd := filepath.Dir(baseDir) // parent of .shugoshin
			tt.payload.Cwd = cwd

			if err := HandleSubmit(payloadJSON(t, tt.payload)); err != nil {
				t.Fatalf("HandleSubmit returned non-nil error: %v", err)
			}

			s, err := state.Load(baseDir, tt.payload.SessionID)
			if err != nil {
				t.Fatalf("state.Load: %v", err)
			}
			if s.CurrentIntent != tt.wantIntent {
				t.Errorf("CurrentIntent = %q, want %q", s.CurrentIntent, tt.wantIntent)
			}
			if s.SessionID != tt.wantSessionID {
				t.Errorf("SessionID = %q, want %q", s.SessionID, tt.wantSessionID)
			}
		})
	}
}

// ---------- HandlePostTool ----------

func TestHandlePostTool(t *testing.T) {
	tests := []struct {
		name        string
		payloads    []types.HookPayload
		wantChanges []string
	}{
		{
			name: "valid edit payload adds file to changes",
			payloads: []types.HookPayload{
				{
					SessionID: "sess-a",
					ToolInput: map[string]interface{}{"file_path": "main.go"},
				},
			},
			wantChanges: []string{"main.go"},
		},
		{
			name: "multiple files appended and deduplicated",
			payloads: []types.HookPayload{
				{SessionID: "sess-b", ToolInput: map[string]interface{}{"file_path": "a.go"}},
				{SessionID: "sess-b", ToolInput: map[string]interface{}{"file_path": "b.go"}},
				{SessionID: "sess-b", ToolInput: map[string]interface{}{"file_path": "a.go"}}, // duplicate
			},
			wantChanges: []string{"a.go", "b.go"},
		},
		{
			name: "missing file_path does not crash and state unchanged",
			payloads: []types.HookPayload{
				{SessionID: "sess-c", ToolInput: map[string]interface{}{"other_key": "value"}},
			},
			wantChanges: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := newBaseDir(t)
			cwd := filepath.Dir(baseDir)

			for i := range tt.payloads {
				tt.payloads[i].Cwd = cwd
				if err := HandlePostTool(payloadJSON(t, tt.payloads[i])); err != nil {
					t.Fatalf("HandlePostTool[%d] returned non-nil error: %v", i, err)
				}
			}

			sessionID := tt.payloads[0].SessionID
			s, err := state.Load(baseDir, sessionID)
			if err != nil {
				t.Fatalf("state.Load: %v", err)
			}

			if len(s.CurrentChanges) != len(tt.wantChanges) {
				t.Fatalf("CurrentChanges = %v, want %v", s.CurrentChanges, tt.wantChanges)
			}
			for i, f := range tt.wantChanges {
				if s.CurrentChanges[i] != f {
					t.Errorf("CurrentChanges[%d] = %q, want %q", i, s.CurrentChanges[i], f)
				}
			}
		})
	}
}

// ---------- HandleStop ----------

// initGitRepo creates a git repo in dir with one committed file, then modifies
// it so that `git diff HEAD` produces non-empty output.
func initGitRepo(t *testing.T, dir, filename, initial, modified string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args[1:], err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "test")

	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	run("git", "add", filename)
	run("git", "commit", "-m", "initial")

	if err := os.WriteFile(p, []byte(modified), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}
}

func TestHandleStop(t *testing.T) {
	tests := []struct {
		name         string
		payload      types.HookPayload
		prepareState func(t *testing.T, baseDir, cwd string)
		wantCleared  bool
	}{
		{
			name: "stop_hook_active exits immediately without report",
			payload: types.HookPayload{
				SessionID:      "s1",
				StopHookActive: true,
			},
			prepareState: nil,
			wantCleared:  false,
		},
		{
			name: "empty current_changes exits without report",
			payload: types.HookPayload{
				SessionID:      "s2",
				StopHookActive: false,
			},
			prepareState: nil,
			wantCleared:  false,
		},
		{
			name: "with changes state is cleared and background spawned",
			payload: types.HookPayload{
				SessionID:      "s3",
				StopHookActive: false,
			},
			prepareState: func(t *testing.T, baseDir, cwd string) {
				t.Helper()
				initGitRepo(t, cwd, "foo.go", "package main\n", "package main\n// changed\n")
				s := &types.SessionState{
					SessionID:      "s3",
					Cwd:            cwd,
					CurrentIntent:  "add comment",
					CurrentChanges: []string{"foo.go"},
				}
				if err := state.Save(baseDir, s); err != nil {
					t.Fatalf("state.Save: %v", err)
				}
			},
			wantCleared: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := newBaseDir(t)
			cwd := filepath.Dir(baseDir)
			tt.payload.Cwd = cwd

			if tt.prepareState != nil {
				tt.prepareState(t, baseDir, cwd)
			}

			if err := HandleStop(payloadJSON(t, tt.payload)); err != nil {
				t.Fatalf("HandleStop returned non-nil error: %v", err)
			}

			if tt.wantCleared {
				s, err := state.Load(baseDir, tt.payload.SessionID)
				if err != nil {
					t.Fatalf("state.Load after stop: %v", err)
				}
				if len(s.CurrentChanges) != 0 {
					t.Errorf("CurrentChanges not cleared: %v", s.CurrentChanges)
				}
				if s.CurrentIntent != "add comment" {
					t.Errorf("CurrentIntent = %q, want %q (should be preserved)", s.CurrentIntent, "add comment")
				}
			}
		})
	}
}

// ---------- HandleAnalyse ----------

func TestHandleAnalyse(t *testing.T) {
	fixedVerdict := &types.Verdict{
		Verdict:       "SAFE",
		Summary:       "all clear",
		AffectedAreas: nil,
		IntentMatch:   true,
		Reasoning:     "no issues found",
	}

	tests := []struct {
		name       string
		req        AnalyseRequest
		executor   *mockAnalyser
		wantCalled bool
		wantReport bool
	}{
		{
			name: "analysis runs and writes report",
			req: AnalyseRequest{
				SessionID:     "s-analyse",
				Intent:        "fix bug",
				ChangedFiles:  []string{"main.go"},
				Diffs:         map[string]string{"main.go": "diff content here"},
				ResponseIndex: 0,
			},
			executor:   &mockAnalyser{verdict: fixedVerdict},
			wantCalled: true,
			wantReport: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := newBaseDir(t)
			tt.req.BaseDir = baseDir
			tt.req.Cwd = filepath.Dir(baseDir)

			// Write request to temp file.
			reqData, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			reqFile := filepath.Join(t.TempDir(), "req.json")
			if err := os.WriteFile(reqFile, reqData, 0o644); err != nil {
				t.Fatalf("write request file: %v", err)
			}

			if err := HandleAnalyse(reqFile, tt.executor); err != nil {
				t.Fatalf("HandleAnalyse returned error: %v", err)
			}

			if tt.executor.called != tt.wantCalled {
				t.Errorf("executor.called = %v, want %v", tt.executor.called, tt.wantCalled)
			}

			// Check report was written.
			reportsDir := filepath.Join(baseDir, "reports")
			hasReport := false
			_ = filepath.WalkDir(reportsDir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() && filepath.Ext(path) == ".json" {
					hasReport = true

					// Verify report content.
					data, readErr := os.ReadFile(path)
					if readErr != nil {
						t.Errorf("reading report: %v", readErr)
						return nil
					}
					var report types.Report
					if err := json.Unmarshal(data, &report); err != nil {
						t.Errorf("parsing report: %v", err)
						return nil
					}
					if report.Verdict.Verdict != fixedVerdict.Verdict {
						t.Errorf("report verdict = %q, want %q", report.Verdict.Verdict, fixedVerdict.Verdict)
					}
					if report.SessionID != tt.req.SessionID {
						t.Errorf("report session = %q, want %q", report.SessionID, tt.req.SessionID)
					}
					if report.Timestamp.After(time.Now().Add(time.Minute)) {
						t.Errorf("report timestamp is in the future: %v", report.Timestamp)
					}
				}
				return nil
			})

			if hasReport != tt.wantReport {
				t.Errorf("report written = %v, want %v", hasReport, tt.wantReport)
			}

			// Request file should be cleaned up.
			if _, err := os.Stat(reqFile); !os.IsNotExist(err) {
				t.Errorf("request file should have been deleted")
			}
		})
	}
}

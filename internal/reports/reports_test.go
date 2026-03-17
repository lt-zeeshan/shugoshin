package reports_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lt-zeeshan/shugoshin/internal/reports"
	"github.com/lt-zeeshan/shugoshin/internal/types"
)

// makeReport is a test helper that builds a minimal types.Report.
func makeReport(t *testing.T, sessionID string, index int, ts time.Time) *types.Report {
	t.Helper()
	return &types.Report{
		SessionID:     sessionID,
		Cwd:           "/tmp/project",
		Timestamp:     ts,
		ResponseIndex: index,
		Intent:        "test intent",
		ChangedFiles:  []string{"main.go"},
		Verdict: types.Verdict{
			Verdict:       "SAFE",
			Summary:       "no issues",
			AffectedAreas: []types.AffectedArea{},
			IntentMatch:   true,
			Reasoning:     "all good",
		},
	}
}

func TestWriteReport(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		sessionID     string
		responseIndex int
		timestamp     time.Time
		wantSuffix    string // path relative to baseDir
	}{
		{
			name:          "basic filename format",
			sessionID:     "sess-abc",
			responseIndex: 0,
			timestamp:     time.Date(2026, 3, 14, 13, 2, 0, 0, time.UTC),
			wantSuffix:    filepath.Join("reports", "sess-abc", "20260314T130200-000.json"),
		},
		{
			name:          "three digit zero-padded index",
			sessionID:     "sess-xyz",
			responseIndex: 42,
			timestamp:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			wantSuffix:    filepath.Join("reports", "sess-xyz", "20260101T000000-042.json"),
		},
		{
			name:          "index at boundary 999",
			sessionID:     "sess-boundary",
			responseIndex: 999,
			timestamp:     time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
			wantSuffix:    filepath.Join("reports", "sess-boundary", "20261231T235959-999.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseDir := t.TempDir()
			r := makeReport(t, tt.sessionID, tt.responseIndex, tt.timestamp)

			got, err := reports.WriteReport(baseDir, r)
			if err != nil {
				t.Fatalf("WriteReport() error = %v", err)
			}

			want := filepath.Join(baseDir, tt.wantSuffix)
			if got != want {
				t.Errorf("WriteReport() path = %q, want %q", got, want)
			}

			// Verify the file actually exists on disk.
			if _, err := os.Stat(got); err != nil {
				t.Errorf("written file does not exist: %v", err)
			}
		})
	}
}

func TestWriteReportRoundtrip(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	ts := time.Date(2026, 3, 14, 13, 2, 0, 0, time.UTC)
	original := makeReport(t, "sess-rt", 1, ts)
	original.Verdict.Verdict = "REVIEW_NEEDED"
	original.Intent = "fix the auth bug"
	original.ChangedFiles = []string{"auth/token.go", "auth/token_test.go"}

	if _, err := reports.WriteReport(baseDir, original); err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	got, err := reports.ListReports(baseDir)
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListReports() returned %d reports, want 1", len(got))
	}

	r := got[0]
	if r.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", r.SessionID, original.SessionID)
	}
	if r.ResponseIndex != original.ResponseIndex {
		t.Errorf("ResponseIndex = %d, want %d", r.ResponseIndex, original.ResponseIndex)
	}
	if r.Verdict.Verdict != original.Verdict.Verdict {
		t.Errorf("Verdict = %q, want %q", r.Verdict.Verdict, original.Verdict.Verdict)
	}
	if !r.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", r.Timestamp, original.Timestamp)
	}
}

func TestListReportsNewestFirst(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	base := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	timestamps := []time.Time{
		base,
		base.Add(30 * time.Minute),
		base.Add(90 * time.Minute),
	}

	for i, ts := range timestamps {
		r := makeReport(t, "sess-order", i, ts)
		if _, err := reports.WriteReport(baseDir, r); err != nil {
			t.Fatalf("WriteReport() index %d error = %v", i, err)
		}
	}

	got, err := reports.ListReports(baseDir)
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListReports() returned %d reports, want 3", len(got))
	}

	for i := 1; i < len(got); i++ {
		if got[i].Timestamp.After(got[i-1].Timestamp) {
			t.Errorf("reports not sorted newest-first: [%d] %v is after [%d] %v",
				i, got[i].Timestamp, i-1, got[i-1].Timestamp)
		}
	}

	// Most recent is index 2 (base + 90min).
	if got[0].ResponseIndex != 2 {
		t.Errorf("first report ResponseIndex = %d, want 2", got[0].ResponseIndex)
	}
}

func TestListReportsMultipleSessions(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	sessions := []struct {
		id string
		ts time.Time
	}{
		{"sess-alpha", time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)},
		{"sess-beta", time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC)},
		{"sess-gamma", time.Date(2026, 3, 14, 8, 0, 0, 0, time.UTC)},
	}

	for _, s := range sessions {
		r := makeReport(t, s.id, 0, s.ts)
		if _, err := reports.WriteReport(baseDir, r); err != nil {
			t.Fatalf("WriteReport() session %s error = %v", s.id, err)
		}
	}

	got, err := reports.ListReports(baseDir)
	if err != nil {
		t.Fatalf("ListReports() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListReports() returned %d reports, want 3", len(got))
	}

	// Verify global newest-first: beta (11:00), alpha (09:00), gamma (08:00).
	wantOrder := []string{"sess-beta", "sess-alpha", "sess-gamma"}
	for i, want := range wantOrder {
		if got[i].SessionID != want {
			t.Errorf("reports[%d].SessionID = %q, want %q", i, got[i].SessionID, want)
		}
	}
}

func TestListReportsBySession(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	base := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)

	// Write two reports for sess-target and one for sess-other.
	for i, ts := range []time.Time{base, base.Add(time.Hour)} {
		r := makeReport(t, "sess-target", i, ts)
		if _, err := reports.WriteReport(baseDir, r); err != nil {
			t.Fatalf("WriteReport() target[%d] error = %v", i, err)
		}
	}
	other := makeReport(t, "sess-other", 0, base.Add(2*time.Hour))
	if _, err := reports.WriteReport(baseDir, other); err != nil {
		t.Fatalf("WriteReport() other error = %v", err)
	}

	got, err := reports.ListReportsBySession(baseDir, "sess-target")
	if err != nil {
		t.Fatalf("ListReportsBySession() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListReportsBySession() returned %d reports, want 2", len(got))
	}
	for _, r := range got {
		if r.SessionID != "sess-target" {
			t.Errorf("unexpected session %q in filtered results", r.SessionID)
		}
	}
	// Newest first: index 1 (base+1h) before index 0 (base).
	if got[0].ResponseIndex != 1 {
		t.Errorf("first report ResponseIndex = %d, want 1", got[0].ResponseIndex)
	}
}

func TestListReportsEmptyDirectory(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	// Reports directory does not exist yet — should return empty slice, no error.
	got, err := reports.ListReports(baseDir)
	if err != nil {
		t.Fatalf("ListReports() on empty dir error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListReports() returned %d reports, want 0", len(got))
	}
}

func TestListReportsBySessionMissingSession(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	got, err := reports.ListReportsBySession(baseDir, "nonexistent-session")
	if err != nil {
		t.Fatalf("ListReportsBySession() on missing session error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListReportsBySession() returned %d reports, want 0", len(got))
	}
}

func TestWriteReportIndentedJSON(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	ts := time.Date(2026, 3, 14, 13, 0, 0, 0, time.UTC)
	r := makeReport(t, "sess-indent", 0, ts)

	path, err := reports.WriteReport(baseDir, r)
	if err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	content := string(data)

	// Indented JSON must contain a newline and leading whitespace.
	if !strings.Contains(content, "\n") {
		t.Error("report file does not appear to be indented (no newlines found)")
	}
	if !strings.Contains(content, "  ") {
		t.Error("report file does not appear to be indented (no leading spaces found)")
	}
}

// ---------------------------------------------------------------------------
// UpdateReport
// ---------------------------------------------------------------------------

func TestUpdateReport(t *testing.T) {
	t.Parallel()

	t.Run("persists Resolved=true", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()
		ts := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
		r := makeReport(t, "sess-upd", 0, ts)
		path, err := reports.WriteReport(baseDir, r)
		if err != nil {
			t.Fatalf("WriteReport() error = %v", err)
		}
		r.FilePath = path

		r.Resolved = true
		if err := reports.UpdateReport(r); err != nil {
			t.Fatalf("UpdateReport() unexpected error = %v", err)
		}

		all, err := reports.ListReports(baseDir)
		if err != nil {
			t.Fatalf("ListReports() error = %v", err)
		}
		if len(all) != 1 {
			t.Fatalf("expected 1 report, got %d", len(all))
		}
		if !all[0].Resolved {
			t.Error("Resolved field was not persisted by UpdateReport")
		}
	})

	t.Run("empty FilePath returns error", func(t *testing.T) {
		t.Parallel()
		r := &types.Report{SessionID: "sess-nopath"}
		if err := reports.UpdateReport(r); err == nil {
			t.Fatal("UpdateReport() expected error for empty FilePath, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// DeleteReport
// ---------------------------------------------------------------------------

func TestDeleteReport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) (baseDir string, r *types.Report)
		verify  func(t *testing.T, baseDir string)
		wantErr bool
	}{
		{
			name: "written report is removed from ListReports",
			setup: func(t *testing.T) (string, *types.Report) {
				t.Helper()
				baseDir := t.TempDir()
				ts := time.Date(2026, 3, 14, 15, 0, 0, 0, time.UTC)
				r := makeReport(t, "sess-del", 0, ts)
				path, err := reports.WriteReport(baseDir, r)
				if err != nil {
					t.Fatalf("WriteReport() error = %v", err)
				}
				r.FilePath = path
				return baseDir, r
			},
			verify: func(t *testing.T, baseDir string) {
				t.Helper()
				all, err := reports.ListReports(baseDir)
				if err != nil {
					t.Fatalf("ListReports() after delete error = %v", err)
				}
				if len(all) != 0 {
					t.Errorf("expected 0 reports after delete, got %d", len(all))
				}
			},
		},
		{
			name: "empty FilePath returns error",
			setup: func(t *testing.T) (string, *types.Report) {
				return "", &types.Report{SessionID: "sess-nopath-del"}
			},
			verify:  func(t *testing.T, _ string) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			baseDir, r := tt.setup(t)
			err := reports.DeleteReport(r)
			if tt.wantErr {
				if err == nil {
					t.Fatal("DeleteReport() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("DeleteReport() unexpected error = %v", err)
			}
			tt.verify(t, baseDir)
		})
	}
}

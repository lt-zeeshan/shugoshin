package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zeeshans/shugoshin/internal/types"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

var (
	t0 = time.Date(2026, 3, 14, 13, 0, 0, 0, time.UTC)
	t1 = time.Date(2026, 3, 14, 13, 15, 0, 0, time.UTC)
	t2 = time.Date(2026, 3, 14, 13, 41, 0, 0, time.UTC)

	reportSafe = &types.Report{
		SessionID:     "ses-a",
		Timestamp:     t0,
		ResponseIndex: 0,
		Intent:        "fix null check",
		Verdict:       types.Verdict{Verdict: "SAFE"},
	}
	reportReview = &types.Report{
		SessionID:     "ses-a",
		Timestamp:     t1,
		ResponseIndex: 1,
		Intent:        "refactor auth",
		Verdict:       types.Verdict{Verdict: "REVIEW_NEEDED"},
	}
	reportHigh = &types.Report{
		SessionID:     "ses-b",
		Timestamp:     t2,
		ResponseIndex: 0,
		Intent:        "modify token validator",
		Verdict:       types.Verdict{Verdict: "HIGH_RISK"},
	}
)

func baseModel() Model {
	reports := []*types.Report{reportSafe, reportReview, reportHigh}
	m := New("/tmp/proj", func(string) ([]*types.Report, error) { return reports, nil })
	m.width = 120
	m.height = 40
	// Simulate a successful load.
	updated, _ := m.Update(reportsLoadedMsg{reports: reports})
	return updated.(Model)
}

// ---------------------------------------------------------------------------
// Cursor navigation
// ---------------------------------------------------------------------------

func TestCursorNavigation(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		wantCursor int
	}{
		{
			name:       "initial cursor is at 0",
			keys:       nil,
			wantCursor: 0,
		},
		{
			name:       "down moves cursor to 1",
			keys:       []string{"down"},
			wantCursor: 1,
		},
		{
			name:       "j moves cursor to 1",
			keys:       []string{"j"},
			wantCursor: 1,
		},
		{
			name:       "down down moves cursor to 2",
			keys:       []string{"down", "down"},
			wantCursor: 2,
		},
		{
			name:       "down past end is clamped",
			keys:       []string{"down", "down", "down", "down"},
			wantCursor: 2,
		},
		{
			name:       "up at 0 stays at 0",
			keys:       []string{"up"},
			wantCursor: 0,
		},
		{
			name:       "k at 0 stays at 0",
			keys:       []string{"k"},
			wantCursor: 0,
		},
		{
			name:       "down then up returns to 0",
			keys:       []string{"down", "up"},
			wantCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModel()
			for _, key := range tt.keys {
				var msg tea.KeyMsg
				switch key {
				case "up":
					msg = tea.KeyMsg{Type: tea.KeyUp}
				case "down":
					msg = tea.KeyMsg{Type: tea.KeyDown}
				default:
					msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
				}
				updated, _ := m.Update(msg)
				m = updated.(Model)
			}
			if m.cursor != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", m.cursor, tt.wantCursor)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Enter toggles expanded state
// ---------------------------------------------------------------------------

func TestEnterTogglesExpanded(t *testing.T) {
	tests := []struct {
		name         string
		presses      int
		wantExpanded bool
	}{
		{name: "one press expands", presses: 1, wantExpanded: true},
		{name: "two presses collapses", presses: 2, wantExpanded: false},
		{name: "three presses expands again", presses: 3, wantExpanded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModel()
			for i := 0; i < tt.presses; i++ {
				updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				m = updated.(Model)
			}
			if m.expanded != tt.wantExpanded {
				t.Errorf("expanded = %v, want %v", m.expanded, tt.wantExpanded)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Session filter cycles
// ---------------------------------------------------------------------------

func TestSessionFilterCycles(t *testing.T) {
	m := baseModel()
	// sessions are ["ses-a", "ses-b"] in first-seen order.
	tests := []struct {
		wantSession string
		wantIdx     int
	}{
		{wantSession: "ses-a", wantIdx: 1},
		{wantSession: "ses-b", wantIdx: 2},
		{wantSession: "", wantIdx: 0}, // wraps back to all
	}

	for _, tt := range tests {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
		m = updated.(Model)
		if m.sessionFilter != tt.wantSession {
			t.Errorf("sessionFilter = %q, want %q", m.sessionFilter, tt.wantSession)
		}
		if m.sessionIdx != tt.wantIdx {
			t.Errorf("sessionIdx = %d, want %d", m.sessionIdx, tt.wantIdx)
		}
	}
}

// ---------------------------------------------------------------------------
// Verdict filter cycles
// ---------------------------------------------------------------------------

func TestVerdictFilterCycles(t *testing.T) {
	tests := []struct {
		name       string
		presses    int
		wantFilter string
	}{
		{name: "1 press → HIGH_RISK", presses: 1, wantFilter: "HIGH_RISK"},
		{name: "2 presses → REVIEW_NEEDED+", presses: 2, wantFilter: "REVIEW_NEEDED+"},
		{name: "3 presses → ALL", presses: 3, wantFilter: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseModel()
			for i := 0; i < tt.presses; i++ {
				updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
				m = updated.(Model)
			}
			if m.verdictFilter != tt.wantFilter {
				t.Errorf("verdictFilter = %q, want %q", m.verdictFilter, tt.wantFilter)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// applyFilters
// ---------------------------------------------------------------------------

func TestApplyFilters(t *testing.T) {
	reports := []*types.Report{reportSafe, reportReview, reportHigh}

	tests := []struct {
		name          string
		sessionFilter string
		verdictFilter string
		wantCount     int
		wantVerdicts  []string
	}{
		{
			name:      "no filters returns all",
			wantCount: 3,
		},
		{
			name:          "session filter ses-a",
			sessionFilter: "ses-a",
			wantCount:     2,
			wantVerdicts:  []string{"SAFE", "REVIEW_NEEDED"},
		},
		{
			name:          "session filter ses-b",
			sessionFilter: "ses-b",
			wantCount:     1,
			wantVerdicts:  []string{"HIGH_RISK"},
		},
		{
			name:          "verdict filter HIGH_RISK",
			verdictFilter: "HIGH_RISK",
			wantCount:     1,
			wantVerdicts:  []string{"HIGH_RISK"},
		},
		{
			name:          "verdict filter REVIEW_NEEDED+",
			verdictFilter: "REVIEW_NEEDED+",
			wantCount:     2,
			wantVerdicts:  []string{"REVIEW_NEEDED", "HIGH_RISK"},
		},
		{
			name:          "session and verdict combined — ses-a REVIEW_NEEDED+",
			sessionFilter: "ses-a",
			verdictFilter: "REVIEW_NEEDED+",
			wantCount:     1,
			wantVerdicts:  []string{"REVIEW_NEEDED"},
		},
		{
			name:          "no match returns empty",
			sessionFilter: "ses-b",
			verdictFilter: "REVIEW_NEEDED+",
			wantCount:     1,
			wantVerdicts:  []string{"HIGH_RISK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				reports:       reports,
				sessionFilter: tt.sessionFilter,
				verdictFilter: tt.verdictFilter,
			}
			applyFilters(&m)

			if len(m.filtered) != tt.wantCount {
				t.Fatalf("filtered count = %d, want %d", len(m.filtered), tt.wantCount)
			}
			for i, v := range tt.wantVerdicts {
				if m.filtered[i].Verdict.Verdict != v {
					t.Errorf("filtered[%d].Verdict.Verdict = %q, want %q",
						i, m.filtered[i].Verdict.Verdict, v)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// reportsLoadedMsg populates model
// ---------------------------------------------------------------------------

func TestReportsLoadedMsg(t *testing.T) {
	reports := []*types.Report{reportSafe, reportReview, reportHigh}
	m := New("/tmp/proj", func(string) ([]*types.Report, error) { return nil, nil })
	m.width = 120
	m.height = 40

	updated, _ := m.Update(reportsLoadedMsg{reports: reports})
	got := updated.(Model)

	if len(got.reports) != 3 {
		t.Fatalf("reports count = %d, want 3", len(got.reports))
	}
	if len(got.filtered) != 3 {
		t.Fatalf("filtered count = %d, want 3", len(got.filtered))
	}
	if len(got.sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2", len(got.sessions))
	}
	if got.sessions[0] != "ses-a" {
		t.Errorf("sessions[0] = %q, want %q", got.sessions[0], "ses-a")
	}
	if got.sessions[1] != "ses-b" {
		t.Errorf("sessions[1] = %q, want %q", got.sessions[1], "ses-b")
	}
	if got.cursor != 0 {
		t.Errorf("cursor = %d, want 0", got.cursor)
	}
	if got.expanded {
		t.Error("expanded should be false after load")
	}
}

// ---------------------------------------------------------------------------
// matchesVerdictFilter
// ---------------------------------------------------------------------------

func TestMatchesVerdictFilter(t *testing.T) {
	tests := []struct {
		verdict string
		filter  string
		want    bool
	}{
		{"SAFE", "", true},
		{"HIGH_RISK", "", true},
		{"SAFE", "HIGH_RISK", false},
		{"HIGH_RISK", "HIGH_RISK", true},
		{"REVIEW_NEEDED", "HIGH_RISK", false},
		{"SAFE", "REVIEW_NEEDED+", false},
		{"REVIEW_NEEDED", "REVIEW_NEEDED+", true},
		{"HIGH_RISK", "REVIEW_NEEDED+", true},
	}

	for _, tt := range tests {
		t.Run(tt.verdict+"/"+tt.filter, func(t *testing.T) {
			got := matchesVerdictFilter(tt.verdict, tt.filter)
			if got != tt.want {
				t.Errorf("matchesVerdictFilter(%q, %q) = %v, want %v",
					tt.verdict, tt.filter, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// WindowSizeMsg
// ---------------------------------------------------------------------------

func TestWindowSizeMsgUpdatesModel(t *testing.T) {
	m := baseModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	got := updated.(Model)
	if got.width != 200 {
		t.Errorf("width = %d, want 200", got.width)
	}
	if got.height != 50 {
		t.Errorf("height = %d, want 50", got.height)
	}
}

// ---------------------------------------------------------------------------
// uniqueSessions
// ---------------------------------------------------------------------------

func TestUniqueSessions(t *testing.T) {
	tests := []struct {
		name    string
		reports []*types.Report
		want    []string
	}{
		{
			name:    "empty",
			reports: nil,
			want:    nil,
		},
		{
			name:    "single session",
			reports: []*types.Report{reportSafe, reportReview},
			want:    []string{"ses-a"},
		},
		{
			name:    "multiple sessions in order",
			reports: []*types.Report{reportSafe, reportHigh, reportReview},
			want:    []string{"ses-a", "ses-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueSessions(tt.reports)
			if len(got) != len(tt.want) {
				t.Fatalf("uniqueSessions() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("sessions[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

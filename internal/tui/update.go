package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zeeshans/shugoshin/internal/types"
)

// verdictFilterCycle defines the ordered sequence for cycling the verdict filter.
var verdictFilterCycle = []string{"", "HIGH_RISK", "REVIEW_NEEDED+"}

// Update handles all incoming messages and key events, returning the updated
// model and any follow-up commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case reportsLoadedMsg:
		if msg.err != nil {
			// Keep the existing state on error — a future reload might succeed.
			return m, nil
		}
		m.reports = msg.reports
		m.sessions = uniqueSessions(msg.reports)
		// Reset navigation on reload.
		m.cursor = 0
		m.expanded = false
		applyFilters(&m)
		return m, nil

	case tea.KeyMsg:
		return handleKey(m, msg)
	}

	return m, nil
}

// handleKey processes keyboard input and returns the updated model.
func handleKey(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.expanded = false
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.expanded = false
		}

	case "enter":
		if len(m.filtered) > 0 {
			m.expanded = !m.expanded
		}

	case "s":
		// Cycle: all → session[0] → session[1] → … → all
		if len(m.sessions) == 0 {
			break
		}
		m.sessionIdx = (m.sessionIdx + 1) % (len(m.sessions) + 1)
		if m.sessionIdx == 0 {
			m.sessionFilter = ""
		} else {
			m.sessionFilter = m.sessions[m.sessionIdx-1]
		}
		m.cursor = 0
		m.expanded = false
		applyFilters(&m)

	case "f":
		// Cycle: ALL → HIGH_RISK → REVIEW_NEEDED+ → ALL
		next := ""
		for i, v := range verdictFilterCycle {
			if v == m.verdictFilter {
				next = verdictFilterCycle[(i+1)%len(verdictFilterCycle)]
				break
			}
		}
		m.verdictFilter = next
		m.cursor = 0
		m.expanded = false
		applyFilters(&m)

	case "r":
		m.expanded = false
		return m, func() tea.Msg {
			reports, err := m.loadReports(m.baseDir)
			return reportsLoadedMsg{reports: reports, err: err}
		}

	case "q", "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

// applyFilters rebuilds m.filtered from m.reports according to the active
// session and verdict filters. The cursor is clamped to the new list length.
func applyFilters(m *Model) {
	var out []*types.Report
	for _, r := range m.reports {
		if m.sessionFilter != "" && r.SessionID != m.sessionFilter {
			continue
		}
		if !matchesVerdictFilter(r.Verdict.Verdict, m.verdictFilter) {
			continue
		}
		out = append(out, r)
	}
	m.filtered = out

	// Clamp cursor.
	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
}

// matchesVerdictFilter reports whether a verdict string passes the active
// filter.
func matchesVerdictFilter(verdict, filter string) bool {
	switch filter {
	case "":
		return true
	case "HIGH_RISK":
		return verdict == "HIGH_RISK"
	case "REVIEW_NEEDED+":
		return verdict == "REVIEW_NEEDED" || verdict == "HIGH_RISK"
	default:
		return true
	}
}

// uniqueSessions returns the ordered list of unique session IDs as they appear
// in the reports slice (first-seen order).
func uniqueSessions(reports []*types.Report) []string {
	seen := make(map[string]struct{})
	var sessions []string
	for _, r := range reports {
		if _, ok := seen[r.SessionID]; !ok {
			seen[r.SessionID] = struct{}{}
			sessions = append(sessions, r.SessionID)
		}
	}
	return sessions
}

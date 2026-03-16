package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zeeshans/shugoshin/internal/analyser"
	"github.com/zeeshans/shugoshin/internal/config"
	"github.com/zeeshans/shugoshin/internal/reports"
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
			return m, nil
		}
		m.reports = msg.reports
		m.sessions = uniqueSessions(msg.reports)
		m.cursor = 0
		m.expanded = false
		m.detailScroll = 0
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
		if m.expanded {
			// Scroll detail pane up.
			if m.detailScroll > 0 {
				m.detailScroll--
			}
		} else {
			if m.cursor > 0 {
				m.cursor--
			}
		}

	case "down", "j":
		if m.expanded {
			// Scroll detail pane down (clamped in view rendering).
			m.detailScroll++
		} else {
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		}

	case "enter":
		if len(m.filtered) > 0 {
			m.expanded = !m.expanded
			m.detailScroll = 0
		}

	case "esc":
		if m.expanded {
			m.expanded = false
			m.detailScroll = 0
		}

	case "s":
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
		m.detailScroll = 0
		applyFilters(&m)

	case "f":
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
		m.detailScroll = 0
		applyFilters(&m)

	case "x":
		if len(m.filtered) > 0 {
			r := m.filtered[m.cursor]
			r.Resolved = !r.Resolved
			_ = reports.UpdateReport(r)
		}

	case "d":
		if len(m.filtered) > 0 {
			r := m.filtered[m.cursor]
			if err := reports.DeleteReport(r); err == nil {
				// Remove from both slices.
				for i, rr := range m.reports {
					if rr == r {
						m.reports = append(m.reports[:i], m.reports[i+1:]...)
						break
					}
				}
				m.expanded = false
				m.detailScroll = 0
				applyFilters(&m)
			}
		}

	case "b":
		backends := analyser.Backends
		next := backends[0]
		for i, b := range backends {
			if b == m.backend {
				next = backends[(i+1)%len(backends)]
				break
			}
		}
		m.backend = next
		_ = config.Save(m.baseDir, &config.Settings{Backend: m.backend})

	case "r":
		m.expanded = false
		m.detailScroll = 0
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

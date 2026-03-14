// Package tui implements the Bubble Tea TUI for browsing Shugoshin reports.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zeeshans/shugoshin/internal/types"
)

// ReportLoader is a function that loads all reports from a base directory.
// It decouples the TUI from the reports package during development.
type ReportLoader func(baseDir string) ([]*types.Report, error)

// reportsLoadedMsg is the internal message type delivered when the
// asynchronous report load completes.
type reportsLoadedMsg struct {
	reports []*types.Report
	err     error
}

// Model is the Bubble Tea model for the Shugoshin TUI.
type Model struct {
	reports       []*types.Report
	filtered      []*types.Report
	cursor        int
	expanded      bool
	detailScroll  int // scroll offset within detail pane
	verdictFilter string // "" = ALL, "HIGH_RISK", "REVIEW_NEEDED+"
	sessionFilter string // "" = all sessions
	sessions      []string
	sessionIdx    int
	baseDir       string
	width         int
	height        int
	loadReports   ReportLoader
}

// New creates a Model ready to be run with tea.NewProgram.
func New(baseDir string, loader ReportLoader) Model {
	return Model{
		baseDir:     baseDir,
		loadReports: loader,
	}
}

// Init returns a Cmd that loads reports from disk asynchronously.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		reports, err := m.loadReports(m.baseDir)
		return reportsLoadedMsg{reports: reports, err: err}
	}
}

// Run creates the model and starts the Bubble Tea program. It blocks until
// the user quits.
func Run(baseDir string, loader ReportLoader) error {
	m := New(baseDir, loader)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

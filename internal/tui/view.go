package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zeeshans/shugoshin/internal/types"
)

// ---------------------------------------------------------------------------
// Colour palette
// ---------------------------------------------------------------------------

var (
	colorGreen  = lipgloss.Color("#22c55e")
	colorYellow = lipgloss.Color("#eab308")
	colorRed    = lipgloss.Color("#ef4444")
	colorGray   = lipgloss.Color("#6b7280")
	colorBlue   = lipgloss.Color("#60a5fa")
	colorWhite  = lipgloss.Color("#f9fafb")
	colorDim    = lipgloss.Color("#9ca3af")
	colorSelect = lipgloss.Color("#1e3a5f")
)

// ---------------------------------------------------------------------------
// Base styles
// ---------------------------------------------------------------------------

var (
	styleSafe   = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleReview = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleHigh   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleGray   = lipgloss.NewStyle().Foreground(colorGray)
	styleTime   = lipgloss.NewStyle().Foreground(colorDim)
	styleLabel  = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	styleDim    = lipgloss.NewStyle().Foreground(colorDim)
	styleBold   = lipgloss.NewStyle().Bold(true)
	styleSelect = lipgloss.NewStyle().Background(colorSelect)
)

// ---------------------------------------------------------------------------
// View entry point
// ---------------------------------------------------------------------------

// View renders the full TUI screen.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	inner := m.width - 2 // subtract border left+right

	var b strings.Builder
	b.WriteString(renderHeader(m, inner))
	b.WriteString(renderList(m, inner))
	if m.expanded && len(m.filtered) > 0 {
		b.WriteString(renderDivider(inner))
		b.WriteString(renderDetail(m, inner))
	}
	b.WriteString(renderFooter(inner))
	b.WriteString(renderHelp())

	return b.String()
}

// ---------------------------------------------------------------------------
// Header
// ---------------------------------------------------------------------------

func renderHeader(m Model, inner int) string {
	project := filepath.Base(m.baseDir)

	session := "all sessions"
	if m.sessionFilter != "" {
		session = m.sessionFilter
	}

	filter := "ALL"
	switch m.verdictFilter {
	case "HIGH_RISK":
		filter = "HIGH_RISK"
	case "REVIEW_NEEDED+":
		filter = "REVIEW_NEEDED+"
	}

	title := fmt.Sprintf("─ Shugoshin — %s ", project)
	bar1 := lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(title)

	meta := fmt.Sprintf(" Session: %-12s  Filter: %s", session, filter)
	meta = styleDim.Render(meta)

	sep := strings.Repeat("─", max(0, inner-lipgloss.Width(title)))

	return "┌" + bar1 + sep + "┐\n" +
		"│" + padRight(meta, inner) + "│\n" +
		"├" + strings.Repeat("─", inner) + "┤\n"
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func renderList(m Model, inner int) string {
	if len(m.filtered) == 0 {
		msg := styleDim.Render("  No reports match the current filter.")
		return "│" + padRight(msg, inner) + "│\n"
	}

	var b strings.Builder
	for i, r := range m.filtered {
		b.WriteString(renderRow(r, i == m.cursor, inner))
	}
	return b.String()
}

func renderRow(r *types.Report, selected bool, inner int) string {
	icon, verdictStyle := verdictIcon(r.Verdict.Verdict)
	label := verdictStyle.Render(fmt.Sprintf("%s %-13s", icon, verdictLabel(r.Verdict.Verdict)))

	ts := r.Timestamp.Local().Format("15:04")
	timeStr := styleTime.Render(ts)
	timeWidth := lipgloss.Width(timeStr)

	// Available space for intent text.
	prefixWidth := 2 + lipgloss.Width(label) + 1 // "  " + label + " "
	intentWidth := inner - prefixWidth - timeWidth - 2
	if intentWidth < 0 {
		intentWidth = 0
	}
	intent := truncate(r.Intent, intentWidth)

	line := fmt.Sprintf("  %s %s", label, padRight(lipgloss.NewStyle().Render(intent), intentWidth))
	line += timeStr

	if selected {
		prefix := "> "
		// Replace leading spaces with the cursor indicator.
		line = prefix + strings.TrimPrefix(line, "  ")
		line = styleSelect.Render(padRight(line, inner))
	}

	return "│" + padRight(line, inner) + "│\n"
}

// ---------------------------------------------------------------------------
// Divider
// ---------------------------------------------------------------------------

func renderDivider(inner int) string {
	return "├" + strings.Repeat("─", inner) + "┤\n"
}

// ---------------------------------------------------------------------------
// Detail pane
// ---------------------------------------------------------------------------

func renderDetail(m Model, inner int) string {
	r := m.filtered[m.cursor]

	intentMatch := "NO"
	if r.Verdict.IntentMatch {
		intentMatch = "YES"
	}

	_, vs := verdictIcon(r.Verdict.Verdict)

	lines := []string{
		fmt.Sprintf(" %s  %s   Intent match: %s",
			styleLabel.Render("Verdict:"),
			vs.Render(verdictLabel(r.Verdict.Verdict)),
			styleBold.Render(intentMatch),
		),
		fmt.Sprintf(" %s  %s",
			styleLabel.Render("Summary:"),
			wrapText(r.Verdict.Summary, inner-12, 11),
		),
	}

	if len(r.Verdict.AffectedAreas) > 0 {
		lines = append(lines, " "+styleLabel.Render("Affected:"))
		for _, a := range r.Verdict.AffectedAreas {
			locs := strings.Join(a.Locations, "  ")
			riskStyle := riskStyle(a.Risk)
			areaLine := fmt.Sprintf("   %-20s %s  %s",
				styleBold.Render(a.Symbol),
				locs,
				riskStyle.Render("("+a.Risk+")"),
			)
			lines = append(lines, areaLine)
		}
	}

	lines = append(lines,
		fmt.Sprintf(" %s  %s",
			styleLabel.Render("Reasoning:"),
			wrapText(r.Verdict.Reasoning, inner-13, 12),
		),
		fmt.Sprintf(" %s  %s",
			styleLabel.Render("Changed:"),
			strings.Join(r.ChangedFiles, "  "),
		),
	)

	var b strings.Builder
	for _, l := range lines {
		b.WriteString("│" + padRight(l, inner) + "│\n")
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Footer / help
// ---------------------------------------------------------------------------

func renderFooter(inner int) string {
	return "└" + strings.Repeat("─", inner) + "┘\n"
}

func renderHelp() string {
	return styleDim.Render("  ↑↓ navigate  enter expand  s session  f filter  r reload  q quit")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func verdictIcon(verdict string) (string, lipgloss.Style) {
	switch verdict {
	case "SAFE":
		return "●", styleSafe
	case "REVIEW_NEEDED":
		return "▲", styleReview
	case "HIGH_RISK":
		return "■", styleHigh
	default:
		return "?", styleGray
	}
}

func verdictLabel(verdict string) string {
	switch verdict {
	case "SAFE":
		return "SAFE"
	case "REVIEW_NEEDED":
		return "REVIEW"
	case "HIGH_RISK":
		return "HIGH RISK"
	case "TIMEOUT":
		return "TIMEOUT"
	case "ERROR":
		return "ERROR"
	default:
		return verdict
	}
}

func riskStyle(risk string) lipgloss.Style {
	switch risk {
	case "HIGH":
		return styleHigh
	case "MEDIUM":
		return styleReview
	default:
		return styleGray
	}
}

// padRight pads or truncates s to exactly width visible characters, using
// the lipgloss width calculation (handles ANSI escapes).
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// truncate shortens s to at most n visible characters, appending "…" if cut.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(runes[:n-1]) + "…"
}

// wrapText returns the first line of text. Subsequent lines are indented by
// indent spaces. This is a simple single-wrap for long fields.
func wrapText(text string, lineWidth, indent int) string {
	if lineWidth <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= lineWidth {
		return text
	}
	first := string(runes[:lineWidth])
	rest := string(runes[lineWidth:])
	return first + "\n" + strings.Repeat(" ", indent+1) + rest
}


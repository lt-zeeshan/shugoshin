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

func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	inner := m.width - 2

	var b strings.Builder
	b.WriteString(renderHeader(m, inner))

	if m.expanded && len(m.filtered) > 0 {
		// When expanded, show only the selected row + detail pane.
		b.WriteString(renderRow(m.filtered[m.cursor], true, inner))
		b.WriteString(renderDivider(inner))
		b.WriteString(renderDetail(m, inner))
	} else {
		b.WriteString(renderList(m, inner))
	}

	b.WriteString(renderFooter(inner))
	b.WriteString(renderHelp(m.expanded))

	return b.String()
}

// ---------------------------------------------------------------------------
// Header
// ---------------------------------------------------------------------------

func renderHeader(m Model, inner int) string {
	project := filepath.Base(filepath.Dir(m.baseDir))

	session := "all sessions"
	if m.sessionFilter != "" {
		session = truncate(m.sessionFilter, 16)
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

	count := fmt.Sprintf("%d reports", len(m.filtered))
	metaStr := fmt.Sprintf(" Session: %-16s  Filter: %-14s  Backend: %-6s  %s", session, filter, m.backend, count)
	if m.hideResolved {
		metaStr += "  (hiding resolved)"
	}
	meta := styleDim.Render(metaStr)

	sep := strings.Repeat("─", max(0, inner-lipgloss.Width(title)))

	header := "┌" + bar1 + sep + "┐\n" +
		"│" + padRight(meta, inner) + "│\n"

	if len(m.analysing) > 0 {
		total := 0
		for _, a := range m.analysing {
			total += a.FileCount
		}
		status := styleDim.Render(fmt.Sprintf(" ⟳ Analysing %d files in background...", total))
		header += "│" + padRight(status, inner) + "│\n"
	}

	header += "├" + strings.Repeat("─", inner) + "┤\n"
	return header
}

// ---------------------------------------------------------------------------
// List (with viewport scrolling)
// ---------------------------------------------------------------------------

func renderList(m Model, inner int) string {
	if len(m.filtered) == 0 {
		msg := styleDim.Render("  No reports match the current filter.")
		return "│" + padRight(msg, inner) + "│\n"
	}

	// Calculate visible window: reserve 5 lines for header(3)+footer(1)+help(1).
	maxVisible := m.height - 5
	if maxVisible < 3 {
		maxVisible = 3
	}

	// Determine scroll window to keep cursor visible.
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.filtered) {
		end = len(m.filtered)
		start = max(0, end-maxVisible)
	}

	// Deduct indicator lines from the visible rows so total stays within budget.
	hasAbove := start > 0
	hasBelow := end < len(m.filtered)
	if hasAbove {
		end = min(end, start+maxVisible-1)
	}
	if hasBelow {
		end = min(end, start+maxVisible-1)
		if hasAbove {
			end = min(end, start+maxVisible-2)
		}
	}

	var b strings.Builder

	if hasAbove {
		indicator := styleDim.Render(fmt.Sprintf("  ↑ %d more above", start))
		b.WriteString("│" + padRight(indicator, inner) + "│\n")
	}

	for i := start; i < end; i++ {
		b.WriteString(renderRow(m.filtered[i], i == m.cursor, inner))
	}

	if hasBelow {
		indicator := styleDim.Render(fmt.Sprintf("  ↓ %d more below", len(m.filtered)-end))
		b.WriteString("│" + padRight(indicator, inner) + "│\n")
	}

	return b.String()
}

func renderRow(r *types.Report, selected bool, inner int) string {
	icon, verdictStyle := verdictIcon(r.Verdict.Verdict)
	if r.Resolved {
		icon = "✓"
		verdictStyle = styleDim
	}
	label := verdictStyle.Render(fmt.Sprintf("%s %-13s", icon, verdictLabel(r.Verdict.Verdict)))

	ts := r.Timestamp.Local().Format("15:04")
	timeStr := styleTime.Render(ts)
	timeWidth := lipgloss.Width(timeStr)

	prefixWidth := 2 + lipgloss.Width(label) + 1
	intentWidth := inner - prefixWidth - timeWidth - 2
	if intentWidth < 0 {
		intentWidth = 0
	}
	intent := truncate(r.Intent, intentWidth)

	line := fmt.Sprintf("  %s %s", label, padRight(lipgloss.NewStyle().Render(intent), intentWidth))
	line += timeStr

	if selected {
		prefix := "> "
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
// Detail pane (with scroll support)
// ---------------------------------------------------------------------------

func renderDetail(m Model, inner int) string {
	r := m.filtered[m.cursor]

	// Build all detail lines with proper word wrapping.
	contentWidth := inner - 2 // 1 char padding each side

	var lines []string
	lines = append(lines, "")

	// Verdict + intent match.
	intentMatch := "NO"
	if r.Verdict.IntentMatch {
		intentMatch = "YES"
	}
	_, vs := verdictIcon(r.Verdict.Verdict)
	resolvedStr := ""
	if r.Resolved {
		resolvedStr = "   " + styleSafe.Render("[RESOLVED]")
	}
	lines = append(lines,
		fmt.Sprintf(" %s  %s   Intent match: %s%s",
			styleLabel.Render("Verdict:"),
			vs.Render(verdictLabel(r.Verdict.Verdict)),
			styleBold.Render(intentMatch),
			resolvedStr,
		),
	)
	lines = append(lines, "")

	// Summary (word-wrapped).
	lines = append(lines, " "+styleLabel.Render("Summary:"))
	for _, wl := range wordWrap(r.Verdict.Summary, contentWidth-3) {
		lines = append(lines, "   "+wl)
	}
	lines = append(lines, "")

	// Affected areas.
	if len(r.Verdict.AffectedAreas) > 0 {
		lines = append(lines, " "+styleLabel.Render("Affected:"))
		for _, a := range r.Verdict.AffectedAreas {
			riskStr := riskStyle(a.Risk).Render("(" + a.Risk + ")")
			catStr := ""
			if a.Category != "" {
				catStr = " " + styleDim.Render("["+a.Category+"]")
			}
			lines = append(lines, "")
			lines = append(lines,
				fmt.Sprintf("   %s  %s%s",
					styleBold.Render(a.Symbol),
					riskStr,
					catStr,
				),
			)
			for _, loc := range a.Locations {
				lines = append(lines, "     "+styleDim.Render(loc))
			}
		}
		lines = append(lines, "")
	}

	// Intent drift.
	if len(r.Verdict.ScopeCreep) > 0 {
		lines = append(lines, " "+styleLabel.Render("Scope creep:"))
		for _, s := range r.Verdict.ScopeCreep {
			for _, wl := range wordWrap("- "+s, contentWidth-3) {
				lines = append(lines, "   "+styleReview.Render(wl))
			}
		}
		lines = append(lines, "")
	}
	if len(r.Verdict.MissingFromIntent) > 0 {
		lines = append(lines, " "+styleLabel.Render("Missing from intent:"))
		for _, s := range r.Verdict.MissingFromIntent {
			for _, wl := range wordWrap("- "+s, contentWidth-3) {
				lines = append(lines, "   "+styleReview.Render(wl))
			}
		}
		lines = append(lines, "")
	}

	// Suggested tests.
	if len(r.Verdict.SuggestedTests) > 0 {
		lines = append(lines, " "+styleLabel.Render("Suggested tests:"))
		for _, st := range r.Verdict.SuggestedTests {
			lines = append(lines, "   "+styleBold.Render(st.File))
			for _, wl := range wordWrap(st.Scenario, contentWidth-5) {
				lines = append(lines, "     "+styleDim.Render(wl))
			}
		}
		lines = append(lines, "")
	}

	// Reasoning (word-wrapped).
	lines = append(lines, " "+styleLabel.Render("Reasoning:"))
	for _, wl := range wordWrap(r.Verdict.Reasoning, contentWidth-3) {
		lines = append(lines, "   "+wl)
	}
	lines = append(lines, "")

	// Changed files.
	lines = append(lines, " "+styleLabel.Render("Changed files:"))
	for _, f := range r.ChangedFiles {
		lines = append(lines, "   "+styleDim.Render(f))
	}
	lines = append(lines, "")

	// Apply scroll offset and viewport clamp.
	// Reserve: header(3) + selected row(1) + divider(1) + footer(1) + help(1) = 7.
	detailHeight := m.height - 7
	if detailHeight < 5 {
		detailHeight = 5
	}

	// Clamp scroll.
	maxScroll := len(lines) - detailHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.detailScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}

	end := scroll + detailHeight
	if end > len(lines) {
		end = len(lines)
	}

	// Deduct indicator lines from visible content so total stays within budget.
	hasAbove := scroll > 0
	hasBelow := end < len(lines)
	if hasAbove {
		end = min(end, scroll+detailHeight-1)
	}
	if hasBelow {
		end = min(end, scroll+detailHeight-1)
		if hasAbove {
			end = min(end, scroll+detailHeight-2)
		}
	}

	var b strings.Builder

	if hasAbove {
		indicator := styleDim.Render(fmt.Sprintf("  ↑ scroll up (%d more)", scroll))
		b.WriteString("│" + padRight(indicator, inner) + "│\n")
	}

	for _, l := range lines[scroll:end] {
		b.WriteString("│" + padRight(l, inner) + "│\n")
	}

	if hasBelow {
		indicator := styleDim.Render(fmt.Sprintf("  ↓ scroll down (%d more)", len(lines)-end))
		b.WriteString("│" + padRight(indicator, inner) + "│\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Footer / help
// ---------------------------------------------------------------------------

func renderFooter(inner int) string {
	return "└" + strings.Repeat("─", inner) + "┘\n"
}

func renderHelp(expanded bool) string {
	if expanded {
		return styleDim.Render("  ↑↓ scroll  esc/enter close  x resolve  d delete  h hide resolved  s session  f filter  b backend  r reload  q quit")
	}
	return styleDim.Render("  ↑↓ navigate  enter expand  x resolve  d delete  h hide resolved  s session  f filter  b backend  r reload  q quit")
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

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

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

// wordWrap breaks text into lines of at most width characters, splitting at
// word boundaries where possible.
func wordWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		current := words[0]
		for _, w := range words[1:] {
			if len([]rune(current))+1+len([]rune(w)) <= width {
				current += " " + w
			} else {
				lines = append(lines, current)
				current = w
			}
		}
		lines = append(lines, current)
	}

	return lines
}

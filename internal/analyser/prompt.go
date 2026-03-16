package analyser

import (
	"strings"
)

// BuildPrompt assembles the analysis prompt from the task intent and a list of
// changed filenames. The model is expected to read files and run git diff itself.
func BuildPrompt(intent string, changedFiles []string) string {
	var b strings.Builder

	b.WriteString("You are a staff-level software engineer performing blast radius analysis.\n")
	b.WriteString("Claude Code just completed a task. Your job is to determine whether the\n")
	b.WriteString("changes are safe, whether they match the stated intent, and whether anything\n")
	b.WriteString("else in the codebase could break as a result.\n\n")

	b.WriteString("RULES:\n")
	b.WriteString("- Read-only analysis. Do NOT modify files or run state-changing commands.\n")
	b.WriteString("- You may read files, search the codebase, and run read-only git commands\n")
	b.WriteString("  (git diff, git log, git show, git blame).\n\n")

	b.WriteString("TASK INTENT:\n")
	b.WriteString(intent)
	b.WriteString("\n\nCHANGED FILES:\n")

	for _, filename := range changedFiles {
		b.WriteString("- ")
		b.WriteString(filename)
		b.WriteString("\n")
	}

	b.WriteString("\nSTEPS:\n")
	b.WriteString("1. Run `git diff HEAD -- <file>` for each changed file to understand what was modified.\n")
	b.WriteString("2. For every function, type, interface, constant, or config value that was changed or deleted,\n")
	b.WriteString("   search the codebase for all callers, importers, and dependents.\n")
	b.WriteString("3. For each call site, reason about whether it is still compatible with the new code.\n")
	b.WriteString("   Pay attention to changed signatures, altered return values, removed exports, and\n")
	b.WriteString("   modified error handling paths.\n")
	b.WriteString("4. Check whether any shared interfaces, contracts, or utility functions were silently\n")
	b.WriteString("   altered in a way that could break downstream consumers.\n")
	b.WriteString("5. Assess operational and runtime risks:\n")
	b.WriteString("   - OS/platform limits (argument length, path length, file descriptor limits)\n")
	b.WriteString("   - Resource constraints (memory, disk, network timeouts, context deadlines)\n")
	b.WriteString("   - Concurrency issues (race conditions, deadlocks, lost updates on shared state)\n")
	b.WriteString("   - Environment assumptions (hardcoded paths, missing env vars, platform-specific behaviour)\n")
	b.WriteString("6. Verify the changes match the stated intent. Flag anything that goes beyond the intent\n")
	b.WriteString("   or that leaves the intent partially unimplemented.\n\n")

	b.WriteString("Respond ONLY with a JSON object matching the provided schema.\n")
	b.WriteString("For affected_areas, include the symbol name, file:line locations, and risk level (LOW/MEDIUM/HIGH).\n")
	b.WriteString("Be specific — cite file paths and line numbers, not vague descriptions.")

	return b.String()
}

package analyser

import (
	"strings"
)

// BuildPrompt assembles the analysis prompt from the task intent and a list of
// changed filenames. The model is expected to read files and run git diff itself.
func BuildPrompt(intent string, changedFiles []string) string {
	var b strings.Builder

	b.WriteString("You are a staff-level software engineer performing blast radius analysis.\n")
	b.WriteString("Claude Code just completed a task. It already verified that modified function\n")
	b.WriteString("signatures are compatible with their direct call sites. YOUR job is different:\n")
	b.WriteString("find the things it DIDN'T check — behavioral regressions, broken invariants,\n")
	b.WriteString("unintended side effects, and operational risks.\n\n")

	b.WriteString("RULES:\n")
	b.WriteString("- Read-only. Do NOT modify files or run state-changing commands.\n")
	b.WriteString("- You may read files, search the codebase, and run git diff/log/show/blame.\n\n")

	b.WriteString("TASK INTENT:\n")
	b.WriteString(intent)
	b.WriteString("\n\nCHANGED FILES:\n")

	for _, filename := range changedFiles {
		b.WriteString("- ")
		b.WriteString(filename)
		b.WriteString("\n")
	}

	b.WriteString("\nSTEPS:\n")
	b.WriteString("1. Run `git diff HEAD -- <file>` for each file to understand what changed.\n\n")

	b.WriteString("2. BEHAVIORAL REGRESSIONS (highest priority):\n")
	b.WriteString("   - Did the change alter the behavior of any existing code path, even if\n")
	b.WriteString("     the function signatures are unchanged? (e.g., a function now returns\n")
	b.WriteString("     early where it didn't before, a default value changed, an error is\n")
	b.WriteString("     swallowed instead of propagated, ordering changed)\n")
	b.WriteString("   - Are there callers that depend on the OLD behavior? Read the actual\n")
	b.WriteString("     call sites and reason about what they expect.\n")
	b.WriteString("   - Were any implicit contracts broken? (e.g., a function that always\n")
	b.WriteString("     returned non-nil now can return nil, a map that was always populated\n")
	b.WriteString("     is now sometimes empty, a channel that was buffered is now unbuffered)\n\n")

	b.WriteString("3. UNINTENDED SIDE EFFECTS:\n")
	b.WriteString("   - Did the change touch shared state, global variables, or package-level\n")
	b.WriteString("     init() functions that other packages depend on?\n")
	b.WriteString("   - Did it change file formats, serialization, or wire protocols in a way\n")
	b.WriteString("     that breaks existing persisted data or external consumers?\n")
	b.WriteString("   - Did it alter environment variable usage, config file parsing, or CLI\n")
	b.WriteString("     flag behavior that downstream scripts or CI depend on?\n\n")

	b.WriteString("4. OPERATIONAL AND RUNTIME RISKS:\n")
	b.WriteString("   - OS/platform limits (arg length, path length, fd limits)\n")
	b.WriteString("   - Resource issues (memory, disk, timeouts, context deadlines)\n")
	b.WriteString("   - Concurrency bugs (races, deadlocks, lost updates on shared state)\n")
	b.WriteString("   - Security regressions (broadened permissions, weakened validation,\n")
	b.WriteString("     secrets in logs, unsafe file permissions)\n\n")

	b.WriteString("5. INTENT MATCH:\n")
	b.WriteString("   - Does the change do what was asked and ONLY what was asked?\n")
	b.WriteString("   - Flag scope creep (changes beyond the intent) and partial\n")
	b.WriteString("     implementation (intent not fully addressed).\n\n")

	b.WriteString("DO NOT waste time verifying that function signatures match their call sites —\n")
	b.WriteString("Claude Code already checked that. Focus on the behavioral and operational\n")
	b.WriteString("risks listed above.\n\n")

	b.WriteString("Respond ONLY with a JSON object matching the provided schema.\n")
	b.WriteString("For affected_areas, cite specific file:line locations and risk level.\n")
	b.WriteString("If everything is genuinely safe, say so — don't invent problems.")

	return b.String()
}

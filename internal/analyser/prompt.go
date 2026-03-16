package analyser

import (
	"fmt"
	"strings"
)

// BuildPrompt assembles the analysis prompt from the task intent and a map of
// filename → diff content. The returned string is passed directly as the
// positional argument to the analysis backend CLI.
func BuildPrompt(intent string, diffs map[string]string) string {
	var b strings.Builder

	b.WriteString("You are a senior code reviewer performing blast radius analysis.\n")
	b.WriteString("Claude Code just completed a task. Your job is to analyse whether \n")
	b.WriteString("the changes are safe and whether anything else in the codebase \n")
	b.WriteString("could be affected.\n")
	b.WriteString("\n")
	b.WriteString("DO NOT modify any files. DO NOT execute any commands that change state.\n")
	b.WriteString("You may read files and search the codebase freely.\n")
	b.WriteString("\n")
	b.WriteString("TASK INTENT:\n")
	b.WriteString(intent)
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("CHANGED FILES AND DIFFS:\n")

	for filename, diff := range diffs {
		fmt.Fprintf(&b, "--- %s\n%s\n\n", filename, diff)
	}

	b.WriteString("YOUR ANALYSIS TASKS:\n")
	b.WriteString("1. Identify every function, type, interface, or constant that was modified or deleted\n")
	b.WriteString("2. Search the codebase for all usages of those symbols\n")
	b.WriteString("3. Reason about whether those call sites are still compatible with the changes\n")
	b.WriteString("4. Check if any shared utilities, interfaces, or contracts were silently altered\n")
	b.WriteString("5. Assess whether the changes match the stated intent and no other existing functionality is broken.\n")
	b.WriteString("\n")
	b.WriteString("Respond ONLY with a JSON object matching the provided schema.")

	return b.String()
}

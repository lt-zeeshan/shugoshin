package shugoshin_init

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeeshans/shugoshin/internal/codex"
	"github.com/zeeshans/shugoshin/internal/logger"
)

// shugoshinDirs are the subdirectories created under .shugoshin/ on init.
var shugoshinDirs = []string{
	filepath.Join(".shugoshin", "schemas"),
	filepath.Join(".shugoshin", "state"),
	filepath.Join(".shugoshin", "reports"),
}

const gitignoreEntry = ".shugoshin/state/"

// Init initialises Shugoshin for the project at projectRoot:
//  1. Creates .shugoshin/schemas/, .shugoshin/state/, and .shugoshin/reports/.
//  2. Writes the verdict JSON schema to .shugoshin/schemas/verdict.json.
//  3. Merges Shugoshin hook entries into .claude/settings.json.
//  4. Adds .shugoshin/state/ to .gitignore (only if not already present).
func Init(projectRoot string) error {
	baseDir := filepath.Join(projectRoot, ".shugoshin")
	logger.Init(baseDir)

	for _, dir := range shugoshinDirs {
		full := filepath.Join(projectRoot, dir)
		if err := os.MkdirAll(full, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", full, err)
		}
	}
	logger.Info("created .shugoshin directories")
	fmt.Println("Created .shugoshin/{schemas,state,reports}/")

	schemaPath := filepath.Join(projectRoot, ".shugoshin", "schemas", "verdict.json")
	if err := os.WriteFile(schemaPath, codex.VerdictSchema, 0o644); err != nil {
		return fmt.Errorf("writing verdict schema: %w", err)
	}
	logger.Info("wrote verdict schema to %s", schemaPath)
	fmt.Println("Wrote .shugoshin/schemas/verdict.json")

	if err := MergeHooks(projectRoot); err != nil {
		return fmt.Errorf("merging hooks: %w", err)
	}
	logger.Info("merged hooks into .claude/settings.json")
	fmt.Println("Merged Shugoshin hooks into .claude/settings.json")

	if err := addGitignoreEntry(projectRoot, gitignoreEntry); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}
	logger.Info("added %s to .gitignore", gitignoreEntry)
	fmt.Println("Added .shugoshin/state/ to .gitignore")

	logger.Info("init complete")
	fmt.Println("Shugoshin initialised successfully.")
	return nil
}

// addGitignoreEntry appends entry to .gitignore if it is not already present.
func addGitignoreEntry(projectRoot, entry string) error {
	path := filepath.Join(projectRoot, ".gitignore")

	existing, err := readLines(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	for _, line := range existing {
		if line == entry {
			return nil // already present
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer f.Close()

	// Ensure the entry is on its own line.
	prefix := ""
	if len(existing) > 0 && existing[len(existing)-1] != "" {
		prefix = "\n"
	}
	if _, err := fmt.Fprintf(f, "%s%s\n", prefix, entry); err != nil {
		return fmt.Errorf("writing .gitignore entry: %w", err)
	}
	return nil
}

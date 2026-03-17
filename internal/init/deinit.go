package shugoshin_init

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lt-zeeshan/shugoshin/internal/analyser"
	"github.com/lt-zeeshan/shugoshin/internal/logger"
)

// Deinit reverses the effects of Init for the project at projectRoot:
//  1. Removes Shugoshin hook entries from .claude/settings.json.
//  2. Removes the .shugoshin/ directory tree entirely.
//  3. Removes the .shugoshin/state/ line from .gitignore.
func Deinit(projectRoot string) error {
	baseDir := filepath.Join(projectRoot, ".shugoshin")
	logger.Init(baseDir)

	if err := RemoveHooks(projectRoot); err != nil {
		return fmt.Errorf("removing hooks: %w", err)
	}
	logger.Info("removed hooks from .claude/settings.json")
	fmt.Println("Removed Shugoshin hooks from .claude/settings.json")

	// Close the logger before removing the directory that contains the log file.
	logger.Info("removing .shugoshin/")
	logger.Close()

	shugoshinDir := filepath.Join(projectRoot, ".shugoshin")
	if err := os.RemoveAll(shugoshinDir); err != nil {
		return fmt.Errorf("removing .shugoshin/: %w", err)
	}
	fmt.Println("Removed .shugoshin/")

	if err := removeGitignoreEntry(projectRoot, gitignoreEntry); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}
	fmt.Println("Removed .shugoshin/state/ from .gitignore")

	if err := analyser.RemoveLeanHome(); err != nil {
		return fmt.Errorf("removing codex home: %w", err)
	}
	fmt.Printf("Removed lean Codex home at %s\n", analyser.LeanHomePath())

	fmt.Println("Shugoshin deinitialised successfully.")
	return nil
}

// removeGitignoreEntry removes all lines in .gitignore that exactly match
// entry. It is a no-op if the file does not exist or the entry is absent.
func removeGitignoreEntry(projectRoot, entry string) error {
	path := filepath.Join(projectRoot, ".gitignore")

	lines, err := readLines(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	filtered := make([]string, 0, len(lines))
	found := false
	for _, line := range lines {
		if line == entry {
			found = true
			continue
		}
		filtered = append(filtered, line)
	}
	if !found {
		return nil
	}

	return writeLines(path, filtered)
}

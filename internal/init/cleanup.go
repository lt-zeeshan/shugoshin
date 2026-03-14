package shugoshin_init

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeeshans/shugoshin/internal/logger"
)

// Cleanup removes and recreates .shugoshin/state/ and .shugoshin/reports/ for
// the project at projectRoot. Hook configuration is left untouched.
func Cleanup(projectRoot string) error {
	baseDir := filepath.Join(projectRoot, ".shugoshin")
	logger.Init(baseDir)

	for _, sub := range []string{"state", "reports"} {
		dir := filepath.Join(projectRoot, ".shugoshin", sub)

		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removing .shugoshin/%s/: %w", sub, err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("recreating .shugoshin/%s/: %w", sub, err)
		}
		logger.Info("reset .shugoshin/%s/", sub)
	}

	logger.Info("cleanup complete")
	fmt.Println("Cleared .shugoshin/state/ and .shugoshin/reports/")
	fmt.Println("Shugoshin cleanup complete.")
	return nil
}

// readLines reads a file and returns its lines with newlines stripped.
// Lines that are blank are preserved. Returns os.ErrNotExist via os.IsNotExist
// when the file does not exist.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// writeLines writes lines to path, each separated by a newline. A trailing
// newline is added after the last line if lines is non-empty.
func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Package reports handles persistence and retrieval of Shugoshin verdict reports.
package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeeshans/shugoshin/internal/types"
)

const timestampLayout = "20060102T150405"

// reportDir returns the session-scoped report directory path.
func reportDir(baseDir, sessionID string) string {
	return filepath.Join(baseDir, "reports", sessionID)
}

// reportFilename builds the filename for a report given its timestamp and index.
func reportFilename(report *types.Report) string {
	return fmt.Sprintf("%s-%03d.json", report.Timestamp.UTC().Format(timestampLayout), report.ResponseIndex)
}

// WriteReport marshals report as indented JSON and writes it to
// {baseDir}/reports/{sessionID}/{timestamp}-{index}.json, creating all parent
// directories as needed. It returns the absolute path of the file written.
func WriteReport(baseDir string, report *types.Report) (string, error) {
	dir := reportDir(baseDir, report.SessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating report directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling report for session %s index %d: %w", report.SessionID, report.ResponseIndex, err)
	}

	path := filepath.Join(dir, reportFilename(report))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing report to %s: %w", path, err)
	}
	return path, nil
}

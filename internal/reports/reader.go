package reports

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lt-zeeshan/shugoshin/internal/types"
)

// parseReportFile reads and unmarshals a single JSON report file.
// The file path is stored in Report.FilePath for later update/delete.
func parseReportFile(path string) (*types.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading report file %s: %w", path, err)
	}
	var r types.Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing report file %s: %w", path, err)
	}
	r.FilePath = path
	return &r, nil
}

// UpdateReport writes the report back to its original file path.
func UpdateReport(report *types.Report) error {
	if report.FilePath == "" {
		return fmt.Errorf("report has no file path")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}
	return os.WriteFile(report.FilePath, data, 0o644)
}

// DeleteReport removes the report file from disk.
func DeleteReport(report *types.Report) error {
	if report.FilePath == "" {
		return fmt.Errorf("report has no file path")
	}
	return os.Remove(report.FilePath)
}

// sortNewestFirst sorts reports descending by Timestamp, then descending by
// ResponseIndex as a tiebreaker.
func sortNewestFirst(reports []*types.Report) {
	sort.Slice(reports, func(i, j int) bool {
		ti, tj := reports[i].Timestamp, reports[j].Timestamp
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return reports[i].ResponseIndex > reports[j].ResponseIndex
	})
}

// ListReports walks {baseDir}/reports/, parses every .json file found across
// all session subdirectories, and returns them sorted newest-first by
// timestamp. An empty or non-existent reports directory returns an empty slice
// without error.
func ListReports(baseDir string) ([]*types.Report, error) {
	root := filepath.Join(baseDir, "reports")

	var reports []*types.Report
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			return fmt.Errorf("walking reports directory: %w", err)
		}
		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		r, err := parseReportFile(path)
		if err != nil {
			return err
		}
		reports = append(reports, r)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sortNewestFirst(reports)
	return reports, nil
}

// ListReportsBySession returns all reports for a single session, sorted
// newest-first. The session directory need not exist; a missing directory
// returns an empty slice without error.
func ListReportsBySession(baseDir, sessionID string) ([]*types.Report, error) {
	dir := reportDir(baseDir, sessionID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session report directory %s: %w", dir, err)
	}

	var reports []*types.Report
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		r, err := parseReportFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}

	sortNewestFirst(reports)
	return reports, nil
}

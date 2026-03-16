package tracking

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
)

type AnalysisStatus struct {
	PID       int    `json:"pid"`
	StartTime int64  `json:"start_time"` // unix timestamp
	FileCount int    `json:"file_count"`
	Backend   string `json:"backend"`
	SessionID string `json:"session_id"`
}

func markerPath(baseDir, sessionID string) string {
	return filepath.Join(baseDir, "state", ".analysing-"+sessionID+".json")
}

// WriteMarker creates a marker file indicating analysis is in progress.
func WriteMarker(baseDir string, status *AnalysisStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return os.WriteFile(markerPath(baseDir, status.SessionID), data, 0o644)
}

// RemoveMarker removes the analysis marker file.
func RemoveMarker(baseDir, sessionID string) {
	os.Remove(markerPath(baseDir, sessionID))
}

// ListActive returns all active (non-stale) analysis statuses.
// It removes stale marker files (where the PID is no longer running).
func ListActive(baseDir string) []AnalysisStatus {
	pattern := filepath.Join(baseDir, "state", ".analysing-*.json")
	matches, _ := filepath.Glob(pattern)

	var active []AnalysisStatus
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			os.Remove(path)
			continue
		}
		var s AnalysisStatus
		if err := json.Unmarshal(data, &s); err != nil {
			os.Remove(path)
			continue
		}
		if !isProcessRunning(s.PID) {
			os.Remove(path)
			continue
		}
		active = append(active, s)
	}
	return active
}

func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

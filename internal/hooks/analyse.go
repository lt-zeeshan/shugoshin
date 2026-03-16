package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zeeshans/shugoshin/internal/analyser"
	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/reports"
	"github.com/zeeshans/shugoshin/internal/tracking"
	"github.com/zeeshans/shugoshin/internal/types"
)

// HandleAnalyse runs the analysis from a serialised AnalyseRequest file.
// This is invoked as a background subprocess by HandleStop.
// If executor is non-nil it is used directly (for tests); otherwise a new
// analyser is created from the request's Backend field.
func HandleAnalyse(reqPath string, executor analyser.Analyser) error {
	// Only delete files that match the expected temp file pattern.
	defer func() {
		if strings.HasPrefix(filepath.Base(reqPath), "shugoshin-analyse-") {
			os.Remove(reqPath)
		}
	}()

	data, err := os.ReadFile(reqPath)
	if err != nil {
		return fmt.Errorf("reading analyse request: %w", err)
	}

	var req AnalyseRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parsing analyse request: %w", err)
	}

	logger.Init(req.BaseDir)
	logger.Info("background analysis started session_id=%s file_count=%d backend=%s", req.SessionID, len(req.ChangedFiles), req.Backend)

	_ = tracking.WriteMarker(req.BaseDir, &tracking.AnalysisStatus{
		PID:       os.Getpid(),
		StartTime: time.Now().Unix(),
		FileCount: len(req.ChangedFiles),
		Backend:   req.Backend,
		SessionID: req.SessionID,
	})
	defer tracking.RemoveMarker(req.BaseDir, req.SessionID)

	if executor == nil {
		executor = analyser.New(req.Backend)
	}

	schemaPath := req.BaseDir + "/schemas/verdict.json"

	verdict, err := executor.Analyse(context.Background(), req.Intent, req.ChangedFiles, schemaPath)
	if err != nil {
		logger.Error("analysis: %v", err)
		return nil
	}
	if verdict == nil {
		logger.Error("analysis returned nil verdict")
		return nil
	}

	logger.Info("%s verdict: %s summary=%q", executor.Name(), verdict.Verdict, verdict.Summary)

	report := types.Report{
		SessionID:     req.SessionID,
		Cwd:           req.Cwd,
		Timestamp:     time.Now().UTC(),
		ResponseIndex: req.ResponseIndex,
		Intent:        req.Intent,
		ChangedFiles:  req.ChangedFiles,
		Verdict:       *verdict,
	}

	reportPath, err := reports.WriteReport(req.BaseDir, &report)
	if err != nil {
		logger.Error("writing report: %v", err)
		return nil
	}
	logger.Info("report written to %s", reportPath)
	logger.Info("background analysis complete")

	return nil
}

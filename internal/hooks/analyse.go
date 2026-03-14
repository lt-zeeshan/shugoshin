package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zeeshans/shugoshin/internal/codex"
	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/reports"
	"github.com/zeeshans/shugoshin/internal/types"
)

// HandleAnalyse runs the Codex analysis from a serialised AnalyseRequest file.
// This is invoked as a background subprocess by HandleStop.
func HandleAnalyse(reqPath string, executor codex.Executor) error {
	defer os.Remove(reqPath)

	data, err := os.ReadFile(reqPath)
	if err != nil {
		return fmt.Errorf("reading analyse request: %w", err)
	}

	var req AnalyseRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parsing analyse request: %w", err)
	}

	logger.Init(req.BaseDir)
	logger.Info("background analysis started session_id=%s file_count=%d", req.SessionID, len(req.Diffs))

	schemaPath := req.BaseDir + "/schemas/verdict.json"

	verdict, err := executor.Analyse(context.Background(), req.Intent, req.Diffs, schemaPath)
	if err != nil {
		logger.Error("codex analysis: %v", err)
		return nil
	}
	if verdict == nil {
		logger.Error("codex analysis returned nil verdict")
		return nil
	}

	logger.Info("codex verdict: %s summary=%q", verdict.Verdict, verdict.Summary)

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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zeeshans/shugoshin/internal/codex"
	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/reports"
	"github.com/zeeshans/shugoshin/internal/state"
	"github.com/zeeshans/shugoshin/internal/types"
)

// HandleStop processes a Stop hook event. It reads the payload from r, builds
// diffs for all changed files, invokes executor.Analyse, writes a report, and
// clears session state. All errors are suppressed — hooks must never crash
// Claude Code.
func HandleStop(r io.Reader, executor codex.Executor) (retErr error) {
	defer func() {
		recover() //nolint:errcheck
		retErr = nil
	}()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}

	var payload types.HookPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}

	baseDir := filepath.Join(payload.Cwd, ".shugoshin")
	logger.Init(baseDir)

	logger.Info("handling stop hook session_id=%s", payload.SessionID)

	if payload.StopHookActive {
		logger.Info("skipping: stop_hook_active")
		return nil
	}

	s, err := state.Load(baseDir, payload.SessionID)
	if err != nil {
		logger.Error("state.Load: %v", err)
		return nil
	}

	if len(s.CurrentChanges) == 0 {
		logger.Info("skipping: no changes")
		return nil
	}

	diffs := make(map[string]string, len(s.CurrentChanges))
	for _, file := range s.CurrentChanges {
		logger.Debug("generating diff for %s", file)
		cmd := exec.Command("git", "diff", "HEAD", "--", file)
		cmd.Dir = payload.Cwd
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			// Untracked or new file — read full contents.
			content, readErr := os.ReadFile(filepath.Join(payload.Cwd, file))
			if readErr != nil {
				logger.Error("reading untracked file %s: %v", file, readErr)
				continue
			}
			diffs[file] = string(content)
		} else {
			diffs[file] = string(out)
		}
	}

	schemaPath := filepath.Join(payload.Cwd, ".shugoshin", "schemas", "verdict.json")

	logger.Info("invoking codex analysis file_count=%d", len(diffs))
	verdict, err := executor.Analyse(context.Background(), s.CurrentIntent, diffs, schemaPath)
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
		SessionID:     s.SessionID,
		Cwd:           s.Cwd,
		Timestamp:     time.Now().UTC(),
		ResponseIndex: s.ResponseIndex,
		Intent:        s.CurrentIntent,
		ChangedFiles:  s.CurrentChanges,
		Verdict:       *verdict,
	}

	reportPath, err := reports.WriteReport(baseDir, &report)
	if err != nil {
		logger.Error("writing report: %v", err)
		return nil
	}
	logger.Info("report written to %s", reportPath)

	fmt.Printf("[shugoshin] %s: %s\n", verdict.Verdict, verdict.Summary)

	if err := state.ClearResponse(baseDir, s); err != nil {
		logger.Error("state.ClearResponse: %v", err)
	}

	logger.Info("stop hook complete")
	return nil
}

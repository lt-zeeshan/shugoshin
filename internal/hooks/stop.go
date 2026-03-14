package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/zeeshans/shugoshin/internal/codex"
	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/state"
	"github.com/zeeshans/shugoshin/internal/types"
)

// AnalyseRequest is serialised to a temp file and passed to the background
// analyse subprocess.
type AnalyseRequest struct {
	BaseDir       string            `json:"base_dir"`
	SessionID     string            `json:"session_id"`
	Cwd           string            `json:"cwd"`
	Intent        string            `json:"intent"`
	ChangedFiles  []string          `json:"changed_files"`
	Diffs         map[string]string `json:"diffs"`
	ResponseIndex int               `json:"response_index"`
}

// HandleStop processes a Stop hook event. It reads the payload, builds diffs,
// clears session state, and spawns a background subprocess for the slow Codex
// analysis. The hook itself exits in milliseconds.
func HandleStop(r io.Reader, _ codex.Executor) (retErr error) {
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

	// Build diffs synchronously (fast — just git commands).
	diffs := make(map[string]string, len(s.CurrentChanges))
	for _, file := range s.CurrentChanges {
		relFile := file
		if filepath.IsAbs(file) {
			if rel, err := filepath.Rel(payload.Cwd, file); err == nil {
				relFile = rel
			}
		}
		logger.Debug("generating diff for %s", relFile)
		cmd := exec.Command("git", "diff", "HEAD", "--", relFile)
		cmd.Dir = payload.Cwd
		out, err := cmd.Output()
		if err != nil || len(out) == 0 {
			absFile := relFile
			if !filepath.IsAbs(relFile) {
				absFile = filepath.Join(payload.Cwd, relFile)
			}
			content, readErr := os.ReadFile(absFile)
			if readErr != nil {
				logger.Error("reading untracked file %s: %v", relFile, readErr)
				continue
			}
			diffs[relFile] = string(content)
		} else {
			diffs[relFile] = string(out)
		}
	}

	// Ensure the on-disk schema matches the embedded version.
	schemaPath := filepath.Join(payload.Cwd, ".shugoshin", "schemas", "verdict.json")
	_ = os.WriteFile(schemaPath, codex.VerdictSchema, 0o644)

	// Clear state now (before async analysis) so the next response starts clean.
	if err := state.ClearResponse(baseDir, s); err != nil {
		logger.Error("state.ClearResponse: %v", err)
	}

	// Write the analyse request to a temp file.
	req := AnalyseRequest{
		BaseDir:       baseDir,
		SessionID:     s.SessionID,
		Cwd:           s.Cwd,
		Intent:        s.CurrentIntent,
		ChangedFiles:  s.CurrentChanges,
		Diffs:         diffs,
		ResponseIndex: s.ResponseIndex,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		logger.Error("marshalling analyse request: %v", err)
		return nil
	}

	reqFile, err := os.CreateTemp("", "shugoshin-analyse-*.json")
	if err != nil {
		logger.Error("creating temp file for analyse request: %v", err)
		return nil
	}
	reqPath := reqFile.Name()
	if _, err := reqFile.Write(reqData); err != nil {
		reqFile.Close()
		logger.Error("writing analyse request: %v", err)
		return nil
	}
	reqFile.Close()

	// Spawn background analysis subprocess — fire and forget.
	bgCmd := exec.Command("shugoshin", "hook", "analyse", reqPath)
	bgCmd.Dir = payload.Cwd
	bgCmd.Stdout = nil
	bgCmd.Stderr = nil
	if err := bgCmd.Start(); err != nil {
		logger.Error("starting background analysis: %v", err)
		os.Remove(reqPath)
		return nil
	}

	// Detach — don't wait for it.
	go func() {
		_ = bgCmd.Wait()
	}()

	logger.Info("spawned background analysis pid=%d file_count=%d", bgCmd.Process.Pid, len(diffs))
	fmt.Printf("[shugoshin] analysing %d changed files in background...\n", len(diffs))

	return nil
}

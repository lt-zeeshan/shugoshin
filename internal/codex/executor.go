package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/types"
)

const analyseTimeout = 120 * time.Second

// Executor analyses a set of diffs against the stated task intent and returns
// a structured verdict. Analyse must never return a Go error — failures are
// encoded as TIMEOUT or ERROR verdicts so the hook pipeline never crashes.
type Executor interface {
	Analyse(ctx context.Context, intent string, diffs map[string]string, schemaPath string) (*types.Verdict, error)
}

// RealExecutor invokes the Codex CLI as a subprocess.
type RealExecutor struct{}

// Analyse builds the prompt, spawns `codex exec`, and parses its structured
// JSON output. Context deadline is augmented with an internal 120-second cap.
// All subprocess failures are returned as TIMEOUT or ERROR verdicts; this
// method never returns a non-nil Go error.
func (r RealExecutor) Analyse(ctx context.Context, intent string, diffs map[string]string, schemaPath string) (*types.Verdict, error) {
	prompt := BuildPrompt(intent, diffs)

	timeoutCtx, cancel := context.WithTimeout(ctx, analyseTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(
		timeoutCtx,
		"codex", "exec",
		"--ephemeral",
		"--profile", "shugoshin",
		"--approval-policy", "never",
		"--output-schema", schemaPath,
		prompt,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Info("running codex exec")
	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	if runErr != nil {
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			logger.Error("codex timed out after %s: %v", elapsed, timeoutCtx.Err())
			return &types.Verdict{
				Verdict:   "TIMEOUT",
				Summary:   "Codex analysis timed out",
				Reasoning: fmt.Sprintf("Codex did not complete within %s. stderr: %s", analyseTimeout, stderr.String()),
			}, nil
		}
		logger.Error("codex exited with error after %s exit_code=%v stderr=%q", elapsed, runErr, stderr.String())
		return &types.Verdict{
			Verdict:   "ERROR",
			Summary:   "Codex exited with error",
			Reasoning: stderr.String(),
		}, nil
	}

	logger.Info("codex completed in %s", elapsed.Round(time.Millisecond))
	return parseVerdict(stdout.Bytes())
}

// parseVerdict unmarshals raw JSON bytes into a Verdict. On failure it returns
// an ERROR verdict rather than a Go error, keeping the hook pipeline stable.
func parseVerdict(raw []byte) (*types.Verdict, error) {
	var v types.Verdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return &types.Verdict{
			Verdict:   "ERROR",
			Summary:   "Invalid JSON from Codex",
			Reasoning: string(raw),
		}, nil
	}
	return &v, nil
}

package analyser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lt-zeeshan/shugoshin/internal/logger"
	"github.com/lt-zeeshan/shugoshin/internal/types"
)

// ClaudeAnalyser invokes the Claude Code CLI as a subprocess.
type ClaudeAnalyser struct{}

func (ClaudeAnalyser) Name() string { return "claude" }

func (ClaudeAnalyser) Analyse(ctx context.Context, intent string, changedFiles []string, schemaPath string) (*types.Verdict, error) {
	prompt := BuildPrompt(intent, changedFiles)

	timeoutCtx, cancel := context.WithTimeout(ctx, analyseTimeout)
	defer cancel()

	// Pass the schema content inline (not a file path).
	schema := string(VerdictSchema)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(
		timeoutCtx,
		"claude", "-p",
		"--output-format", "json",
		"--allowedTools", "View,Read,Glob,Grep,Bash(git diff:*),Bash(git log:*),Bash(git show:*),Bash(git blame:*)",
		"--json-schema", schema,
	)
	// Pipe the prompt via stdin to avoid CLI arg length limits.
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Info("running claude -p")
	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	if runErr != nil {
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			logger.Error("claude timed out after %s: %v", elapsed, timeoutCtx.Err())
			return &types.Verdict{
				Verdict:   "TIMEOUT",
				Summary:   "Claude analysis timed out",
				Reasoning: fmt.Sprintf("Claude did not complete within %s. stderr: %s", analyseTimeout, stderr.String()),
			}, nil
		}
		logger.Error("claude exited with error after %s exit_code=%v stderr=%q", elapsed, runErr, stderr.String())
		return &types.Verdict{
			Verdict:   "ERROR",
			Summary:   "Claude exited with error",
			Reasoning: stderr.String(),
		}, nil
	}

	logger.Info("claude completed in %s", elapsed.Round(time.Millisecond))
	if stderr.Len() > 0 {
		s := stderr.String()
		if len(s) > 200 {
			s = s[:200] + "... (truncated)"
		}
		logger.Debug("claude stderr: %s", s)
	}
	return parseClaudeOutput(stdout.Bytes())
}

// claudeEnvelope is the JSON envelope returned by `claude -p --output-format json`.
type claudeEnvelope struct {
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	IsError          bool            `json:"is_error"`
}

// parseClaudeOutput extracts the verdict from Claude's JSON envelope.
// It first tries structured_output (present when --json-schema is used),
// then falls back to parsing the result text field.
func parseClaudeOutput(raw []byte) (*types.Verdict, error) {
	var env claudeEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &types.Verdict{
			Verdict:   "ERROR",
			Summary:   "Invalid JSON envelope from Claude",
			Reasoning: string(raw),
		}, nil
	}

	if env.IsError {
		return &types.Verdict{
			Verdict:   "ERROR",
			Summary:   "Claude returned an error",
			Reasoning: env.Result,
		}, nil
	}

	// Prefer structured_output when available.
	if len(env.StructuredOutput) > 0 {
		return parseVerdict(env.StructuredOutput)
	}

	// Fall back to parsing the result text.
	return parseVerdict([]byte(env.Result))
}

package analyser

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/types"
)

const analyseTimeout = 600 * time.Second

// CodexAnalyser invokes the Codex CLI as a subprocess.
type CodexAnalyser struct{}

func (CodexAnalyser) Name() string { return "codex" }

// LeanHomePath returns the path to the lean CODEX_HOME directory used by
// Shugoshin. This is a temp directory with no MCP servers configured.
func LeanHomePath() string {
	return filepath.Join(os.TempDir(), "shugoshin-codex")
}

// SetupLeanHome creates a lean CODEX_HOME directory with no MCP servers.
// It symlinks auth.json from ~/.codex so credentials are shared, and writes
// a minimal config.toml with just the model.
func SetupLeanHome() error {
	dir := LeanHomePath()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating codex home %s: %w", dir, err)
	}

	// Symlink auth from real codex home.
	realHome, _ := os.UserHomeDir()
	realAuth := filepath.Join(realHome, ".codex", "auth.json")
	link := filepath.Join(dir, "auth.json")
	_ = os.Remove(link)
	if err := os.Symlink(realAuth, link); err != nil {
		// Fall back to copy if symlink fails.
		data, readErr := os.ReadFile(realAuth)
		if readErr != nil {
			return fmt.Errorf("reading codex auth: %w", readErr)
		}
		_ = os.WriteFile(link, data, 0o600)
	}

	// Minimal config — just the model, no MCP servers.
	cfg := filepath.Join(dir, "config.toml")
	_ = os.WriteFile(cfg, []byte("model = \"gpt-5.4\"\n"), 0o644)

	return nil
}

// RemoveLeanHome removes the lean CODEX_HOME directory.
func RemoveLeanHome() error {
	return os.RemoveAll(LeanHomePath())
}

// Analyse builds the prompt, spawns `codex exec`, and parses its structured
// JSON output.
func (CodexAnalyser) Analyse(ctx context.Context, intent string, diffs map[string]string, schemaPath string) (*types.Verdict, error) {
	prompt := BuildPrompt(intent, diffs)

	timeoutCtx, cancel := context.WithTimeout(ctx, analyseTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(
		timeoutCtx,
		"codex", "exec",
		"--ephemeral",
		"--full-auto",
		"--output-schema", schemaPath,
		prompt,
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Use a lean CODEX_HOME with no MCP servers.
	home := LeanHomePath()
	if _, err := os.Stat(filepath.Join(home, "auth.json")); err != nil {
		if setupErr := SetupLeanHome(); setupErr != nil {
			logger.Error("setting up lean codex home: %v", setupErr)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "auth.json")); err == nil {
		cmd.Env = append(os.Environ(), "CODEX_HOME="+home)
	}

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

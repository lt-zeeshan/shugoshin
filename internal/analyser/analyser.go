// Package analyser provides the analysis abstraction for Shugoshin.
// Each backend (Codex, Claude, etc.) implements the Analyser interface.
package analyser

import (
	"context"
	"encoding/json"

	"github.com/zeeshans/shugoshin/internal/types"
)

// Analyser analyses changed files against the stated task intent and returns
// a structured verdict. Analyse must never return a Go error — failures are
// encoded as TIMEOUT or ERROR verdicts so the hook pipeline never crashes.
type Analyser interface {
	Analyse(ctx context.Context, intent string, changedFiles []string, schemaPath string) (*types.Verdict, error)
	Name() string
}

// Backends is the ordered list of supported backend names.
var Backends = []string{"claude", "codex"}

// ValidBackend reports whether name is a recognised backend.
func ValidBackend(name string) bool {
	for _, b := range Backends {
		if b == name {
			return true
		}
	}
	return false
}

// New returns an Analyser for the given backend name.
// Unrecognised values fall back to claude (the default).
func New(backend string) Analyser {
	switch backend {
	case "codex":
		return CodexAnalyser{}
	default:
		return ClaudeAnalyser{}
	}
}

// parseVerdict unmarshals raw JSON bytes into a Verdict. On failure it returns
// an ERROR verdict rather than a Go error, keeping the hook pipeline stable.
func parseVerdict(raw []byte) (*types.Verdict, error) {
	var v types.Verdict
	if err := json.Unmarshal(raw, &v); err != nil {
		return &types.Verdict{
			Verdict:   "ERROR",
			Summary:   "Invalid JSON from analyser",
			Reasoning: string(raw),
		}, nil
	}
	return &v, nil
}

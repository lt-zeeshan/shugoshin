// Package state manages persistent session state for Shugoshin hook processing.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeeshans/shugoshin/internal/types"
)

func statePath(baseDir, sessionID string) string {
	return filepath.Join(baseDir, "state", sessionID+".json")
}

// Load reads the session state for sessionID from baseDir/state/{sessionID}.json.
// If the file does not exist, a zero-value state with SessionID set is returned
// without error.
func Load(baseDir, sessionID string) (*types.SessionState, error) {
	path := statePath(baseDir, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &types.SessionState{SessionID: sessionID}, nil
		}
		return nil, fmt.Errorf("reading state file %s: %w", path, err)
	}

	var state types.SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshaling state from %s: %w", path, err)
	}
	return &state, nil
}

// Save writes state atomically to baseDir/state/{SessionID}.json, creating
// parent directories as needed. The write is performed via a temp file and
// os.Rename to avoid partial writes.
func Save(baseDir string, state *types.SessionState) error {
	dir := filepath.Join(baseDir, "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory %s: %w", dir, err)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling state for session %s: %w", state.SessionID, err)
	}

	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing state to temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	final := statePath(baseDir, state.SessionID)
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file to %s: %w", final, err)
	}
	return nil
}

// ClearResponse resets CurrentChanges, increments ResponseIndex, and persists
// the updated state via Save. CurrentIntent is preserved so that subsequent
// Stop events in the same conversation turn still have the user's intent.
func ClearResponse(baseDir string, state *types.SessionState) error {
	state.CurrentChanges = nil
	state.ResponseIndex++
	return Save(baseDir, state)
}

// Package hooks implements Claude Code hook handlers for Shugoshin.
package hooks

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/zeeshans/shugoshin/internal/logger"
	"github.com/zeeshans/shugoshin/internal/state"
	"github.com/zeeshans/shugoshin/internal/types"
)

// HandleSubmit processes a UserPromptSubmit hook event. It reads the payload
// from r, records the current intent in session state, and returns nil. All
// errors are suppressed — hooks must never crash Claude Code.
func HandleSubmit(r io.Reader) (retErr error) {
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

	logger.Info("handling submit hook session_id=%s", payload.SessionID)

	s, err := state.Load(baseDir, payload.SessionID)
	if err != nil {
		logger.Error("state.Load: %v", err)
		return nil
	}

	s.CurrentIntent = payload.Prompt
	s.Cwd = payload.Cwd
	s.SessionID = payload.SessionID

	intent := payload.Prompt
	if len(intent) > 100 {
		intent = intent[:100]
	}
	logger.Debug("saved intent: %q", intent)

	if err := state.Save(baseDir, s); err != nil {
		logger.Error("state.Save: %v", err)
	}
	return nil
}

package hooks

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/lt-zeeshan/shugoshin/internal/logger"
	"github.com/lt-zeeshan/shugoshin/internal/state"
	"github.com/lt-zeeshan/shugoshin/internal/types"
)

// HandlePostTool processes a PostToolUse hook event. It reads the payload from
// r, extracts the file_path from ToolInput, appends it to the session's
// CurrentChanges (deduplicating), and persists state. All errors are
// suppressed — hooks must never crash Claude Code.
func HandlePostTool(r io.Reader) (retErr error) {
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

	logger.Debug("handling posttool hook session_id=%s tool_name=%s", payload.SessionID, payload.ToolName)

	filePath, ok := payload.ToolInput["file_path"].(string)
	if !ok || filePath == "" {
		logger.Debug("skipped: no file_path in tool input")
		return nil
	}

	s, err := state.Load(baseDir, payload.SessionID)
	if err != nil {
		logger.Error("state.Load: %v", err)
		return nil
	}

	if !containsString(s.CurrentChanges, filePath) {
		s.CurrentChanges = append(s.CurrentChanges, filePath)
		logger.Info("tracked new file: %s", filePath)
	}

	if err := state.Save(baseDir, s); err != nil {
		logger.Error("state.Save: %v", err)
	}
	return nil
}

// containsString reports whether slice contains s.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

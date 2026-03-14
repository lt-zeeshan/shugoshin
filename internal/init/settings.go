// Package shugoshin_init manages the lifecycle of Shugoshin's project integration:
// creating the directory structure, merging Claude Code hooks, and cleaning up.
package shugoshin_init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const settingsPath = ".claude/settings.json"

// shugoshinHooks is the hook configuration Shugoshin merges into settings.json.
// Each entry is tagged with _shugoshin: true for targeted removal on deinit.
var shugoshinHooks = map[string][]map[string]any{
	"UserPromptSubmit": {
		{
			"_shugoshin": true,
			"hooks": []any{
				map[string]any{"type": "command", "command": "shugoshin hook submit"},
			},
		},
	},
	"PostToolUse": {
		{
			"_shugoshin": true,
			"matcher": "Edit|Write|MultiEdit",
			"hooks": []any{
				map[string]any{"type": "command", "command": "shugoshin hook posttool"},
			},
		},
	},
	"Stop": {
		{
			"_shugoshin": true,
			"hooks": []any{
				map[string]any{"type": "command", "command": "shugoshin hook stop"},
			},
		},
	},
}

// MergeHooks reads .claude/settings.json (creating the file and directory if
// necessary), adds Shugoshin hook entries under each event key if they are not
// already present, and writes the file back. All pre-existing entries are
// preserved unchanged.
func MergeHooks(projectRoot string) error {
	settings, err := readSettings(projectRoot)
	if err != nil {
		return err
	}

	hooks := extractHooks(settings)

	changed := false
	for event, entries := range shugoshinHooks {
		existing, _ := hooks[event].([]any)
		if shugoshinPresent(existing) {
			continue
		}
		// Convert our typed entries to []any so they merge cleanly.
		for _, e := range entries {
			existing = append(existing, e)
		}
		hooks[event] = existing
		changed = true
	}

	if !changed {
		return nil
	}

	settings["hooks"] = hooks
	return writeSettings(projectRoot, settings)
}

// RemoveHooks reads .claude/settings.json and removes every hook group entry
// that carries _shugoshin: true. It is a no-op when the file does not exist or
// contains no Shugoshin entries.
func RemoveHooks(projectRoot string) error {
	path := filepath.Join(projectRoot, settingsPath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	settings, err := readSettings(projectRoot)
	if err != nil {
		return err
	}

	hooks := extractHooks(settings)
	modified := false

	for event, raw := range hooks {
		existing, ok := raw.([]any)
		if !ok {
			continue
		}
		filtered := filterShugoshin(existing)
		if len(filtered) != len(existing) {
			hooks[event] = filtered
			modified = true
		}
	}

	if !modified {
		return nil
	}

	settings["hooks"] = hooks
	return writeSettings(projectRoot, settings)
}

// readSettings reads and parses .claude/settings.json. If the file does not
// exist the directory and file are created and an empty map is returned.
func readSettings(projectRoot string) (map[string]any, error) {
	path := filepath.Join(projectRoot, settingsPath)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return settings, nil
}

// writeSettings serialises settings and writes it to .claude/settings.json
// with two-space indentation for human readability.
func writeSettings(projectRoot string, settings map[string]any) error {
	path := filepath.Join(projectRoot, settingsPath)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// extractHooks returns the "hooks" sub-map from settings, initialising it when
// absent. The returned map is always non-nil and stored back into settings.
func extractHooks(settings map[string]any) map[string]any {
	raw, ok := settings["hooks"]
	if !ok {
		hooks := map[string]any{}
		settings["hooks"] = hooks
		return hooks
	}
	hooks, ok := raw.(map[string]any)
	if !ok {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	return hooks
}

// shugoshinPresent reports whether any entry in the slice has _shugoshin: true.
func shugoshinPresent(entries []any) bool {
	for _, e := range entries {
		if isShugoshinEntry(e) {
			return true
		}
	}
	return false
}

// filterShugoshin returns a new slice omitting entries that have _shugoshin: true.
func filterShugoshin(entries []any) []any {
	out := make([]any, 0, len(entries))
	for _, e := range entries {
		if !isShugoshinEntry(e) {
			out = append(out, e)
		}
	}
	return out
}

// isShugoshinEntry reports whether e is a map with _shugoshin set to a truthy value.
func isShugoshinEntry(e any) bool {
	m, ok := e.(map[string]any)
	if !ok {
		return false
	}
	v, exists := m["_shugoshin"]
	if !exists {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

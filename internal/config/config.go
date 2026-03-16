package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DefaultBackend = "claude"

type Settings struct {
	Backend string `json:"backend"`
}

func Load(baseDir string) (*Settings, error) {
	path := filepath.Join(baseDir, "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{Backend: DefaultBackend}, nil
		}
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return &Settings{Backend: DefaultBackend}, nil
	}
	if !validBackend(s.Backend) {
		s.Backend = DefaultBackend
	}
	return &s, nil
}

// validBackend reports whether name is a recognised backend.
// Duplicated from analyser to avoid a circular import.
func validBackend(name string) bool {
	switch name {
	case "claude", "codex":
		return true
	default:
		return false
	}
}

func Save(baseDir string, s *Settings) error {
	path := filepath.Join(baseDir, "settings.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

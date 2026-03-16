package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Backend != DefaultBackend {
		t.Errorf("Backend = %q, want %q", s.Backend, DefaultBackend)
	}
}

func TestLoad_ReadsExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"backend":"claude"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Backend != "claude" {
		t.Errorf("Backend = %q, want %q", s.Backend, "claude")
	}
}

func TestLoad_DefaultsOnInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Backend != DefaultBackend {
		t.Errorf("Backend = %q, want %q", s.Backend, DefaultBackend)
	}
}

func TestLoad_DefaultsOnEmptyBackend(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"backend":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Backend != DefaultBackend {
		t.Errorf("Backend = %q, want %q", s.Backend, DefaultBackend)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	want := &Settings{Backend: "claude"}
	if err := Save(dir, want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Backend != want.Backend {
		t.Errorf("Backend = %q, want %q", got.Backend, want.Backend)
	}
}

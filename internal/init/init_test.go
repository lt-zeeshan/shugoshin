package shugoshin_init_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	shugoshin_init "github.com/zeeshans/shugoshin/internal/init"
)

// ---- helpers ----------------------------------------------------------------

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return m
}

func readGitignore(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	return string(data)
}

// countShugoshinEntries counts hook group entries tagged _shugoshin:true across
// all events in the given hooks map.
func countShugoshinEntries(hooks map[string]any) int {
	count := 0
	for _, raw := range hooks {
		entries, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, e := range entries {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if b, ok := m["_shugoshin"].(bool); ok && b {
				count++
			}
		}
	}
	return count
}

// ---- Init tests -------------------------------------------------------------

func TestInit_CreatesDirectoryStructure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	dirs := []string{
		filepath.Join(root, ".shugoshin", "schemas"),
		filepath.Join(root, ".shugoshin", "state"),
		filepath.Join(root, ".shugoshin", "reports"),
	}
	for _, d := range dirs {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory %s not found: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", d)
		}
	}
}

func TestInit_WritesVerdictSchema(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	schemaPath := filepath.Join(root, ".shugoshin", "schemas", "verdict.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("reading verdict schema: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("verdict schema is empty")
	}

	// Must be valid JSON.
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("verdict schema is not valid JSON: %v", err)
	}

	// Spot-check required fields from the spec.
	if _, ok := schema["type"]; !ok {
		t.Error("verdict schema missing 'type' field")
	}
}

func TestInit_Idempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("first Init() error: %v", err)
	}
	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("second Init() error: %v", err)
	}

	// Hooks must not be duplicated.
	settings := readJSON(t, filepath.Join(root, ".claude", "settings.json"))
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks key missing after double Init")
	}
	got := countShugoshinEntries(hooks)
	if got != 3 {
		t.Errorf("expected 3 shugoshin hook entries after idempotent Init, got %d", got)
	}

	// .gitignore must not contain duplicate entries.
	gi := readGitignore(t, root)
	count := strings.Count(gi, ".shugoshin/state/")
	if count != 1 {
		t.Errorf(".gitignore contains %d occurrences of .shugoshin/state/, want 1", count)
	}
}

// ---- MergeHooks tests -------------------------------------------------------

func TestMergeHooks_CreatesSettingsWhenAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.MergeHooks(root); err != nil {
		t.Fatalf("MergeHooks() error: %v", err)
	}

	path := filepath.Join(root, ".claude", "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings.json not created: %v", err)
	}

	settings := readJSON(t, path)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks key missing")
	}
	if countShugoshinEntries(hooks) != 3 {
		t.Errorf("expected 3 shugoshin entries, got %d", countShugoshinEntries(hooks))
	}
}

func TestMergeHooks_PreservesExistingNonShugoshinHooks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Write pre-existing settings with a non-Shugoshin hook.
	existing := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "myapp hook"},
					},
				},
			},
		},
	}
	clauDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(clauDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(existing)
	if err := os.WriteFile(filepath.Join(clauDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shugoshin_init.MergeHooks(root); err != nil {
		t.Fatalf("MergeHooks() error: %v", err)
	}

	settings := readJSON(t, filepath.Join(clauDir, "settings.json"))
	hooks, _ := settings["hooks"].(map[string]any)
	entries, _ := hooks["UserPromptSubmit"].([]any)

	// Should have 2 entries: the pre-existing one + the shugoshin one.
	if len(entries) != 2 {
		t.Errorf("UserPromptSubmit entries = %d, want 2", len(entries))
	}

	// The pre-existing entry must still be there (no _shugoshin key).
	foundOther := false
	for _, e := range entries {
		m, _ := e.(map[string]any)
		if _, has := m["_shugoshin"]; !has {
			foundOther = true
		}
	}
	if !foundOther {
		t.Error("pre-existing non-shugoshin hook was not preserved")
	}
}

// ---- RemoveHooks tests ------------------------------------------------------

func TestRemoveHooks_RemovesOnlyShugoshinEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clauDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(clauDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write settings containing both shugoshin and non-shugoshin entries.
	initial := map[string]any{
		"hooks": map[string]any{
			"UserPromptSubmit": []any{
				map[string]any{"_shugoshin": true, "hooks": []any{}},
				map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "other"}}},
			},
		},
	}
	data, _ := json.Marshal(initial)
	settingsPath := filepath.Join(clauDir, "settings.json")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shugoshin_init.RemoveHooks(root); err != nil {
		t.Fatalf("RemoveHooks() error: %v", err)
	}

	settings := readJSON(t, settingsPath)
	hooks, _ := settings["hooks"].(map[string]any)
	entries, _ := hooks["UserPromptSubmit"].([]any)

	if len(entries) != 1 {
		t.Errorf("UserPromptSubmit entries after remove = %d, want 1", len(entries))
	}
	m, _ := entries[0].(map[string]any)
	if _, has := m["_shugoshin"]; has {
		t.Error("shugoshin entry was not removed")
	}
}

func TestRemoveHooks_GracefulWhenNoShugoshinHooks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clauDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(clauDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Settings with no _shugoshin entries.
	noShugoshin := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "other-stop"}}},
			},
		},
	}
	data, _ := json.Marshal(noShugoshin)
	if err := os.WriteFile(filepath.Join(clauDir, "settings.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shugoshin_init.RemoveHooks(root); err != nil {
		t.Fatalf("RemoveHooks() returned error when no shugoshin hooks present: %v", err)
	}
}

func TestRemoveHooks_GracefulWhenFileAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.RemoveHooks(root); err != nil {
		t.Fatalf("RemoveHooks() on missing file returned error: %v", err)
	}
}

// ---- Deinit tests -----------------------------------------------------------

func TestDeinit_RemovesShugoshinDirAndHooks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Init first so there is something to remove.
	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	if err := shugoshin_init.Deinit(root); err != nil {
		t.Fatalf("Deinit() error: %v", err)
	}

	// .shugoshin/ must be gone.
	if _, err := os.Stat(filepath.Join(root, ".shugoshin")); !os.IsNotExist(err) {
		t.Error(".shugoshin/ still exists after Deinit")
	}

	// No shugoshin hooks must remain.
	settingsFile := filepath.Join(root, ".claude", "settings.json")
	if _, err := os.Stat(settingsFile); err == nil {
		settings := readJSON(t, settingsFile)
		hooks, _ := settings["hooks"].(map[string]any)
		if n := countShugoshinEntries(hooks); n != 0 {
			t.Errorf("shugoshin hook entries still present after Deinit: %d", n)
		}
	}
}

// ---- Cleanup tests ----------------------------------------------------------

func TestCleanup_PreservesHooksClearsStatAndReports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	// Plant a file in state and reports to verify they are cleared.
	stateFile := filepath.Join(root, ".shugoshin", "state", "sess.json")
	if err := os.WriteFile(stateFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportFile := filepath.Join(root, ".shugoshin", "reports", "report.json")
	if err := os.WriteFile(reportFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shugoshin_init.Cleanup(root); err != nil {
		t.Fatalf("Cleanup() error: %v", err)
	}

	// state/ and reports/ must exist but be empty.
	for _, sub := range []string{"state", "reports"} {
		dir := filepath.Join(root, ".shugoshin", sub)
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf(".shugoshin/%s/ missing after Cleanup: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf(".shugoshin/%s is not a directory", sub)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Errorf("reading .shugoshin/%s/: %v", sub, err)
			continue
		}
		if len(entries) != 0 {
			t.Errorf(".shugoshin/%s/ contains %d entries, want 0", sub, len(entries))
		}
	}

	// Hooks must still be present.
	settings := readJSON(t, filepath.Join(root, ".claude", "settings.json"))
	hooks, _ := settings["hooks"].(map[string]any)
	if n := countShugoshinEntries(hooks); n != 3 {
		t.Errorf("expected 3 shugoshin hook entries after Cleanup, got %d", n)
	}
}

// ---- Gitignore tests --------------------------------------------------------

func TestGitignore_AddsEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	gi := readGitignore(t, root)
	if !strings.Contains(gi, ".shugoshin/state/") {
		t.Errorf(".gitignore does not contain .shugoshin/state/; got:\n%s", gi)
	}
}

func TestGitignore_DoesNotDuplicate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Write an existing .gitignore already containing the entry.
	existing := ".shugoshin/state/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	gi := readGitignore(t, root)
	if count := strings.Count(gi, ".shugoshin/state/"); count != 1 {
		t.Errorf(".gitignore has %d occurrences of .shugoshin/state/, want 1; got:\n%s", count, gi)
	}
}

func TestGitignore_RemovedOnDeinit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if err := shugoshin_init.Deinit(root); err != nil {
		t.Fatalf("Deinit() error: %v", err)
	}

	gi := readGitignore(t, root)
	if strings.Contains(gi, ".shugoshin/state/") {
		t.Errorf(".gitignore still contains .shugoshin/state/ after Deinit; got:\n%s", gi)
	}
}

func TestGitignore_PreservesOtherEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Pre-populate .gitignore with unrelated entries.
	preExisting := "*.log\nnode_modules/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(preExisting), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := shugoshin_init.Init(root); err != nil {
		t.Fatalf("Init() error: %v", err)
	}
	if err := shugoshin_init.Deinit(root); err != nil {
		t.Fatalf("Deinit() error: %v", err)
	}

	gi := readGitignore(t, root)
	for _, want := range []string{"*.log", "node_modules/"} {
		if !strings.Contains(gi, want) {
			t.Errorf(".gitignore missing %q after Deinit; got:\n%s", want, gi)
		}
	}
}

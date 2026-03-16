# CLAUDE.md — Shugoshin Development Guide

## What is this project?

Shugoshin is a blast radius analyser for Claude Code. It hooks into Claude Code's lifecycle, runs a second AI (Claude or Codex) after every response to analyse what changed, and presents structured verdicts through a TUI. Written in Go.

## Quick reference

```bash
go build ./...           # build
go test -race ./...      # test (always use -race)
go vet ./...             # lint
go install ./cmd/shugoshin/  # install binary (required after changes for hooks to pick up)
shugoshin init           # set up in a project
shugoshin deinit         # remove from a project
shugoshin                # launch TUI
```

## Architecture

```
cmd/shugoshin/main.go     Cobra CLI — routes to hooks, init, TUI
internal/
  analyser/               Analyser interface + backends (Claude, Codex)
  config/                 Settings load/save (.shugoshin/settings.json)
  hooks/                  Claude Code hook handlers (submit, posttool, stop, analyse)
  init/                   Project setup/teardown (init, deinit, cleanup)
  logger/                 File-based debug logger
  reports/                Report read/write
  state/                  Session state persistence
  tracking/               Background analysis PID tracking
  tui/                    Bubble Tea TUI (model, update, view)
  types/                  Shared structs (no logic)
```

### Data flow

1. **UserPromptSubmit** hook → `hooks/submit.go` → saves intent to state
2. **PostToolUse** hook → `hooks/posttool.go` → appends changed file to state
3. **Stop** hook → `hooks/stop.go` → reads state, spawns background analysis subprocess
4. Background subprocess → `hooks/analyse.go` → calls `analyser.New(backend).Analyse()` → writes report
5. TUI → `tui/` → reads reports from disk, displays verdicts

### Key design decisions

- Hooks must exit in milliseconds — analysis runs as a detached background process
- Hooks must never crash Claude Code — all errors are suppressed, failures become ERROR verdicts
- The `Analyser` interface abstracts backends; `New(backend)` factory returns the right one
- State uses atomic writes (temp file + rename) to prevent corruption
- Settings in `.shugoshin/settings.json`, not a global config

## Conventions

### Go

- **Table-driven tests** — all tests use `[]struct` with `t.Run` subtests
- **No mocks beyond interfaces** — test doubles implement the `Analyser` interface directly
- **Atomic file writes** — temp file + `os.Rename` for state and config
- **Never return Go errors from hooks** — encode failures as TIMEOUT/ERROR verdicts
- **`internal/` only** — no exported packages; everything is internal

### Code style

- Match existing patterns exactly (naming, imports, error handling)
- No comments on obvious code; comments only where logic isn't self-evident
- No logging unless it adds diagnostic value (DEBUG for noisy, INFO for meaningful events)
- Imports grouped: stdlib, then external, then internal (goimports order)

### File permissions

- Auth files, temp request files, debug logs: `0o600` (owner-only)
- Schema files, config files: `0o644`
- Directories: `0o755`

### Security

- Claude backend is restricted to read-only tools: `View,Read,Glob,Grep,Bash(git diff:*),Bash(git log:*),Bash(git show:*),Bash(git blame:*)`
- Codex backend uses a lean CODEX_HOME (copy, never symlink, with empty-file guard)
- Hook analyse request files are validated by filename prefix before deletion
- Never log secrets, tokens, or full stderr (truncate to 200 chars)

## Adding a new analysis backend

1. Create `internal/analyser/yourbackend.go` implementing the `Analyser` interface
2. Add a case in `New()` factory in `analyser.go`
3. Add the backend name to `Backends` slice in `analyser.go`
4. Add the name to `validBackend()` in `config/config.go`
5. Run `go test -race ./...`

## Testing

```bash
go test -race ./...              # all tests
go test -race ./internal/tui/    # single package
go test -race -run TestFoo ./internal/analyser/  # single test
```

- Always run with `-race`
- Tests must not depend on external services (Codex, Claude)
- Use `t.TempDir()` for file-based tests
- The TUI tests use `baseModel()` helper and `keyMsg()` for simulating key presses

## Common tasks

### Changing the analysis prompt

Edit `internal/analyser/prompt.go`. Update test assertions in `analyser_test.go`. Rebuild binary with `go install ./cmd/shugoshin/`.

### Adding a TUI keybinding

1. Add case in `handleKey()` in `tui/update.go`
2. Add to help text in `renderHelp()` in `tui/view.go`
3. Add test in `tui/tui_test.go`
4. If the key modifies model state, add the field to `Model` in `tui/model.go`

### Modifying the verdict schema

Edit `internal/analyser/verdict.json`. This is embedded via `//go:embed` in `schema.go`. The schema is written to `.shugoshin/schemas/verdict.json` on init and before each analysis.

## What NOT to do

- Don't modify `~/.codex/auth.json` — only read and copy it
- Don't add MCP servers to the lean CODEX_HOME
- Don't make hook subcommands slow — they block Claude Code's response cycle
- Don't return non-nil errors from hook handlers — suppress and log instead
- Don't commit `.shugoshin/` contents (state, reports, logs)
- Don't hardcode absolute paths in source files

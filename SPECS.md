# Shugoshin — Technical Specification

Shugoshin (守護神, "guardian deity") is a blast radius analyser that sits alongside Claude Code. It watches every response Claude Code makes, analyses what changed and whether it could have unintended side effects, and presents findings through a manually-invoked TUI. Phase 1 is purely informational — no blocking, no gating.

---

## Core Concept

When Claude Code fixes a bug or implements a feature, it tends to be fix-focused. It solves the immediate problem but doesn't always reason about what else in the codebase depends on what it changed. Shugoshin fills that gap by invoking a second AI (Codex CLI) after every Claude Code response to analyse the blast radius of the changes made.

---

## Architecture Overview

Three Claude Code hooks feed a shared session-scoped state file. On every Stop event where files were modified, Codex CLI is invoked as a subprocess to produce a structured verdict. Verdicts are persisted as JSON reports. A manually-invoked TUI reads those reports and presents them for review.

```
UserPromptSubmit  →  capture intent → write to state file
       ↓
[Claude Code works]
       ↓
PostToolUse (Edit|Write|MultiEdit)  →  append changed file to state file
PostToolUse (Edit|Write|MultiEdit)  →  append changed file to state file
       ↓
Stop  →  read intent + changes from state file
      →  check stop_hook_active (exit 0 if true, prevent infinite loop)
      →  check if any files were modified (exit 0 if none)
      →  invoke `codex exec` subprocess
      →  write structured JSON verdict to reports directory
      →  clear current-response changelist from state file
      →  print one-line summary to terminal
```

---

## Implementation Language

Go. Single binary, millisecond startup, no runtime dependencies. All subcommands are part of one `shugoshin` binary. Bubble Tea for the TUI.

---

## Directory Structure

All Shugoshin files live under `.shugoshin/` at the project root.

```
.shugoshin/
  hooks/
    (hook logic is built into the shugoshin binary, invoked as subcommands)
  schemas/
    verdict.json          ← JSON Schema for codex exec --output-schema
  state/
    {session_id}.json     ← ephemeral per-session state (gitignored)
  reports/
    {session_id}/
      {timestamp}-{index}.json   ← one verdict file per Stop event
```

`.shugoshin/state/` must be added to `.gitignore` by `shugoshin init`.
`.shugoshin/reports/` is the user's choice — it can be committed as an audit trail or gitignored.

---

## Go Project Structure

```
shugoshin/
  cmd/
    shugoshin/
      main.go             ← entry point, subcommand routing
  internal/
    hooks/
      submit.go           ← UserPromptSubmit handler
      posttool.go         ← PostToolUse handler
      stop.go             ← Stop handler, orchestrates analysis
    state/
      manager.go          ← read/write session state JSON
    reports/
      writer.go           ← write verdict JSON to reports dir
      reader.go           ← read and list reports for TUI
    codex/
      executor.go         ← codex exec subprocess wrapper
    tui/
      model.go            ← Bubble Tea model
      view.go             ← rendering
      update.go           ← keyboard and event handling
    init/
      init.go             ← shugoshin init logic
      deinit.go           ← shugoshin deinit logic
      cleanup.go          ← shugoshin cleanup logic
  go.mod
  go.sum
  README.md
```

---

## Command Surface

```
shugoshin                    ← launch TUI for current project directory
shugoshin init               ← bind hooks, create directory structure, update .gitignore
shugoshin deinit             ← unbind hooks, remove .shugoshin/ entirely
shugoshin cleanup            ← delete state/ and reports/ only, hooks remain intact
shugoshin hook submit        ← called by UserPromptSubmit hook (reads stdin JSON)
shugoshin hook posttool      ← called by PostToolUse hook (reads stdin JSON)
shugoshin hook stop          ← called by Stop hook (reads stdin JSON)
```

`shugoshin hook *` subcommands are not meant to be called by the user directly. They are invoked by Claude Code's hook system.

---

## Claude Code Hook Configuration

`shugoshin init` reads the existing `.claude/settings.json`, merges the following entries without clobbering existing hooks, and writes it back. Each entry is tagged with `"_shugoshin": true` so `shugoshin deinit` can find and remove exactly these entries.

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "_shugoshin": true,
        "hooks": [
          {
            "type": "command",
            "command": "shugoshin hook submit"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "_shugoshin": true,
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "shugoshin hook posttool"
          }
        ]
      }
    ],
    "Stop": [
      {
        "_shugoshin": true,
        "hooks": [
          {
            "type": "command",
            "command": "shugoshin hook stop"
          }
        ]
      }
    ]
  }
}
```

### `shugoshin init` behaviour

1. Check if `.shugoshin/` already exists — if so, warn and ask for confirmation before proceeding
2. Create `.shugoshin/` directory structure
3. Copy `verdict.json` schema to `.shugoshin/schemas/`
4. Read `.claude/settings.json` (create it if it does not exist)
5. Merge hook entries tagged with `_shugoshin: true` — do not duplicate if already present
6. Write `.claude/settings.json` back
7. Add `.shugoshin/state/` to `.gitignore`
8. Print confirmation of what was done

### `shugoshin deinit` behaviour

1. Read `.claude/settings.json`
2. Remove all hook entries where `_shugoshin: true`
3. Write `.claude/settings.json` back
4. Remove `.shugoshin/` directory entirely
5. Remove `.shugoshin/state/` from `.gitignore`
6. Print confirmation

### `shugoshin cleanup` behaviour

1. Delete `.shugoshin/state/` and all its contents
2. Delete `.shugoshin/reports/` and all its contents
3. Recreate empty `state/` and `reports/` directories
4. Hook configuration is untouched — Shugoshin continues running
5. Print confirmation

---

## Hook Payloads (stdin JSON from Claude Code)

### UserPromptSubmit payload

```json
{
  "session_id": "abc123",
  "hook_event_name": "UserPromptSubmit",
  "cwd": "/path/to/project",
  "prompt": "fix the null pointer bug in user.go"
}
```

Handler writes `prompt` as `current_intent` to `.shugoshin/state/{session_id}.json`.

### PostToolUse payload

```json
{
  "session_id": "abc123",
  "hook_event_name": "PostToolUse",
  "cwd": "/path/to/project",
  "tool_name": "Edit",
  "tool_input": {
    "file_path": "internal/user/user.go"
  },
  "tool_response": {
    "content": "..."
  }
}
```

Handler appends `tool_input.file_path` to `current_changes` in the state file.

### Stop payload

```json
{
  "session_id": "abc123",
  "hook_event_name": "Stop",
  "cwd": "/path/to/project",
  "stop_hook_active": false,
  "transcript_path": "/path/to/transcript.jsonl",
  "last_assistant_message": "I've fixed the null pointer dereference in user.go"
}
```

---

## Session State File

`.shugoshin/state/{session_id}.json`

```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "current_intent": "fix the null pointer bug in user.go",
  "current_changes": [
    "internal/user/user.go",
    "internal/user/user_test.go"
  ],
  "response_index": 3
}
```

`response_index` increments on every Stop event and is used to name report files in order.

After a Stop event is processed, `current_intent` and `current_changes` are cleared. `response_index` is incremented.

---

## Stop Hook Logic (detailed)

```
1. Read stdin JSON payload
2. If stop_hook_active == true → exit 0 immediately (prevent infinite loop)
3. Load state file for this session_id
4. If current_changes is empty → exit 0 (no files modified, skip analysis)
5. For each file in current_changes, generate a git diff (git diff HEAD -- <file>)
   If git diff is empty (file is untracked), read the full file content as the diff
6. Build Codex prompt (see below)
7. Invoke codex exec subprocess (see below)
8. Parse structured JSON response from codex exec
9. Write verdict JSON to .shugoshin/reports/{session_id}/{timestamp}-{index}.json
10. Print one-line summary to stdout (visible in Claude Code transcript)
11. Clear current_intent and current_changes from state file, increment response_index
12. Exit 0
```

---

## Codex CLI Invocation

```bash
codex exec \
  --ephemeral \
  --profile shugoshin \
  --approval-policy never \
  --output-schema .shugoshin/schemas/verdict.json \
  "{prompt}"
```

### Codex Profile

Shugoshin uses a dedicated `[profile.shugoshin]` in `~/.codex/config.toml` with `disable_mcp = true`. This skips MCP server startup (which can add several seconds) since blast radius analysis only needs file reading and code search — capabilities Codex has natively. Users keep their MCP servers for normal interactive Codex usage.

`shugoshin init` should add this profile to `~/.codex/config.toml` if not already present.

### Guardrails for Codex

- `--ephemeral` — no session state persists between analyses
- `--profile shugoshin` — uses the bare profile with MCP servers disabled for fast startup
- `--approval-policy never` — Codex must not pause and wait for human input; this runs in an automated hook context
- `--output-schema` — forces structured JSON output matching the verdict schema
- Codex runs in read-only mode by default (`codex exec` defaults to read-only sandbox) — it must NOT modify any files
- The prompt explicitly instructs Codex to only read and analyse, never write or execute commands
- Timeout: set an explicit timeout on the subprocess (suggested: 120 seconds). If Codex exceeds it, write a verdict of `TIMEOUT` and move on. Never block Claude Code indefinitely.
- If Codex exits with a non-zero code or produces invalid JSON, write a verdict of `ERROR` with the raw output captured. Never crash the hook silently.

### Prompt Structure

```
You are a senior code reviewer performing blast radius analysis.
Claude Code just completed a task. Your job is to analyse whether 
the changes are safe and whether anything else in the codebase 
could be affected.

DO NOT modify any files. DO NOT execute any commands that change state.
You may read files and search the codebase freely.

TASK INTENT:
{current_intent}

CHANGED FILES AND DIFFS:
{for each file: filename + diff}

YOUR ANALYSIS TASKS:
1. Identify every function, type, interface, or constant that was 
   modified or deleted
2. Search the codebase for all usages of those symbols
3. Reason about whether those call sites are still compatible 
   with the changes
4. Check if any shared utilities, interfaces, or contracts were 
   silently altered
5. Assess whether the changes match the stated intent — did Claude 
   do what was asked, and only that?

Respond ONLY with a JSON object matching the provided schema.
```

---

## Verdict JSON Schema

`.shugoshin/schemas/verdict.json`

```json
{
  "type": "object",
  "required": ["verdict", "summary", "affected_areas", "reasoning", "intent_match"],
  "additionalProperties": false,
  "properties": {
    "verdict": {
      "type": "string",
      "enum": ["SAFE", "REVIEW_NEEDED", "HIGH_RISK", "TIMEOUT", "ERROR"]
    },
    "summary": {
      "type": "string",
      "description": "One sentence summary of the finding"
    },
    "affected_areas": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["symbol", "locations", "risk"],
        "properties": {
          "symbol": { "type": "string" },
          "locations": {
            "type": "array",
            "items": { "type": "string" }
          },
          "risk": {
            "type": "string",
            "enum": ["LOW", "MEDIUM", "HIGH"]
          }
        }
      }
    },
    "intent_match": {
      "type": "boolean",
      "description": "Whether the changes appear to match the stated task intent"
    },
    "reasoning": {
      "type": "string",
      "description": "Full explanation of the blast radius analysis"
    }
  }
}
```

---

## Report File

`.shugoshin/reports/{session_id}/{timestamp}-{index}.json`

```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "timestamp": "2026-03-14T13:02:00Z",
  "response_index": 3,
  "intent": "fix the null pointer bug in user.go",
  "changed_files": ["internal/user/user.go", "internal/user/user_test.go"],
  "verdict": {
    "verdict": "REVIEW_NEEDED",
    "summary": "Null check added in user.go but 3 callers pass unchecked values",
    "affected_areas": [
      {
        "symbol": "GetUser()",
        "locations": ["api/handlers/user.go:42", "api/handlers/auth.go:87", "cmd/cli/user.go:14"],
        "risk": "MEDIUM"
      }
    ],
    "intent_match": true,
    "reasoning": "The fix correctly addresses the null dereference..."
  }
}
```

---

## TUI Design

Invoked manually with `shugoshin` from the project root. Reads `.shugoshin/reports/` for the current directory. No background process, no file watching — point-in-time reader.

### Layout

```
┌─ Shugoshin — my-project ─────────────────────────────────────────┐
│ Session: abc123   Branch: feature/auth    2026-03-14             │
├───────────────────────────────────────────────────────────────────┤
│  ●  SAFE          fix the null check in user.go          13:02   │
│  ▲  REVIEW        refactor auth middleware                13:15   │
│  ●  SAFE          add unit test for login flow            13:28   │
│  ■  HIGH RISK     modify shared token validator           13:41   │
├───────────────────────────────────────────────────────────────────┤
│ DETAIL — modify shared token validator                            │
│                                                                   │
│ Verdict:  HIGH RISK                                               │
│ Intent match: YES                                                 │
│ Summary:  Token validator used by 6 routes, 2 have no test cover  │
│                                                                   │
│ Affected:                                                         │
│   GetToken()   api/routes/users.go:42   api/routes/auth.go:87    │
│   ValidateJWT() middleware/session.go:14  (HIGH)                  │
│                                                                   │
│ Reasoning:                                                        │
│   The shared token validator was modified to reject expired       │
│   tokens more aggressively. All 6 routes that call ValidateJWT() │
│   may now reject tokens they previously accepted...               │
│                                                                   │
│ Changed files: auth/token.go  middleware/session.go               │
└───────────────────────────────────────────────────────────────────┘
  ↑↓ navigate   enter expand/collapse   s filter by session   q quit
```

### Colour coding

- `SAFE` → green dot `●`
- `REVIEW_NEEDED` → yellow triangle `▲`
- `HIGH_RISK` → red square `■`
- `TIMEOUT` / `ERROR` → grey `?`

### Keyboard navigation

- `↑` / `↓` — navigate list
- `Enter` — expand/collapse detail pane
- `s` — filter by session (cycle through sessions)
- `f` — filter by verdict (ALL → HIGH_RISK only → REVIEW_NEEDED+ → ALL)
- `r` — reload reports from disk
- `q` — quit

### Session grouping

The TUI groups reports by session ID. When multiple sessions exist for the same project, they are shown as collapsible groups, newest first.

---

## TUI Library

Use Bubble Tea (github.com/charmbracelet/bubbletea) with Lip Gloss (github.com/charmbracelet/lipgloss) for styling.

---

## Key Dependencies

```
github.com/charmbracelet/bubbletea    ← TUI framework
github.com/charmbracelet/lipgloss     ← TUI styling
github.com/spf13/cobra                ← CLI subcommand routing
```

Avoid unnecessary dependencies. JSON, file I/O, subprocess invocation, and directory walking are all Go stdlib.

---

## Error Handling Principles

- Hook scripts must never crash silently. Always exit 0 unless you have a specific reason to signal Claude Code.
- If the state file does not exist at PostToolUse or Stop time, create it rather than erroring.
- If Codex times out, write a `TIMEOUT` verdict and continue. Never leave Claude Code hanging.
- If Codex produces malformed JSON, write an `ERROR` verdict with raw output captured in `reasoning`.
- If `.claude/settings.json` does not exist at `shugoshin init` time, create it with just the Shugoshin hook entries.
- If `shugoshin deinit` is run but no Shugoshin hooks are found, exit gracefully with a message rather than erroring.

---

## Development Phases

### Phase 1 (current scope)
- All three hooks implemented and working
- Codex invoked per Stop event with structured output
- Reports persisted to `.shugoshin/reports/`
- TUI reads and displays reports
- `init`, `deinit`, `cleanup` commands
- Purely informational — no blocking behaviour

### Phase 2 (future, not in scope now)
- Blocking mode — Stop hook can return `decision: block` to force Claude Code to re-examine changes when verdict is HIGH_RISK
- Configurable risk threshold for blocking
- Per-project configuration file `.shugoshin/config.toml`
- Report export to markdown for PR descriptions

---

## Development Instructions for Claude Code

- Use Go modules. Run `go mod init github.com/yourname/shugoshin` to initialise.
- Write tests for state manager, report reader/writer, and the Codex prompt builder.
- The hook subcommands (`shugoshin hook submit`, `shugoshin hook posttool`, `shugoshin hook stop`) must read from stdin, never from command-line arguments, as Claude Code passes hook payloads via stdin JSON.
- Use `cobra` for subcommand routing. The root command with no subcommand launches the TUI.
- Keep hook subcommand binaries fast — they run on every Claude Code response. Avoid any heavy initialisation in the hook path.
- The `codex exec` invocation should capture both stdout (the verdict JSON) and stderr (Codex progress logs). Only stdout is parsed as the verdict. Stderr can be discarded or logged to a debug file.
- All file paths in the state and report files should be relative to `cwd` for portability.
- Test the full hook pipeline manually before considering it done: configure the hooks in a test project, make a change with Claude Code, verify the report file is written correctly, verify the TUI displays it.

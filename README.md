# Shugoshin (守護神)

Blast radius analyser for [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Shugoshin hooks into Claude Code's lifecycle, invokes a configurable analysis backend after every response to analyse what changed and what else in the codebase could be affected, and presents structured verdicts through a terminal UI.

When Claude Code fixes a bug or implements a feature, it solves the immediate problem but doesn't always reason about what else in the codebase depends on what it changed. Shugoshin fills that gap by running a second AI as a read-only reviewer after every response.

Supported analysis backends:
- **Codex CLI** (`codex exec`) — default
- **Claude Code** (`claude -p`) — uses Claude as the analyser

## How it works

```
 You type a prompt
       │
       ▼
 ┌─────────────────────┐
 │  UserPromptSubmit    │──── capture intent ──── write to state file
 └─────────────────────┘
       │
       ▼
 ┌─────────────────────┐
 │  Claude Code works   │
 │  (edits files)       │
 └─────────────────────┘
       │
       ▼
 ┌─────────────────────┐
 │  PostToolUse         │──── track each changed file ──── append to state
 │  (per Edit/Write)    │
 └─────────────────────┘
       │
       ▼
 ┌─────────────────────┐     ┌──────────────────────────────────┐
 │  Stop                │────▶│  analyser (codex or claude)      │
 └─────────────────────┘     └──────────────────────────────────┘
       │                              │
       │                              ▼
       │                     ┌──────────────────────┐
       │                     │  Structured verdict   │
       │                     │  (JSON report)        │
       │                     └──────────────────────┘
       ▼
 One-line summary printed to terminal
```

Three Claude Code hooks run automatically. On every stop event where files were modified, the configured analysis backend generates diffs, analyses the blast radius, and writes a structured JSON verdict to `.shugoshin/reports/`. Phase 1 is purely informational — no blocking, no gating.

### Verdicts

| Verdict | Meaning |
|---------|---------|
| **SAFE** | Changes are contained, no unintended side effects detected |
| **REVIEW_NEEDED** | Changes may affect other parts of the codebase |
| **HIGH_RISK** | Changes to shared symbols with wide blast radius |
| **TIMEOUT** | Analysis backend exceeded the 240-second timeout |
| **ERROR** | Analysis backend crashed or returned invalid output |

## Prerequisites

- Go 1.24+
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) (hooks are configured automatically)
- At least one analysis backend:
  - [Codex CLI](https://github.com/openai/codex) installed and authenticated (default), or
  - [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude` command available in PATH)

## Install

```bash
go install github.com/lt-zeeshan/shugoshin/cmd/shugoshin@latest
```

Or build from source:

```bash
git clone https://github.com/zeeshans/shugoshin.git
cd shugoshin
go install ./cmd/shugoshin/
```

## Quick start

```bash
cd your-project
shugoshin init
```

This will:
1. Create `.shugoshin/` directory structure (schemas, state, reports)
2. Write the verdict JSON schema for structured output
3. Write `.shugoshin/settings.json` with default backend (`claude`)
4. Merge hooks into `.claude/settings.json` (tagged with `_shugoshin: true` for clean removal)
5. Add `.shugoshin/state/` to `.gitignore`

Now use Claude Code normally. After every response that modifies files, you'll see a one-line verdict summary in the transcript:

```
[shugoshin] SAFE: Changes are contained to the auth module with no external callers affected
```

```
[shugoshin] HIGH_RISK: Token validator used by 6 routes, 2 have no test coverage
```

## Viewing reports

```bash
shugoshin
```

Opens an interactive TUI:

```
┌─ Shugoshin — my-project ─────────────────────────────────────────┐
│ Session: all sessions   Filter: ALL   Backend: claude   3 reports  │
├───────────────────────────────────────────────────────────────────┤
│  ●  SAFE          fix the null check in user.go          13:02   │
│  ▲  REVIEW        refactor auth middleware                13:15   │
│  ●  SAFE          add unit test for login flow            13:28   │
│> ■  HIGH RISK     modify shared token validator           13:41   │
├───────────────────────────────────────────────────────────────────┤
│ Verdict:  HIGH RISK                                               │
│ Intent match: YES                                                 │
│ Summary:  Token validator used by 6 routes, 2 have no test cover  │
│                                                                   │
│ Affected:                                                         │
│   GetToken()     api/routes/users.go:42  api/routes/auth.go:87    │
│   ValidateJWT()  middleware/session.go:14  (HIGH)                  │
│                                                                   │
│ Reasoning:                                                        │
│   The shared token validator was modified to reject expired       │
│   tokens more aggressively. All 6 routes that call ValidateJWT() │
│   may now reject tokens they previously accepted...               │
│                                                                   │
│ Changed files: auth/token.go  middleware/session.go               │
└───────────────────────────────────────────────────────────────────┘
  ↑↓ navigate   enter expand   s session   f filter   b backend   r reload   q quit
```

### Keyboard

| Key | Action |
|-----|--------|
| `↑` / `k` | Navigate up (or scroll detail pane when expanded) |
| `↓` / `j` | Navigate down (or scroll detail pane when expanded) |
| `Enter` | Expand/collapse detail pane |
| `Esc` | Close detail pane |
| `s` | Cycle session filter (all sessions → session 1 → session 2 → ...) |
| `f` | Cycle verdict filter (ALL → HIGH_RISK only → REVIEW_NEEDED+ → ALL) |
| `b` | Cycle analysis backend (claude → codex → claude), persists to settings |
| `r` | Reload reports from disk |
| `q` | Quit |

## Commands

| Command | Description |
|---------|-------------|
| `shugoshin` | Launch the TUI to browse reports |
| `shugoshin init` | Set up Shugoshin in the current project |
| `shugoshin cleanup` | Clear state and reports, keep hooks active |
| `shugoshin deinit` | Remove Shugoshin entirely (hooks, dirs, gitignore) |

`shugoshin hook {submit,posttool,stop}` are internal commands invoked by Claude Code hooks — not meant for direct use.

## Example report

Each verdict is stored as a JSON file in `.shugoshin/reports/{session_id}/`:

```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "timestamp": "2026-03-14T13:41:00Z",
  "response_index": 3,
  "intent": "modify shared token validator",
  "changed_files": ["auth/token.go", "middleware/session.go"],
  "verdict": {
    "verdict": "HIGH_RISK",
    "summary": "Token validator used by 6 routes, 2 have no test coverage",
    "affected_areas": [
      {
        "symbol": "ValidateJWT()",
        "locations": ["middleware/session.go:14", "api/routes/auth.go:87"],
        "risk": "HIGH"
      }
    ],
    "intent_match": true,
    "reasoning": "The shared token validator was modified to reject expired tokens more aggressively..."
  }
}
```

Reports can be committed as an audit trail or gitignored — your choice.

## Backend configuration

The analysis backend is set in `.shugoshin/settings.json`:

```json
{
  "backend": "claude"
}
```

Supported values: `"claude"` (default), `"codex"`. Toggle in the TUI with `b`, or edit the file directly.

### Codex backend

Shugoshin uses a lean CODEX_HOME at `$TMPDIR/shugoshin-codex` with no MCP servers configured. This skips MCP server startup (which can add several seconds per analysis) since blast radius analysis only needs Codex's native file reading and code search. Auth is symlinked from `~/.codex/auth.json`. Your normal Codex usage keeps all MCP servers — only Shugoshin's automated analysis runs lean.

### Claude backend

Invokes `claude -p` with `--output-format json` and `--json-schema` for structured output. No special configuration needed — uses your existing Claude Code authentication.

## File layout

```
.shugoshin/                     Created by `shugoshin init`
  settings.json                 Backend selection (codex or claude)
  schemas/verdict.json          JSON Schema for structured output
  state/{session_id}.json       Ephemeral per-session state (gitignored)
  reports/{session_id}/         Verdict reports
    {timestamp}-{index}.json    One report per Claude Code response
  debug.log                     Hook and analyser logs
```

## Project structure

```
cmd/shugoshin/main.go           CLI entry point (Cobra)
internal/
  types/types.go                Shared structs (HookPayload, SessionState, Verdict, Report)
  state/manager.go              Atomic session state read/write
  reports/
    writer.go                   Write verdict JSON to reports dir
    reader.go                   Read and list reports for TUI
  analyser/
    analyser.go                 Analyser interface, New() factory, parseVerdict()
    codex.go                    CodexAnalyser — Codex CLI backend (240s timeout)
    claude.go                   ClaudeAnalyser — Claude Code backend
    prompt.go                   Shared analysis prompt builder
    schema.go                   Embedded verdict JSON schema
  config/
    config.go                   Settings Load/Save (.shugoshin/settings.json)
  hooks/
    submit.go                   UserPromptSubmit — capture intent
    posttool.go                 PostToolUse — track changed files
    stop.go                     Stop — orchestrate analysis pipeline
    analyse.go                  Background analysis subprocess handler
  init/
    init.go                     shugoshin init (creates settings.json)
    deinit.go                   shugoshin deinit
    cleanup.go                  shugoshin cleanup
    settings.go                 .claude/settings.json merge/remove logic
  logger/logger.go              File-based debug logger (never crashes)
  tui/
    model.go                    Bubble Tea model (includes backend field)
    update.go                   Keyboard and event handling (b key for backend)
    view.go                     Rendering with Lip Gloss styling
```

## Debugging

Logs are written to `.shugoshin/debug.log` in append mode. Every hook invocation, state change, Codex invocation (with duration), and error is logged:

```
2026-03-14T13:02:10.283 [INFO] handling submit hook session_id=abc123
2026-03-14T13:02:10.284 [DEBUG] saved intent: "fix the null pointer bug in user.go"
2026-03-14T13:02:15.100 [INFO] handling posttool hook session_id=abc123 tool=Edit
2026-03-14T13:02:15.101 [DEBUG] tracked file change: internal/user/user.go
2026-03-14T13:02:20.500 [INFO] handling stop hook session_id=abc123
2026-03-14T13:02:20.502 [INFO] generating diff for internal/user/user.go
2026-03-14T13:02:20.510 [INFO] invoking codex analysis files=1
2026-03-14T13:02:20.511 [INFO] running codex exec
2026-03-14T13:02:35.200 [INFO] codex completed in 14.689s
2026-03-14T13:02:35.201 [INFO] codex verdict: SAFE
2026-03-14T13:02:35.203 [INFO] report written to .shugoshin/reports/abc123/20260314T130235-003.json
2026-03-14T13:02:35.204 [INFO] stop hook complete
```

If hooks aren't producing reports, check this file first.

## Limitations

- **Phase 1 is informational only** — Shugoshin reports findings but does not block Claude Code from proceeding, even on HIGH_RISK verdicts.
- **Requires at least one backend** — analysis depends on either `codex` or `claude` being available in PATH and authenticated.
- **10-minute timeout** — complex analyses may hit the timeout and produce a TIMEOUT verdict. The timeout prevents hooks from blocking Claude Code indefinitely.
- **No incremental analysis** — each stop event analyses all changed files from scratch, not incrementally from the previous verdict.
- **New session required** — after installing or updating the binary, you must restart your Claude Code session for hooks to pick up the new version.

## Roadmap (Phase 2)

- Blocking mode — stop hook returns `decision: block` to force Claude Code to re-examine HIGH_RISK changes
- Configurable risk threshold for blocking
- Additional analysis backends (Gemini CLI, local models, etc.)
- Report export to markdown for PR descriptions

## License

MIT

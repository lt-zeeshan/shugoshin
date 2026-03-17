# Shugoshin (守護神)

**Autonomous blast radius analyser for [Claude Code](https://docs.anthropic.com/en/docs/claude-code).**

When Claude Code fixes a bug or builds a feature, it solves the immediate problem but doesn't always reason about what else depends on what it changed. Shugoshin fills that gap — it hooks into Claude Code's lifecycle and runs a second AI as a read-only reviewer after every response, producing structured verdicts about what could break.

## How it works

```
 You type a prompt
       │
       ▼
 ┌─────────────────────┐
 │  Claude Code works   │──── Shugoshin tracks intent + changed files
 └─────────────────────┘
       │
       ▼
 ┌─────────────────────┐     ┌──────────────────────────────────┐
 │  Claude Code stops   │────▶│  Background analysis (Claude/    │
 └─────────────────────┘     │  Codex reads diffs + codebase)   │
                              └──────────────────────────────────┘
                                             │
                                             ▼
                              ┌──────────────────────┐
                              │  JSON verdict report   │
                              └──────────────────────┘
```

Three Claude Code hooks run automatically. On every stop event where files were modified, the analysis backend reads the diffs, searches for affected callers, and writes a structured JSON verdict. Analysis runs in the background — it never blocks your workflow.

## Verdicts

| Verdict | Meaning |
|---------|---------|
| **SAFE** | Changes are contained, no unintended side effects |
| **REVIEW_NEEDED** | Changes may affect other parts of the codebase |
| **HIGH_RISK** | Shared symbols with wide blast radius — review before proceeding |
| **TIMEOUT** | Analysis exceeded the 10-minute timeout |
| **ERROR** | Backend crashed or returned invalid output |

## Getting started

### Prerequisites

- Go 1.24+
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
- At least one analysis backend:
  - **Claude Code CLI** (`claude` in PATH) — default, no extra setup
  - **[Codex CLI](https://github.com/openai/codex)** — alternative backend

### Install

```bash
go install github.com/lt-zeeshan/shugoshin/cmd/shugoshin@latest
```

If `shugoshin` gives "command not found", add Go's bin directory to your PATH:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```

Or build from source:

```bash
git clone https://github.com/lt-zeeshan/shugoshin.git
cd shugoshin
go install ./cmd/shugoshin/
```

### Set up in your project

```bash
cd your-project
shugoshin init
```

This creates `.shugoshin/` (schemas, state, reports), writes default settings, merges hooks into `.claude/settings.json`, and updates `.gitignore`. Codex setup is optional — if you don't have Codex installed, init skips it and uses Claude as the default backend.

Now use Claude Code normally — after every response that modifies files, you'll see:

```
[shugoshin] analysing 3 changed files in background (backend: claude)...
```

### View reports

```bash
shugoshin
```

Opens an interactive TUI:

```
┌─ Shugoshin — my-project ─────────────────────────────────────────┐
│ Session: all sessions   Filter: ALL   Backend: claude   4 reports │
│ ⟳ Analysing 3 files in background...                             │
├───────────────────────────────────────────────────────────────────┤
│  ●  SAFE          fix the null check in user.go          13:02   │
│  ▲  REVIEW        refactor auth middleware                13:15   │
│  ✓  SAFE          add unit test for login flow            13:28   │
│> ■  HIGH RISK     modify shared token validator           13:41   │
└───────────────────────────────────────────────────────────────────┘
  ↑↓ navigate  enter expand  x resolve  d delete  h hide resolved
  s session  f filter  b backend  r reload  q quit
```

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Navigate list (or scroll detail pane) |
| `Enter` | Expand/collapse detail view |
| `Esc`, `Backspace` | Close detail view |
| `x` | Toggle resolved (marks a concern as addressed) |
| `d` | Delete report from disk |
| `h` | Hide/show resolved reports |
| `s` | Cycle session filter |
| `f` | Cycle verdict filter (ALL → HIGH_RISK → REVIEW_NEEDED+ → ALL) |
| `b` | Cycle backend (claude → codex), persists to settings |
| `r` | Reload reports from disk |
| `q` | Quit |

## Commands

| Command | What it does |
|---------|-------------|
| `shugoshin` | Open the TUI |
| `shugoshin init` | Set up hooks and directory structure |
| `shugoshin cleanup` | Clear state and reports, keep hooks |
| `shugoshin deinit` | Remove everything (hooks, dirs, gitignore entries) |

## Switching backends

The analysis backend is configured in `.shugoshin/settings.json`:

```json
{
  "backend": "claude"
}
```

Toggle with `b` in the TUI, or edit the file directly. Supported: `"claude"` (default), `"codex"`.

**Claude backend** — invokes `claude -p` with structured JSON output. Restricted to read-only tools (file reading, git diff/log/show/blame). No extra setup needed.

**Codex backend** — invokes `codex exec` with a lean CODEX_HOME (no MCP servers). Auth is copied from `~/.codex/auth.json`. Requires Codex CLI installed and authenticated.

## Example report

```json
{
  "session_id": "abc123",
  "timestamp": "2026-03-14T13:41:00Z",
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

## File layout

```
.shugoshin/                     Created by `shugoshin init`
  settings.json                 Backend selection
  schemas/verdict.json          JSON schema for structured output
  state/{session_id}.json       Per-session state (gitignored)
  reports/{session_id}/         Verdict reports
    {timestamp}-{index}.json    One report per response
  debug.log                     Debug logs
```

## Debugging

Check `.shugoshin/debug.log` — every hook invocation, backend call (with duration), and error is logged. Example:

```
[INFO]  handling stop hook session_id=abc123
[INFO]  spawned background analysis pid=12345 file_count=2 backend=claude
[INFO]  claude completed in 54.926s
[INFO]  claude verdict: SAFE summary="Changes are contained..."
[INFO]  report written to .shugoshin/reports/abc123/20260314T130235-003.json
```

## Limitations

- **Informational only (Phase 1)** — verdicts are advisory, Shugoshin does not block Claude Code
- **Requires a backend CLI** — either `claude` or `codex` must be in PATH and authenticated
- **10-minute timeout** — very large changes may time out
- **Not incremental** — each analysis starts fresh from the current diffs
- **Binary updates need a session restart** — Claude Code caches hook binary paths

## Roadmap

- Blocking mode for HIGH_RISK verdicts
- Configurable risk thresholds
- Additional backends (Gemini, local models)
- Report export to markdown for PRs

## Security

- Analysis backends run with restricted, read-only permissions
- No secrets are stored in source or committed to git
- Hook request files are created with `0o600` permissions
- Debug logs are owner-readable only
- Codex auth is copied (never symlinked) to prevent corruption

## License

MIT

# Codex Hybrid Autopilot

`codex-hybrid-autopilot` is a Go wrapper around the official `codex` CLI. It keeps the upstream Codex binary unpatched, proxies normal CLI invocations straight through when autopilot is not applicable, and turns the interactive root CLI into an orchestrated session loop when autopilot is active.

The wrapper is designed for this workflow:

- visible Codex output in the same terminal
- argument forwarding shaped like the original interactive `codex` CLI
- PTY-backed interactive child sessions for root prompt and root `resume`
- automatic continuation after the interactive child exits, using the last report from Codex's own session artifacts
- occasional operator engagement between turns
- repo-local observability under `.codex-autopilot/`
- backend-decided verification, commit, and push via structured `post_turn_actions`

## Build

```bash
cd /Users/zongbaolu/work/codex-hybrid-autopilot
go build -o bin/codexa ./cmd/codex-hybrid-autopilot
```

The built binary lives at [bin/codexa](/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa).

## Quick start

1. Build the binary.
2. Make sure the real `codex` binary is on `PATH`.
3. Run the wrapper from the target repository with a plain prompt or `exec` form.
4. Let the wrapper save its state under `.codex-autopilot/` in that target repository.

Example:

```bash
cd /path/to/target/repo
/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo \
  --search \
  "Continue the highest-leverage work until no concrete task remains."
```

For a longer step-by-step guide, see [USAGE.md](/Users/zongbaolu/work/codex-hybrid-autopilot/USAGE.md).

## Basic usage

Interactive-style invocations are intercepted and orchestrated automatically:

```bash
bin/codexa --yolo
bin/codexa --yolo --search "Continue the highest-leverage work until no concrete task remains."
bin/codexa --yolo resume --last
bin/codexa --yolo resume --last "Continue after the blocker investigation."
```

Non-interactive `exec` forms are still supported:

```bash
bin/codexa exec "Fix the top failing test and keep going."
bin/codexa exec resume --last "Continue from the current repo state."
```

The wrapper currently intercepts these autopilot-compatible forms:

- root prompt form: `codexa [root codex args] "prompt"`
- bare interactive root form: `codexa [root codex args]`
- root resume form: `codexa [root codex args] resume --last [prompt]`
- `exec` form: `codexa [root codex args] exec [exec args] "prompt"`
- `exec resume` form: `codexa [root codex args] exec resume --last "prompt"`

Everything else is passed through to the real `codex` binary unchanged.

`--yolo` is a wrapper convenience alias for `-p yolo`. It is normalized before either autopilot interception or passthrough, so `codexa --yolo ...` behaves like a native top-level startup form.

Pass-through invocations are forwarded directly to the real `codex` binary:

```bash
bin/codexa --help
bin/codexa login
bin/codexa review
```

The wrapper resolves the real Codex binary from `PATH`. If the wrapper itself is named `codex`, set `CODEX_AUTOPILOT_REAL_BIN` to the upstream binary path.

## How the turn loop works

1. For bare root, root prompt, and root `resume` entrypoints, the wrapper launches the real interactive `codex` child attached to your terminal.
2. When that child exits, the wrapper reads the latest matching session JSONL under `~/.codex/sessions/` and extracts the last assistant message.
3. If the first launch was bare `codexa [root args]`, the wrapper derives the project objective from that finished session's first user message and bootstraps `.codex-autopilot/session_state.json` from it.
4. It saves the turn prompt, extracted last assistant message, and parsed report under `.codex-autopilot/`.
5. It parses the required JSON footer between `AUTO_REPORT_JSON_BEGIN` and `AUTO_REPORT_JSON_END`.
6. It executes any backend-provided `post_turn_actions` after the turn.
7. It pauses briefly for operator input, then either resumes the same interactive session or stops.

If Codex forgets the JSON footer or emits an invalid report, the wrapper immediately resumes the same session with a protocol-repair prompt instead of stopping. The next real turn only starts after a valid report has been recovered.

When you start with `codexa --yolo resume --last` and omit a fresh prompt, the wrapper reuses the previously recorded objective from `.codex-autopilot/session_state.json` when it exists. If there is no wrapper state yet, the wrapper still launches the resumed interactive session, lets you provide the first real prompt naturally, then derives the wrapper objective from that resumed turn and continues under protocol afterward.

For `exec` and `exec resume`, the wrapper keeps using non-interactive Codex commands and `-o/--output-last-message` capture as before.

## Post-turn actions

The wrapper does not hard-code repo-specific verification, commit, or push commands. Instead, Codex must provide exact shell commands in `post_turn_actions` whenever the turn leaves source-code changes that still need verification or finalization.

Example:

```json
[
  {"kind":"verify","command":"go test ./...","description":"Verify the repo."},
  {"kind":"commit","command":"git add -A && git commit -m 'autopilot: finish parser fix'","description":"Commit the verified changes."},
  {"kind":"push","command":"git push upstream HEAD","description":"Push to the preferred remote."}
]
```

If a turn leaves source-code changes dirty and the report omits `post_turn_actions`, the wrapper stops instead of guessing.

## Operator engagement

After each turn, the wrapper prints a short summary. If the pause window expires, it follows the report's `auto_mode_next`. If you hit Enter before the timeout, the wrapper opens operator input mode.

Operator input mode supports:

- plain text: queue a user prompt for the next turn
- `/show`: inspect the queued prompts
- `/clear`: clear the prompt queue
- `/stop`: stop after the current turn

## Observability

Each workspace gets:

- `.codex-autopilot/runtime.json`
- `.codex-autopilot/session_state.json`
- `.codex-autopilot/prompts/turn-XXXX.md`
- `.codex-autopilot/messages/turn-XXXX.md`
- `.codex-autopilot/reports/turn-XXXX.json`
- `.codex-autopilot/action-logs/turn-XXXX-YY-*.log`

`runtime.json` is written immediately when `codexa` enters wrapper mode, before the first interactive child starts. If that file does not appear in the target repo right after `codexa --yolo ...` launches, the session was not started under wrapper control.

## Configuration

Optional workspace-local config lives at `.codex-autopilot/config.json`.

Example:

```json
{
  "pause_window_seconds": 8,
  "skill_hint": true,
  "real_codex_bin": "/opt/homebrew/bin/codex"
}
```

Environment overrides:

- `CODEX_AUTOPILOT_REAL_BIN`
- `CODEX_AUTOPILOT_PAUSE_SECONDS`

## Skill

The repo includes an optional companion skill under [skills/codex-session-autopilot](/Users/zongbaolu/work/codex-hybrid-autopilot/skills/codex-session-autopilot). The Go wrapper does not require it, but the skill makes the same protocol available in manual Codex sessions.

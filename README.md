# Codex Hybrid Autopilot

`codex-hybrid-autopilot` is a Go wrapper around the official `codex` CLI. It keeps the upstream Codex binary unpatched, proxies normal CLI invocations straight through when autopilot is not applicable, and takes over only for turn-based flows that can be resumed safely.

The wrapper is designed for this workflow:

- visible Codex output in the same terminal
- argument forwarding shaped like the original `codex` CLI
- automatic continuation across turns using `codex exec` and `codex exec resume`
- occasional operator engagement between turns
- repo-local observability under `.codex-autopilot/`
- backend-decided verification, commit, and push via structured `post_turn_actions`

## Build

```bash
cd /Users/zongbaolu/work/codex-hybrid-autopilot
go build -o bin/codex-hybrid-autopilot ./cmd/codex-hybrid-autopilot
```

The built binary lives at [bin/codex-hybrid-autopilot](/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codex-hybrid-autopilot).

## Quick start

1. Build the binary.
2. Make sure the real `codex` binary is on `PATH`.
3. Run the wrapper from the target repository with a plain prompt or `exec` form.
4. Let the wrapper save its state under `.codex-autopilot/` in that target repository.

Example:

```bash
cd /path/to/target/repo
/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codex-hybrid-autopilot \
  -p yolo \
  --search \
  "Continue the highest-leverage work until no concrete task remains."
```

For a longer step-by-step guide, see [USAGE.md](/Users/zongbaolu/work/codex-hybrid-autopilot/USAGE.md).

## Basic usage

Compatible turn-based invocations are intercepted and resumed automatically:

```bash
bin/codex-hybrid-autopilot -p yolo --search "Continue the highest-leverage work until no concrete task remains."
bin/codex-hybrid-autopilot exec "Fix the top failing test and keep going."
bin/codex-hybrid-autopilot exec resume --last "Continue from the current repo state."
```

The wrapper currently intercepts these autopilot-compatible forms:

- root prompt form: `codex-hybrid-autopilot [root codex args] "prompt"`
- `exec` form: `codex-hybrid-autopilot [root codex args] exec [exec args] "prompt"`
- `resume` form: `codex-hybrid-autopilot [root codex args] resume --last "prompt"`
- `exec resume` form: `codex-hybrid-autopilot [root codex args] exec resume --last "prompt"`

Everything else is passed through to the real `codex` binary unchanged.

Pass-through invocations are forwarded directly to the real `codex` binary:

```bash
bin/codex-hybrid-autopilot --help
bin/codex-hybrid-autopilot login
bin/codex-hybrid-autopilot review
```

The wrapper resolves the real Codex binary from `PATH`. If the wrapper itself is named `codex`, set `CODEX_AUTOPILOT_REAL_BIN` to the upstream binary path.

## How the turn loop works

1. The wrapper runs the first compatible turn through `codex exec` or `codex exec resume`.
2. It saves the turn prompt, last assistant message, and parsed report under `.codex-autopilot/`.
3. It parses the required JSON footer between `AUTO_REPORT_JSON_BEGIN` and `AUTO_REPORT_JSON_END`.
4. It executes any backend-provided `post_turn_actions` after the turn.
5. It pauses briefly for operator input, then either resumes or stops.

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

- `.codex-autopilot/session_state.json`
- `.codex-autopilot/prompts/turn-XXXX.md`
- `.codex-autopilot/messages/turn-XXXX.md`
- `.codex-autopilot/reports/turn-XXXX.json`
- `.codex-autopilot/action-logs/turn-XXXX-YY-*.log`

## Configuration

Optional workspace-local config lives at `.codex-autopilot/config.json`.

Example:

```json
{
  "max_turns": 30,
  "pause_window_seconds": 8,
  "skill_hint": true,
  "real_codex_bin": "/opt/homebrew/bin/codex"
}
```

Environment overrides:

- `CODEX_AUTOPILOT_REAL_BIN`
- `CODEX_AUTOPILOT_MAX_TURNS`
- `CODEX_AUTOPILOT_PAUSE_SECONDS`

## Skill

The repo includes an optional companion skill under [skills/codex-session-autopilot](/Users/zongbaolu/work/codex-hybrid-autopilot/skills/codex-session-autopilot). The Go wrapper does not require it, but the skill makes the same protocol available in manual Codex sessions.

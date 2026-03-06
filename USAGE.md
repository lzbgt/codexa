# Usage

This guide shows the normal way to run the Go wrapper against a target repository.

## 1. Build the wrapper

```bash
cd /Users/zongbaolu/work/codex-hybrid-autopilot
go build -o bin/codexa ./cmd/codex-hybrid-autopilot
```

## 2. Verify the real Codex binary

The wrapper proxies to the upstream `codex` binary. By default it resolves `codex` from `PATH`.

Check:

```bash
which codex
codex --version
```

If you need to override the path:

```bash
export CODEX_AUTOPILOT_REAL_BIN=/opt/homebrew/bin/codex
```

## 3. Run the wrapper from the target repo

Use a plain prompt if you want the wrapper to start the real interactive Codex child and then orchestrate follow-up turns after that child exits:

```bash
cd /path/to/target/repo
/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo \
  --search \
  "Continue the highest-leverage engineering work until no concrete task remains."
```

You can also start from interactive-style `resume` or explicit `exec` forms:

```bash
/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo \
  resume

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo \
  resume --last

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo \
  resume 019cc422-dc94-7553-a6e9-acfc3d0e183b

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  --yolo \
  resume --last \
  "Continue from the current repo state."

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  exec \
  "Fix the top failing test and keep going."

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codexa \
  exec resume --last \
  "Continue from the current repo state."
```

## 4. Understand what the wrapper intercepts

Autopilot interception applies to:

- bare root interactive startup
- root prompt form
- root `resume` picker form with or without resume flags
- root `resume --last` with or without a fresh prompt
- root `resume <session-id>` with or without a fresh prompt
- `exec "prompt"`
- `exec resume --last "prompt"`

`--yolo` is a wrapper alias for `-p yolo`, so `codexa --yolo ...` is the preferred startup form when you want the wrapper to feel like the interactive CLI.

The key runtime difference is:

- root prompt and root `resume` forms launch the real interactive `codex` process attached to your terminal
- bare `codexa --yolo` also launches the real interactive child; when that first session exits, the wrapper derives the objective from the first user message in the session artifact and continues under autopilot
- that attached child now runs behind a PTY bridge, so fullscreen/inline terminal behavior is much closer to native `codex`
- after that child process exits, the wrapper reads Codex's session JSONL under `~/.codex/sessions/`, extracts the last assistant message, parses the autopilot report, and decides whether to respawn the session
- if the assistant forgot the `AUTO_REPORT_JSON` footer, the wrapper auto-resumes the same session with a repair prompt and only proceeds once a valid report is recovered
- `exec` forms stay fully non-interactive

Pass-through applies to commands such as:

- `--help`
- `login`
- `logout`
- `review`
- `mcp`
- `features`

Those are forwarded directly to the real `codex` binary.

## 5. Watch the runtime state

The wrapper stores runtime state in the current target repo:

- `.codex-autopilot/runtime.json`
- `.codex-autopilot/session_state.json`
- `.codex-autopilot/prompts/`
- `.codex-autopilot/messages/`
- `.codex-autopilot/reports/`
- `.codex-autopilot/action-logs/`

This is where you inspect the previous turn prompt, the final assistant message, and the parsed JSON report.
The wrapper also tracks the matched Codex session id and session file path there, but the authoritative interactive transcript remains in `~/.codex/sessions/`.

`runtime.json` is the first file to check. It is created before the first interactive child starts and records:

- whether wrapper mode is active
- the workspace and state dir
- the normalized mode and forwarded args
- the real upstream `codex` binary path
- the current wrapper phase such as `starting`, `bootstrapping`, `running_turn`, `decision_made`, `continuing`, or `stopped`

Quick check:

```bash
cd /path/to/target/repo
ls .codex-autopilot/runtime.json
cat .codex-autopilot/runtime.json
```

If `runtime.json` does not exist immediately after `codexa --yolo ...` starts, that session was not launched under wrapper control.

If you resume with `codexa --yolo resume --last` and do not provide a new prompt, the wrapper reuses the existing objective from `.codex-autopilot/session_state.json` when it exists. If you use `codexa --yolo resume` or `codexa --yolo resume <session-id>`, the wrapper always launches the real resume flow first so your picker choice or explicit session id is honored, then after your first natural resumed prompt/turn exits it bootstraps protocol state from that resumed session and continues automatically.

## 6. Operator engagement

After each turn, the wrapper prints a summary. During the pause window:

- do nothing: the wrapper follows `auto_mode_next`
- press Enter: open operator input mode
- type prompts in operator input mode: queue them for the next turn
- use `/show`, `/clear`, or `/stop` as needed

## 7. Post-turn verification, commit, and push

The wrapper does not guess repo-specific commands. Codex must emit exact `post_turn_actions` in the JSON footer when a turn leaves dirty source-code changes that still need verification or finalization.

Example:

```json
[
  {"kind":"verify","command":"go test ./...","description":"Verify the repo."},
  {"kind":"commit","command":"git add -A && git commit -m 'autopilot: finish parser fix'","description":"Commit the verified changes."},
  {"kind":"push","command":"git push upstream HEAD","description":"Push to the preferred remote."}
]
```

If the turn leaves new dirty source changes and the footer omits `post_turn_actions`, the wrapper stops instead of inventing commands.

## 8. Workspace-local config

Optional config file:

`/path/to/target/repo/.codex-autopilot/config.json`

Example:

```json
{
  "pause_window_seconds": 8,
  "skill_hint": true,
  "real_codex_bin": "/opt/homebrew/bin/codex"
}
```

Equivalent environment overrides:

```bash
export CODEX_AUTOPILOT_PAUSE_SECONDS=8
export CODEX_AUTOPILOT_REAL_BIN=/opt/homebrew/bin/codex
```

## 9. Troubleshooting

- If the wrapper passes a command straight through instead of entering autopilot mode, use one of the supported prompt or `exec` forms above.
- If a session appears to behave like plain `codex`, check `.codex-autopilot/runtime.json` first. No file means no wrapper engagement for that run.
- If `codexa --yolo resume`, `codexa --yolo resume --last`, or `codexa --yolo resume <session-id>` is pointed at a native Codex session with no wrapper state yet, give the resumed session one real prompt first; after that first resumed turn exits, the wrapper can bootstrap and continue automatically.
- If the wrapper reports that multiple Codex sessions changed in the same workspace during one turn, close the extra session and rerun. The wrapper now refuses to guess which JSONL belongs to the turn.
- If the wrapper cannot resolve the real Codex binary, set `CODEX_AUTOPILOT_REAL_BIN`.
- If the wrapper stops because the repo is still dirty, inspect `.codex-autopilot/reports/turn-XXXX.json` and check whether Codex emitted the expected `post_turn_actions`.

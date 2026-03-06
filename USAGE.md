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
- after that child process exits, the wrapper reads Codex's session JSONL under `~/.codex/sessions/`, quotes the last assistant message into the next prompt, merges/reweights against current TODOs, and decides whether to respawn the session
- `AUTO_MODE_NEXT=stop` stops the loop; if no stop/continue marker is present, the wrapper defaults to continuing
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

The authoritative runtime state is the Codex transcript under `~/.codex/sessions/`.

`codexa` no longer creates a repo-local `.codex-autopilot/` directory. To inspect what happened, find the matching session JSONL for the target workspace and read the last assistant reply there.

If you resume with `codexa --yolo resume --last` and do not provide a new prompt, the wrapper still does not need any saved wrapper state. It continues from the resumed Codex session itself.

## 6. Operator engagement

After each turn, the wrapper prints a summary. During the pause window:

- do nothing: the wrapper follows `auto_mode_next`
- press Enter: open operator input mode
- type prompts in operator input mode: queue them for the next turn
- use `/show`, `/clear`, or `/stop` as needed

## 7. Configuration

Environment overrides:

```bash
export CODEX_AUTOPILOT_PAUSE_SECONDS=8
export CODEX_AUTOPILOT_REAL_BIN=/opt/homebrew/bin/codex
```

## 8. Troubleshooting

- If the wrapper passes a command straight through instead of entering autopilot mode, use one of the supported prompt or `exec` forms above.
- If a session appears to behave like plain `codex`, confirm you launched `codexa`, not the upstream `codex`, and inspect the matching JSONL under `~/.codex/sessions/`.
- If `codexa --yolo resume`, `codexa --yolo resume --last`, or `codexa --yolo resume <session-id>` is pointed at a native Codex session, give the resumed session one real prompt first if needed; after that first resumed turn exits, the wrapper can continue automatically from the session artifact.
- If the wrapper reports that multiple Codex sessions changed in the same workspace during one turn, close the extra session and rerun. The wrapper now refuses to guess which JSONL belongs to the turn.
- If the wrapper cannot resolve the real Codex binary, set `CODEX_AUTOPILOT_REAL_BIN`.

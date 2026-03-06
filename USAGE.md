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

Use a plain prompt if you want the wrapper to start the real interactive Codex child and then orchestrate follow-up turns inside that same live session:

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
- the attached child runs behind a PTY bridge and `codexa` forces `--no-alt-screen` on that live child so the output stream remains visible and capturable
- after each wrapper-generated turn, the wrapper prefers the assistant footer line `AUTO_MODE_NEXT=continue|stop` or compatibility `AUTO_CONTINUE_MODE=continue|stop` as the fast completion signal
- if a user-driven turn omits the footer, the wrapper falls back to the upstream Codex session transcript to recover the last reply, then quotes that reply into the next prompt and continues unless the footer explicitly said `stop`
- `AUTO_MODE_NEXT=stop` stops the loop; missing marker defaults to `continue`
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

The authoritative runtime state is the live terminal transcript plus the upstream Codex session. `codexa` no longer creates a repo-local `.codex-autopilot/` directory.

If you resume with `codexa --yolo resume --last` and do not provide a new prompt, the wrapper still does not need any saved wrapper state. The first resumed turn can be fully manual; `codexa` will bootstrap from that live turn after it finishes.

## 6. Interaction during idle pauses

While the real `codex` child is running, stdin remains Codex’s own stdin, including any native queued steer behavior the upstream CLI supports.

After each turn, the wrapper prints a summary. During the pause window:

- do nothing: the wrapper auto-continues unless the last reply explicitly ended with `AUTO_MODE_NEXT=stop`
- press Enter: return control to the idle Codex child
- type a line and press Enter: send that line to the idle Codex child as the next user prompt
- press `Ctrl+C`: send an interrupt to the idle Codex child

## 7. Configuration

Environment overrides:

```bash
export CODEX_AUTOPILOT_PAUSE_SECONDS=8
export CODEX_AUTOPILOT_REAL_BIN=/opt/homebrew/bin/codex
```

## 8. Troubleshooting

- If the wrapper passes a command straight through instead of entering autopilot mode, use one of the supported prompt or `exec` forms above.
- If a session appears to behave like plain `codex`, confirm you launched `codexa`, not the upstream `codex`. Wrapper-generated turns should end with `AUTO_MODE_NEXT=continue|stop`, but user-driven turns can still be recovered through the session transcript fallback.
- If `codexa --yolo resume`, `codexa --yolo resume --last`, or `codexa --yolo resume <session-id>` is pointed at a native Codex session, give the resumed session one real prompt first if needed. The wrapper will bootstrap from that completed turn.
- If the wrapper cannot resolve the real Codex binary, set `CODEX_AUTOPILOT_REAL_BIN`.

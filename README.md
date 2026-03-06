# Codex Hybrid Autopilot

`codex-hybrid-autopilot` is a Go wrapper around the official `codex` CLI. It keeps the upstream Codex binary unpatched, proxies normal CLI invocations straight through when autopilot is not applicable, and turns the interactive root CLI into an orchestrated session loop when autopilot is active.

The wrapper is designed for this workflow:

- visible Codex output in the same terminal
- argument forwarding shaped like the original interactive `codex` CLI
- PTY-backed interactive child sessions for root prompt and root `resume`
- automatic continuation inside the same live interactive child
- Codex itself retains stdin steer and queued-input behavior while a turn is running
- light wrapper intervention only after a turn finishes
- no repo-local wrapper cache or state directory
- one-line stop or continue marker only

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
4. Let the wrapper continue from the live Codex session only. The repo itself is not used as a wrapper cache.

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
bin/codexa --yolo resume
bin/codexa --yolo --search "Continue the highest-leverage work until no concrete task remains."
bin/codexa --yolo resume --last
bin/codexa --yolo resume 019cc422-dc94-7553-a6e9-acfc3d0e183b
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
- root resume picker form: `codexa [root codex args] resume [resume flags]`
- root resume form: `codexa [root codex args] resume --last [prompt]`
- root resume explicit-session form: `codexa [root codex args] resume <session-id> [prompt]`
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
2. The wrapper forces `--no-alt-screen` for its live interactive child so the PTY stream is scrollback-friendly and machine-readable enough to capture the final footer line.
3. It quotes the last assistant response into the next prompt, asks Codex to identify new tasks, merge and reweight them against current TODOs, and execute the highest-leverage task.
4. For wrapper-generated turns, it prefers the assistant footer `AUTO_MODE_NEXT=continue|stop` or compatibility `AUTO_CONTINUE_MODE=continue|stop` as the fast-path completion signal.
5. For user-driven turns that omit the footer, it falls back to the upstream Codex session transcript to recover the last reply and then defaults to continuing unless the footer explicitly said `stop`.
6. Once the turn is over, the wrapper pauses briefly. If you do nothing it sends one synthetic continuation prompt into the same live session. If you press Enter, it hands control back to the idle Codex child. If you press `Ctrl+C`, it forwards an interrupt to the idle child, matching native Codex exit behavior more closely.
6. It does not parse JSON reports or execute wrapper-side post-turn actions.

When you start with `codexa --yolo resume --last` and omit a fresh prompt, the wrapper still does not depend on any saved wrapper state. The first resumed turn can be completely manual; the wrapper will bootstrap from the live session once that turn finishes.

For `exec` and `exec resume`, the wrapper keeps using non-interactive Codex commands and `-o/--output-last-message` capture as before.

## Interaction model

While the real `codex` child is running, stdin belongs to Codex. If native Codex supports queued steer/input while it is working, `codexa` does not interfere with that.

After a turn finishes and the wrapper is idle:

- do nothing: the wrapper auto-continues unless the last reply ended with `AUTO_MODE_NEXT=stop`
- press Enter: return control to the idle Codex child
- type a line and press Enter: hand that line to the idle Codex child as the next user prompt
- press `Ctrl+C`: send an interrupt to the idle Codex child

## Observability

The authoritative runtime state is the live terminal transcript plus the upstream Codex session itself. `codexa` does not create a repo-local `.codex-autopilot/` directory anymore.

## Configuration

Environment overrides:

- `CODEX_AUTOPILOT_REAL_BIN`
- `CODEX_AUTOPILOT_PAUSE_SECONDS`

## Skill

The repo includes an optional companion skill under [skills/codex-session-autopilot](/Users/zongbaolu/work/codex-hybrid-autopilot/skills/codex-session-autopilot). The Go wrapper does not require it, but the skill makes the same protocol available in manual Codex sessions.

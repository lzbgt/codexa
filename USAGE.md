# Usage

This guide shows the normal way to run the Go wrapper against a target repository.

## 1. Build the wrapper

```bash
cd /Users/zongbaolu/work/codex-hybrid-autopilot
go build -o bin/codex-hybrid-autopilot ./cmd/codex-hybrid-autopilot
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

Use a plain prompt if you want the wrapper to convert that into a turn-based `codex exec` flow:

```bash
cd /path/to/target/repo
/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codex-hybrid-autopilot \
  -p yolo \
  --search \
  "Continue the highest-leverage engineering work until no concrete task remains."
```

You can also start from explicit `exec` or `resume` forms:

```bash
/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codex-hybrid-autopilot \
  exec \
  "Fix the top failing test and keep going."

/Users/zongbaolu/work/codex-hybrid-autopilot/bin/codex-hybrid-autopilot \
  exec resume --last \
  "Continue from the current repo state."
```

## 4. Understand what the wrapper intercepts

Autopilot interception applies to:

- root prompt form
- `exec "prompt"`
- `resume --last "prompt"`
- `exec resume --last "prompt"`

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

- `.codex-autopilot/session_state.json`
- `.codex-autopilot/prompts/`
- `.codex-autopilot/messages/`
- `.codex-autopilot/reports/`
- `.codex-autopilot/action-logs/`

This is where you inspect the previous turn prompt, the final assistant message, and the parsed JSON report.

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
  "max_turns": 30,
  "pause_window_seconds": 8,
  "skill_hint": true,
  "real_codex_bin": "/opt/homebrew/bin/codex"
}
```

Equivalent environment overrides:

```bash
export CODEX_AUTOPILOT_MAX_TURNS=30
export CODEX_AUTOPILOT_PAUSE_SECONDS=8
export CODEX_AUTOPILOT_REAL_BIN=/opt/homebrew/bin/codex
```

## 9. Troubleshooting

- If the wrapper passes a command straight through instead of entering autopilot mode, use one of the supported prompt or `exec` forms above.
- If the wrapper cannot resolve the real Codex binary, set `CODEX_AUTOPILOT_REAL_BIN`.
- If the wrapper stops because the repo is still dirty, inspect `.codex-autopilot/reports/turn-XXXX.json` and check whether Codex emitted the expected `post_turn_actions`.

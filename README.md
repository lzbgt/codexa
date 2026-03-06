# Codex Hybrid Autopilot

`codex-hybrid-autopilot` is a thin wrapper around the official `codex` CLI. It keeps Codex itself unmodified, runs with the official `yolo` profile by default, shows the normal Codex output directly in your terminal, and adds a structured continuation loop between turns.

The wrapper is designed for the workflow discussed in this session:

- visible Codex output instead of a hidden background daemon
- hybrid execution using the official binary
- automatic continuation when concrete work remains
- occasional operator engagement by queueing free-form user prompts for the next turn
- repo-local observability via saved prompts, last messages, and parsed reports

## How it works

1. The wrapper starts Codex with `codex -p yolo exec ...`.
2. It writes the turn prompt to `.codex-autopilot/prompts/`.
3. It asks Codex to write the final assistant message to `.codex-autopilot/messages/`.
4. It parses a machine-readable JSON block from the final assistant message.
5. It decides whether to continue automatically or pause for operator input.
6. In `hybrid` mode it resumes the prior session with `codex exec resume --last`; in `stateless` mode it starts a fresh `exec` turn every time.

This avoids patching the Codex client while still giving a loop that is observable and restartable.

## Install

```bash
cd /Users/zongbaolu/work/codex-hybrid-autopilot
python3 -m pip install -e .
```

## Run

```bash
codex-hybrid-autopilot \
  --workspace /path/to/target/repo \
  --objective "Continue the highest-leverage engineering work until no concrete task remains." \
  --codex-root-arg --search
```

Important defaults:

- profile: `yolo`
- strategy: `hybrid`
- pause window after each turn: `10s`
- state directory: `<workspace>/.codex-autopilot`

## Operator engagement

After each turn, the wrapper shows a short summary. If no input arrives during the pause window, it follows the agent's `continue` or `stop` decision. If you type anything, the wrapper enters operator input mode.

Operator input mode supports:

- plain text: queue a user prompt for the next turn
- `/show`: show queued prompts and pending tasks
- `/clear`: clear queued prompts
- `/stop`: stop after the current turn
- `/help`: show the command list

Press `Enter` on an empty line to continue with the queued prompts.

## Observability

For each target workspace, the wrapper stores:

- `.codex-autopilot/session_state.json`
- `.codex-autopilot/prompts/turn-XXXX.md`
- `.codex-autopilot/messages/turn-XXXX.md`
- `.codex-autopilot/reports/turn-XXXX.json`

This gives a stable record of what was asked, what Codex answered, and why the wrapper continued or stopped.

## Protocol

The wrapper requires the final assistant message to include:

```text
AUTO_REPORT_JSON_BEGIN
{ ...json... }
AUTO_REPORT_JSON_END
```

The JSON includes:

- `auto_mode_next`
- `summary`
- `recommended_next_prompt`
- `user_engagement_needed`
- `pending_tasks`
- `discovered_tasks`
- `reweighting_rationale`
- `verification`

The optional companion skill in [skills/codex-session-autopilot](/Users/zongbaolu/work/codex-hybrid-autopilot/skills/codex-session-autopilot) documents the same contract for manual Codex sessions.

## Limitations

- In `hybrid` mode, session continuation uses `codex exec resume --last` unless you provide `--session-id`. Avoid running unrelated Codex sessions in the same workspace concurrently.
- The wrapper is intentionally conservative: if the machine-readable report is missing or malformed, it stops instead of guessing.
- Operator prompts are queued between turns, not injected while Codex is actively running.

## Skill

The repo includes an optional skill scaffold under [skills/codex-session-autopilot](/Users/zongbaolu/work/codex-hybrid-autopilot/skills/codex-session-autopilot). You can install or copy it into your Codex skills directory if you want the same continuation protocol available in normal manual sessions.


---
name: codex-session-autopilot
description: Structured continuation protocol for multi-turn Codex runs that are supervised by an external wrapper or operator. Use when Codex must keep working across turns, reweight remaining tasks from repo evidence and queued operator prompts, and emit a machine-readable continue/stop report at the end of every turn.
---

# Codex Session Autopilot

## Overview

Follow a strict continuation protocol so an external wrapper can decide whether to resume the session automatically or pause for operator input. Keep task selection grounded in explicit user prompts, repo state, and concrete verification results.

## Workflow

1. Read the latest explicit user request and queued operator prompts first.
2. Reweight the remaining concrete tasks using repo evidence, verification failures, and documented plans.
3. Execute the highest-leverage concrete task instead of only restating a plan.
4. Verify the changed area before finalizing.
5. Append the required machine-readable footer described in `references/protocol.md`.

## Task Selection Rules

- Prefer tasks backed by failing verification, explicit TODO items, or new operator prompts.
- Do not invent speculative follow-up work.
- Stop only when no concrete task remains or operator input is genuinely required.
- If priorities changed, explain why in the structured footer.

## Footer Contract

Read [references/protocol.md](references/protocol.md) and follow it exactly. The wrapper depends on the exact `AUTO_REPORT_JSON_BEGIN` and `AUTO_REPORT_JSON_END` markers.

## Reporting Guidance

- Keep the human-readable summary concise.
- Keep `pending_tasks` short and concrete.
- Use `user_engagement_needed: true` when the wrapper should pause for operator attention.
- Always include verification status, even if verification could not run.

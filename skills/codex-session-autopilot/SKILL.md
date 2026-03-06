---
name: codex-session-autopilot
description: Structured continuation protocol for multi-turn Codex runs that are supervised by an external wrapper or operator. Use when Codex must keep working across turns, reweight remaining tasks from repo evidence, and emit a machine-readable continue or stop marker at the end of every turn.
---

# Codex Session Autopilot

## Overview

Follow a strict continuation protocol so an external wrapper can decide whether to resume the session automatically or hand control back to the live Codex session. Keep task selection grounded in explicit user prompts, repo state, and concrete verification results.

## Workflow

1. Read the latest explicit user request first.
2. Reweight the remaining concrete tasks using repo evidence, verification failures, and documented plans.
3. Execute the highest-leverage concrete task instead of only restating a plan.
4. If the turn leaves source-code changes dirty, handle verification and finalization within the turn when appropriate.
5. Append the required machine-readable footer described in `references/protocol.md`.

## Task Selection Rules

- Prefer tasks backed by failing verification, explicit TODO items, or newly discovered concrete blockers.
- Do not invent speculative follow-up work.
- Stop only when no concrete task remains or operator input is genuinely required.
- If priorities changed, explain why in the structured footer.
- Prefer the upstream remote over origin when both exist.

## Footer Contract

Read [references/protocol.md](references/protocol.md) and follow it exactly. The wrapper depends on the exact stop or continue marker.

## Reporting Guidance

- Keep the human-readable summary concise.
- Keep the human-readable summary concise and concrete.
- If operator input is genuinely required, say so in the human-readable summary before the final stop or continue marker.

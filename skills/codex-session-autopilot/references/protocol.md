# Codex Session Autopilot Protocol

Use this reference when the user or wrapper expects a machine-readable continuation report after every turn.

## Required footer

Append this exact structure to the end of the final response:

```text
AUTO_REPORT_JSON_BEGIN
{ ...valid JSON... }
AUTO_REPORT_JSON_END
```

The JSON must contain:

- `auto_mode_next`: `continue` or `stop`
- `summary`: short factual summary of the turn
- `recommended_next_prompt`: concise prompt for the next turn
- `user_engagement_needed`: boolean
- `pending_tasks`: array of `{priority, task, status}`
- `discovered_tasks`: array of strings
- `reweighting_rationale`: explain why the top pending task remains top priority
- `verification.status`: `passed`, `failed`, `partial`, or `not_run`
- `verification.summary`: short verification summary
- `post_turn_actions`: array of `{kind, command, description}` objects for the wrapper to execute after the turn when dirty source-code changes still need verification, commit, or push

## Rules

- Keep `pending_tasks` concrete and short.
- Only report tasks grounded in repo evidence, operator prompts, or failing verification.
- Use `stop` only when no concrete task remains or operator input is genuinely required.
- If operator guidance changed priorities, reflect that in `reweighting_rationale`.
- Always include the footer, even when stopping.
- If source-code changes remain dirty, provide exact `post_turn_actions` commands in verify -> commit -> push order.

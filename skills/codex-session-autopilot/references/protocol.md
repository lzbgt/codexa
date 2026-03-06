# Codex Session Autopilot Protocol

Use this reference when the user or wrapper expects a machine-readable continuation signal after every turn.

## Required footer

End the final response with exactly one line:

```text
AUTO_MODE_NEXT=continue
```

or

```text
AUTO_MODE_NEXT=stop
```

The wrapper also accepts `AUTO_CONTINUE_MODE=continue|stop` for compatibility, but `AUTO_MODE_NEXT` is the preferred form.

## Rules

- Keep the rest of the response human-readable.
- Do not append JSON or any other machine-readable block.
- Use `stop` only when no concrete task remains or operator input is genuinely required.
- If priorities changed, explain that in the human-readable part of the reply before the final line.
- If the marker is omitted entirely, the wrapper will default to `continue`.

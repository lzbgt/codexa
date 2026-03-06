package autopilot

import (
	"fmt"
	"strings"
)

func buildPrompt(state *State, snapshot GitSnapshot, skillHint bool) string {
	var b strings.Builder
	if skillHint {
		b.WriteString("Use $codex-session-autopilot if it is installed; otherwise follow the autopilot protocol below exactly.\n\n")
	}
	fmt.Fprintf(&b, "Wrapper mode:\n- Strategy: %s\n- Workspace: %s\n- An external wrapper is orchestrating turn boundaries.\n- Keep tasks grounded in the current repo state, TODO documents, the quoted last assistant response, and concrete verification results.\n- Prefer high-leverage tasks that increase throughput per turn.\n\n", state.Strategy, state.Workspace)
	fmt.Fprintf(&b, "Session objective: %s\nTurn index: %d\n\n", state.InitialPrompt, state.TurnIndex)
	fmt.Fprintf(&b, "Git context before this turn:\n- In git repo: %t\n- Dirty worktree: %t\n- Current branch: %s\n- Has upstream remote: %t\n- Has origin remote: %t\n\n", snapshot.IsRepo, snapshot.Dirty, snapshot.Branch, snapshot.HasUpstream, snapshot.HasOrigin)
	if strings.TrimSpace(state.LastAssistantMessage) == "" {
		b.WriteString("Initial turn requirements:\n- Reweight the repo's concrete tasks from the current code and docs.\n- Execute the highest-leverage concrete task instead of only restating a plan.\n- Verify the changed area before finalizing.\n\n")
	} else {
		b.WriteString("Quoted last assistant response:\n")
		b.WriteString("<<<LAST_ASSISTANT_RESPONSE\n")
		b.WriteString(strings.TrimSpace(state.LastAssistantMessage))
		b.WriteString("\nLAST_ASSISTANT_RESPONSE>>>\n\n")
	}
	b.WriteString("Queued operator prompts for this turn:\n")
	if len(state.PendingUserPrompts) == 0 {
		b.WriteString("- None queued by the operator.\n\n")
	} else {
		for index, prompt := range state.PendingUserPrompts {
			fmt.Fprintf(&b, "- Operator prompt %d: %s\n", index+1, prompt)
		}
		b.WriteString("\n")
	}
	b.WriteString("Execution requirements:\n")
	b.WriteString("- Use the most recent context and proceed without waiting for user input.\n")
	b.WriteString("- Priority order:\n")
	b.WriteString("  1. If the most recent user message contains explicit tasks or questions, execute those first.\n")
	b.WriteString("  2. Else if the most recent assistant message ended with choices or options, prefer the best default and take multiple compatible options when that increases leverage.\n")
	b.WriteString("  3. Else pick a batch of high-leverage tasks, usually 2 to 6, that compound and reduce future maintenance.\n")
	b.WriteString("- Start by identifying any new tasks or blockers from the quoted last assistant response.\n")
	b.WriteString("- Merge and reweight those findings against the current repo TODOs / task documents before deciding what to do next.\n")
	b.WriteString("- Reweight tasks using the operator prompts first, then the remaining concrete blockers.\n")
	b.WriteString("- Feature-delivering work comes before maintenance unless maintenance unblocks features or mitigates P0/P1 risks.\n")
	b.WriteString("- Use multiple plans within the turn: macro plan first, then micro steps, and keep executing without stopping after the first completed item.\n")
	b.WriteString("- Prefer fundamental fixes over ad-hoc tweaks. Keep boundaries clean, reduce coupling, and add tests that lock in behavior.\n")
	b.WriteString("- Keep documentation and implementation in sync when behavior, config, workflows, or examples change.\n")
	b.WriteString("- Maintain a succinct task tracker if one exists: add newly discovered tasks and reweight them.\n")
	b.WriteString("- Keep the workspace lean, but do not delete useful caches or artifacts by default.\n")
	b.WriteString("- Execute the highest-leverage concrete task for this turn.\n")
	b.WriteString("- If the repo is dirty with source-code changes at the end of the turn, handle verification, `git diff --stat`, commit, and push within the turn when appropriate.\n")
	b.WriteString("- Prefer the upstream remote over origin when both exist.\n")
	b.WriteString("- Ask exactly one tight clarifying question only when correctness, data loss, security, or long-term architecture would otherwise be materially affected.\n")
	b.WriteString("- Stop only when no concrete task remains or operator input is genuinely required.\n\n")
	b.WriteString(protocolInstructions())
	b.WriteString("\n")
	return b.String()
}

package autopilot

import (
	"fmt"
	"strings"
)

func buildPrompt(state *State, report *AutoReport, snapshot GitSnapshot, skillHint bool) string {
	var b strings.Builder
	if skillHint {
		b.WriteString("Use $codex-session-autopilot if it is installed; otherwise follow the autopilot protocol below exactly.\n\n")
	}
	fmt.Fprintf(&b, "Wrapper mode:\n- Strategy: %s\n- Workspace: %s\n- An external wrapper is orchestrating turn boundaries and executing any post_turn_actions you request.\n- Keep tasks grounded in repo state, operator prompts, and concrete verification results.\n- Prefer high-leverage tasks that increase throughput per turn.\n\n", state.Strategy, state.Workspace)
	fmt.Fprintf(&b, "Session objective: %s\nTurn index: %d\n\n", state.InitialPrompt, state.TurnIndex)
	fmt.Fprintf(&b, "Git context before this turn:\n- In git repo: %t\n- Dirty worktree: %t\n- Current branch: %s\n- Has upstream remote: %t\n- Has origin remote: %t\n\n", snapshot.IsRepo, snapshot.Dirty, snapshot.Branch, snapshot.HasUpstream, snapshot.HasOrigin)
	if report == nil {
		b.WriteString("Initial turn requirements:\n- Reweight the repo's concrete tasks from the current code and docs.\n- Execute the highest-leverage concrete task instead of only restating a plan.\n- Verify the changed area before finalizing.\n\n")
	} else {
		fmt.Fprintf(&b, "Previous turn summary:\n- Summary: %s\n- Verification: %s\n- Reweighting rationale: %s\n- Recommended next prompt: %s\n\n", report.Summary, report.Verification.Summary, report.ReweightingRationale, report.RecommendedNextPrompt)
		b.WriteString("Pending tasks from the previous turn:\n")
		if len(report.PendingTasks) == 0 {
			b.WriteString("- None reported.\n")
		}
		for _, task := range report.PendingTasks {
			fmt.Fprintf(&b, "- %s [%s] %s\n", task.Priority, task.Status, task.Task)
		}
		b.WriteString("\nDiscovered tasks from the previous turn:\n")
		if len(report.DiscoveredTasks) == 0 {
			b.WriteString("- None.\n")
		}
		for _, task := range report.DiscoveredTasks {
			fmt.Fprintf(&b, "- %s\n", task)
		}
		b.WriteString("\n")
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
	b.WriteString("- Reweight tasks using the operator prompts first, then the remaining concrete blockers.\n")
	b.WriteString("- Execute the highest-leverage concrete task for this turn.\n")
	b.WriteString("- If the repo is dirty with source-code changes at the end of the turn, either finish verification, commit, and push during the turn or provide exact post_turn_actions for the wrapper to execute.\n")
	b.WriteString("- Prefer the upstream remote over origin when both exist.\n")
	b.WriteString("- Stop only when no concrete task remains or operator input is genuinely required.\n\n")
	b.WriteString(protocolInstructions())
	b.WriteString("\n")
	return b.String()
}

package autopilot

import (
	"strings"
	"testing"
)

func TestBuildPromptIncludesProtocol(t *testing.T) {
	state := &State{
		Workspace:          "/tmp/repo",
		InitialPrompt:      "Fix the parser.",
		Strategy:           "hybrid",
		TurnIndex:          1,
		PendingUserPrompts: []string{"Prioritize flaky test coverage."},
	}
	prompt := buildPrompt(state, nil, GitSnapshot{IsRepo: true, Branch: "main"}, true)
	if !strings.Contains(prompt, "Prioritize flaky test coverage.") {
		t.Fatalf("prompt did not include queued operator prompt")
	}
	if !strings.Contains(prompt, "post_turn_actions") {
		t.Fatalf("prompt did not include post_turn_actions contract")
	}
}

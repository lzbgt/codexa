package autopilot

import (
	"strings"
	"testing"
)

func TestBuildPromptIncludesProtocol(t *testing.T) {
	state := &State{
		Workspace:            "/tmp/repo",
		InitialPrompt:        "Fix the parser.",
		Strategy:             "hybrid",
		TurnIndex:            1,
		LastAssistantMessage: "Implemented the parser fix and updated TODOS.",
		PendingUserPrompts:   []string{"Prioritize flaky test coverage."},
	}
	prompt := buildPrompt(state, GitSnapshot{IsRepo: true, Branch: "main"}, true)
	if !strings.Contains(prompt, "Prioritize flaky test coverage.") {
		t.Fatalf("prompt did not include queued operator prompt")
	}
	if !strings.Contains(prompt, "post_turn_actions") {
		t.Fatalf("prompt did not include post_turn_actions contract")
	}
	if !strings.Contains(prompt, "LAST_ASSISTANT_RESPONSE") || !strings.Contains(prompt, "Implemented the parser fix and updated TODOS.") {
		t.Fatalf("prompt did not include the quoted last assistant response")
	}
}

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
	}
	prompt := buildPrompt(state, GitSnapshot{IsRepo: true, Branch: "main"}, true)
	if strings.Contains(prompt, "Queued operator prompts") {
		t.Fatalf("prompt should not include wrapper-side operator queue text")
	}
	if strings.Contains(prompt, "post_turn_actions") {
		t.Fatalf("prompt should not include post_turn_actions contract anymore")
	}
	if !strings.Contains(prompt, "LAST_ASSISTANT_RESPONSE") || !strings.Contains(prompt, "Implemented the parser fix and updated TODOS.") {
		t.Fatalf("prompt did not include the quoted last assistant response")
	}
}

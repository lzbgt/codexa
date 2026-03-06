package autopilot

import (
	"strings"
)

type State struct {
	Workspace            string
	InitialPrompt        string
	Strategy             string
	TurnIndex            int
	SessionStarted       bool
	ExplicitSessionID    string
	LastSessionID        string
	LastSessionPath      string
	LastAssistantMessage string
	LastReport           *AutoReport
}

func newState(workspace, prompt, strategy, explicitSessionID string) *State {
	return &State{
		Workspace:         strings.TrimSpace(workspace),
		InitialPrompt:     strings.TrimSpace(prompt),
		Strategy:          strings.TrimSpace(strategy),
		ExplicitSessionID: strings.TrimSpace(explicitSessionID),
	}
}

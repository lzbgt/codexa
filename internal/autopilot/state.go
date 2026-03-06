package autopilot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Workspace            string      `json:"workspace"`
	InitialPrompt        string      `json:"initial_prompt"`
	Strategy             string      `json:"strategy"`
	CreatedAt            string      `json:"created_at"`
	UpdatedAt            string      `json:"updated_at"`
	TurnIndex            int         `json:"turn_index"`
	SessionStarted       bool        `json:"session_started"`
	ExplicitSessionID    string      `json:"explicit_session_id,omitempty"`
	LastSessionID        string      `json:"last_session_id,omitempty"`
	LastSessionPath      string      `json:"last_session_path,omitempty"`
	LastAssistantMessage string      `json:"last_assistant_message,omitempty"`
	PendingUserPrompts   []string    `json:"pending_user_prompts"`
	LastReport           *AutoReport `json:"last_report,omitempty"`
	LastMessagePath      string      `json:"last_message_path,omitempty"`
	LastPromptPath       string      `json:"last_prompt_path,omitempty"`
}

type StateDirs struct {
	Base       string
	Prompts    string
	Messages   string
	Reports    string
	ActionLogs string
}

func ensureStateDirs(base string) (StateDirs, error) {
	dirs := StateDirs{
		Base:       base,
		Prompts:    filepath.Join(base, "prompts"),
		Messages:   filepath.Join(base, "messages"),
		Reports:    filepath.Join(base, "reports"),
		ActionLogs: filepath.Join(base, "action-logs"),
	}
	for _, dir := range []string{dirs.Base, dirs.Prompts, dirs.Messages, dirs.Reports, dirs.ActionLogs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return StateDirs{}, err
		}
	}
	return dirs, nil
}

func newState(workspace, prompt, strategy, explicitSessionID string) *State {
	now := time.Now().UTC().Format(time.RFC3339)
	return &State{
		Workspace:          workspace,
		InitialPrompt:      prompt,
		Strategy:           strategy,
		CreatedAt:          now,
		UpdatedAt:          now,
		ExplicitSessionID:  explicitSessionID,
		PendingUserPrompts: []string{},
	}
}

func loadOrCreateState(path, workspace, prompt, strategy, explicitSessionID string) (*State, error) {
	state, err := loadStateIfExists(path)
	if err != nil {
		return nil, err
	}
	if state != nil {
		if prompt != "" && prompt != state.InitialPrompt {
			return newState(workspace, prompt, strategy, explicitSessionID), nil
		}
		if strategy != "" {
			state.Strategy = strategy
		}
		if explicitSessionID != "" {
			state.ExplicitSessionID = explicitSessionID
			state.LastSessionID = explicitSessionID
		}
		return state, nil
	}
	if prompt == "" {
		return nil, errors.New("an initial project goal is required when no prior autopilot state exists; start with codexa --yolo \"<goal>\" before using resume-only autopilot flows")
	}
	state = newState(workspace, prompt, strategy, explicitSessionID)
	if explicitSessionID != "" {
		state.LastSessionID = explicitSessionID
	}
	return state, nil
}

func loadStateIfExists(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var state State
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, err
		}
		return &state, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return nil, err
}

func (s *State) save(path string) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

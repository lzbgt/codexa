package autopilot

import (
	"path/filepath"
	"time"
)

type RuntimeStatus struct {
	WrapperActive     bool     `json:"wrapper_active"`
	WrapperPID        int      `json:"wrapper_pid"`
	WrapperStartedAt  string   `json:"wrapper_started_at"`
	UpdatedAt         string   `json:"updated_at"`
	Workspace         string   `json:"workspace"`
	StateDir          string   `json:"state_dir"`
	Mode              string   `json:"mode"`
	RealCodex         string   `json:"real_codex"`
	OriginalArgs      []string `json:"original_args"`
	ForwardArgs       []string `json:"forward_args"`
	RootArgs          []string `json:"root_args"`
	ResumeTarget      string   `json:"resume_target,omitempty"`
	ExplicitSessionID string   `json:"explicit_session_id,omitempty"`
	PromptSupplied    bool     `json:"prompt_supplied"`
	StateBootstrapped bool     `json:"state_bootstrapped"`
	CurrentPhase      string   `json:"current_phase"`
	LastTurnIndex     int      `json:"last_turn_index,omitempty"`
	LastSessionID     string   `json:"last_session_id,omitempty"`
	LastSessionPath   string   `json:"last_session_path,omitempty"`
	LastDecision      string   `json:"last_decision,omitempty"`
	Note              string   `json:"note,omitempty"`
}

func newRuntimeStatus(inv Invocation, stateDir, realCodex string) *RuntimeStatus {
	now := time.Now().UTC().Format(time.RFC3339)
	return &RuntimeStatus{
		WrapperActive:     true,
		WrapperPID:        currentProcessID(),
		WrapperStartedAt:  now,
		UpdatedAt:         now,
		Workspace:         inv.Workspace,
		StateDir:          stateDir,
		Mode:              string(inv.Mode),
		RealCodex:         realCodex,
		OriginalArgs:      append([]string{}, inv.OriginalArgs...),
		ForwardArgs:       append([]string{}, inv.ForwardArgs...),
		RootArgs:          append([]string{}, inv.RootArgs...),
		ResumeTarget:      inv.ResumeTarget,
		ExplicitSessionID: inv.ExplicitSessionID,
		PromptSupplied:    inv.Prompt != "",
		CurrentPhase:      "starting",
	}
}

func (s *RuntimeStatus) save(path string) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSON(path, s)
}

func (s *RuntimeStatus) setPhase(phase, note string) {
	s.CurrentPhase = phase
	s.Note = note
}

func (s *RuntimeStatus) trackState(state *State) {
	if state == nil {
		return
	}
	s.StateBootstrapped = true
	s.LastTurnIndex = state.TurnIndex
	if state.LastSessionID != "" {
		s.LastSessionID = state.LastSessionID
	}
	if state.LastSessionPath != "" {
		s.LastSessionPath = state.LastSessionPath
	}
}

func runtimeStatusPath(dirs StateDirs) string {
	return filepath.Join(dirs.Base, "runtime.json")
}

func currentProcessID() int {
	return processID()
}

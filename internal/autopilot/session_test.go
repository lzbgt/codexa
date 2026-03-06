package autopilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractLastAgentMessagePrefersTaskComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"session_meta","payload":{"id":"session-1","cwd":"/tmp/repo"}}
{"type":"event_msg","payload":{"type":"agent_message","message":"intermediate reply"}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"assistant response item"}]}}
{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"final assistant message"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	message, err := extractLastAgentMessage(path)
	if err != nil {
		t.Fatalf("extractLastAgentMessage returned error: %v", err)
	}
	if message != "final assistant message" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func TestReadSessionMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"session_meta","payload":{"id":"session-1","cwd":"/tmp/repo"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	sessionID, cwd, err := readSessionMeta(path)
	if err != nil {
		t.Fatalf("readSessionMeta returned error: %v", err)
	}
	if sessionID != "session-1" || cwd != "/tmp/repo" {
		t.Fatalf("unexpected meta: %q %q", sessionID, cwd)
	}
}

func TestExtractInitialUserGoalSkipsHarnessNoise(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"session_meta","payload":{"id":"session-1","cwd":"/tmp/repo"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions for /tmp/repo"}]}}
{"type":"event_msg","payload":{"type":"user_message","message":"review the project status and continue until no concrete task remains"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	goal, err := extractInitialUserGoal(path)
	if err != nil {
		t.Fatalf("extractInitialUserGoal returned error: %v", err)
	}
	if goal != "review the project status and continue until no concrete task remains" {
		t.Fatalf("unexpected goal: %q", goal)
	}
}

func TestExtractBootstrapUserGoalUsesOnlyNewMessages(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"timestamp":"2026-03-06T17:00:00Z","type":"session_meta","payload":{"id":"session-1","cwd":"/tmp/repo"}}
{"timestamp":"2026-03-06T17:00:01Z","type":"event_msg","payload":{"type":"user_message","message":"old session goal"}}
{"timestamp":"2026-03-06T17:10:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# AGENTS.md instructions for /tmp/repo"}]}}
{"timestamp":"2026-03-06T17:10:01Z","type":"event_msg","payload":{"type":"user_message","message":"continue the CI hardening work and keep going automatically"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	since := time.Date(2026, 3, 6, 17, 9, 59, 0, time.UTC)
	goal, err := extractBootstrapUserGoal(path, since)
	if err != nil {
		t.Fatalf("extractBootstrapUserGoal returned error: %v", err)
	}
	if goal != "continue the CI hardening work and keep going automatically" {
		t.Fatalf("unexpected bootstrap goal: %q", goal)
	}
}

func TestSelectSessionCandidatePrefersSingleChangedSession(t *testing.T) {
	now := time.Date(2026, 3, 7, 2, 0, 0, 0, time.UTC)
	candidates := []sessionCandidate{
		{
			SessionID: "session-a",
			Path:      "/tmp/a.jsonl",
			ModTime:   now.Add(2 * time.Second),
		},
		{
			SessionID: "session-b",
			Path:      "/tmp/b.jsonl",
			ModTime:   now.Add(3 * time.Second),
		},
	}
	before := sessionInventory{
		"/tmp/a.jsonl": {SessionID: "session-a", ModTime: now.Add(2 * time.Second)},
	}
	path, sessionID, err := selectSessionCandidate(candidates, before, now, "")
	if err != nil {
		t.Fatalf("selectSessionCandidate returned error: %v", err)
	}
	if path != "/tmp/b.jsonl" || sessionID != "session-b" {
		t.Fatalf("unexpected selection: %q %q", path, sessionID)
	}
}

func TestSelectSessionCandidateRejectsAmbiguousChangedSessions(t *testing.T) {
	now := time.Date(2026, 3, 7, 2, 0, 0, 0, time.UTC)
	candidates := []sessionCandidate{
		{
			SessionID: "session-a",
			Path:      "/tmp/a.jsonl",
			ModTime:   now.Add(2 * time.Second),
		},
		{
			SessionID: "session-b",
			Path:      "/tmp/b.jsonl",
			ModTime:   now.Add(3 * time.Second),
		},
	}
	before := sessionInventory{
		"/tmp/a.jsonl": {SessionID: "session-a", ModTime: now},
		"/tmp/b.jsonl": {SessionID: "session-b", ModTime: now},
	}
	_, _, err := selectSessionCandidate(candidates, before, now, "")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to guess") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSelectSessionCandidateUsesWantedSessionID(t *testing.T) {
	now := time.Date(2026, 3, 7, 2, 0, 0, 0, time.UTC)
	candidates := []sessionCandidate{
		{
			SessionID: "session-a",
			Path:      "/tmp/a.jsonl",
			ModTime:   now,
		},
		{
			SessionID: "session-b",
			Path:      "/tmp/b.jsonl",
			ModTime:   now.Add(5 * time.Second),
		},
	}
	path, sessionID, err := selectSessionCandidate(candidates, nil, now, "session-a")
	if err != nil {
		t.Fatalf("selectSessionCandidate returned error: %v", err)
	}
	if path != "/tmp/a.jsonl" || sessionID != "session-a" {
		t.Fatalf("unexpected selection: %q %q", path, sessionID)
	}
}

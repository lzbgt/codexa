package autopilot

import (
	"os"
	"path/filepath"
	"testing"
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

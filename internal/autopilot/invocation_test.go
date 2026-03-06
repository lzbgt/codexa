package autopilot

import "testing"

func TestParseRootPromptInvocation(t *testing.T) {
	inv, err := parseInvocation([]string{"-p", "yolo", "--search", "fix", "the", "tests"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeExec {
		t.Fatalf("expected exec mode, got %s", inv.Mode)
	}
	if inv.Prompt != "fix the tests" {
		t.Fatalf("unexpected prompt: %q", inv.Prompt)
	}
}

func TestParseExecResumeLast(t *testing.T) {
	inv, err := parseInvocation([]string{"exec", "resume", "--last", "continue", "work"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeResume {
		t.Fatalf("expected resume mode, got %s", inv.Mode)
	}
	if inv.ResumeTarget != "--last" {
		t.Fatalf("unexpected resume target: %q", inv.ResumeTarget)
	}
}

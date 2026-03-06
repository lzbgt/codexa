package autopilot

import "testing"

func TestParseRootPromptInvocation(t *testing.T) {
	inv, err := parseInvocation([]string{"-p", "yolo", "--search", "fix", "the", "tests"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeInteractive {
		t.Fatalf("expected interactive mode, got %s", inv.Mode)
	}
	if inv.Prompt != "fix the tests" {
		t.Fatalf("unexpected prompt: %q", inv.Prompt)
	}
}

func TestParseYoloAliasInvocation(t *testing.T) {
	inv, err := parseInvocation([]string{"--yolo", "--search", "fix", "the", "tests"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeInteractive {
		t.Fatalf("expected interactive mode, got %s", inv.Mode)
	}
	if len(inv.RootArgs) < 2 || inv.RootArgs[0] != "-p" || inv.RootArgs[1] != "yolo" {
		t.Fatalf("expected normalized yolo args, got %#v", inv.RootArgs)
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

func TestParseRootResumeLastWithoutPrompt(t *testing.T) {
	inv, err := parseInvocation([]string{"--yolo", "resume", "--last"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeInteractiveResume {
		t.Fatalf("expected interactive resume mode, got %s", inv.Mode)
	}
	if inv.ResumeTarget != "--last" {
		t.Fatalf("unexpected resume target: %q", inv.ResumeTarget)
	}
	if inv.Prompt != "" {
		t.Fatalf("expected empty prompt, got %q", inv.Prompt)
	}
	if len(inv.ForwardArgs) < 3 || inv.ForwardArgs[0] != "-p" || inv.ForwardArgs[1] != "yolo" {
		t.Fatalf("expected normalized passthrough args, got %#v", inv.ForwardArgs)
	}
}

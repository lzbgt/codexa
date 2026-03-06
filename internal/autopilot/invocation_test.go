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

func TestParseBareInteractiveInvocation(t *testing.T) {
	inv, err := parseInvocation([]string{"--yolo", "--search"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeInteractiveBare {
		t.Fatalf("expected interactive bare mode, got %s", inv.Mode)
	}
	if inv.Prompt != "" {
		t.Fatalf("expected empty prompt, got %q", inv.Prompt)
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

func TestParseRootResumePickerWithoutPrompt(t *testing.T) {
	inv, err := parseInvocation([]string{"--yolo", "resume"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeInteractiveResume {
		t.Fatalf("expected interactive resume mode, got %s", inv.Mode)
	}
	if inv.ResumeTarget != "" {
		t.Fatalf("expected empty resume target, got %q", inv.ResumeTarget)
	}
	if inv.Prompt != "" {
		t.Fatalf("expected empty prompt, got %q", inv.Prompt)
	}
}

func TestParseRootResumeSessionIDWithoutPrompt(t *testing.T) {
	inv, err := parseInvocation([]string{"--yolo", "resume", "019cc422-dc94-7553-a6e9-acfc3d0e183b"}, "/tmp/repo")
	if err != nil {
		t.Fatalf("parseInvocation returned error: %v", err)
	}
	if inv.Mode != modeInteractiveResume {
		t.Fatalf("expected interactive resume mode, got %s", inv.Mode)
	}
	if inv.ExplicitSessionID != "019cc422-dc94-7553-a6e9-acfc3d0e183b" {
		t.Fatalf("unexpected session id: %q", inv.ExplicitSessionID)
	}
	if inv.Prompt != "" {
		t.Fatalf("expected empty prompt, got %q", inv.Prompt)
	}
}

func TestInitialInteractiveArgsForResumePicker(t *testing.T) {
	inv := Invocation{
		Mode:            modeInteractiveResume,
		RootArgs:        []string{"-p", "yolo"},
		InitialExecArgs: []string{"--all"},
	}
	got := inv.initialInteractiveArgs("")
	want := []string{"-p", "yolo", "resume", "--all"}
	if len(got) != len(want) {
		t.Fatalf("unexpected args length: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected arg at %d: got %#v want %#v", i, got, want)
		}
	}
}

func TestInitialCommandArgsForExecResumeDoesNotDuplicateLast(t *testing.T) {
	inv := Invocation{
		Mode:            modeResume,
		InitialExecArgs: []string{"--last"},
		ResumeTarget:    "--last",
	}
	got := inv.initialCommandArgs("/tmp/last.md")
	count := 0
	for _, arg := range got {
		if arg == "--last" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one --last, got %#v", got)
	}
}

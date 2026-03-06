package autopilot

import "testing"

func TestOutputCaptureExtractsTurnMessageFromANSIStream(t *testing.T) {
	capture := newOutputCapture()
	capture.StartTurn()
	capture.Append([]byte("\x1b[19;1H• hello\r\n"))
	capture.Append([]byte("AUTO_MODE_NEXT=stop\r\n"))

	message, ok := capture.ExtractTurnMessage("")
	if !ok {
		t.Fatal("expected marker-driven turn extraction to succeed")
	}
	if message != "hello" {
		t.Fatalf("unexpected extracted message: %q", message)
	}
}

func TestEnsureNoAltScreenPrependsOnlyOnce(t *testing.T) {
	args := ensureNoAltScreen([]string{"-p", "yolo", "resume", "--last"})
	if args[0] != "--no-alt-screen" {
		t.Fatalf("expected --no-alt-screen to be prepended, got %#v", args)
	}

	args = ensureNoAltScreen([]string{"--no-alt-screen", "-p", "yolo"})
	count := 0
	for _, arg := range args {
		if arg == "--no-alt-screen" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one --no-alt-screen flag, got %#v", args)
	}
}

func TestRecordUserInputCapturesFirstPrompt(t *testing.T) {
	bridge := &attachedInteractiveBridge{}
	bridge.recordUserInput([]byte("continue the work\n"))
	bridge.recordUserInput([]byte("second line\n"))

	if got := bridge.firstSubmittedPrompt(); got != "continue the work" {
		t.Fatalf("unexpected first prompt: %q", got)
	}
}

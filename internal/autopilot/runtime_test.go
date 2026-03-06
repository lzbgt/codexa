package autopilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeStatusSave(t *testing.T) {
	tmp := t.TempDir()
	status := newRuntimeStatus(Invocation{
		Mode:         modeInteractiveBare,
		Workspace:    "/tmp/repo",
		OriginalArgs: []string{"--yolo"},
		ForwardArgs:  []string{"-p", "yolo"},
		RootArgs:     []string{"-p", "yolo"},
	}, filepath.Join(tmp, ".codex-autopilot"), "/opt/homebrew/bin/codex")
	status.setPhase("bootstrapping", "launching first interactive session")
	status.LastDecision = "continue"
	path := filepath.Join(tmp, "runtime.json")
	if err := status.save(path); err != nil {
		t.Fatalf("save returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"wrapper_active": true`,
		`"mode": "interactive_bare"`,
		`"current_phase": "bootstrapping"`,
		`"last_decision": "continue"`,
		`"real_codex": "/opt/homebrew/bin/codex"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("runtime status missing %q in %s", want, text)
		}
	}
}

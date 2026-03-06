package autopilot

import "testing"

func TestShouldReuseExistingStateForInteractiveResume(t *testing.T) {
	tests := []struct {
		name string
		inv  Invocation
		want bool
	}{
		{
			name: "resume last without prompt does not reuse state",
			inv: Invocation{
				Mode:         modeInteractiveResume,
				ResumeTarget: "--last",
			},
			want: false,
		},
		{
			name: "resume picker without prompt bootstraps",
			inv: Invocation{
				Mode: modeInteractiveResume,
			},
			want: false,
		},
		{
			name: "resume explicit session without prompt bootstraps",
			inv: Invocation{
				Mode:              modeInteractiveResume,
				ResumeTarget:      "019cc422-dc94-7553-a6e9-acfc3d0e183b",
				ExplicitSessionID: "019cc422-dc94-7553-a6e9-acfc3d0e183b",
			},
			want: false,
		},
		{
			name: "resume last with prompt does not reuse state",
			inv: Invocation{
				Mode:         modeInteractiveResume,
				ResumeTarget: "--last",
				Prompt:       "continue the work",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		if got := shouldReuseExistingStateForInteractiveResume(tt.inv); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}

func TestShouldBootstrapInteractiveResume(t *testing.T) {
	tests := []struct {
		name string
		inv  Invocation
		want bool
	}{
		{
			name: "resume picker bootstraps",
			inv: Invocation{
				Mode: modeInteractiveResume,
			},
			want: true,
		},
		{
			name: "resume explicit session bootstraps",
			inv: Invocation{
				Mode:              modeInteractiveResume,
				ResumeTarget:      "019cc422-dc94-7553-a6e9-acfc3d0e183b",
				ExplicitSessionID: "019cc422-dc94-7553-a6e9-acfc3d0e183b",
			},
			want: true,
		},
		{
			name: "resume last without prompt bootstraps without local wrapper state",
			inv: Invocation{
				Mode:         modeInteractiveResume,
				ResumeTarget: "--last",
			},
			want: true,
		},
		{
			name: "resume with inline prompt does not bootstrap",
			inv: Invocation{
				Mode:   modeInteractiveResume,
				Prompt: "continue the work",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		if got := shouldBootstrapInteractiveResume(tt.inv); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}

func TestPostTurnDecisionStopsExplicitly(t *testing.T) {
	report := &AutoReport{AutoModeNext: "stop", Summary: "done"}
	got := postTurnDecision(0, report)
	if got.Action != "stop" {
		t.Fatalf("unexpected decision: %#v", got)
	}
}

func TestPostTurnDecisionDefaultsToContinue(t *testing.T) {
	report := &AutoReport{AutoModeNext: "continue", Summary: "more work remains"}
	got := postTurnDecision(0, report)
	if got.Action != "continue" {
		t.Fatalf("unexpected decision: %#v", got)
	}
}

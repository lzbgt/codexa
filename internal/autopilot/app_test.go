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

func TestHandleOperatorLine(t *testing.T) {
	report := &AutoReport{AutoModeNext: "continue"}
	queue := []string{}

	if got := handleOperatorLine(report, &queue, "ship it"); got != "" {
		t.Fatalf("unexpected decision for queued prompt: %q", got)
	}
	if len(queue) != 1 || queue[0] != "ship it" {
		t.Fatalf("unexpected queue: %#v", queue)
	}
	if got := handleOperatorLine(report, &queue, "/stop"); got != "stop" {
		t.Fatalf("unexpected stop decision: %q", got)
	}
	if got := handleOperatorLine(report, &queue, ""); got != "continue" {
		t.Fatalf("unexpected empty-line decision: %q", got)
	}
}

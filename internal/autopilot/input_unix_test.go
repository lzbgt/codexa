//go:build darwin || linux

package autopilot

import "testing"

func TestClassifyOperatorTrigger(t *testing.T) {
	operatorInputBufferMu.Lock()
	operatorInputBuffer = nil
	operatorInputBufferMu.Unlock()

	tests := []struct {
		name string
		data []byte
		want operatorTriggerResult
	}{
		{name: "newline", data: []byte{'\n'}, want: operatorTriggerResult{Trigger: operatorTriggerEnter, Line: ""}},
		{name: "carriage return", data: []byte{'\r'}, want: operatorTriggerResult{Trigger: operatorTriggerEnter, Line: ""}},
		{name: "line with command", data: []byte("/stop\n"), want: operatorTriggerResult{Trigger: operatorTriggerEnter, Line: "/stop"}},
		{name: "ctrl c", data: []byte{3}, want: operatorTriggerResult{Trigger: operatorTriggerInterrupt}},
		{name: "other text", data: []byte("abc"), want: operatorTriggerResult{Trigger: operatorTriggerNone}},
	}
	for _, tt := range tests {
		if got := classifyOperatorTrigger(tt.data); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}

func TestClassifyOperatorTriggerBuffersPartialLine(t *testing.T) {
	operatorInputBufferMu.Lock()
	operatorInputBuffer = nil
	operatorInputBufferMu.Unlock()

	first := classifyOperatorTrigger([]byte("ship"))
	if first.Trigger != operatorTriggerNone {
		t.Fatalf("unexpected trigger from partial input: %#v", first)
	}
	second := classifyOperatorTrigger([]byte(" it\n"))
	if second.Trigger != operatorTriggerEnter || second.Line != "ship it" {
		t.Fatalf("unexpected buffered line result: %#v", second)
	}
}

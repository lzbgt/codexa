//go:build darwin || linux

package autopilot

import "testing"

func TestClassifyOperatorTrigger(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want operatorTrigger
	}{
		{name: "newline", data: []byte{'\n'}, want: operatorTriggerEnter},
		{name: "carriage return", data: []byte{'\r'}, want: operatorTriggerEnter},
		{name: "ctrl c", data: []byte{3}, want: operatorTriggerInterrupt},
		{name: "other text", data: []byte("abc"), want: operatorTriggerNone},
	}
	for _, tt := range tests {
		if got := classifyOperatorTrigger(tt.data); got != tt.want {
			t.Fatalf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}

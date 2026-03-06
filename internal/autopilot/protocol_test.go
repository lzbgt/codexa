package autopilot

import (
	"strings"
	"testing"
)

func TestExtractReportReadsExplicitStopContinueMarker(t *testing.T) {
	report, err := extractReport("Finished the remaining work.\nAUTO_MODE_NEXT=stop")
	if err != nil {
		t.Fatalf("extractReport returned error: %v", err)
	}
	if report.AutoModeNext != "stop" {
		t.Fatalf("unexpected auto_mode_next: %q", report.AutoModeNext)
	}
}

func TestExtractReportDefaultsToContinueWithoutFooter(t *testing.T) {
	report, err := extractReport("Implemented the CI workflow and updated TODOS.")
	if err != nil {
		t.Fatalf("extractReport returned error: %v", err)
	}
	if report.AutoModeNext != "continue" {
		t.Fatalf("unexpected auto_mode_next: %q", report.AutoModeNext)
	}
	if !strings.Contains(report.Summary, "Implemented the CI workflow") {
		t.Fatalf("unexpected fallback summary: %q", report.Summary)
	}
}

func TestProtocolInstructionsMentionMarkerOnly(t *testing.T) {
	text := protocolInstructions()
	for _, want := range []string{"AUTO_MODE_NEXT=continue", "AUTO_CONTINUE_MODE=continue|stop", "Do not append JSON"} {
		if !strings.Contains(text, want) {
			t.Fatalf("protocol instructions missing %q", want)
		}
	}
	if strings.Contains(text, "AUTO_REPORT_JSON") {
		t.Fatalf("protocol instructions should not mention JSON markers anymore")
	}
}

package autopilot

import "testing"

func TestExtractReport(t *testing.T) {
	reportText := `
AUTO_REPORT_JSON_BEGIN
{
  "auto_mode_next": "continue",
  "summary": "Implemented the parser fix.",
  "recommended_next_prompt": "Run the remaining verification.",
  "user_engagement_needed": false,
  "pending_tasks": [
    {"priority": "P0", "task": "Finish the parser verification.", "status": "pending"}
  ],
  "discovered_tasks": ["Add a regression test."],
  "reweighting_rationale": "Verification remains the blocker.",
  "verification": {"status": "partial", "summary": "Targeted tests passed."},
  "post_turn_actions": [
    {"kind": "verify", "command": "go test ./...", "description": "Verify the repo."}
  ]
}
AUTO_REPORT_JSON_END
`
	report, err := extractReport(reportText)
	if err != nil {
		t.Fatalf("extractReport returned error: %v", err)
	}
	if report.AutoModeNext != "continue" {
		t.Fatalf("unexpected auto_mode_next: %q", report.AutoModeNext)
	}
	if len(report.PostTurnActions) != 1 {
		t.Fatalf("expected 1 post-turn action, got %d", len(report.PostTurnActions))
	}
}

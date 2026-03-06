package autopilot

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	beginMarker = "AUTO_REPORT_JSON_BEGIN"
	endMarker   = "AUTO_REPORT_JSON_END"
)

type Task struct {
	Priority string `json:"priority"`
	Task     string `json:"task"`
	Status   string `json:"status"`
}

type PostTurnAction struct {
	Kind        string `json:"kind"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

type VerificationSummary struct {
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type AutoReport struct {
	AutoModeNext          string              `json:"auto_mode_next"`
	Summary               string              `json:"summary"`
	RecommendedNextPrompt string              `json:"recommended_next_prompt"`
	UserEngagementNeeded  bool                `json:"user_engagement_needed"`
	PendingTasks          []Task              `json:"pending_tasks"`
	DiscoveredTasks       []string            `json:"discovered_tasks"`
	ReweightingRationale  string              `json:"reweighting_rationale"`
	Verification          VerificationSummary `json:"verification"`
	PostTurnActions       []PostTurnAction    `json:"post_turn_actions"`
}

func (r *AutoReport) Validate() error {
	if r.AutoModeNext != "continue" && r.AutoModeNext != "stop" {
		return fmt.Errorf("invalid auto_mode_next: %q", r.AutoModeNext)
	}
	if strings.TrimSpace(r.Summary) == "" || strings.TrimSpace(r.RecommendedNextPrompt) == "" || strings.TrimSpace(r.ReweightingRationale) == "" {
		return errors.New("summary, recommended_next_prompt, and reweighting_rationale are required")
	}
	if strings.TrimSpace(r.Verification.Status) == "" || strings.TrimSpace(r.Verification.Summary) == "" {
		return errors.New("verification status and summary are required")
	}
	for _, task := range r.PendingTasks {
		if strings.TrimSpace(task.Priority) == "" || strings.TrimSpace(task.Task) == "" || strings.TrimSpace(task.Status) == "" {
			return errors.New("pending_tasks entries must include priority, task, and status")
		}
	}
	for _, action := range r.PostTurnActions {
		if strings.TrimSpace(action.Kind) == "" || strings.TrimSpace(action.Command) == "" {
			return errors.New("post_turn_actions entries must include kind and command")
		}
	}
	return nil
}

var reportPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(beginMarker) + `\s*(\{.*\})\s*` + regexp.QuoteMeta(endMarker))

func extractReport(text string) (*AutoReport, error) {
	match := reportPattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return nil, errors.New("could not find AUTO_REPORT_JSON block")
	}
	var report AutoReport
	if err := json.Unmarshal([]byte(match[1]), &report); err != nil {
		return nil, err
	}
	if err := report.Validate(); err != nil {
		return nil, err
	}
	return &report, nil
}

func protocolInstructions() string {
	example := AutoReport{
		AutoModeNext:          "continue",
		Summary:               "Implemented the parser fix and reran the targeted verification.",
		RecommendedNextPrompt: "Continue from the clean repo state and finish the remaining P0 task before widening verification scope.",
		UserEngagementNeeded:  false,
		PendingTasks: []Task{
			{Priority: "P0", Task: "Finish the remaining parser parity failure.", Status: "pending"},
			{Priority: "P1", Task: "Add the regression test for the parity case.", Status: "pending"},
		},
		DiscoveredTasks:      []string{"Document the parser edge case in the task tracker."},
		ReweightingRationale: "The parity failure still blocks broader verification, so it stays above the regression test.",
		Verification: VerificationSummary{
			Status:  "partial",
			Summary: "The targeted parser test passed; the full parity suite still has one failing case.",
		},
		PostTurnActions: []PostTurnAction{
			{Kind: "verify", Command: "go test ./...", Description: "Verify the Go workspace before finalization."},
			{Kind: "commit", Command: "git add -A && git commit -m 'autopilot: finish parser parity fix'", Description: "Commit the verified changes."},
			{Kind: "push", Command: "git push upstream HEAD", Description: "Push to the preferred remote if it exists."},
		},
	}
	data, _ := json.MarshalIndent(example, "", "  ")
	return strings.Join([]string{
		"At the end of your final response, append a machine-readable JSON report.",
		fmt.Sprintf("Write the exact begin marker on its own line: %s", beginMarker),
		"Then write valid JSON matching the required shape.",
		fmt.Sprintf("Write the exact end marker on its own line: %s", endMarker),
		"If you omit the JSON report or break its schema, the wrapper will immediately resume the same session and ask you to repair the report before any next turn can start.",
		"If your turn leaves source-code changes dirty, either finish verification and finalization during the turn or provide exact shell commands in post_turn_actions. The wrapper will execute those commands after your message.",
		string(data),
	}, "\n")
}

func protocolRepairPrompt(cause error) string {
	var b strings.Builder
	b.WriteString("Your last reply did not include a valid autopilot report.\n")
	if cause != nil {
		fmt.Fprintf(&b, "Repair cause: %s\n", cause)
	}
	b.WriteString("Do not redo the analysis. Do not inspect files unless you must verify whether the repo is dirty before declaring post_turn_actions.\n")
	b.WriteString("Use your immediately previous reply as the source of truth, preserve its findings/tasks, and emit a corrected machine-readable report now.\n")
	b.WriteString("Reply with an optional one-line preface plus the exact report block. Do not omit the markers.\n\n")
	b.WriteString(protocolInstructions())
	b.WriteString("\n")
	return b.String()
}

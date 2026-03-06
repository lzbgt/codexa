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

var modeNextPattern = regexp.MustCompile(`(?m)^\s*AUTO_(?:MODE_NEXT|CONTINUE_MODE)=(continue|stop)\s*$`)

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

func extractModeNext(text string) (string, error) {
	matches := modeNextPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 || len(matches[len(matches)-1]) != 2 {
		return "", errors.New("could not find AUTO_MODE_NEXT or AUTO_CONTINUE_MODE line")
	}
	return matches[len(matches)-1][1], nil
}

func stripReportBlock(text string) string {
	text = reportPattern.ReplaceAllString(text, "")
	text = modeNextPattern.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func extractReport(text string) (*AutoReport, error) {
	match := reportPattern.FindStringSubmatch(text)
	if len(match) == 2 {
		var report AutoReport
		if err := json.Unmarshal([]byte(match[1]), &report); err != nil {
			return nil, err
		}
		if modeNext, err := extractModeNext(text); err == nil && report.AutoModeNext == "" {
			report.AutoModeNext = modeNext
		}
		if err := report.Validate(); err != nil {
			return nil, err
		}
		return &report, nil
	}

	modeNext, err := extractModeNext(text)
	if err != nil {
		modeNext = "continue"
	}
	summary := stripReportBlock(text)
	if summary == "" {
		summary = "No structured summary was provided; the wrapper will continue based on the assistant's last reply."
	}
	return &AutoReport{
		AutoModeNext:          modeNext,
		Summary:               summary,
		RecommendedNextPrompt: "Re-read the repo state, merge the quoted last response into current TODOs, reweight the remaining work, and execute the highest-leverage task.",
		ReweightingRationale:  "Fallback derived from the assistant's last reply because no structured JSON report was provided.",
		Verification: VerificationSummary{
			Status:  "unknown",
			Summary: "No structured verification summary was provided in the last reply.",
		},
	}, nil
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
		"At the end of your final response, append a machine-readable stop/continue marker.",
		"End your final response with exactly one line: `AUTO_MODE_NEXT=continue` or `AUTO_MODE_NEXT=stop`.",
		"The wrapper also accepts `AUTO_CONTINUE_MODE=continue|stop` for compatibility, but prefer `AUTO_MODE_NEXT`.",
		"If source-code changes remain dirty, or if you need the wrapper to execute post-turn actions, also append a machine-readable JSON report.",
		fmt.Sprintf("Write the exact begin marker on its own line: %s", beginMarker),
		"Then write valid JSON matching the required shape.",
		fmt.Sprintf("Write the exact end marker on its own line: %s", endMarker),
		"If you omit the stop/continue line entirely, the wrapper will default to continuing with the auto-generated next-turn prompt.",
		"If your turn leaves source-code changes dirty, either finish verification and finalization during the turn or provide exact shell commands in post_turn_actions. The wrapper will execute those commands after your message.",
		"A clean no-code-change turn may omit the JSON block if the AUTO_MODE_NEXT line is present.",
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
	b.WriteString("Use your immediately previous reply as the source of truth, preserve its findings/tasks, and emit a corrected machine-readable footer now.\n")
	b.WriteString("Reply with an optional one-line preface, the exact AUTO_MODE_NEXT line, and if needed the exact report block. Do not omit required markers.\n\n")
	b.WriteString(protocolInstructions())
	b.WriteString("\n")
	return b.String()
}

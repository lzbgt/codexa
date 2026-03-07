package autopilot

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var modeNextPattern = regexp.MustCompile(`(?m)^\s*AUTO_(?:MODE_NEXT|CONTINUE_MODE)=(continue|stop)\s*$`)

type VerificationSummary struct {
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type AutoReport struct {
	AutoModeNext         string              `json:"auto_mode_next"`
	Summary              string              `json:"summary"`
	UserEngagementNeeded bool                `json:"user_engagement_needed"`
	Verification         VerificationSummary `json:"verification"`
}

func (r *AutoReport) Validate() error {
	if r.AutoModeNext != "continue" && r.AutoModeNext != "stop" {
		return fmt.Errorf("invalid auto_mode_next: %q", r.AutoModeNext)
	}
	if strings.TrimSpace(r.Summary) == "" {
		return errors.New("summary is required")
	}
	return nil
}

func extractModeNext(text string) (string, error) {
	matches := modeNextPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 || len(matches[len(matches)-1]) != 2 {
		return "", errors.New("could not find AUTO_MODE_NEXT or AUTO_CONTINUE_MODE line")
	}
	return matches[len(matches)-1][1], nil
}

func stripReportBlock(text string) string {
	text = modeNextPattern.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

func extractReport(text string) (*AutoReport, error) {
	modeNext, err := extractModeNext(text)
	if err != nil {
		modeNext = "continue"
	}
	summary := stripReportBlock(text)
	if summary == "" {
		summary = "No structured summary was provided; the wrapper will continue based on the assistant's last reply."
	}
	return &AutoReport{
		AutoModeNext: modeNext,
		Summary:      summary,
		Verification: VerificationSummary{
			Status:  "unknown",
			Summary: "The wrapper is only reading the stop or continue marker from the last reply.",
		},
	}, nil
}

func protocolInstructions() string {
	return strings.Join([]string{
		"At the end of your final response, append a machine-readable stop/continue marker.",
		"End your final response with exactly one footer line that uses the key `AUTO_MODE_NEXT` and the value `continue` or `stop`.",
		"The wrapper also accepts the compatibility key `AUTO_CONTINUE_MODE`, but prefer `AUTO_MODE_NEXT`.",
		"Wrapper-generated turns should include that final line so the live session can continue promptly.",
		"User-driven interactive turns may omit the marker; in that case the wrapper will still default to continuing after it recovers the last reply.",
		"Do not append JSON or any other machine-readable block. The stop/continue line is the entire wrapper protocol.",
	}, "\n")
}

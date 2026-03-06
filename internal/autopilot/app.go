package autopilot

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type App struct {
	realCodex string
}

type reportResolution struct {
	Report *AutoReport
	Result *turnResult
}

func NewApp() (*App, error) {
	return &App{}, nil
}

func (a *App) Run(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	inv, err := parseInvocation(args, cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	cfg, err := loadConfig(inv.Workspace)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	a.realCodex, err = resolveRealCodex(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if inv.Mode == modePassthrough {
		return runPassthrough(a.realCodex, inv.ForwardArgs)
	}

	fail := func(err error) int {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	var state *State
	if strings.TrimSpace(inv.Prompt) != "" {
		state = newState(inv.Workspace, inv.Prompt, "hybrid", inv.ExplicitSessionID)
		if inv.ExplicitSessionID != "" {
			state.LastSessionID = inv.ExplicitSessionID
		}
	}

	for {
		if state == nil {
			snapshotBefore := captureGitSnapshot(inv.Workspace)
			bootstrapResult, bootstrapState, err := a.bootstrapInteractiveState(inv)
			if err != nil {
				return fail(err)
			}
			state = bootstrapState
			resolution, err := a.extractReportResolution(inv, state, bootstrapResult)
			if err != nil {
				return fail(err)
			}
			state.LastReport = resolution.Report
			state.LastAssistantMessage = stripReportBlock(bootstrapResult.LastMessage)
			if resolution.Result.SessionID != "" {
				state.LastSessionID = resolution.Result.SessionID
			}
			if resolution.Result.SessionPath != "" {
				state.LastSessionPath = resolution.Result.SessionPath
			}
			snapshotAfter := captureGitSnapshot(inv.Workspace)
			if err := executePostTurnActions(inv.Workspace, state.TurnIndex, resolution.Report, snapshotBefore, snapshotAfter); err != nil {
				return fail(err)
			}
			decision := postTurnDecision(cfg.PauseWindowSeconds, resolution.Report, &state.PendingUserPrompts)
			if decision == "stop" {
				fmt.Printf("\n=== Wrapper Stop ===\nStopping after turn %d.\n", state.TurnIndex)
				return 0
			}
			continue
		}

		state.TurnIndex++
		snapshotBefore := captureGitSnapshot(inv.Workspace)
		prompt := buildPrompt(state, snapshotBefore, cfg.SkillHint)
		result, err := a.runSessionTurn(inv, state, inv.Workspace, prompt)
		if err != nil {
			return fail(err)
		}
		state.SessionStarted = true
		state.PendingUserPrompts = nil
		state.LastAssistantMessage = stripReportBlock(result.LastMessage)
		if result.SessionID != "" {
			state.LastSessionID = result.SessionID
		}
		if result.SessionPath != "" {
			state.LastSessionPath = result.SessionPath
		}
		if result.ReturnCode != 0 {
			fmt.Fprintf(os.Stderr, "codex exited with status %d\n", result.ReturnCode)
			return result.ReturnCode
		}

		resolution, err := a.extractReportResolution(inv, state, result)
		if err != nil {
			return fail(err)
		}
		state.LastReport = resolution.Report
		if resolution.Result.SessionID != "" {
			state.LastSessionID = resolution.Result.SessionID
		}
		if resolution.Result.SessionPath != "" {
			state.LastSessionPath = resolution.Result.SessionPath
		}

		snapshotAfter := captureGitSnapshot(inv.Workspace)
		if err := executePostTurnActions(inv.Workspace, state.TurnIndex, resolution.Report, snapshotBefore, snapshotAfter); err != nil {
			return fail(err)
		}

		decision := postTurnDecision(cfg.PauseWindowSeconds, resolution.Report, &state.PendingUserPrompts)
		if decision == "stop" {
			fmt.Printf("\n=== Wrapper Stop ===\nStopping after turn %d.\n", state.TurnIndex)
			return 0
		}
	}
}

func (a *App) bootstrapInteractiveState(inv Invocation) (*turnResult, *State, error) {
	turnIndex := 1
	startedAt := time.Now()
	result, err := runInteractiveCodexTurn(a.realCodex, inv.Workspace, inv.initialInteractiveArgs(""), "")
	if err != nil {
		return nil, nil, err
	}
	if result.ReturnCode != 0 {
		return nil, nil, fmt.Errorf("codex exited with status %d during initial interactive session", result.ReturnCode)
	}
	artifact, err := findLatestSessionArtifact(inv.Workspace, time.Time{}, result.SessionID)
	if err != nil {
		return nil, nil, err
	}
	initialGoal, err := extractBootstrapUserGoal(artifact.SessionPath, startedAt)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(initialGoal) == "" {
		initialGoal = strings.TrimSpace(artifact.InitialUserGoal)
	}
	if initialGoal == "" {
		return nil, nil, fmt.Errorf("could not derive an initial project goal from the first interactive session in %s", artifact.SessionPath)
	}
	state := newState(inv.Workspace, initialGoal, "hybrid", result.SessionID)
	state.TurnIndex = turnIndex
	state.SessionStarted = true
	state.LastSessionID = result.SessionID
	state.LastSessionPath = result.SessionPath
	return result, state, nil
}

func shouldReuseExistingStateForInteractiveResume(inv Invocation) bool {
	return false
}

func shouldBootstrapInteractiveResume(inv Invocation) bool {
	return inv.Mode == modeInteractiveResume && strings.TrimSpace(inv.Prompt) == ""
}

func (a *App) runSessionTurn(inv Invocation, state *State, workspace, prompt string) (*turnResult, error) {
	if inv.Mode == modeInteractive || inv.Mode == modeInteractiveResume || inv.Mode == modeInteractiveBare {
		var cmdArgs []string
		sessionHint := state.LastSessionID
		if !state.SessionStarted {
			cmdArgs = inv.initialInteractiveArgs(prompt)
			if inv.ExplicitSessionID != "" {
				sessionHint = inv.ExplicitSessionID
			}
		} else {
			cmdArgs = inv.resumeInteractiveArgs(prompt, state.LastSessionID)
		}
		return runInteractiveCodexTurn(a.realCodex, workspace, cmdArgs, sessionHint)
	}

	messageFile, err := os.CreateTemp("", "codexa-last-message-*.md")
	if err != nil {
		return nil, err
	}
	messagePath := messageFile.Name()
	if err := messageFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(messagePath)

	var cmdArgs []string
	if !state.SessionStarted {
		cmdArgs = inv.initialCommandArgs(messagePath)
	} else {
		cmdArgs = inv.resumeCommandArgs(messagePath)
	}
	return runCodexTurn(a.realCodex, workspace, prompt, messagePath, cmdArgs)
}

func (a *App) extractReportResolution(inv Invocation, state *State, result *turnResult) (*reportResolution, error) {
	report, err := extractReport(result.LastMessage)
	if err != nil {
		return nil, err
	}
	return &reportResolution{
		Report: report,
		Result: result,
	}, nil
}

func executePostTurnActions(workspace string, turnIndex int, report *AutoReport, before, after GitSnapshot) error {
	if !after.Dirty && len(report.PostTurnActions) == 0 {
		return nil
	}
	needsFinalization := after.hasNewCodeChangesComparedTo(before)
	if needsFinalization && len(report.PostTurnActions) == 0 {
		return fmt.Errorf("repo is still dirty after turn %d, but the reply did not provide post_turn_actions", turnIndex)
	}
	for _, action := range report.PostTurnActions {
		if err := runAction(workspace, action); err != nil {
			return err
		}
	}
	if needsFinalization {
		if post := captureGitSnapshot(workspace); post.hasNewCodeChangesComparedTo(before) {
			return fmt.Errorf("repo still has new dirty source changes after post_turn_actions completed")
		}
	}
	return nil
}

func postTurnDecision(pauseSeconds int, report *AutoReport, queue *[]string) string {
	fmt.Printf("\n=== Turn Summary ===\n%s\nauto_mode_next=%s | verification=%s | pending_tasks=%d\n", report.Summary, report.AutoModeNext, report.Verification.Status, len(report.PendingTasks))
	if report.UserEngagementNeeded {
		return operatorLoop(report, queue, "")
	}
	if pauseSeconds <= 0 {
		return report.AutoModeNext
	}
	fmt.Printf("Next turn decision in %ds. Press Enter now to open operator input mode.\n", pauseSeconds)
	trigger := waitForOperatorTrigger(time.Duration(pauseSeconds) * time.Second)
	switch trigger.Trigger {
	case operatorTriggerEnter:
		return operatorLoop(report, queue, trigger.Line)
	case operatorTriggerInterrupt:
		fmt.Println("\nOperator input mode requested via Ctrl+C.")
		return operatorLoop(report, queue, "")
	}
	return report.AutoModeNext
}

func operatorLoop(report *AutoReport, queue *[]string, initialLine string) string {
	fmt.Println("Operator input mode. Enter text to queue a prompt, /show, /clear, /stop, or an empty line to continue.")
	if initialLine != "" {
		fmt.Printf("autopilot> %s\n", initialLine)
		return handleOperatorLine(report, queue, initialLine)
	}
	for {
		fmt.Print("autopilot> ")
		result, err := waitForOperatorLine()
		if err != nil {
			return report.AutoModeNext
		}
		if result.Trigger == operatorTriggerInterrupt {
			fmt.Println("^C")
			return report.AutoModeNext
		}
		decision := handleOperatorLine(report, queue, result.Line)
		if decision != "" {
			return decision
		}
	}
}

func handleOperatorLine(report *AutoReport, queue *[]string, line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return report.AutoModeNext
	}
	switch line {
	case "/show":
		fmt.Println("Queued prompts:")
		for _, item := range *queue {
			fmt.Println("-", item)
		}
		return ""
	case "/clear":
		*queue = nil
		fmt.Println("Queue cleared.")
		return ""
	case "/stop":
		return "stop"
	default:
		*queue = append(*queue, line)
		fmt.Println("Queued.")
		return ""
	}
}

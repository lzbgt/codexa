package autopilot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type App struct {
	realCodex string
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

	stateDir := filepath.Join(inv.Workspace, cfg.StateDirName)
	dirs, err := ensureStateDirs(stateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	statePath := filepath.Join(dirs.Base, "session_state.json")
	state, err := loadOrCreateState(statePath, inv.Workspace, inv.Prompt, "hybrid", inv.ExplicitSessionID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if inv.Mode != modeInteractive && inv.Mode != modeInteractiveResume {
		fmt.Printf("Workspace: %s\nState dir: %s\nReal codex: %s\nMode: %s\n", inv.Workspace, dirs.Base, a.realCodex, inv.Mode)
	}
	if err := state.save(statePath); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	for {
		state.TurnIndex++
		snapshotBefore := captureGitSnapshot(inv.Workspace)
		prompt := buildPrompt(state, state.LastReport, snapshotBefore, cfg.SkillHint)
		promptPath := filepath.Join(dirs.Prompts, fmt.Sprintf("turn-%04d.md", state.TurnIndex))
		messagePath := filepath.Join(dirs.Messages, fmt.Sprintf("turn-%04d.md", state.TurnIndex))
		state.LastPromptPath = promptPath
		state.LastMessagePath = ""
		if err := state.save(statePath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}

		result, err := a.runSessionTurn(inv, state, inv.Workspace, prompt, promptPath, messagePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		state.SessionStarted = true
		state.PendingUserPrompts = nil
		state.LastMessagePath = result.LastMessagePath
		if result.SessionID != "" {
			state.LastSessionID = result.SessionID
		}
		if result.SessionPath != "" {
			state.LastSessionPath = result.SessionPath
		}
		if err := state.save(statePath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		if result.ReturnCode != 0 {
			fmt.Fprintf(os.Stderr, "codex exited with status %d\n", result.ReturnCode)
			return result.ReturnCode
		}

		report, result, err := a.extractOrRepairReport(inv, state, dirs, result)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		state.LastMessagePath = result.LastMessagePath
		state.LastPromptPath = result.PromptPath
		if result.SessionID != "" {
			state.LastSessionID = result.SessionID
		}
		if result.SessionPath != "" {
			state.LastSessionPath = result.SessionPath
		}
		reportPath := filepath.Join(dirs.Reports, fmt.Sprintf("turn-%04d.json", state.TurnIndex))
		if err := writeJSON(reportPath, report); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		state.LastReport = report
		if err := state.save(statePath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}

		snapshotAfter := captureGitSnapshot(inv.Workspace)
		if err := executePostTurnActions(inv.Workspace, dirs, state.TurnIndex, report, snapshotBefore, snapshotAfter); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}

		decision := postTurnDecision(cfg.PauseWindowSeconds, report, &state.PendingUserPrompts)
		if err := state.save(statePath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		if decision == "stop" {
			fmt.Printf("\n=== Wrapper Stop ===\nStopping after turn %d. Last report: %s\n", state.TurnIndex, reportPath)
			return 0
		}
	}
}

func (a *App) runSessionTurn(inv Invocation, state *State, workspace, prompt, promptPath, messagePath string) (*turnResult, error) {
	if inv.Mode == modeInteractive || inv.Mode == modeInteractiveResume {
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
		return runInteractiveCodexTurn(a.realCodex, workspace, prompt, promptPath, messagePath, cmdArgs, sessionHint)
	}

	var cmdArgs []string
	if !state.SessionStarted {
		cmdArgs = inv.initialCommandArgs(messagePath)
	} else {
		cmdArgs = inv.resumeCommandArgs(messagePath)
	}
	return runCodexTurn(a.realCodex, workspace, prompt, promptPath, cmdArgs)
}

func (a *App) extractOrRepairReport(inv Invocation, state *State, dirs StateDirs, result *turnResult) (*AutoReport, *turnResult, error) {
	current := result
	for repairAttempt := 0; ; repairAttempt++ {
		rawMessage, err := os.ReadFile(current.LastMessagePath)
		if err != nil {
			return nil, nil, err
		}
		report, err := extractReport(string(rawMessage))
		if err == nil {
			return report, current, nil
		}

		fmt.Printf("\n=== Protocol Repair ===\nRepairing missing or invalid report for turn %d: %v\n", state.TurnIndex, err)
		repairPrompt := protocolRepairPrompt(err)
		repairPromptPath := filepath.Join(dirs.Prompts, fmt.Sprintf("turn-%04d-repair-%02d.md", state.TurnIndex, repairAttempt+1))
		repairMessagePath := filepath.Join(dirs.Messages, fmt.Sprintf("turn-%04d-repair-%02d.md", state.TurnIndex, repairAttempt+1))
		repairResult, repairErr := a.runSessionTurn(inv, state, state.Workspace, repairPrompt, repairPromptPath, repairMessagePath)
		if repairErr != nil {
			return nil, nil, repairErr
		}
		if repairResult.ReturnCode != 0 {
			return nil, nil, fmt.Errorf("codex exited with status %d while repairing missing report", repairResult.ReturnCode)
		}
		state.LastPromptPath = repairPromptPath
		state.LastMessagePath = repairResult.LastMessagePath
		if repairResult.SessionID != "" {
			state.LastSessionID = repairResult.SessionID
		}
		if repairResult.SessionPath != "" {
			state.LastSessionPath = repairResult.SessionPath
		}
		current = repairResult
	}
}

func executePostTurnActions(workspace string, dirs StateDirs, turnIndex int, report *AutoReport, before, after GitSnapshot) error {
	if !after.Dirty && len(report.PostTurnActions) == 0 {
		return nil
	}
	needsFinalization := after.hasNewCodeChangesComparedTo(before)
	if needsFinalization && len(report.PostTurnActions) == 0 {
		return fmt.Errorf("repo is still dirty after a source-code turn, but the report did not provide post_turn_actions")
	}
	for index, action := range report.PostTurnActions {
		logPath := filepath.Join(dirs.ActionLogs, fmt.Sprintf("turn-%04d-%02d-%s.log", turnIndex, index+1, sanitize(action.Kind)))
		if err := runAction(workspace, logPath, action); err != nil {
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
		return operatorLoop(report, queue)
	}
	if pauseSeconds <= 0 {
		return report.AutoModeNext
	}
	fmt.Printf("Next turn decision in %ds. Press Enter now to open operator input mode.\n", pauseSeconds)
	done := make(chan struct{}, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
		done <- struct{}{}
	}()
	select {
	case <-time.After(time.Duration(pauseSeconds) * time.Second):
		return report.AutoModeNext
	case <-done:
		return operatorLoop(report, queue)
	}
}

func operatorLoop(report *AutoReport, queue *[]string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Operator input mode. Enter text to queue a prompt, /show, /clear, /stop, or an empty line to continue.")
	for {
		fmt.Print("autopilot> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return report.AutoModeNext
		}
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
		case "/clear":
			*queue = nil
			fmt.Println("Queue cleared.")
		case "/stop":
			return "stop"
		default:
			*queue = append(*queue, line)
			fmt.Println("Queued.")
		}
	}
}

func sanitize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	if value == "" {
		return "action"
	}
	return value
}

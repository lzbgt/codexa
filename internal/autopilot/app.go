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

type turnDecision struct {
	Action string
	Line   string
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
	var live *interactiveSession
	defer func() {
		if live != nil {
			_ = live.Close()
		}
	}()
	if strings.TrimSpace(inv.Prompt) != "" {
		state = newState(inv.Workspace, inv.Prompt, "hybrid", inv.ExplicitSessionID)
		if inv.ExplicitSessionID != "" {
			state.LastSessionID = inv.ExplicitSessionID
		}
	}

	var result *turnResult
	if state == nil {
		bootstrapResult, bootstrapState, session, err := a.bootstrapInteractiveState(inv)
		if err != nil {
			return fail(err)
		}
		live = session
		state = bootstrapState
		result = bootstrapResult
	} else {
		state.TurnIndex++
		snapshotBefore := captureGitSnapshot(inv.Workspace)
		prompt := buildPrompt(state, snapshotBefore, cfg.SkillHint)
		var err error
		result, live, err = a.runSessionTurn(inv, state, prompt, live)
		if err != nil {
			return fail(err)
		}
	}

	for {
		state.SessionStarted = true
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

		decision := postTurnDecision(cfg.PauseWindowSeconds, resolution.Report)
		switch decision.Action {
		case "stop":
			if live != nil {
				_ = live.Close()
				live = nil
			}
			fmt.Printf("\n=== Wrapper Stop ===\nStopping after turn %d.\n", state.TurnIndex)
			return 0
		case "handoff":
			if live == nil {
				return fail(fmt.Errorf("live interactive session is unavailable for user handoff"))
			}
			if err := live.ResumeUserControl(decision.Line); err != nil {
				return fail(err)
			}
			state.TurnIndex++
			result, err = live.WaitForTurn()
			if err != nil {
				return fail(err)
			}
		case "interrupt":
			if live == nil {
				fmt.Println("\nWrapper interrupted while idle.")
				return 0
			}
			if err := live.SendIdleInterrupt(); err != nil {
				return fail(err)
			}
			result, err = live.WaitForTurn()
			if err != nil {
				return fail(err)
			}
			if result.ReturnCode == 0 {
				fmt.Println("\nWrapper interrupted while idle.")
			}
			return 0
		default:
			state.TurnIndex++
			snapshotBefore := captureGitSnapshot(inv.Workspace)
			prompt := buildPrompt(state, snapshotBefore, cfg.SkillHint)
			result, live, err = a.runSessionTurn(inv, state, prompt, live)
			if err != nil {
				return fail(err)
			}
		}
	}
}

func (a *App) bootstrapInteractiveState(inv Invocation) (*turnResult, *State, *interactiveSession, error) {
	turnIndex := 1
	session, err := startInteractiveSession(a.realCodex, inv.Workspace, inv.initialInteractiveArgs(""), "")
	if err != nil {
		return nil, nil, nil, err
	}
	result, err := session.WaitForTurn()
	if err != nil {
		_ = session.Close()
		return nil, nil, nil, err
	}
	if result.ReturnCode != 0 {
		_ = session.Close()
		return nil, nil, nil, fmt.Errorf("codex exited with status %d during initial interactive session", result.ReturnCode)
	}
	initialGoal := session.InitialGoal()
	if strings.TrimSpace(initialGoal) == "" {
		_ = session.Close()
		return nil, nil, nil, fmt.Errorf("could not derive an initial project goal from the first interactive turn; start with an explicit user prompt")
	}
	state := newState(inv.Workspace, initialGoal, "hybrid", result.SessionID)
	state.TurnIndex = turnIndex
	state.SessionStarted = true
	state.LastSessionID = result.SessionID
	state.LastSessionPath = result.SessionPath
	return result, state, session, nil
}

func shouldReuseExistingStateForInteractiveResume(inv Invocation) bool {
	return false
}

func shouldBootstrapInteractiveResume(inv Invocation) bool {
	return inv.Mode == modeInteractiveResume && strings.TrimSpace(inv.Prompt) == ""
}

func (a *App) runSessionTurn(inv Invocation, state *State, prompt string, live *interactiveSession) (*turnResult, *interactiveSession, error) {
	if inv.Mode == modeInteractive || inv.Mode == modeInteractiveResume || inv.Mode == modeInteractiveBare {
		if live == nil {
			session, err := startInteractiveSession(a.realCodex, state.Workspace, inv.initialInteractiveArgs(prompt), state.LastSessionID)
			if err != nil {
				return nil, nil, err
			}
			result, err := session.WaitForTurn()
			return result, session, err
		}
		if err := live.Continue(prompt); err != nil {
			return nil, nil, err
		}
		result, err := live.WaitForTurn()
		return result, live, err
	}

	messageFile, err := os.CreateTemp("", "codexa-last-message-*.md")
	if err != nil {
		return nil, live, err
	}
	messagePath := messageFile.Name()
	if err := messageFile.Close(); err != nil {
		return nil, live, err
	}
	defer os.Remove(messagePath)

	var cmdArgs []string
	if !state.SessionStarted {
		cmdArgs = inv.initialCommandArgs(messagePath)
	} else {
		cmdArgs = inv.resumeCommandArgs(messagePath)
	}
	result, err := runCodexTurn(a.realCodex, state.Workspace, prompt, messagePath, cmdArgs)
	return result, live, err
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

func postTurnDecision(pauseSeconds int, report *AutoReport) turnDecision {
	fmt.Printf("\n=== Turn Summary ===\n%s\nauto_mode_next=%s\n", report.Summary, report.AutoModeNext)
	if report.AutoModeNext == "stop" {
		return turnDecision{Action: "stop"}
	}
	if pauseSeconds <= 0 {
		return turnDecision{Action: "continue"}
	}
	fmt.Printf("Auto-continue in %ds. Press Enter to return control to Codex, or Ctrl+C to send an interrupt to the idle Codex session.\n", pauseSeconds)
	trigger := waitForOperatorTrigger(time.Duration(pauseSeconds) * time.Second)
	switch trigger.Trigger {
	case operatorTriggerEnter:
		return turnDecision{Action: "handoff", Line: trigger.Line}
	case operatorTriggerInterrupt:
		return turnDecision{Action: "interrupt"}
	}
	return turnDecision{Action: "continue"}
}

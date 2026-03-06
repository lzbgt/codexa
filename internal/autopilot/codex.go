package autopilot

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

type turnResult struct {
	ReturnCode  int
	LastMessage string
	SessionID   string
	SessionPath string
}

func resolveRealCodex(config Config) (string, error) {
	if config.RealCodexBin != "" {
		return config.RealCodexBin, nil
	}
	self, _ := os.Executable()
	path, err := exec.LookPath("codex")
	if err == nil {
		selfEval, _ := filepath.EvalSymlinks(self)
		pathEval, _ := filepath.EvalSymlinks(path)
		if selfEval != pathEval {
			return path, nil
		}
	}
	return "", fmt.Errorf("could not resolve the real codex binary; set CODEX_AUTOPILOT_REAL_BIN")
}

func runPassthrough(realCodex string, args []string) int {
	cmd := exec.Command(realCodex, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func runCodexTurn(realCodex, workspace, prompt, lastMessagePath string, codexArgs []string) (*turnResult, error) {
	fmt.Printf("\n=== Starting Turn ===\n%s %s\n", realCodex, strings.Join(quoteArgs(codexArgs), " "))
	cmd := exec.Command(realCodex, codexArgs...)
	cmd.Dir = workspace
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}
	message, err := os.ReadFile(lastMessagePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &turnResult{
		ReturnCode:  exitCode,
		LastMessage: string(message),
	}, nil
}

func runInteractiveCodexTurn(realCodex, workspace string, codexArgs []string, sessionIDHint string) (*turnResult, error) {
	startedAt := time.Now()
	beforeInventory, err := snapshotSessionInventory(workspace)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(realCodex, codexArgs...)
	cmd.Dir = workspace
	if err := runAttachedInteractiveCommand(cmd); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result, lookupErr := collectInteractiveTurnResult(workspace, beforeInventory, startedAt, sessionIDHint, exitErr.ExitCode())
			if lookupErr != nil {
				return nil, lookupErr
			}
			return result, nil
		}
		return nil, err
	}
	return collectInteractiveTurnResult(workspace, beforeInventory, startedAt, sessionIDHint, 0)
}

func runAttachedInteractiveCommand(cmd *exec.Cmd) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
		return err
	}
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	winch <- syscall.SIGWINCH

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), oldState)
	}()

	copyDone := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(os.Stdout, ptmx)
		copyDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(ptmx, os.Stdin)
		copyDone <- struct{}{}
	}()

	waitErr := cmd.Wait()
	_ = ptmx.Close()
	<-copyDone
	<-copyDone
	return waitErr
}

func collectInteractiveTurnResult(workspace string, beforeInventory sessionInventory, startedAt time.Time, sessionIDHint string, returnCode int) (*turnResult, error) {
	artifact, err := findTurnSessionArtifact(workspace, beforeInventory, startedAt, sessionIDHint)
	if err != nil {
		return nil, err
	}
	return &turnResult{
		ReturnCode:  returnCode,
		LastMessage: artifact.LastAgentMessage,
		SessionID:   artifact.SessionID,
		SessionPath: artifact.SessionPath,
	}, nil
}

func runAction(workspace string, action PostTurnAction) error {
	fmt.Printf("\n=== Post-Turn Action (%s) ===\n%s\n", action.Kind, action.Command)
	cmd := exec.Command("/bin/zsh", "-lc", action.Command)
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func quoteArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'") {
			out = append(out, fmt.Sprintf("%q", arg))
		} else {
			out = append(out, arg)
		}
	}
	return out
}

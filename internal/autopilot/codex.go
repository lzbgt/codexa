package autopilot

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type turnResult struct {
	ReturnCode      int
	PromptPath      string
	LastMessagePath string
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

func runCodexTurn(realCodex, workspace, prompt, promptPath string, codexArgs []string) (*turnResult, error) {
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return nil, err
	}
	fmt.Printf("\n=== Starting Turn ===\n%s %s\n", realCodex, strings.Join(quoteArgs(codexArgs), " "))
	cmd := exec.Command(realCodex, codexArgs...)
	cmd.Dir = workspace
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &turnResult{
				ReturnCode:      exitErr.ExitCode(),
				PromptPath:      promptPath,
				LastMessagePath: codexArgs[len(codexArgs)-2],
			}, nil
		}
		return nil, err
	}
	return &turnResult{
		ReturnCode:      0,
		PromptPath:      promptPath,
		LastMessagePath: codexArgs[len(codexArgs)-2],
	}, nil
}

func runAction(workspace, logPath string, action PostTurnAction) error {
	fmt.Printf("\n=== Post-Turn Action (%s) ===\n%s\n", action.Kind, action.Command)
	logFile, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()
	writer := io.MultiWriter(os.Stdout, logFile)
	cmd := exec.Command("/bin/zsh", "-lc", action.Command)
	cmd.Dir = workspace
	cmd.Stdout = writer
	cmd.Stderr = writer
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

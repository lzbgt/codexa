package autopilot

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type interactiveSession struct {
	bridge             *attachedInteractiveBridge
	capture            *outputCapture
	firstSubmittedGoal string
	workspace          string
	beforeInventory    sessionInventory
	turnStartedAt      time.Time
	sessionIDHint      string
	lastPrompt         string
}

type attachedInteractiveBridge struct {
	cmd        *exec.Cmd
	ptmx       *os.File
	terminal   bool
	stdinFD    int
	stdoutDone chan struct{}
	exited     chan struct{}
	winch      chan os.Signal
	capture    *outputCapture

	mu              sync.Mutex
	restoreState    *term.State
	rawActive       bool
	closed          bool
	waitErr         error
	forwardToChild  bool
	inputLineBuffer string
	firstPromptLine string
}

func startInteractiveSession(realCodex, workspace string, codexArgs []string, sessionIDHint string) (*interactiveSession, error) {
	capture := newOutputCapture()
	beforeInventory, err := snapshotSessionInventory(workspace)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(realCodex, ensureNoAltScreen(codexArgs)...)
	cmd.Dir = workspace
	bridge, err := startAttachedInteractiveBridge(cmd, capture)
	if err != nil {
		return nil, err
	}
	capture.StartTurn()
	return &interactiveSession{
		bridge:          bridge,
		capture:         capture,
		workspace:       workspace,
		beforeInventory: beforeInventory,
		turnStartedAt:   time.Now(),
		sessionIDHint:   sessionIDHint,
	}, nil
}

func ensureNoAltScreen(args []string) []string {
	for _, arg := range args {
		if arg == "--no-alt-screen" {
			return append([]string{}, args...)
		}
	}
	return append([]string{"--no-alt-screen"}, args...)
}

func (s *interactiveSession) WaitForTurn() (*turnResult, error) {
	for {
		if s.lastPrompt != "" {
			if message, ok := s.capture.ExtractTurnMessage(s.lastPrompt); ok {
				if err := s.bridge.pauseForWrapper(); err != nil {
					return nil, err
				}
				if s.firstSubmittedGoal == "" {
					s.firstSubmittedGoal = s.bridge.firstSubmittedPrompt()
				}
				artifact := s.resolveTurnArtifact(2 * time.Second)
				if artifact != nil {
					s.sessionIDHint = artifact.SessionID
					return &turnResult{
						ReturnCode:  0,
						LastMessage: message,
						SessionID:   artifact.SessionID,
						SessionPath: artifact.SessionPath,
					}, nil
				}
				return &turnResult{
					ReturnCode:  0,
					LastMessage: message,
					SessionID:   s.sessionIDHint,
				}, nil
			}
		}
		if artifact := s.resolveTurnArtifact(0); artifact != nil {
			if err := s.bridge.pauseForWrapper(); err != nil {
				return nil, err
			}
			if s.firstSubmittedGoal == "" {
				s.firstSubmittedGoal = s.bridge.firstSubmittedPrompt()
			}
			s.sessionIDHint = artifact.SessionID
			return &turnResult{
				ReturnCode:  0,
				LastMessage: artifact.LastAgentMessage,
				SessionID:   artifact.SessionID,
				SessionPath: artifact.SessionPath,
			}, nil
		}

		select {
		case <-s.bridge.exited:
			_ = s.bridge.pauseForWrapper()
			return &turnResult{
				ReturnCode:  exitCodeFromError(s.bridge.exitErr()),
				LastMessage: normalizeTurnTranscript(s.capture.CurrentTurnText()),
			}, nil
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func (s *interactiveSession) resolveTurnArtifact(timeout time.Duration) *sessionArtifact {
	if timeout <= 0 {
		artifact, err := findTurnSessionArtifact(s.workspace, s.beforeInventory, s.turnStartedAt, s.sessionIDHint)
		if err != nil {
			return nil
		}
		return artifact
	}
	artifact, err := waitForTurnSessionArtifact(s.workspace, s.beforeInventory, s.turnStartedAt, s.sessionIDHint, timeout)
	if err != nil {
		return nil
	}
	return artifact
}

func (s *interactiveSession) Continue(prompt string) error {
	s.capture.StartTurn()
	s.lastPrompt = prompt
	beforeInventory, err := snapshotSessionInventory(s.workspace)
	if err != nil {
		return err
	}
	s.beforeInventory = beforeInventory
	s.turnStartedAt = time.Now()
	if err := s.bridge.resumeForChild(); err != nil {
		return err
	}
	return s.bridge.sendPrompt(prompt)
}

func (s *interactiveSession) ResumeUserControl(line string) error {
	s.capture.StartTurn()
	s.lastPrompt = ""
	beforeInventory, err := snapshotSessionInventory(s.workspace)
	if err != nil {
		return err
	}
	s.beforeInventory = beforeInventory
	s.turnStartedAt = time.Now()
	if err := s.bridge.resumeForChild(); err != nil {
		return err
	}
	if strings.TrimSpace(line) != "" {
		return s.bridge.sendPrompt(line)
	}
	return nil
}

func (s *interactiveSession) SendIdleInterrupt() error {
	if err := s.bridge.resumeForChild(); err != nil {
		return err
	}
	return s.bridge.sendCtrlC()
}

func (s *interactiveSession) InitialGoal() string {
	return strings.TrimSpace(s.firstSubmittedGoal)
}

func (s *interactiveSession) Close() error {
	if s.bridge == nil {
		return nil
	}
	err := s.bridge.closeChild()
	s.bridge = nil
	return err
}

func startAttachedInteractiveBridge(cmd *exec.Cmd, capture *outputCapture) (*attachedInteractiveBridge, error) {
	bridge := &attachedInteractiveBridge{
		cmd:        cmd,
		terminal:   term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		stdinFD:    int(os.Stdin.Fd()),
		stdoutDone: make(chan struct{}, 1),
		exited:     make(chan struct{}),
		capture:    capture,
	}
	if !bridge.terminal {
		cmd.Stdin = os.Stdin
		cmd.Stdout = io.MultiWriter(os.Stdout, captureWriter{capture: capture})
		cmd.Stderr = io.MultiWriter(os.Stderr, captureWriter{capture: capture})
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		go bridge.waitForExit()
		return bridge, nil
	}

	clearOperatorInputBuffer()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	bridge.ptmx = ptmx
	if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	bridge.winch = make(chan os.Signal, 1)
	signal.Notify(bridge.winch, syscall.SIGWINCH)
	go func() {
		for range bridge.winch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	bridge.winch <- syscall.SIGWINCH

	if err := bridge.resumeForChild(); err != nil {
		signal.Stop(bridge.winch)
		_ = ptmx.Close()
		return nil, err
	}

	go func() {
		writer := io.MultiWriter(os.Stdout, captureWriter{capture: capture})
		_, _ = io.Copy(writer, ptmx)
		bridge.stdoutDone <- struct{}{}
	}()
	go bridge.waitForExit()
	go bridge.pumpInput()
	return bridge, nil
}

func (b *attachedInteractiveBridge) waitForExit() {
	err := b.cmd.Wait()
	b.mu.Lock()
	b.waitErr = err
	b.mu.Unlock()
	close(b.exited)
}

func (b *attachedInteractiveBridge) pumpInput() {
	if !b.terminal || b.ptmx == nil {
		return
	}
	buffer := make([]byte, 4096)
	ptmxFD := int(b.ptmx.Fd())
	for {
		select {
		case <-b.exited:
			return
		default:
		}

		if !b.isForwarding() {
			time.Sleep(25 * time.Millisecond)
			continue
		}

		pollFds := []unix.PollFd{{
			Fd:     int32(b.stdinFD),
			Events: unix.POLLIN,
		}}
		n, err := unix.Poll(pollFds, 100)
		if err == unix.EINTR {
			continue
		}
		if err != nil || n <= 0 || pollFds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		readBytes, err := unix.Read(b.stdinFD, buffer)
		if err == unix.EINTR {
			continue
		}
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			continue
		}
		if err != nil || readBytes == 0 {
			return
		}
		b.recordUserInput(buffer[:readBytes])
		written := 0
		for written < readBytes {
			chunkWritten, writeErr := unix.Write(ptmxFD, buffer[written:readBytes])
			if writeErr == unix.EINTR {
				continue
			}
			if writeErr != nil {
				return
			}
			written += chunkWritten
		}
	}
}

func (b *attachedInteractiveBridge) recordUserInput(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range data {
		switch ch {
		case '\r', '\n':
			line := strings.TrimSpace(b.inputLineBuffer)
			if line != "" && !strings.HasPrefix(line, "/") && b.firstPromptLine == "" {
				b.firstPromptLine = line
			}
			b.inputLineBuffer = ""
		case 3:
			b.inputLineBuffer = ""
		default:
			if ch >= 32 && ch != 127 {
				b.inputLineBuffer += string(ch)
			}
		}
	}
}

func (b *attachedInteractiveBridge) firstSubmittedPrompt() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.firstPromptLine
}

func (b *attachedInteractiveBridge) pauseForWrapper() error {
	b.mu.Lock()
	b.forwardToChild = false
	b.mu.Unlock()
	if !b.terminal {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.rawActive {
		return nil
	}
	if err := term.Restore(b.stdinFD, b.restoreState); err != nil {
		return err
	}
	b.rawActive = false
	return nil
}

func (b *attachedInteractiveBridge) resumeForChild() error {
	if !b.terminal {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.rawActive {
		state, err := term.MakeRaw(b.stdinFD)
		if err != nil {
			return err
		}
		b.restoreState = state
		b.rawActive = true
	}
	b.forwardToChild = true
	return nil
}

func (b *attachedInteractiveBridge) isForwarding() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.forwardToChild
}

func (b *attachedInteractiveBridge) sendPrompt(prompt string) error {
	if prompt == "" {
		return nil
	}
	if b.ptmx == nil {
		return fmt.Errorf("interactive child PTY is unavailable")
	}
	_, err := io.WriteString(b.ptmx, prompt+"\n")
	return err
}

func (b *attachedInteractiveBridge) sendCtrlC() error {
	if b.ptmx == nil {
		return nil
	}
	_, err := b.ptmx.Write([]byte{3})
	return err
}

func (b *attachedInteractiveBridge) closeChild() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	if b.terminal {
		_ = b.pauseForWrapper()
	}
	if b.cmd.ProcessState == nil || !b.cmd.ProcessState.Exited() {
		_ = b.sendCtrlC()
		select {
		case <-b.exited:
		case <-time.After(1500 * time.Millisecond):
			_ = b.cmd.Process.Kill()
			select {
			case <-b.exited:
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	if b.winch != nil {
		signal.Stop(b.winch)
		close(b.winch)
	}
	if b.ptmx != nil {
		_ = b.ptmx.Close()
	}
	select {
	case <-b.stdoutDone:
	case <-time.After(500 * time.Millisecond):
	}
	return nil
}

func (b *attachedInteractiveBridge) exitErr() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.waitErr
}

type captureWriter struct {
	capture *outputCapture
}

func (w captureWriter) Write(p []byte) (int, error) {
	w.capture.Append(p)
	return len(p), nil
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

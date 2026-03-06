//go:build darwin || linux

package autopilot

import (
	"bytes"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type operatorTrigger int

const (
	operatorTriggerNone operatorTrigger = iota
	operatorTriggerEnter
	operatorTriggerInterrupt
)

type operatorTriggerResult struct {
	Trigger operatorTrigger
	Line    string
}

func waitForOperatorTrigger(timeout time.Duration) operatorTriggerResult {
	if timeout <= 0 {
		return operatorTriggerResult{Trigger: operatorTriggerNone}
	}

	fd := int(os.Stdin.Fd())
	deadline := time.Now().Add(timeout)
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	defer signal.Stop(sigint)

	for {
		select {
		case <-sigint:
			return operatorTriggerResult{Trigger: operatorTriggerInterrupt}
		default:
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return operatorTriggerResult{Trigger: operatorTriggerNone}
		}
		wait := remaining
		if wait > 200*time.Millisecond {
			wait = 200 * time.Millisecond
		}
		timeoutMs := int(wait.Milliseconds())
		if timeoutMs <= 0 {
			timeoutMs = 1
		}

		pollFds := []unix.PollFd{{
			Fd:     int32(fd),
			Events: unix.POLLIN,
		}}
		n, err := unix.Poll(pollFds, timeoutMs)
		if err == unix.EINTR {
			continue
		}
		if err != nil || n <= 0 || pollFds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		trigger, err := readOperatorTrigger(fd)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return operatorTriggerResult{Trigger: operatorTriggerNone}
		}
		if trigger.Trigger != operatorTriggerNone {
			return trigger
		}
	}
}

func waitForOperatorLine() (operatorTriggerResult, error) {
	fd := int(os.Stdin.Fd())
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	defer signal.Stop(sigint)

	for {
		select {
		case <-sigint:
			return operatorTriggerResult{Trigger: operatorTriggerInterrupt}, nil
		default:
		}

		pollFds := []unix.PollFd{{
			Fd:     int32(fd),
			Events: unix.POLLIN,
		}}
		n, err := unix.Poll(pollFds, 200)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return operatorTriggerResult{Trigger: operatorTriggerNone}, err
		}
		if n <= 0 || pollFds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		trigger, err := readOperatorTrigger(fd)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return operatorTriggerResult{Trigger: operatorTriggerNone}, err
		}
		if trigger.Trigger != operatorTriggerNone {
			return trigger, nil
		}
	}
}

func readOperatorTrigger(fd int) (operatorTriggerResult, error) {
	if err := unix.SetNonblock(fd, true); err != nil {
		return operatorTriggerResult{Trigger: operatorTriggerNone}, err
	}
	defer unix.SetNonblock(fd, false)

	buf := make([]byte, 128)
	n, err := unix.Read(fd, buf)
	if err == unix.EAGAIN || err == unix.EWOULDBLOCK || n == 0 {
		return operatorTriggerResult{Trigger: operatorTriggerNone}, nil
	}
	if err != nil {
		return operatorTriggerResult{Trigger: operatorTriggerNone}, err
	}
	return classifyOperatorTrigger(buf[:n]), nil
}

func classifyOperatorTrigger(data []byte) operatorTriggerResult {
	if bytes.IndexByte(data, 3) >= 0 {
		return operatorTriggerResult{Trigger: operatorTriggerInterrupt}
	}
	if index := bytes.IndexAny(data, "\r\n"); index >= 0 {
		line := strings.TrimSpace(string(data[:index]))
		return operatorTriggerResult{
			Trigger: operatorTriggerEnter,
			Line:    line,
		}
	}
	return operatorTriggerResult{Trigger: operatorTriggerNone}
}

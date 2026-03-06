//go:build darwin || linux

package autopilot

import (
	"os"
	"os/signal"
	"time"

	"golang.org/x/sys/unix"
)

type operatorTrigger int

const (
	operatorTriggerNone operatorTrigger = iota
	operatorTriggerEnter
	operatorTriggerInterrupt
)

func waitForOperatorTrigger(timeout time.Duration) operatorTrigger {
	if timeout <= 0 {
		return operatorTriggerNone
	}

	fd := int(os.Stdin.Fd())
	deadline := time.Now().Add(timeout)
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)
	defer signal.Stop(sigint)

	for {
		select {
		case <-sigint:
			return operatorTriggerInterrupt
		default:
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return operatorTriggerNone
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
			return operatorTriggerNone
		}
		if trigger != operatorTriggerNone {
			return trigger
		}
	}
}

func readOperatorTrigger(fd int) (operatorTrigger, error) {
	if err := unix.SetNonblock(fd, true); err != nil {
		return operatorTriggerNone, err
	}
	defer unix.SetNonblock(fd, false)

	buf := make([]byte, 128)
	n, err := unix.Read(fd, buf)
	if err == unix.EAGAIN || err == unix.EWOULDBLOCK || n == 0 {
		return operatorTriggerNone, nil
	}
	if err != nil {
		return operatorTriggerNone, err
	}
	return classifyOperatorTrigger(buf[:n]), nil
}

func classifyOperatorTrigger(data []byte) operatorTrigger {
	for _, b := range data {
		switch b {
		case 3:
			return operatorTriggerInterrupt
		case '\n', '\r':
			return operatorTriggerEnter
		}
	}
	return operatorTriggerNone
}

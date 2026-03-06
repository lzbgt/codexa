//go:build darwin || linux

package autopilot

import (
	"bufio"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func waitForOperatorTrigger(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}
	fd := int(os.Stdin.Fd())
	pollTimeoutMs := int(timeout.Milliseconds())
	if pollTimeoutMs == 0 {
		pollTimeoutMs = 1
	}
	pollFds := []unix.PollFd{{
		Fd:     int32(fd),
		Events: unix.POLLIN,
	}}
	n, err := unix.Poll(pollFds, pollTimeoutMs)
	if err != nil || n <= 0 {
		return false
	}
	return pollFds[0].Revents&unix.POLLIN != 0
}

func consumeOperatorTrigger() {
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

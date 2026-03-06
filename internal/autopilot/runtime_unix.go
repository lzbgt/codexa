//go:build darwin || linux

package autopilot

import "os"

func processID() int {
	return os.Getpid()
}

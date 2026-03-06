package main

import (
	"fmt"
	"os"

	"codex-hybrid-autopilot/internal/autopilot"
)

func main() {
	app, err := autopilot.NewApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(app.Run(os.Args[1:]))
}

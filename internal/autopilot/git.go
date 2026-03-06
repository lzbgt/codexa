package autopilot

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
)

type GitSnapshot struct {
	IsRepo      bool
	Dirty       bool
	Branch      string
	Changed     []string
	HasOrigin   bool
	HasUpstream bool
}

func captureGitSnapshot(workspace string) GitSnapshot {
	snap := GitSnapshot{Branch: "(none)"}
	if !commandOK(workspace, "git rev-parse --is-inside-work-tree") {
		return snap
	}
	snap.IsRepo = true
	branch := strings.TrimSpace(commandOutput(workspace, "git rev-parse --abbrev-ref HEAD"))
	if branch != "" {
		snap.Branch = branch
	}
	status := commandOutput(workspace, "git status --porcelain")
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		snap.Dirty = true
		if len(line) > 3 {
			path := strings.TrimSpace(line[3:])
			if idx := strings.Index(path, " -> "); idx >= 0 {
				path = strings.TrimSpace(path[idx+4:])
			}
			snap.Changed = append(snap.Changed, path)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(commandOutput(workspace, "git remote")), "\n") {
		switch strings.TrimSpace(line) {
		case "origin":
			snap.HasOrigin = true
		case "upstream":
			snap.HasUpstream = true
		}
	}
	return snap
}

func (g GitSnapshot) hasCodeChanges() bool {
	for _, path := range g.Changed {
		if isCodePath(path) {
			return true
		}
	}
	return false
}

func (g GitSnapshot) hasNewCodeChangesComparedTo(before GitSnapshot) bool {
	beforeSet := map[string]struct{}{}
	for _, path := range before.Changed {
		beforeSet[path] = struct{}{}
	}
	for _, path := range g.Changed {
		if !isCodePath(path) {
			continue
		}
		if _, ok := beforeSet[path]; !ok {
			return true
		}
	}
	return false
}

func isCodePath(path string) bool {
	for _, ext := range []string{
		".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".c", ".cc", ".cpp",
		".h", ".hpp", ".java", ".kt", ".swift", ".sh", ".zsh", ".bash", ".rb",
		".php", ".html", ".css", ".scss", ".sql",
	} {
		if strings.EqualFold(filepath.Ext(path), ext) {
			return true
		}
	}
	switch filepath.Base(path) {
	case "go.mod", "go.sum", "Cargo.toml", "Cargo.lock", "package.json", "package-lock.json", "pnpm-lock.yaml", "pyproject.toml", "Makefile", "Dockerfile":
		return true
	}
	return false
}

func commandOK(workspace, shell string) bool {
	cmd := exec.Command("/bin/zsh", "-lc", shell)
	cmd.Dir = workspace
	return cmd.Run() == nil
}

func commandOutput(workspace, shell string) string {
	cmd := exec.Command("/bin/zsh", "-lc", shell)
	cmd.Dir = workspace
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

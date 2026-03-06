package autopilot

import (
	"path/filepath"
	"regexp"
	"strings"
)

type invocationMode string

const (
	modePassthrough       invocationMode = "passthrough"
	modeExec              invocationMode = "exec"
	modeResume            invocationMode = "resume"
	modeInteractive       invocationMode = "interactive"
	modeInteractiveBare   invocationMode = "interactive_bare"
	modeInteractiveResume invocationMode = "interactive_resume"
)

type Invocation struct {
	Mode              invocationMode
	OriginalArgs      []string
	ForwardArgs       []string
	Workspace         string
	RootArgs          []string
	InitialExecArgs   []string
	ResumeCompatible  []string
	Prompt            string
	ResumeTarget      string
	ExplicitSessionID string
}

var passthroughCommands = map[string]bool{
	"login":      true,
	"logout":     true,
	"mcp":        true,
	"mcp-server": true,
	"app-server": true,
	"app":        true,
	"completion": true,
	"sandbox":    true,
	"debug":      true,
	"apply":      true,
	"cloud":      true,
	"features":   true,
	"review":     true,
	"fork":       true,
	"help":       true,
}

var uuidish = regexp.MustCompile(`^[0-9a-fA-F-]{8,}$`)

func parseInvocation(args []string, cwd string) (Invocation, error) {
	inv := Invocation{
		Mode:         modePassthrough,
		OriginalArgs: append([]string{}, args...),
		ForwardArgs:  append([]string{}, args...),
		Workspace:    cwd,
	}
	if len(args) == 0 {
		return inv, nil
	}

	rootArgs, workspace, next, ok := parseFlags(args, 0, rootFlagSpec(), true)
	if !ok {
		return inv, nil
	}
	inv.RootArgs = rootArgs
	inv.ForwardArgs = append(append([]string{}, rootArgs...), args[next:]...)
	if workspace != "" {
		if !filepath.IsAbs(workspace) {
			workspace = filepath.Join(cwd, workspace)
		}
		inv.Workspace = filepath.Clean(workspace)
	}

	if next >= len(args) {
		inv.Mode = modeInteractiveBare
		return inv, nil
	}
	token := inv.ForwardArgs[len(inv.RootArgs)]
	if passthroughCommands[token] {
		return inv, nil
	}
	if token == "-h" || token == "--help" || token == "-V" || token == "--version" {
		return inv, nil
	}
	switch token {
	case "exec":
		return parseExecInvocation(inv, args[next+1:])
	case "resume":
		return parseResumeInvocation(inv, args[next+1:])
	default:
		prompt := strings.TrimSpace(strings.Join(args[next:], " "))
		if prompt == "" {
			return inv, nil
		}
		inv.Mode = modeInteractive
		inv.Prompt = prompt
		return inv, nil
	}
}

func parseExecInvocation(inv Invocation, rest []string) (Invocation, error) {
	flags, _, next, ok := parseFlags(rest, 0, execFlagSpec(), false)
	if !ok {
		return passthroughInvocation(inv), nil
	}
	inv.InitialExecArgs = append(inv.InitialExecArgs, flags...)
	inv.ResumeCompatible = append(inv.ResumeCompatible, filterResumeCompatible(flags)...)
	if next >= len(rest) {
		return passthroughInvocation(inv), nil
	}
	if rest[next] == "resume" {
		return parseExecResumeInvocation(inv, rest[next+1:])
	}
	prompt := strings.TrimSpace(strings.Join(rest[next:], " "))
	if prompt == "" {
		return passthroughInvocation(inv), nil
	}
	inv.Mode = modeExec
	inv.Prompt = prompt
	return inv, nil
}

func parseResumeInvocation(inv Invocation, rest []string) (Invocation, error) {
	parsed, err := parseExecResumeInvocation(inv, rest)
	if err != nil {
		return parsed, err
	}
	switch parsed.Mode {
	case modeResume:
		parsed.Mode = modeInteractiveResume
	}
	return parsed, nil
}

func parseExecResumeInvocation(inv Invocation, rest []string) (Invocation, error) {
	flags, _, next, ok := parseFlags(rest, 0, resumeFlagSpec(), false)
	if !ok {
		return passthroughInvocation(inv), nil
	}
	inv.InitialExecArgs = append(inv.InitialExecArgs, flags...)
	inv.ResumeCompatible = append(inv.ResumeCompatible, flags...)
	if containsFlag(flags, "--last") {
		prompt := strings.TrimSpace(strings.Join(rest[next:], " "))
		inv.Mode = modeResume
		inv.ResumeTarget = "--last"
		inv.Prompt = prompt
		return inv, nil
	}
	positional := rest[next:]
	if len(positional) < 2 || !uuidish.MatchString(positional[0]) {
		return passthroughInvocation(inv), nil
	}
	inv.Mode = modeResume
	inv.ResumeTarget = positional[0]
	inv.ExplicitSessionID = positional[0]
	inv.Prompt = strings.TrimSpace(strings.Join(positional[1:], " "))
	if inv.Prompt == "" {
		return passthroughInvocation(inv), nil
	}
	return inv, nil
}

func passthroughInvocation(inv Invocation) Invocation {
	return Invocation{
		Mode:         modePassthrough,
		OriginalArgs: append([]string{}, inv.OriginalArgs...),
		ForwardArgs:  append([]string{}, inv.ForwardArgs...),
		Workspace:    inv.Workspace,
	}
}

type flagSpec struct {
	withValue map[string]bool
	boolean   map[string]bool
}

func rootFlagSpec() flagSpec {
	return flagSpec{
		withValue: map[string]bool{
			"-c":                 true,
			"--config":           true,
			"--enable":           true,
			"--disable":          true,
			"-i":                 true,
			"--image":            true,
			"-m":                 true,
			"--model":            true,
			"--local-provider":   true,
			"-p":                 true,
			"--profile":          true,
			"-s":                 true,
			"--sandbox":          true,
			"-a":                 true,
			"--ask-for-approval": true,
			"-C":                 true,
			"--cd":               true,
			"--add-dir":          true,
		},
		boolean: map[string]bool{
			"--oss":       true,
			"--full-auto": true,
			"--dangerously-bypass-approvals-and-sandbox": true,
			"--search":        true,
			"--no-alt-screen": true,
		},
	}
}

func execFlagSpec() flagSpec {
	return flagSpec{
		withValue: map[string]bool{
			"-c":        true,
			"--config":  true,
			"--enable":  true,
			"--disable": true,
			"-i":        true,
			"--image":   true,
			"-m":        true,
			"--model":   true,
			"--color":   true,
		},
		boolean: map[string]bool{
			"--full-auto": true,
			"--dangerously-bypass-approvals-and-sandbox": true,
			"--skip-git-repo-check":                      true,
			"--ephemeral":                                true,
			"--progress-cursor":                          true,
		},
	}
}

func resumeFlagSpec() flagSpec {
	return flagSpec{
		withValue: map[string]bool{
			"-c":        true,
			"--config":  true,
			"--enable":  true,
			"--disable": true,
			"-i":        true,
			"--image":   true,
			"-m":        true,
			"--model":   true,
		},
		boolean: map[string]bool{
			"--last":      true,
			"--all":       true,
			"--full-auto": true,
			"--dangerously-bypass-approvals-and-sandbox": true,
			"--skip-git-repo-check":                      true,
			"--ephemeral":                                true,
		},
	}
}

func parseFlags(args []string, start int, spec flagSpec, allowWorkspace bool) ([]string, string, int, bool) {
	result := []string{}
	workspace := ""
	i := start
	for i < len(args) {
		token := args[i]
		if token == "--json" || token == "-o" || token == "--output-last-message" {
			return nil, "", start, false
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			break
		}
		flagName := token
		valueToken := ""
		if strings.Contains(token, "=") {
			parts := strings.SplitN(token, "=", 2)
			flagName = parts[0]
			valueToken = parts[1]
		}
		if flagName == "--yolo" {
			result = append(result, "-p", "yolo")
			i++
			continue
		}
		if spec.withValue[flagName] {
			if valueToken != "" {
				result = append(result, token)
				if allowWorkspace && (flagName == "-C" || flagName == "--cd") {
					workspace = valueToken
				}
				i++
				continue
			}
			if i+1 >= len(args) {
				return nil, "", start, false
			}
			result = append(result, token, args[i+1])
			if allowWorkspace && (flagName == "-C" || flagName == "--cd") {
				workspace = args[i+1]
			}
			i += 2
			continue
		}
		if spec.boolean[flagName] {
			result = append(result, token)
			i++
			continue
		}
		return nil, "", start, false
	}
	return result, workspace, i, true
}

func filterResumeCompatible(flags []string) []string {
	ignored := map[string]bool{"--color": true, "--progress-cursor": true}
	result := []string{}
	for _, token := range flags {
		name := token
		if strings.Contains(token, "=") {
			name = strings.SplitN(token, "=", 2)[0]
		}
		if ignored[name] {
			continue
		}
		result = append(result, token)
	}
	return result
}

func containsFlag(flags []string, want string) bool {
	for _, token := range flags {
		if token == want || strings.HasPrefix(token, want+"=") {
			return true
		}
	}
	return false
}

func (inv Invocation) initialCommandArgs(lastMessagePath string) []string {
	args := append([]string{}, inv.RootArgs...)
	switch inv.Mode {
	case modeExec:
		args = append(args, "exec")
		args = append(args, inv.InitialExecArgs...)
	case modeResume:
		args = append(args, "exec", "resume")
		if inv.ResumeTarget != "" {
			args = append(args, inv.ResumeTarget)
		} else {
			args = append(args, "--last")
		}
		args = append(args, inv.InitialExecArgs...)
	}
	args = append(args, "-o", lastMessagePath, "-")
	return args
}

func (inv Invocation) resumeCommandArgs(lastMessagePath string) []string {
	args := append([]string{}, inv.RootArgs...)
	args = append(args, "exec", "resume")
	if inv.ExplicitSessionID != "" {
		args = append(args, inv.ExplicitSessionID)
	} else {
		args = append(args, "--last")
	}
	args = append(args, inv.ResumeCompatible...)
	args = append(args, "-o", lastMessagePath, "-")
	return args
}

func (inv Invocation) initialInteractiveArgs(prompt string) []string {
	args := append([]string{}, inv.RootArgs...)
	switch inv.Mode {
	case modeInteractive:
		args = append(args, prompt)
	case modeInteractiveResume:
		args = append(args, "resume")
		if inv.ResumeTarget != "" {
			args = append(args, inv.ResumeTarget)
		} else if inv.ExplicitSessionID != "" {
			args = append(args, inv.ExplicitSessionID)
		} else {
			args = append(args, "--last")
		}
		if prompt != "" {
			args = append(args, prompt)
		}
	}
	return args
}

func (inv Invocation) resumeInteractiveArgs(prompt, sessionID string) []string {
	args := append([]string{}, inv.RootArgs...)
	args = append(args, "resume")
	switch {
	case sessionID != "":
		args = append(args, sessionID)
	case inv.ExplicitSessionID != "":
		args = append(args, inv.ExplicitSessionID)
	default:
		args = append(args, "--last")
	}
	if prompt != "" {
		args = append(args, prompt)
	}
	return args
}

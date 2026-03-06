package autopilot

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
)

var (
	ansiCSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
)

type outputCapture struct {
	mu        sync.Mutex
	text      string
	turnStart int
}

func newOutputCapture() *outputCapture {
	return &outputCapture{}
}

func (c *outputCapture) Append(data []byte) {
	cleaned := cleanTerminalBytes(data)
	if cleaned == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.text += cleaned
	if len(c.text) > 512*1024 {
		excess := len(c.text) - 512*1024
		c.text = c.text[excess:]
		if c.turnStart >= excess {
			c.turnStart -= excess
		} else {
			c.turnStart = 0
		}
	}
}

func (c *outputCapture) StartTurn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turnStart = len(c.text)
}

func (c *outputCapture) CurrentTurnText() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.turnStart >= len(c.text) {
		return ""
	}
	return c.text[c.turnStart:]
}

func (c *outputCapture) ExtractTurnMessage(prompt string) (string, bool) {
	text := normalizeTurnTranscript(c.CurrentTurnText())
	text = stripPromptEcho(text, prompt)
	report, err := extractReport(text)
	if err != nil {
		return "", false
	}
	if !strings.Contains(text, "AUTO_MODE_NEXT=") && !strings.Contains(text, "AUTO_CONTINUE_MODE=") {
		return "", false
	}
	return report.Summary, true
}

func stripPromptEcho(text, prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return text
	}
	normalizedPrompt := normalizeTurnTranscript(prompt)
	normalizedPrompt = strings.TrimSpace(normalizedPrompt)
	if normalizedPrompt == "" {
		return text
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, normalizedPrompt) {
		return strings.TrimSpace(strings.TrimPrefix(text, normalizedPrompt))
	}
	return strings.TrimSpace(strings.Replace(text, normalizedPrompt, "", 1))
}

func cleanTerminalBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	data = ansiOSC.ReplaceAll(data, nil)
	data = ansiCSI.ReplaceAll(data, nil)

	var out bytes.Buffer
	for _, b := range data {
		switch {
		case b == '\n' || b == '\t':
			out.WriteByte(b)
		case b == '\r':
			continue
		case b >= 32 && b != 127:
			out.WriteByte(b)
		}
	}
	return out.String()
}

func normalizeTurnTranscript(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "• ")
		switch {
		case trimmed == "":
			if len(out) == 0 || out[len(out)-1] == "" {
				continue
			}
			out = append(out, "")
		case strings.HasPrefix(trimmed, "Tip:"):
			continue
		case strings.HasPrefix(trimmed, "Token usage:"):
			continue
		case strings.HasPrefix(trimmed, "To continue this session, run codex resume"):
			continue
		case strings.HasPrefix(trimmed, "gpt-") && strings.Contains(trimmed, "left"):
			continue
		case strings.HasPrefix(trimmed, "Working"):
			continue
		case strings.HasPrefix(trimmed, "Run /review"):
			continue
		case strings.HasPrefix(trimmed, "OpenAI Codex"):
			continue
		case strings.HasPrefix(trimmed, "model:"):
			continue
		case strings.HasPrefix(trimmed, "directory:"):
			continue
		default:
			out = append(out, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

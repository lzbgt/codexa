package autopilot

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sessionArtifact struct {
	SessionID        string
	SessionPath      string
	InitialUserGoal  string
	LastAgentMessage string
}

func findLatestSessionArtifact(workspace string, since time.Time, wantedSessionID string) (*sessionArtifact, error) {
	root, err := codexSessionsRoot()
	if err != nil {
		return nil, err
	}
	cleanWorkspace := filepath.Clean(workspace)
	since = since.Add(-5 * time.Second)
	var latestPath string
	var latestTime time.Time
	var latestID string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if wantedSessionID == "" && info.ModTime().Before(since) {
			return nil
		}
		sessionID, cwd, err := readSessionMeta(path)
		if err != nil {
			return nil
		}
		if filepath.Clean(cwd) != cleanWorkspace {
			return nil
		}
		if wantedSessionID != "" && sessionID != wantedSessionID {
			return nil
		}
		if latestPath == "" || info.ModTime().After(latestTime) {
			latestPath = path
			latestTime = info.ModTime()
			latestID = sessionID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if latestPath == "" {
		if wantedSessionID != "" {
			return nil, fmt.Errorf("could not find Codex session artifact for session %s", wantedSessionID)
		}
		return nil, errors.New("could not find a recent Codex session artifact for this workspace")
	}
	lastAgentMessage, err := extractLastAgentMessage(latestPath)
	if err != nil {
		return nil, err
	}
	initialUserGoal, err := extractInitialUserGoal(latestPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(lastAgentMessage) == "" {
		return nil, fmt.Errorf("session artifact %s does not contain a final assistant message", latestPath)
	}
	return &sessionArtifact{
		SessionID:        latestID,
		SessionPath:      latestPath,
		InitialUserGoal:  initialUserGoal,
		LastAgentMessage: lastAgentMessage,
	}, nil
}

func codexSessionsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "sessions"), nil
}

func readSessionMeta(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	scanner := newSessionScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", "", err
		}
		return "", "", io.EOF
	}
	var entry struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		return "", "", err
	}
	if entry.Type != "session_meta" || entry.Payload.ID == "" || entry.Payload.Cwd == "" {
		return "", "", errors.New("missing session metadata")
	}
	return entry.Payload.ID, entry.Payload.Cwd, nil
}

func extractLastAgentMessage(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := newSessionScanner(file)
	lastMessage := ""
	for scanner.Scan() {
		line := scanner.Bytes()
		var envelope struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		switch envelope.Type {
		case "event_msg":
			var payload struct {
				Type             string `json:"type"`
				Message          string `json:"message"`
				LastAgentMessage string `json:"last_agent_message"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				continue
			}
			switch payload.Type {
			case "agent_message":
				if strings.TrimSpace(payload.Message) != "" {
					lastMessage = payload.Message
				}
			case "task_complete":
				if strings.TrimSpace(payload.LastAgentMessage) != "" {
					lastMessage = payload.LastAgentMessage
				}
			}
		case "response_item":
			message, ok := parseAssistantResponseItem(envelope.Payload)
			if ok && strings.TrimSpace(message) != "" {
				lastMessage = message
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return lastMessage, nil
}

func extractInitialUserGoal(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := newSessionScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var envelope struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		switch envelope.Type {
		case "event_msg":
			var payload struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				continue
			}
			if payload.Type == "user_message" {
				goal := normalizeUserGoal(payload.Message)
				if goal != "" {
					return goal, nil
				}
			}
		case "response_item":
			goal, ok := parseUserResponseItem(envelope.Payload)
			if ok && goal != "" {
				return goal, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func extractBootstrapUserGoal(path string, since time.Time) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	cutoff := since.Add(-5 * time.Second)
	scanner := newSessionScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var envelope struct {
			Timestamp string          `json:"timestamp"`
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339Nano, envelope.Timestamp)
		if err != nil || entryTime.Before(cutoff) {
			continue
		}
		switch envelope.Type {
		case "event_msg":
			var payload struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
				continue
			}
			if payload.Type == "user_message" {
				goal := normalizeUserGoal(payload.Message)
				if goal != "" {
					return goal, nil
				}
			}
		case "response_item":
			goal, ok := parseUserResponseItem(envelope.Payload)
			if ok && goal != "" {
				return goal, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func parseAssistantResponseItem(payload json.RawMessage) (string, bool) {
	var item struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(payload, &item); err != nil {
		return "", false
	}
	if item.Type != "message" || item.Role != "assistant" {
		return "", false
	}
	parts := make([]string, 0, len(item.Content))
	for _, content := range item.Content {
		if content.Text != "" {
			parts = append(parts, content.Text)
		}
	}
	return strings.Join(parts, "\n"), true
}

func parseUserResponseItem(payload json.RawMessage) (string, bool) {
	var item struct {
		Type    string `json:"type"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(payload, &item); err != nil {
		return "", false
	}
	if item.Type != "message" || item.Role != "user" {
		return "", false
	}
	parts := make([]string, 0, len(item.Content))
	for _, content := range item.Content {
		if content.Text != "" {
			parts = append(parts, content.Text)
		}
	}
	return normalizeUserGoal(strings.Join(parts, "\n")), true
}

func normalizeUserGoal(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "# AGENTS.md instructions for ") || strings.HasPrefix(text, "<environment_context>") {
		return ""
	}
	if strings.HasPrefix(text, "<turn_aborted>") {
		return ""
	}
	return text
}

func newSessionScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return scanner
}

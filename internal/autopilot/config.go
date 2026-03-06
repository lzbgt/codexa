package autopilot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	MaxTurns           int    `json:"max_turns"`
	PauseWindowSeconds int    `json:"pause_window_seconds"`
	SkillHint          bool   `json:"skill_hint"`
	StateDirName       string `json:"state_dir_name"`
	RealCodexBin       string `json:"real_codex_bin"`
}

func defaultConfig() Config {
	return Config{
		MaxTurns:           20,
		PauseWindowSeconds: 10,
		SkillHint:          true,
		StateDirName:       ".codex-autopilot",
	}
}

func loadConfig(workspace string) (Config, error) {
	cfg := defaultConfig()
	configPath := filepath.Join(workspace, cfg.StateDirName, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if value := os.Getenv("CODEX_AUTOPILOT_MAX_TURNS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, err
		}
		cfg.MaxTurns = parsed
	}
	if value := os.Getenv("CODEX_AUTOPILOT_PAUSE_SECONDS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, err
		}
		cfg.PauseWindowSeconds = parsed
	}
	if value := os.Getenv("CODEX_AUTOPILOT_REAL_BIN"); value != "" {
		cfg.RealCodexBin = value
	}
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 1
	}
	if cfg.PauseWindowSeconds < 0 {
		cfg.PauseWindowSeconds = 0
	}
	if cfg.StateDirName == "" {
		cfg.StateDirName = ".codex-autopilot"
	}
	return cfg, nil
}

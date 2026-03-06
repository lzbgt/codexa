package autopilot

import (
	"os"
	"strconv"
)

type Config struct {
	PauseWindowSeconds int    `json:"pause_window_seconds"`
	SkillHint          bool   `json:"skill_hint"`
	StateDirName       string `json:"state_dir_name"`
	RealCodexBin       string `json:"real_codex_bin"`
}

func defaultConfig() Config {
	return Config{
		PauseWindowSeconds: 10,
		SkillHint:          true,
	}
}

func loadConfig(workspace string) (Config, error) {
	_ = workspace
	cfg := defaultConfig()
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
	if cfg.PauseWindowSeconds < 0 {
		cfg.PauseWindowSeconds = 0
	}
	return cfg, nil
}

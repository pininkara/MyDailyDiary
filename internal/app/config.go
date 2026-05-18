package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func resolveConfigPath() string {
	cfgPath := os.Getenv("DIARY_CONFIG")
	if cfgPath == "" {
		if _, err := os.Stat("data/config.toml"); err == nil {
			cfgPath = "data/config.toml"
		} else {
			cfgPath = "config.toml"
		}
	}
	return cfgPath
}

func loadConfig(path string) *Config {
	cfg := &Config{}
	cfg.Server.Address = ":8080"
	cfg.Auth.Token = "changeme"
	cfg.Database.Path = "data/diary.db"
	cfg.UI.AvatarURL = ""
	cfg.UI.Username = ""
	cfg.LLM.Enabled = false
	cfg.LLM.BaseURL = ""
	cfg.LLM.APIKey = ""
	cfg.LLM.Model = ""
	cfg.LLM.Prompt = "请为下面这篇日记生成一个简短标题，只返回标题，不要解释。"

	b, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[INFO] config not found (%s), using defaults", path)
		return cfg
	}
	if err := toml.Unmarshal(b, cfg); err != nil {
		log.Printf("[WARN] parse config failed: %v; using defaults", err)
	}
	return cfg
}

func saveConfig(path string, cfg *Config) error {
	b, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func maskToken(tok string) string {
	if len(tok) <= 6 {
		return "******"
	}
	return tok[:2] + strings.Repeat("*", len(tok)-4) + tok[len(tok)-2:]
}

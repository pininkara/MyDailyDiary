package app

import (
	"database/sql"
	"time"
)

// App holds app-wide dependencies
type App struct {
	DB         *sql.DB
	Token      string
	Cfg        *Config
	ConfigPath string
}

type Entry struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	Content         string    `json:"content"`
	Created         time.Time `json:"created_at"`
	Updated         time.Time `json:"updated_at"`
	Day             string    `json:"day"`
	EditCount       int       `json:"edit_count"`
	Mood            int       `json:"mood"`
	Fulfill         int       `json:"fulfillment"`
	BaseWeather     string    `json:"base_weather"`
	AmbientWeathers []string  `json:"ambient_weathers"`
	AutoTitle       bool      `json:"auto_title"`
}

type DayCell struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
	Words int    `json:"words"`
	Level int    `json:"level"`
}

type Config struct {
	Server struct {
		Address string `toml:"address"`
	} `toml:"server"`
	Auth struct {
		Token string `toml:"token"`
	} `toml:"auth"`
	Database struct {
		Path string `toml:"path"`
	} `toml:"database"`
	UI struct {
		AvatarURL string `toml:"avatar_url"`
		Username  string `toml:"username"`
	} `toml:"ui"`
	LLM struct {
		Enabled bool   `toml:"enabled"`
		BaseURL string `toml:"base_url"`
		APIKey  string `toml:"api_key"`
		Model   string `toml:"model"`
		Prompt  string `toml:"prompt"`
	} `toml:"llm"`
}

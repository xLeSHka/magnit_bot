package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	TelegramBotToken string `envconfig:"TELEGRAM_BOT_TOKEN"`
	ChecklistPath    string `envconfig:"CHECKLIST_PATH" default:"checklist.txt"`

	Debug        bool   `envconfig:"DEBUG" default:"false"`
	TimeZone     string `envconfig:"TIMEZONE" default:"UTC"`
	TimeLocation *time.Location
	LogToFile    bool   `envconfig:"LOG_TO_FILE" default:"true"`
	LogsDir      string `envconfig:"LOGS_DIR" default:"./logs"`
	LogFormat    string `envconfig:"LOG_FORMAT" default:"2006-01-02_15-04"`

}

func New() (*Config, error) {
	cfg := &Config{}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	var envPath string
	if strings.HasPrefix(wd, "/app") {
		wd = "/app"
		envPath = filepath.Join(wd, ".env")
	} else {
		wd = filepath.Join(wd)
		envPath = filepath.Join(wd, "dev.env")
	}
	if _, err := os.Stat(envPath); err == nil {
		_ = godotenv.Load(envPath)
	}

	if err := envconfig.Process("", cfg); err != nil {
		return cfg, err
	}
	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, err
	}
	cfg.TimeLocation = location
	return cfg, nil
}

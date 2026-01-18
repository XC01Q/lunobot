package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	TelegramToken string
	DatabasePath  string
	Debug         bool
}

func Load() *Config {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "bot.db"
	}

	debug := false
	if debugStr := os.Getenv("DEBUG"); debugStr != "" {
		if d, err := strconv.ParseBool(debugStr); err == nil {
			debug = d
		}
	}

	return &Config{
		TelegramToken: token,
		DatabasePath:  dbPath,
		Debug:         debug,
	}
}

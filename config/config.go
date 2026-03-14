package config

import (
	"os"
	"strings"

	"github.com/umakantv/go-utils/db"
)

// Config holds application configuration
type Config struct {
	DB db.DatabaseConfig
}

// Load reads configuration from .env file
func Load() (*Config, error) {
	cfg := &Config{
		DB: db.DatabaseConfig{},
	}

	f, err := os.ReadFile(".env")
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(f), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "DRIVER":
				cfg.DB.DRIVER = val
			case "HOST":
				cfg.DB.HOST = val
			case "PORT":
				cfg.DB.PORT = val
			case "USER":
				cfg.DB.USER = val
			case "PASSWORD":
				cfg.DB.PASSWORD = val
			case "DB":
				cfg.DB.DB = val
			}
		}
	}

	return cfg, nil
}

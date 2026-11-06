package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	MasterPassword string
	Domain         string
	SessionSecret  string
	DataDir        string
	ListenAddr     string
}

func Load() (Config, error) {
	cfg := Config{
		MasterPassword: os.Getenv("MASTER_PASSWORD"),
		Domain:         strings.TrimRight(os.Getenv("DOMAIN"), "/"),
		SessionSecret:  os.Getenv("SESSION_SECRET"),
		DataDir:        getenvDefault("DATA_DIR", "/data"),
		ListenAddr:     getenvDefault("LISTEN_ADDR", ":8080"),
	}
	if cfg.MasterPassword == "" {
		return Config{}, fmt.Errorf("MASTER_PASSWORD is required")
	}
	if cfg.Domain == "" {
		return Config{}, fmt.Errorf("DOMAIN is required")
	}
	if cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("SESSION_SECRET is required")
	}
	if len(cfg.SessionSecret) < 16 {
		return Config{}, fmt.Errorf("SESSION_SECRET must be at least 16 characters")
	}
	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

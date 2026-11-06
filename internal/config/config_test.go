package config

import (
	"testing"
)

func TestLoadRequiresEnv(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "")
	t.Setenv("DOMAIN", "")
	t.Setenv("SESSION_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when env missing")
	}
}

func TestLoadOK(t *testing.T) {
	t.Setenv("MASTER_PASSWORD", "secret")
	t.Setenv("DOMAIN", "https://share.example.com/")
	t.Setenv("SESSION_SECRET", "0123456789abcdef")
	t.Setenv("DATA_DIR", "")
	t.Setenv("LISTEN_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domain != "https://share.example.com" {
		t.Fatalf("domain trim: got %q", cfg.Domain)
	}
	if cfg.DataDir != "/data" || cfg.ListenAddr != ":8080" {
		t.Fatalf("defaults: %+v", cfg)
	}
}

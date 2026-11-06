package main

import (
	"log"
	"net/http"
	"time"

	"github.com/chrishenschel/simple-fileshare/internal/auth"
	"github.com/chrishenschel/simple-fileshare/internal/cleanup"
	"github.com/chrishenschel/simple-fileshare/internal/config"
	"github.com/chrishenschel/simple-fileshare/internal/storage"
	"github.com/chrishenschel/simple-fileshare/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := storage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	sessions := auth.NewManager(cfg.SessionSecret, 7*24*time.Hour)
	srv, err := web.New(cfg, store, sessions)
	if err != nil {
		log.Fatalf("web: %v", err)
	}

	cleanup.New(store, time.Minute).Start()

	log.Printf("fileshare listening on %s (data=%s)", cfg.ListenAddr, cfg.DataDir)
	if err := http.ListenAndServe(cfg.ListenAddr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

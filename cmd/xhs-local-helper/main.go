package main

import (
	"log"

	"github.com/liuhaotian/xhs-local-helper/internal/app"
	"github.com/liuhaotian/xhs-local-helper/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	server, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("create server failed: %v", err)
	}

	log.Printf("xhs local helper listening on %s", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

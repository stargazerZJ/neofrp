package main

import (
	"flag"
	"os"

	"neofrp/internal/config"
	"neofrp/internal/server"

	"github.com/charmbracelet/log"
)

func main() {
	configPath := flag.String("c", "frps.toml", "config file path")
	flag.Parse()

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	srv := server.NewServer(cfg)
	if err := srv.Start(); err != nil {
		log.Fatal("Failed to start server", "error", err)
		os.Exit(1)
	}
}
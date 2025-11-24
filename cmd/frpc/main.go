package main

import (
	"flag"
	"os"

	"neofrp/internal/client"
	"neofrp/internal/config"

	"github.com/charmbracelet/log"
)

func main() {
	configPath := flag.String("c", "frpc.toml", "config file path")
	flag.Parse()

	cfg, err := config.LoadClientConfig(*configPath)
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	cli := client.NewClient(cfg)
	if err := cli.Start(); err != nil {
		log.Fatal("Failed to start client", "error", err)
		os.Exit(1)
	}
}
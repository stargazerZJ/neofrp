package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
)

// ServerConfig holds the server configuration
type ServerConfig struct {
	BindAddr      string
	BindPort      int
	AuthKey       string
	LogLevel      string
}

// ClientConfig holds the client configuration
type ClientConfig struct {
	ServerAddr    string
	ServerPort    int
	AuthKey       string
	LocalAddr     string
	LocalPort     int
	RemotePort    int
	PoolSize      int
	LogLevel      string
}

// LoadServerConfig loads server configuration from a TOML file
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &ServerConfig{
		BindAddr: "0.0.0.0",
		BindPort: 7000,
		LogLevel: "info",
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "bind_addr":
			cfg.BindAddr = value
		case "bind_port":
			fmt.Sscanf(value, "%d", &cfg.BindPort)
		case "auth_key":
			cfg.AuthKey = value
		case "log_level":
			cfg.LogLevel = value
		}
	}

	if cfg.AuthKey == "" {
		return nil, fmt.Errorf("auth_key is required in server config")
	}

	setLogLevel(cfg.LogLevel)
	return cfg, nil
}

// LoadClientConfig loads client configuration from a TOML file
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &ClientConfig{
		ServerAddr: "127.0.0.1",
		ServerPort: 7000,
		LocalAddr:  "127.0.0.1",
		LocalPort:  22,
		RemotePort: 6000,
		PoolSize:   5,
		LogLevel:   "info",
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "server_addr":
			cfg.ServerAddr = value
		case "server_port":
			fmt.Sscanf(value, "%d", &cfg.ServerPort)
		case "auth_key":
			cfg.AuthKey = value
		case "local_addr":
			cfg.LocalAddr = value
		case "local_port":
			fmt.Sscanf(value, "%d", &cfg.LocalPort)
		case "remote_port":
			fmt.Sscanf(value, "%d", &cfg.RemotePort)
		case "pool_size":
			fmt.Sscanf(value, "%d", &cfg.PoolSize)
		case "log_level":
			cfg.LogLevel = value
		}
	}

	if cfg.AuthKey == "" {
		return nil, fmt.Errorf("auth_key is required in client config")
	}

	setLogLevel(cfg.LogLevel)
	return cfg, nil
}

func setLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}
// Package config loads the agent configuration.
//
// Priority (highest first): command-line flags > environment variables > defaults.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config is the agent configuration.
type Config struct {
	// ServerAddr is the host:port of the SVMM gRPC server.
	ServerAddr string
	// AgentID uniquely identifies this agent. Defaults to "svma-<hostname>".
	AgentID string
	// HeartbeatInterval is the time between heartbeats.
	HeartbeatInterval time.Duration
	// Insecure disables TLS. Only for development: there is no transport
	// security or authentication yet.
	Insecure bool
}

// Defaults.
const (
	DefaultServerAddr        = "localhost:50051"
	DefaultHeartbeatInterval = 30 * time.Second
)

// Load parses args (without the program name) and the environment.
func Load(args []string, getenv func(string) string, hostname func() (string, error)) (Config, error) {
	cfg := Config{
		ServerAddr:        firstNonEmpty(getenv("SVMM_SERVER_ADDR"), DefaultServerAddr),
		AgentID:           getenv("AGENT_ID"),
		HeartbeatInterval: DefaultHeartbeatInterval,
		Insecure:          getenv("SVMM_INSECURE") == "true",
	}
	if v := getenv("HEARTBEAT_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("HEARTBEAT_INTERVAL: %w", err)
		}
		cfg.HeartbeatInterval = d
	}

	fs := flag.NewFlagSet("svmm-agent", flag.ContinueOnError)
	fs.StringVar(&cfg.ServerAddr, "server", cfg.ServerAddr, "SVMM gRPC server address")
	fs.StringVar(&cfg.AgentID, "agent-id", cfg.AgentID, "agent ID (default svma-<hostname>)")
	fs.DurationVar(&cfg.HeartbeatInterval, "heartbeat-interval", cfg.HeartbeatInterval, "heartbeat interval")
	fs.BoolVar(&cfg.Insecure, "insecure", cfg.Insecure, "disable TLS (development only)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if cfg.AgentID == "" {
		h, err := hostname()
		if err != nil {
			return Config{}, fmt.Errorf("determining hostname: %w", err)
		}
		cfg.AgentID = "svma-" + h
	}
	if cfg.HeartbeatInterval <= 0 {
		return Config{}, fmt.Errorf("heartbeat interval must be positive, got %s", cfg.HeartbeatInterval)
	}
	return cfg, nil
}

// FromOS loads the configuration from os.Args and the process environment.
func FromOS() (Config, error) {
	return Load(os.Args[1:], os.Getenv, os.Hostname)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

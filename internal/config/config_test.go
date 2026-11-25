package config_test

import (
	"testing"
	"time"

	"github.com/propedeutica/svmm-agent/internal/config"
)

func host() (string, error) { return "h1", nil }

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	cfg, err := config.Load(nil, env(nil), host)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerAddr != config.DefaultServerAddr || cfg.AgentID != "svma-h1" || cfg.HeartbeatInterval != 30*time.Second || cfg.Insecure {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestFlagsOverrideEnv(t *testing.T) {
	cfg, err := config.Load([]string{"-server", "flag:1"}, env(map[string]string{
		"SVMM_SERVER_ADDR": "env:2", "AGENT_ID": "a1", "HEARTBEAT_INTERVAL": "5s", "SVMM_INSECURE": "true",
	}), host)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerAddr != "flag:1" || cfg.AgentID != "a1" || cfg.HeartbeatInterval != 5*time.Second || !cfg.Insecure {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestInvalidInterval(t *testing.T) {
	if _, err := config.Load(nil, env(map[string]string{"HEARTBEAT_INTERVAL": "nope"}), host); err == nil {
		t.Fatal("expected error")
	}
	if _, err := config.Load([]string{"-heartbeat-interval", "0s"}, env(nil), host); err == nil {
		t.Fatal("expected error for zero interval")
	}
}

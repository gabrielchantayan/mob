package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mob-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Write test config
	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
[daemon]
heartbeat_interval = "3m"
max_concurrent_agents = 10

[safety]
branch_prefix = "mob/"
command_blacklist = ["sudo", "rm -rf"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values
	if cfg.Daemon.HeartbeatInterval != "3m" {
		t.Errorf("expected heartbeat_interval '3m', got '%s'", cfg.Daemon.HeartbeatInterval)
	}
	if cfg.Daemon.MaxConcurrentAgents != 10 {
		t.Errorf("expected max_concurrent_agents 10, got %d", cfg.Daemon.MaxConcurrentAgents)
	}
	if cfg.Safety.BranchPrefix != "mob/" {
		t.Errorf("expected branch_prefix 'mob/', got '%s'", cfg.Safety.BranchPrefix)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create temp directory with empty config
	tmpDir, err := os.MkdirTemp("", "mob-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Should have defaults
	if cfg.Daemon.HeartbeatInterval != "2m" {
		t.Errorf("expected default heartbeat_interval '2m', got '%s'", cfg.Daemon.HeartbeatInterval)
	}
	if cfg.Daemon.MaxConcurrentAgents != 5 {
		t.Errorf("expected default max_concurrent_agents 5, got %d", cfg.Daemon.MaxConcurrentAgents)
	}
}

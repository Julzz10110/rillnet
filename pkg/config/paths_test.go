package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPath_EnvOverride(t *testing.T) {
	t.Setenv("RILLNET_CONFIG_PATH", "configs/config.staging.yaml")
	if got := ResolveConfigPath(); got != "configs/config.staging.yaml" {
		t.Fatalf("ResolveConfigPath() = %q, want env path", got)
	}
}

func TestResolveConfigPath_DefaultFile(t *testing.T) {
	t.Setenv("RILLNET_CONFIG_PATH", "")
	root := findModuleRoot(t)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	path := ResolveConfigPath()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %q: %v", path, err)
	}
}

func TestApplyEnvOverrides_Redis(t *testing.T) {
	t.Setenv("RILLNET_REDIS_ENABLED", "true")
	t.Setenv("RILLNET_REDIS_ADDRESS", "redis:6379")
	cfg := DefaultConfig()
	cfg.applyEnvOverrides()
	if !cfg.Redis.Enabled {
		t.Fatal("expected redis enabled")
	}
	if cfg.Redis.Address != "redis:6379" {
		t.Fatalf("redis address = %q", cfg.Redis.Address)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("go.mod not found")
		}
		dir = parent
	}
}

func TestLoadResolved(t *testing.T) {
	root := findModuleRoot(t)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadResolved()
	if err != nil {
		t.Fatalf("LoadResolved: %v", err)
	}
	if cfg.Server.Address == "" {
		t.Fatal("empty server address")
	}
}

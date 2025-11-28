package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"rillnet/pkg/config"

	"github.com/stretchr/testify/assert"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestLoad_UsesDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := config.Load("non-existent-config.yaml")
	assert.NoError(t, err)
	assert.Equal(t, ":8080", cfg.Server.Address)
	assert.Equal(t, ":8081", cfg.Signal.Address)
	assert.Equal(t, "info", cfg.Logging.Level)
}

func TestLoad_LoadsFromYAMLAndAppliesEnvOverrides(t *testing.T) {
	path := writeTempConfig(t, `
server:
  address: ":9000"
  read_timeout: 10s
  write_timeout: 15s

signal:
  address: ":9001"
  ping_interval: 5s
  pong_timeout: 10s

mesh:
  max_connections: 8
  health_check_interval: 5s
  reconnect_attempts: 5

monitoring:
  prometheus_enabled: true
  prometheus_port: 9100
  metrics_interval: 15s

logging:
  level: "debug"
  format: "json"
`)

	// Set env overrides
	t.Setenv("RILLNET_SERVER_ADDRESS", ":7000")
	t.Setenv("RILLNET_SIGNAL_ADDRESS", ":7001")
	t.Setenv("RILLNET_LOG_LEVEL", "warn")

	cfg, err := config.Load(path)
	assert.NoError(t, err)

	// YAML values
	assert.Equal(t, 10*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 8, cfg.Mesh.MaxConnections)
	assert.Equal(t, 5*time.Second, cfg.Mesh.HealthCheckInterval)
	assert.Equal(t, 5, cfg.Mesh.ReconnectAttempts)
	assert.True(t, cfg.Monitoring.PrometheusEnabled)
	assert.Equal(t, 9100, cfg.Monitoring.PrometheusPort)
	assert.Equal(t, 15*time.Second, cfg.Monitoring.MetricsInterval)
	assert.Equal(t, "json", cfg.Logging.Format)

	// Env overrides
	assert.Equal(t, ":7000", cfg.Server.Address)
	assert.Equal(t, ":7001", cfg.Signal.Address)
	assert.Equal(t, "warn", cfg.Logging.Level)
}

func TestLoad_InvalidConfigFailsValidation(t *testing.T) {
	path := writeTempConfig(t, `
server:
  address: ""
  read_timeout: 0s
  write_timeout: 0s

signal:
  address: ""
  ping_interval: 0s
  pong_timeout: 0s

mesh:
  max_connections: 0
  health_check_interval: 0s
  reconnect_attempts: -1

monitoring:
  prometheus_enabled: true
  prometheus_port: 0
  metrics_interval: 0s

logging:
  level: ""
  format: "json"
`)

	_, err := config.Load(path)
	assert.Error(t, err)
}

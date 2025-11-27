package config

import (
	"os"
	"time"
)

type Config struct {
	Server struct {
		Address      string        `yaml:"address"`
		ReadTimeout  time.Duration `yaml:"read_timeout"`
		WriteTimeout time.Duration `yaml:"write_timeout"`
	} `yaml:"server"`

	Signal struct {
		Address      string        `yaml:"address"`
		PingInterval time.Duration `yaml:"ping_interval"`
		PongTimeout  time.Duration `yaml:"pong_timeout"`
	} `yaml:"signal"`

	WebRTC struct {
		ICEServers []struct {
			URLs []string `yaml:"urls"`
		} `yaml:"ice_servers"`
		PortRange struct {
			Min uint16 `yaml:"min"`
			Max uint16 `yaml:"max"`
		} `yaml:"port_range"`
		Simulcast  bool `yaml:"simulcast"`
		MaxBitrate int  `yaml:"max_bitrate"`
	} `yaml:"webrtc"`

	Mesh struct {
		MaxConnections      int           `yaml:"max_connections"`
		HealthCheckInterval time.Duration `yaml:"health_check_interval"`
		ReconnectAttempts   int           `yaml:"reconnect_attempts"`
	} `yaml:"mesh"`

	Monitoring struct {
		PrometheusEnabled bool          `yaml:"prometheus_enabled"`
		PrometheusPort    int           `yaml:"prometheus_port"`
		MetricsInterval   time.Duration `yaml:"metrics_interval"`
	} `yaml:"monitoring"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`
}

func Load(configPath string) (*Config, error) {
	// Check if the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, err
	}

	// If file exists, load it
	// (YAML loading implementation should be here)
	// For now, return default configuration
	return DefaultConfig(), nil
}

func DefaultConfig() *Config {
	cfg := &Config{}

	// Default values
	cfg.Server.Address = ":8080"
	cfg.Signal.Address = ":8081"
	cfg.Logging.Level = "info"

	return cfg
}

func (c *Config) applyEnvOverrides() {
	// Apply environment variable overrides
	if addr := os.Getenv("RILLNET_SERVER_ADDRESS"); addr != "" {
		c.Server.Address = addr
	}
	if addr := os.Getenv("RILLNET_SIGNAL_ADDRESS"); addr != "" {
		c.Signal.Address = addr
	}
	if level := os.Getenv("RILLNET_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
}

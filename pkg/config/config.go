package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		Address         string        `yaml:"address"`
		ReadTimeout     time.Duration `yaml:"read_timeout"`
		WriteTimeout    time.Duration `yaml:"write_timeout"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"server"`

	Signal struct {
		Address         string        `yaml:"address"`
		PingInterval    time.Duration `yaml:"ping_interval"`
		PongTimeout     time.Duration `yaml:"pong_timeout"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	} `yaml:"signal"`

	WebRTC struct {
		ICEServers []struct {
			URLs       []string `yaml:"urls"`
			Username   string   `yaml:"username,omitempty"`
			Credential string   `yaml:"credential,omitempty"`
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

	Redis struct {
		Enabled  bool   `yaml:"enabled"`
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		PoolSize int    `yaml:"pool_size"`
	} `yaml:"redis"`

	Auth struct {
		JWTSecret        string        `yaml:"jwt_secret"`
		AccessTokenTTL   time.Duration `yaml:"access_token_ttl"`
		RefreshTokenTTL  time.Duration `yaml:"refresh_token_ttl"`
		AllowedOrigins   []string      `yaml:"allowed_origins"`
	} `yaml:"auth"`

	RateLimiting struct {
		Enabled bool `yaml:"enabled"`

		HTTP struct {
			RequestsPerSecond   float64 `yaml:"requests_per_second"`
			Burst               int     `yaml:"burst"`
			MaxConcurrent       int     `yaml:"max_concurrent"` // global concurrent HTTP requests
		} `yaml:"http"`

		WebSocket struct {
			ConnectionsPerMinute   int     `yaml:"connections_per_minute"`
			MessagesPerSecond      float64 `yaml:"messages_per_second"`
			Burst                  int     `yaml:"burst"`
			MaxConcurrent          int     `yaml:"max_concurrent_connections"`
			MaxMessageSizeBytes    int64   `yaml:"max_message_size_bytes"`
		} `yaml:"websocket"`
	} `yaml:"rate_limiting"`
}

// Validate checks that configuration values are within acceptable ranges.
func (c *Config) Validate() error {
	// Server
	if c.Server.Address == "" {
		return fmt.Errorf("server.address must not be empty")
	}
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server.read_timeout must be > 0")
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("server.write_timeout must be > 0")
	}
	if c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server.shutdown_timeout must be > 0")
	}

	// Signal
	if c.Signal.Address == "" {
		return fmt.Errorf("signal.address must not be empty")
	}
	if c.Signal.PingInterval <= 0 {
		return fmt.Errorf("signal.ping_interval must be > 0")
	}
	if c.Signal.PongTimeout <= 0 {
		return fmt.Errorf("signal.pong_timeout must be > 0")
	}
	if c.Signal.ShutdownTimeout <= 0 {
		return fmt.Errorf("signal.shutdown_timeout must be > 0")
	}

	// WebRTC
	if c.WebRTC.PortRange.Min > 0 || c.WebRTC.PortRange.Max > 0 {
		if c.WebRTC.PortRange.Min == 0 || c.WebRTC.PortRange.Max == 0 {
			return fmt.Errorf("webrtc.port_range.min and max must both be set when one is set")
		}
		if c.WebRTC.PortRange.Min >= c.WebRTC.PortRange.Max {
			return fmt.Errorf("webrtc.port_range.min must be < max")
		}
	}

	// Mesh
	if c.Mesh.MaxConnections <= 0 {
		return fmt.Errorf("mesh.max_connections must be > 0")
	}
	if c.Mesh.HealthCheckInterval <= 0 {
		return fmt.Errorf("mesh.health_check_interval must be > 0")
	}
	if c.Mesh.ReconnectAttempts < 0 {
		return fmt.Errorf("mesh.reconnect_attempts must be >= 0")
	}

	// Monitoring
	if c.Monitoring.PrometheusEnabled && c.Monitoring.PrometheusPort <= 0 {
		return fmt.Errorf("monitoring.prometheus_port must be > 0 when prometheus_enabled=true")
	}
	if c.Monitoring.MetricsInterval <= 0 {
		return fmt.Errorf("monitoring.metrics_interval must be > 0")
	}

	// Logging
	if c.Logging.Level == "" {
		return fmt.Errorf("logging.level must not be empty")
	}

	// Redis
	if c.Redis.Enabled {
		if c.Redis.Address == "" {
			return fmt.Errorf("redis.address must not be empty when redis.enabled=true")
		}
		if c.Redis.PoolSize <= 0 {
			return fmt.Errorf("redis.pool_size must be > 0 when redis.enabled=true")
		}
	}

	// Auth
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("auth.jwt_secret must not be empty")
	}
	if c.Auth.AccessTokenTTL <= 0 {
		return fmt.Errorf("auth.access_token_ttl must be > 0")
	}
	if c.Auth.RefreshTokenTTL <= 0 {
		return fmt.Errorf("auth.refresh_token_ttl must be > 0")
	}

	// Rate limiting
	if c.RateLimiting.Enabled {
		if c.RateLimiting.HTTP.RequestsPerSecond <= 0 {
			return fmt.Errorf("rate_limiting.http.requests_per_second must be > 0 when rate limiting is enabled")
		}
		if c.RateLimiting.HTTP.Burst <= 0 {
			return fmt.Errorf("rate_limiting.http.burst must be > 0 when rate limiting is enabled")
		}
		if c.RateLimiting.HTTP.MaxConcurrent < 0 {
			return fmt.Errorf("rate_limiting.http.max_concurrent must be >= 0 when rate limiting is enabled")
		}
		if c.RateLimiting.WebSocket.ConnectionsPerMinute <= 0 {
			return fmt.Errorf("rate_limiting.websocket.connections_per_minute must be > 0 when rate limiting is enabled")
		}
		if c.RateLimiting.WebSocket.MessagesPerSecond <= 0 {
			return fmt.Errorf("rate_limiting.websocket.messages_per_second must be > 0 when rate limiting is enabled")
		}
		if c.RateLimiting.WebSocket.Burst <= 0 {
			return fmt.Errorf("rate_limiting.websocket.burst must be > 0 when rate limiting is enabled")
		}
		if c.RateLimiting.WebSocket.MaxConcurrent < 0 {
			return fmt.Errorf("rate_limiting.websocket.max_concurrent_connections must be >= 0 when rate limiting is enabled")
		}
		if c.RateLimiting.WebSocket.MaxMessageSizeBytes < 0 {
			return fmt.Errorf("rate_limiting.websocket.max_message_size_bytes must be >= 0 when rate limiting is enabled")
		}
	}

	return nil
}

// Load reads configuration from YAML file, applies defaults and env overrides.
func Load(configPath string) (*Config, error) {
	// If file does not exist, fall back to defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		cfg.applyEnvOverrides()
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config yaml: %w", err)
	}

	cfg.applyEnvOverrides()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// DefaultConfig returns configuration with sane defaults.
func DefaultConfig() *Config {
	cfg := &Config{}

	// Default values
	cfg.Server.Address = ":8080"
	cfg.Server.ReadTimeout = 30 * time.Second
	cfg.Server.WriteTimeout = 30 * time.Second
	cfg.Server.ShutdownTimeout = 30 * time.Second

	cfg.Signal.Address = ":8081"
	cfg.Signal.PingInterval = 30 * time.Second
	cfg.Signal.PongTimeout = 60 * time.Second
	cfg.Signal.ShutdownTimeout = 30 * time.Second

	cfg.Mesh.MaxConnections = 4
	cfg.Mesh.HealthCheckInterval = 10 * time.Second
	cfg.Mesh.ReconnectAttempts = 3

	cfg.Monitoring.PrometheusEnabled = true
	cfg.Monitoring.PrometheusPort = 9090
	cfg.Monitoring.MetricsInterval = 30 * time.Second

	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"

	cfg.Redis.Enabled = false
	cfg.Redis.Address = "localhost:6379"
	cfg.Redis.DB = 0
	cfg.Redis.PoolSize = 10

	cfg.Auth.JWTSecret = "change-me-in-production"
	cfg.Auth.AccessTokenTTL = 15 * time.Minute
	cfg.Auth.RefreshTokenTTL = 7 * 24 * time.Hour // 7 days
	cfg.Auth.AllowedOrigins = []string{"*"}

	// Rate limiting defaults (disabled by default)
	cfg.RateLimiting.Enabled = false
	cfg.RateLimiting.HTTP.RequestsPerSecond = 50
	cfg.RateLimiting.HTTP.Burst = 100
	cfg.RateLimiting.HTTP.MaxConcurrent = 0
	cfg.RateLimiting.WebSocket.ConnectionsPerMinute = 60
	cfg.RateLimiting.WebSocket.MessagesPerSecond = 100
	cfg.RateLimiting.WebSocket.Burst = 200
	cfg.RateLimiting.WebSocket.MaxConcurrent = 0
	cfg.RateLimiting.WebSocket.MaxMessageSizeBytes = 64 * 1024

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
	if secret := os.Getenv("RILLNET_JWT_SECRET"); secret != "" {
		c.Auth.JWTSecret = secret
	}
}

package config

import (
	"testing"
	"time"
)

// helper to build a minimal valid config that can be tweaked in tests.
func validBaseConfig() *Config {
	cfg := DefaultConfig()
	cfg.RateLimiting.Enabled = true
	cfg.RateLimiting.HTTP.RequestsPerSecond = 10
	cfg.RateLimiting.HTTP.Burst = 20
	cfg.RateLimiting.HTTP.MaxConcurrent = 5
	cfg.RateLimiting.WebSocket.ConnectionsPerMinute = 60
	cfg.RateLimiting.WebSocket.MessagesPerSecond = 50
	cfg.RateLimiting.WebSocket.Burst = 100
	cfg.RateLimiting.WebSocket.MaxConcurrent = 10
	cfg.RateLimiting.WebSocket.MaxMessageSizeBytes = 65536
	return cfg
}

func TestValidate_RateLimitingDisabled_AllowsZeroValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RateLimiting.Enabled = false
	// Zero out rate limiting values to ensure they are ignored when disabled.
	cfg.RateLimiting.HTTP.RequestsPerSecond = 0
	cfg.RateLimiting.HTTP.Burst = 0
	cfg.RateLimiting.HTTP.MaxConcurrent = 0
	cfg.RateLimiting.WebSocket.ConnectionsPerMinute = 0
	cfg.RateLimiting.WebSocket.MessagesPerSecond = 0
	cfg.RateLimiting.WebSocket.Burst = 0
	cfg.RateLimiting.WebSocket.MaxConcurrent = 0
	cfg.RateLimiting.WebSocket.MaxMessageSizeBytes = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid when rate limiting disabled, got error: %v", err)
	}
}

func TestValidate_RateLimiting_InvalidValues(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Config)
	}{
		{
			name: "http rps must be > 0",
			mutate: func(c *Config) {
				c.RateLimiting.HTTP.RequestsPerSecond = 0
			},
		},
		{
			name: "http burst must be > 0",
			mutate: func(c *Config) {
				c.RateLimiting.HTTP.Burst = 0
			},
		},
		{
			name: "http max concurrent must be >= 0",
			mutate: func(c *Config) {
				c.RateLimiting.HTTP.MaxConcurrent = -1
			},
		},
		{
			name: "ws connections per minute must be > 0",
			mutate: func(c *Config) {
				c.RateLimiting.WebSocket.ConnectionsPerMinute = 0
			},
		},
		{
			name: "ws messages per second must be > 0",
			mutate: func(c *Config) {
				c.RateLimiting.WebSocket.MessagesPerSecond = 0
			},
		},
		{
			name: "ws burst must be > 0",
			mutate: func(c *Config) {
				c.RateLimiting.WebSocket.Burst = 0
			},
		},
		{
			name: "ws max concurrent must be >= 0",
			mutate: func(c *Config) {
				c.RateLimiting.WebSocket.MaxConcurrent = -1
			},
		},
		{
			name: "ws max message size must be >= 0",
			mutate: func(c *Config) {
				c.RateLimiting.WebSocket.MaxMessageSizeBytes = -1
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaseConfig()
			// ensure other timing fields are valid to isolate rate limiting
			cfg.Server.ReadTimeout = time.Second
			cfg.Server.WriteTimeout = time.Second
			cfg.Signal.PingInterval = time.Second
			cfg.Signal.PongTimeout = time.Second
			tc.mutate(cfg)

			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected validation error for case %q, got nil", tc.name)
			}
		})
	}
}



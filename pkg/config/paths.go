package config

import "os"

// DefaultConfigSearchPaths lists config files in priority order (first existing wins).
var DefaultConfigSearchPaths = []string{
	"configs/config.yaml",
	"./configs/config.yaml",
	"/root/configs/config.yaml",
	"config.yaml",
}

// ResolveConfigPath returns the config file path to load.
// RILLNET_CONFIG_PATH overrides search order.
func ResolveConfigPath() string {
	if p := os.Getenv("RILLNET_CONFIG_PATH"); p != "" {
		return p
	}
	for _, p := range DefaultConfigSearchPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "configs/config.yaml"
}

// LoadResolved loads configuration from ResolveConfigPath().
func LoadResolved() (*Config, error) {
	return Load(ResolveConfigPath())
}

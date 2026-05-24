package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validatedConfigPath returns a cleaned config path safe for os.ReadFile.
func validatedConfigPath(configPath string) (string, error) {
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return "", fmt.Errorf("empty config path")
	}

	clean := filepath.Clean(configPath)
	if clean == "." || strings.Contains(clean, "..") {
		return "", fmt.Errorf("invalid config path: %s", configPath)
	}

	return clean, nil
}

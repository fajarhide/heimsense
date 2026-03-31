package config

import (
	"bufio"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv reads KEY=VALUE pairs from a .env file and sets them as
// environment variables, but only if they are not already set.
// It looks for $HOME/.heimsense/.env.
func LoadDotEnv() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".heimsense", ".env")

	f, err := os.Open(path)
	if err != nil {
		return // file doesn't exist — that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Shell env takes precedence
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("error reading .env file", "path", path, "error", err)
	}
}

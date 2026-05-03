package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// MigrateFromDotEnv reads ~/.heimsense/.env, converts it to config.toml,
// writes the TOML file, and renames the .env file to .env.backup.
func MigrateFromDotEnv() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	envPath := filepath.Join(home, ".heimsense", ".env")
	tomlPath := ConfigFile()

	// Parse existing .env (we'll implement parseEnvFile locally or use dotenv parser)
	LoadDotEnv()

	root := RootConfig{
		Server: ServerConfig{
			ListenAddr: envOrDefault("LISTEN_ADDR", ":8080"),
		},
		Omni: OmniConfig{
			Enabled:         false,
			MCPURL:          "http://localhost:7070",
			MinContentBytes: 1024,
		},
		ModelMap: ModelMapConfig{
			Haiku:  os.Getenv("MODEL_MAP_HAIKU"),
			Sonnet: os.Getenv("MODEL_MAP_SONNET"),
			Opus:   os.Getenv("MODEL_MAP_OPUS"),
		},
	}

	timeoutMs, _ := strconv.Atoi(envOrDefault("REQUEST_TIMEOUT_MS", "120000"))
	root.Server.RequestTimeoutMs = timeoutMs
	if root.Server.RequestTimeoutMs == 0 {
		root.Server.RequestTimeoutMs = 120000
	}

	retries, _ := strconv.Atoi(envOrDefault("MAX_RETRIES", "3"))
	if retries == 0 {
		retries = 3
	}

	// Migrate main provider
	mainProvider := ProviderConfig{
		Name:         "primary",
		BaseURL:      envOrDefault("ANTHROPIC_BASE_URL", "https://api.openai.com/v1"),
		APIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		DefaultModel: envOrDefault("ANTHROPIC_CUSTOM_MODEL_OPTION", ""),
		ForceModel:   envOrDefault("ANTHROPIC_CUSTOM_FORCE_MODEL", ""),
		Priority:     1,
		MaxRetries:   retries,
	}
	root.Providers = []ProviderConfig{mainProvider}

	// Write to TOML
	f, err := os.OpenFile(tomlPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating config.toml: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(root); err != nil {
		return fmt.Errorf("encoding toml: %w", err)
	}

	// Rename .env to .env.backup
	backupPath := filepath.Join(home, ".heimsense", ".env.backup")
	if err := os.Rename(envPath, backupPath); err != nil {
		return fmt.Errorf("backing up .env: %w", err)
	}

	fmt.Printf("Migrated config from .env to config.toml. Old .env backed up to .env.backup\n")
	return nil
}

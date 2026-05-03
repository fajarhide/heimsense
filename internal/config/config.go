package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// ListenAddr is the address the server listens on (default ":8080").
	ListenAddr string

	// UpstreamBaseURL is the OpenAI-compatible API base URL.
	UpstreamBaseURL string

	// APIKey is the default API key sent upstream if the client doesn't provide one.
	APIKey string

	// DefaultModel is the fallback model when the request doesn't specify one.
	DefaultModel string

	// ForceModel overrides the model requested by the client to be this model.
	ForceModel string

	// RequestTimeout is the maximum duration for upstream requests.
	RequestTimeout time.Duration

	// MaxRetries is the number of retry attempts for transient upstream failures.
	MaxRetries int

	// ModelMapHaiku overrides any Claude Haiku requests locally.
	ModelMapHaiku string

	// ModelMapSonnet overrides any Claude Sonnet requests locally.
	ModelMapSonnet string

	// ModelMapOpus overrides any Claude Opus requests locally.
	ModelMapOpus string

	// OmniEnabled determines if the Omni distillation hook is active.
	OmniEnabled bool

	// OmniMCPURL is the local URL to the Omni MCP server.
	OmniMCPURL string

	// OmniMinContentBytes is the minimum size of a tool_result to trigger distillation.
	OmniMinContentBytes int

	// Providers is the list of upstream providers for the fallback chain.
	Providers []ProviderConfig
}

// Load reads configuration. It first tries to load config.toml.
// If config.toml doesn't exist but .env does, it migrates .env to config.toml.
func Load() (*Config, error) {
	// Check if TOML exists
	if _, err := os.Stat(ConfigFile()); os.IsNotExist(err) {
		// Check if .env exists
		home, _ := os.UserHomeDir()
		envPath := filepath.Join(home, ".heimsense", ".env")
		if _, err := os.Stat(envPath); err == nil {
			// Migrate .env to config.toml
			if err := MigrateFromDotEnv(); err != nil {
				return nil, fmt.Errorf("migration failed: %w", err)
			}
		}
	}

	// Try loading from TOML, if it fails fallback to .env loading for extreme fallback
	if cfg, err := LoadTOML(); err == nil {
		return cfg, nil
	}

	// Fallback logic if TOML decoding failed or didn't exist and no .env existed
	LoadDotEnv()

	cfg := &Config{
		ListenAddr:      envOrDefault("LISTEN_ADDR", ":8080"),
		UpstreamBaseURL: envOrDefault("ANTHROPIC_BASE_URL", "https://api.openai.com/v1"),
		APIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		DefaultModel:    envOrDefault("ANTHROPIC_CUSTOM_MODEL_OPTION", ""),
		ForceModel:      envOrDefault("ANTHROPIC_CUSTOM_FORCE_MODEL", ""),
		MaxRetries:      3,
		ModelMapHaiku:   os.Getenv("MODEL_MAP_HAIKU"),
		ModelMapSonnet:  os.Getenv("MODEL_MAP_SONNET"),
		ModelMapOpus:    os.Getenv("MODEL_MAP_OPUS"),
	}

	timeoutMs, err := strconv.Atoi(envOrDefault("REQUEST_TIMEOUT_MS", "120000"))
	if err != nil {
		return nil, fmt.Errorf("invalid REQUEST_TIMEOUT_MS: %w", err)
	}
	cfg.RequestTimeout = time.Duration(timeoutMs) * time.Millisecond

	retries := envOrDefault("MAX_RETRIES", "3")
	if r, err := strconv.Atoi(retries); err == nil {
		cfg.MaxRetries = r
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

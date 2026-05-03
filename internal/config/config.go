package config

import (
	"fmt"
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

// Load reads configuration. It only loads from config.toml now.
func Load() (*Config, error) {
	// Try loading from TOML
	if cfg, err := LoadTOML(); err == nil {
		return cfg, nil
	}

	return nil, fmt.Errorf("failed to load config.toml, please run 'heimsense setup'")
}

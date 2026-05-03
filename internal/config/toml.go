package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// RootConfig represents the structure of the config.toml file
type RootConfig struct {
	Server   ServerConfig     `toml:"server"`
	Omni     OmniConfig       `toml:"omni"`
	Providers []ProviderConfig `toml:"providers"`
	ModelMap ModelMapConfig   `toml:"model_map"`
}

type ServerConfig struct {
	ListenAddr       string `toml:"listen_addr"`
	RequestTimeoutMs int    `toml:"request_timeout_ms"`
}

type OmniConfig struct {
	Enabled         bool   `toml:"enabled"`
	MCPURL          string `toml:"mcp_url"`
	MinContentBytes int    `toml:"min_content_bytes"`
}

type ProviderConfig struct {
	Name         string `toml:"name"`
	BaseURL      string `toml:"base_url"`
	APIKey       string `toml:"api_key"`
	DefaultModel string `toml:"default_model"`
	ForceModel   string `toml:"force_model"`
	Priority     int    `toml:"priority"`
	MaxRetries   int    `toml:"max_retries"`
}

type ModelMapConfig struct {
	Haiku  string `toml:"haiku"`
	Sonnet string `toml:"sonnet"`
	Opus   string `toml:"opus"`
}

// ConfigFile returns the path to ~/.heimsense/config.toml
func ConfigFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".heimsense", "config.toml")
}

// LoadTOML loads the TOML configuration from the default path.
func LoadTOML() (*Config, error) {
	path := ConfigFile()
	var root RootConfig
	if _, err := toml.DecodeFile(path, &root); err != nil {
		return nil, fmt.Errorf("decoding toml: %w", err)
	}

	return rootToConfig(root)
}

// rootToConfig converts the TOML representation into the internal Config model.
func rootToConfig(root RootConfig) (*Config, error) {
	cfg := &Config{
		ListenAddr:      root.Server.ListenAddr,
		RequestTimeout:  time.Duration(root.Server.RequestTimeoutMs) * time.Millisecond,
		ModelMapHaiku:   root.ModelMap.Haiku,
		ModelMapSonnet:  root.ModelMap.Sonnet,
		ModelMapOpus:    root.ModelMap.Opus,
		OmniEnabled:     root.Omni.Enabled,
		OmniMCPURL:      root.Omni.MCPURL,
		OmniMinContentBytes: root.Omni.MinContentBytes,
		Providers:       root.Providers,
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 120 * time.Second
	}
	if cfg.OmniMCPURL == "" {
		cfg.OmniMCPURL = "http://localhost:7070"
	}
	if cfg.OmniMinContentBytes == 0 {
		cfg.OmniMinContentBytes = 1024
	}

	// For backward compatibility until we fully move routing logic to use Providers array
	if len(root.Providers) > 0 {
		cfg.UpstreamBaseURL = root.Providers[0].BaseURL
		cfg.APIKey = root.Providers[0].APIKey
		cfg.DefaultModel = root.Providers[0].DefaultModel
		cfg.ForceModel = root.Providers[0].ForceModel
		cfg.MaxRetries = root.Providers[0].MaxRetries
	}

	return cfg, nil
}

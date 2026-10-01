package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// ListenAddr is the address the server listens on (default "127.0.0.1:8080").
	ListenAddr string

	// ProxyAPIKey, when set, is the credential a caller must present to reach
	// /v1/messages. Empty means no inbound authentication, which is only safe
	// on a loopback address. See Config.Validate.
	ProxyAPIKey string

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
}

// Load reads configuration from environment variables with sensible defaults.
// It first loads ~/.heimsense/.env if present (shell env vars take precedence).
func Load() (*Config, error) {
	LoadDotEnv()

	cfg := &Config{
		ListenAddr:      envOrDefault("LISTEN_ADDR", "127.0.0.1:8080"),
		UpstreamBaseURL: envOrDefault("ANTHROPIC_BASE_URL", "https://api.openai.com/v1"),
		APIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		ProxyAPIKey:     os.Getenv("PROXY_API_KEY"),
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

// Validate refuses a configuration that would serve the operator's upstream key
// to anyone who can reach the port.
//
// Without a ProxyAPIKey there is no inbound authentication, and a request that
// sends no Authorization header falls back to the configured upstream key. That
// is fine on loopback, where only local processes can reach it, and it is an
// open relay on any other address. Kept out of Load so that reading the
// configuration stays free of policy.
func (c *Config) Validate() error {
	if c.ProxyAPIKey != "" || isLoopbackAddr(c.ListenAddr) {
		return nil
	}

	return fmt.Errorf(
		"LISTEN_ADDR=%q is reachable from the network but PROXY_API_KEY is unset, "+
			"so any caller could spend the configured upstream key: set PROXY_API_KEY, "+
			"or bind a loopback address such as 127.0.0.1:8080",
		c.ListenAddr,
	)
}

// isLoopbackAddr reports whether addr accepts connections only from this host.
// A missing host, as in ":8080", means every interface and is not loopback.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

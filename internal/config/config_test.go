package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	// Create a temp .env file
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, ".heimsense")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(envDir, ".env")
	content := `# Test config
ANTHROPIC_BASE_URL=https://api.test.com/v1
ANTHROPIC_API_KEY=test-key-123
ANTHROPIC_CUSTOM_MODEL_OPTION=gpt-test
ANTHROPIC_CUSTOM_FORCE_MODEL=
LISTEN_ADDR=:9090
REQUEST_TIMEOUT_MS=30000
MAX_RETRIES=5
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME to point to temp dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Clear relevant env vars so .env values get picked up
	for _, key := range []string{
		"ANTHROPIC_BASE_URL", "ANTHROPIC_API_KEY",
		"ANTHROPIC_CUSTOM_MODEL_OPTION", "ANTHROPIC_CUSTOM_FORCE_MODEL",
		"LISTEN_ADDR", "REQUEST_TIMEOUT_MS", "MAX_RETRIES",
	} {
		os.Unsetenv(key)
	}

	// Load config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify values from .env
	if cfg.UpstreamBaseURL != "https://api.test.com/v1" {
		t.Errorf("UpstreamBaseURL = %q, want %q", cfg.UpstreamBaseURL, "https://api.test.com/v1")
	}
	if cfg.APIKey != "test-key-123" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test-key-123")
	}
	if cfg.DefaultModel != "gpt-test" {
		t.Errorf("DefaultModel = %q, want %q", cfg.DefaultModel, "gpt-test")
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, 5)
	}
}

func TestLoadDotEnv_ShellEnvTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	envDir := filepath.Join(tmpDir, ".heimsense")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	envFile := filepath.Join(envDir, ".env")
	if err := os.WriteFile(envFile, []byte("ANTHROPIC_API_KEY=from-dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set shell env BEFORE loading
	os.Setenv("ANTHROPIC_API_KEY", "from-shell")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.APIKey != "from-shell" {
		t.Errorf("APIKey = %q, want %q (shell should win)", cfg.APIKey, "from-shell")
	}
}

func TestLoadDotEnv_NoFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", "/nonexistent/path")
	defer os.Setenv("HOME", origHome)

	// Should not panic, just use defaults
	os.Unsetenv("LISTEN_ADDR")
	os.Unsetenv("ANTHROPIC_BASE_URL")
	os.Unsetenv("ANTHROPIC_API_KEY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want default :8080", cfg.ListenAddr)
	}
	if cfg.UpstreamBaseURL != "https://api.openai.com/v1" {
		t.Errorf("UpstreamBaseURL = %q, want default", cfg.UpstreamBaseURL)
	}
}

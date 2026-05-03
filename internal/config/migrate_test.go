package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateFromDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Unset env vars that might leak from other tests
	os.Unsetenv("LISTEN_ADDR")
	os.Unsetenv("ANTHROPIC_BASE_URL")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_CUSTOM_MODEL_OPTION")
	os.Unsetenv("MAX_RETRIES")

	// Create a mock .env file
	heimsenseDir := filepath.Join(tmpDir, ".heimsense")
	os.MkdirAll(heimsenseDir, 0o755)
	
	envContent := `
LISTEN_ADDR=:9090
ANTHROPIC_BASE_URL=https://api.deepseek.com/v1
ANTHROPIC_API_KEY=sk-test123
ANTHROPIC_CUSTOM_MODEL_OPTION=deepseek-chat
MAX_RETRIES=5
`
	envPath := filepath.Join(heimsenseDir, ".env")
	os.WriteFile(envPath, []byte(envContent), 0o644)

	// Run migration
	err := MigrateFromDotEnv()
	if err != nil {
		t.Fatalf("MigrateFromDotEnv failed: %v", err)
	}

	// Verify .env was backed up
	if _, err := os.Stat(filepath.Join(heimsenseDir, ".env.backup")); os.IsNotExist(err) {
		t.Error("expected .env.backup to exist")
	}

	// Verify config.toml was created
	tomlPath := filepath.Join(heimsenseDir, "config.toml")
	if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
		t.Fatal("expected config.toml to exist")
	}

	// Try loading the generated TOML
	cfg, err := LoadTOML()
	if err != nil {
		t.Fatalf("failed to load generated TOML: %v", err)
	}

	// Assert mapped values
	if cfg.ListenAddr != ":9090" {
		t.Errorf("expected ListenAddr ':9090', got '%s'", cfg.ListenAddr)
	}
	
	if len(cfg.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(cfg.Providers))
	}
	
	p := cfg.Providers[0]
	if p.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("expected BaseURL 'https://api.deepseek.com/v1', got '%s'", p.BaseURL)
	}
	if p.APIKey != "sk-test123" {
		t.Errorf("expected APIKey 'sk-test123', got '%s'", p.APIKey)
	}
	if p.DefaultModel != "deepseek-chat" {
		t.Errorf("expected DefaultModel 'deepseek-chat', got '%s'", p.DefaultModel)
	}
	if p.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", p.MaxRetries)
	}
}

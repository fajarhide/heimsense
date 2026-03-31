package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNeedsSetup(t *testing.T) {
	// Point HOME to a temp dir without .heimsense
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	if !NeedsSetup() {
		t.Error("NeedsSetup() = false, want true when no config exists")
	}

	// Create the config file
	cfgDir := filepath.Join(tmpDir, ".heimsense")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, ".env"), []byte("KEY=value\n"), 0o644)

	if NeedsSetup() {
		t.Error("NeedsSetup() = true, want false when config exists")
	}
}

func TestWriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := SetupConfig{
		BaseURL:    "https://api.test.com/v1",
		APIKey:     "sk-test-key-123",
		Model:      "gpt-test",
		ModelName:  "Test Model",
		ModelDesc:  "A test model",
		ListenAddr: ":9090",
	}

	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig() error: %v", err)
	}

	envPath := filepath.Join(tmpDir, ".heimsense", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading config file: %v", err)
	}

	s := string(content)
	checks := map[string]string{
		"ANTHROPIC_BASE_URL":                   "https://api.test.com/v1",
		"ANTHROPIC_API_KEY":                    "sk-test-key-123",
		"ANTHROPIC_CUSTOM_MODEL_OPTION=":       "gpt-test",
		"ANTHROPIC_CUSTOM_MODEL_OPTION_NAME=":  "Test Model",
		"LISTEN_ADDR":                          ":9090",
	}
	for key, want := range checks {
		if !contains(s, key) || !contains(s, want) {
			t.Errorf("config missing %s=%s", key, want)
		}
	}

	// Verify file permissions (0600 — owner only)
	info, _ := os.Stat(envPath)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file perm = %o, want 0600", perm)
	}
}

func TestConfigureClaudeCode_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := SetupConfig{
		BaseURL:    "https://api.openai.com/v1",
		APIKey:     "sk-abc123",
		Model:      "gpt-5",
		ModelName:  "My Model",
		ModelDesc:  "My description",
		ListenAddr: ":8080",
	}

	if err := ConfigureClaudeCode(cfg); err != nil {
		t.Fatalf("ConfigureClaudeCode() error: %v", err)
	}

	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}

	env, ok := data["env"].(map[string]interface{})
	if !ok {
		t.Fatal("settings.json missing 'env' key")
	}

	if env["ANTHROPIC_BASE_URL"] != "http://localhost:8080" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want http://localhost:8080", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_CUSTOM_MODEL_OPTION"] != "gpt-5" {
		t.Errorf("ANTHROPIC_CUSTOM_MODEL_OPTION = %q, want gpt-5", env["ANTHROPIC_CUSTOM_MODEL_OPTION"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-abc123" {
		t.Errorf("ANTHROPIC_AUTH_TOKEN = %q, want sk-abc123", env["ANTHROPIC_AUTH_TOKEN"])
	}
}

func TestConfigureClaudeCode_MergeExisting(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create existing settings with custom keys
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	existing := map[string]interface{}{
		"$schema":     "https://json.schemastore.org/claude-code-settings.json",
		"customField": "should-persist",
		"env": map[string]interface{}{
			"EXISTING_VAR": "keep-this",
		},
	}
	raw, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), raw, 0o644)

	cfg := SetupConfig{
		BaseURL:    "https://api.deepseek.com/v1",
		APIKey:     "sk-deep",
		Model:      "deepseek-v3",
		ModelName:  "DeepSeek",
		ModelDesc:  "DeepSeek model",
		ListenAddr: ":3000",
	}

	if err := ConfigureClaudeCode(cfg); err != nil {
		t.Fatalf("ConfigureClaudeCode() error: %v", err)
	}

	updated, _ := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	var data map[string]interface{}
	json.Unmarshal(updated, &data)

	// Custom field should persist
	if data["customField"] != "should-persist" {
		t.Error("existing customField was lost during merge")
	}

	env := data["env"].(map[string]interface{})

	// Existing env var should persist
	if env["EXISTING_VAR"] != "keep-this" {
		t.Error("existing EXISTING_VAR was lost during merge")
	}

	// New values should be set
	if env["ANTHROPIC_CUSTOM_MODEL_OPTION"] != "deepseek-v3" {
		t.Errorf("model = %q, want deepseek-v3", env["ANTHROPIC_CUSTOM_MODEL_OPTION"])
	}

	// Custom port should flow to ANTHROPIC_BASE_URL
	if env["ANTHROPIC_BASE_URL"] != "http://localhost:3000" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want http://localhost:3000", env["ANTHROPIC_BASE_URL"])
	}

	// Backup should exist
	if _, err := os.Stat(filepath.Join(claudeDir, "settings.json.bak")); os.IsNotExist(err) {
		t.Error("backup file was not created")
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sk-abc123def456", "sk-a•••••••f456"},  // 15 chars: 4 + 7 dots + 4
		{"short", "•••••"},                       // 5 chars: all dots
		{"12345678", "••••••••"},                  // 8 chars: all dots (<=8)
		{"abcdefghij", "abcd••ghij"},             // 10 chars: 4 + 2 dots + 4
	}
	for _, tt := range tests {
		got := maskKey(tt.input)
		if got != tt.want {
			t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ANSI color helpers
const (
	cyan  = "\033[0;36m"
	green = "\033[0;32m"
	bold  = "\033[1m"
	dim   = "\033[2m"
	nc    = "\033[0m"
)

// Provider represents a supported LLM provider.
type Provider struct {
	Name    string
	BaseURL string
}

// providers is the list of known providers.
var providers = []Provider{
	{Name: "OpenAI", BaseURL: "https://api.openai.com/v1"},
	{Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1"},
	{Name: "Groq", BaseURL: "https://api.groq.com/openai/v1"},
	{Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1"},
	{Name: "Ollama (local)", BaseURL: "http://localhost:11434/v1"},
}

// SetupConfig holds user-provided setup values.
type SetupConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	ModelName  string
	ModelDesc  string
	ListenAddr string
}

// ConfigDir returns the path to ~/.heimsense.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".heimsense")
}

// ConfigPath returns the path to ~/.heimsense/.env.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), ".env")
}

// NeedsSetup returns true if ~/.heimsense/.env does not exist.
func NeedsSetup() bool {
	_, err := os.Stat(ConfigPath())
	return os.IsNotExist(err)
}

// RunWizard runs the interactive first-run setup wizard.
// It prompts the user for provider, API key, model, then writes
// config files and configures Claude Code.
func RunWizard() error {
	reader := bufio.NewReader(os.Stdin)

	printHeader()

	// 1. Provider selection
	baseURL, err := promptProvider(reader)
	if err != nil {
		return fmt.Errorf("provider selection: %w", err)
	}

	// 2. API Key (masked input)
	apiKey, err := promptAPIKey()
	if err != nil {
		return fmt.Errorf("api key input: %w", err)
	}

	// 3. Model name
	model, err := promptModel(reader)
	if err != nil {
		return fmt.Errorf("model input: %w", err)
	}

	// 4. Listen port
	listenAddr, err := promptPort(reader)
	if err != nil {
		return fmt.Errorf("port input: %w", err)
	}

	cfg := SetupConfig{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		ModelName:  "Heimsense Custom Model",
		ModelDesc:  "Custom model via Heimsense adapter",
		ListenAddr: listenAddr,
	}

	// Show summary
	fmt.Println()
	fmt.Printf("  %s┌─ Summary ─────────────────────────────────┐%s\n", dim, nc)
	fmt.Printf("  %s│%s  Provider   %s%s%s\n", dim, nc, cyan, cfg.BaseURL, nc)
	fmt.Printf("  %s│%s  API Key    %s%s%s\n", dim, nc, dim, maskKey(cfg.APIKey), nc)
	fmt.Printf("  %s│%s  Model      %s%s%s\n", dim, nc, cyan, cfg.Model, nc)
	fmt.Printf("  %s│%s  Listen     %s%s%s\n", dim, nc, cyan, cfg.ListenAddr, nc)
	fmt.Printf("  %s└────────────────────────────────────────────┘%s\n", dim, nc)
	fmt.Println()

	// 5. Write config
	if err := WriteConfig(cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Printf("  %s✓%s Config saved    %s~/.heimsense/.env%s\n", green, nc, dim, nc)

	// 6. Configure Claude Code
	if err := ConfigureClaudeCode(cfg); err != nil {
		fmt.Printf("  %s!%s Claude Code     %sskipped (%v)%s\n", "\033[1;33m", nc, dim, err, nc)
	} else {
		fmt.Printf("  %s✓%s Claude Code    %s~/.claude/settings.json%s\n", green, nc, dim, nc)
	}

	fmt.Println()
	fmt.Printf("  %s%sSetup complete!%s\n", bold, green, nc)
	fmt.Println()
	fmt.Printf("  %s1.%s Server will start on %s%s%s\n", bold, nc, cyan, cfg.ListenAddr, nc)
	fmt.Printf("  %s2.%s Open another terminal and run %sclaude%s\n", bold, nc, cyan, nc)
	fmt.Printf("  %s3.%s Inside Claude, run %s/model%s and select the custom model\n", bold, nc, cyan, nc)
	fmt.Println()
	fmt.Printf("  %sEdit config anytime: %s~/.heimsense/.env%s\n", dim, cyan, nc)
	fmt.Printf("  %sRe-run setup:        %sheimsense setup%s\n", dim, cyan, nc)
	fmt.Println()

	return nil
}

func printHeader() {
	fmt.Println()
	fmt.Printf("  %s%sHEIM·SENSE%s  %ssetup%s\n", bold, cyan, nc, dim, nc)
	fmt.Printf("  %sUnlock Your Claude Code for Any LLM%s\n", "\033[3;36m", nc)
	fmt.Println()
	fmt.Printf("  %sLet's configure your LLM provider.%s\n", dim, nc)
	fmt.Println()
}

func promptProvider(reader *bufio.Reader) (string, error) {
	fmt.Printf("  %sSelect your provider:%s\n\n", bold, nc)
	for i, p := range providers {
		fmt.Printf("    %s%d%s  %s  %s(%s)%s\n", bold, i+1, nc, p.Name, dim, p.BaseURL, nc)
	}
	fmt.Printf("    %s%d%s  Custom URL\n", bold, len(providers)+1, nc)
	fmt.Println()

	for {
		fmt.Printf("  %sChoice [1]: %s", bold, nc)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)

		if input == "" {
			input = "1"
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(providers)+1 {
			fmt.Printf("  %sPlease enter a number 1-%d%s\n", "\033[0;31m", len(providers)+1, nc)
			continue
		}

		if choice <= len(providers) {
			return providers[choice-1].BaseURL, nil
		}

		// Custom URL
		fmt.Printf("  %sBase URL: %s", bold, nc)
		customURL, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		customURL = strings.TrimSpace(customURL)
		if customURL == "" {
			fmt.Printf("  %sURL cannot be empty%s\n", "\033[0;31m", nc)
			continue
		}
		return customURL, nil
	}
}

func promptAPIKey() (string, error) {
	fmt.Printf("  %sAPI Key: %s", bold, nc)

	// Read password without echo
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		keyBytes, err := term.ReadPassword(fd)
		fmt.Println() // newline after hidden input
		if err != nil {
			return "", err
		}
		key := strings.TrimSpace(string(keyBytes))
		if key == "" {
			return "", fmt.Errorf("API key cannot be empty")
		}
		return key, nil
	}

	// Fallback for non-terminal (e.g. pipe)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(input)
	if key == "" {
		return "", fmt.Errorf("API key cannot be empty")
	}
	return key, nil
}

func promptModel(reader *bufio.Reader) (string, error) {
	fmt.Printf("  %sModel [gpt-4o-mini]: %s", bold, nc)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return "gpt-4o-mini", nil
	}
	return input, nil
}

func promptPort(reader *bufio.Reader) (string, error) {
	fmt.Printf("  %sPort [8080]: %s", bold, nc)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return ":8080", nil
	}
	// Validate it's a number
	port, err := strconv.Atoi(input)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port: %s (must be 1-65535)", input)
	}
	return fmt.Sprintf(":%d", port), nil
}

// maskKey returns a masked representation of an API key.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}

// WriteConfig writes the setup config to ~/.heimsense/.env.
func WriteConfig(cfg SetupConfig) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	content := fmt.Sprintf(`# Heimsense config — generated by setup wizard
# Edit this file to change settings, then restart heimsense.

ANTHROPIC_BASE_URL=%s
ANTHROPIC_API_KEY=%s
ANTHROPIC_CUSTOM_MODEL_OPTION=%s
ANTHROPIC_CUSTOM_MODEL_OPTION_NAME=%s
ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION=%s

LISTEN_ADDR=%s
REQUEST_TIMEOUT_MS=120000
MAX_RETRIES=3
`, cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.ModelName, cfg.ModelDesc, cfg.ListenAddr)

	return os.WriteFile(ConfigPath(), []byte(content), 0o600)
}

// ConfigureClaudeCode writes or updates ~/.claude/settings.json
// to point at the local Heimsense adapter.
func ConfigureClaudeCode(cfg SetupConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	claudeDir := filepath.Join(home, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}

	// Load existing or create new
	var data map[string]interface{}
	if raw, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			data = make(map[string]interface{})
		}
	} else {
		data = map[string]interface{}{
			"$schema": "https://json.schemastore.org/claude-code-settings.json",
		}
	}

	// Backup existing
	if _, err := os.Stat(settingsPath); err == nil {
		backupPath := settingsPath + ".bak"
		if raw, err := os.ReadFile(settingsPath); err == nil {
			os.WriteFile(backupPath, raw, 0o644)
		}
	}

	// Merge env settings
	env, ok := data["env"].(map[string]interface{})
	if !ok {
		env = make(map[string]interface{})
	}
	env["ANTHROPIC_BASE_URL"] = "http://localhost" + cfg.ListenAddr
	env["ANTHROPIC_CUSTOM_MODEL_OPTION"] = cfg.Model
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_NAME"] = cfg.ModelName
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION"] = cfg.ModelDesc
	env["ANTHROPIC_AUTH_TOKEN"] = cfg.APIKey
	env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
	data["env"] = env

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		return err
	}

	// Bypass onboarding in ~/.claude.json
	claudeJSON := filepath.Join(home, ".claude.json")
	if raw, err := os.ReadFile(claudeJSON); err == nil {
		var cj map[string]interface{}
		if json.Unmarshal(raw, &cj) == nil {
			cj["hasCompletedOnboarding"] = true
			if out, err := json.MarshalIndent(cj, "", "  "); err == nil {
				os.WriteFile(claudeJSON, append(out, '\n'), 0o644)
			}
		}
	}

	return nil
}

// SyncToClaude reads ~/.heimsense/.env, extracts settings, and updates
// ~/.claude/settings.json to match. This allows users to edit the .env
// file manually and sync changes without re-running the wizard.
func SyncToClaude() error {
	envPath := ConfigPath()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("config not found at %s — run 'heimsense setup' first", envPath)
	}

	env, err := parseEnvFile(envPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	listenAddr := env["LISTEN_ADDR"]
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	cfg := SetupConfig{
		BaseURL:    env["ANTHROPIC_BASE_URL"],
		APIKey:     env["ANTHROPIC_API_KEY"],
		Model:      env["ANTHROPIC_CUSTOM_MODEL_OPTION"],
		ModelName:  env["ANTHROPIC_CUSTOM_MODEL_OPTION_NAME"],
		ModelDesc:  env["ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION"],
		ListenAddr: listenAddr,
	}

	if cfg.ModelName == "" {
		cfg.ModelName = "Heimsense Custom Model"
	}
	if cfg.ModelDesc == "" {
		cfg.ModelDesc = "Custom model via Heimsense adapter"
	}

	if err := ConfigureClaudeCode(cfg); err != nil {
		return err
	}

	fmt.Printf("\n  %s%sHEIM·SENSE%s  %ssync%s\n\n", bold, cyan, nc, dim, nc)
	fmt.Printf("  %s✓%s Synced to %s~/.claude/settings.json%s\n\n", green, nc, dim, nc)
	fmt.Printf("  %s┌─ Synced values ────────────────────────────┐%s\n", dim, nc)
	fmt.Printf("  %s│%s  Provider   %s%s%s\n", dim, nc, cyan, cfg.BaseURL, nc)
	fmt.Printf("  %s│%s  API Key    %s%s%s\n", dim, nc, dim, maskKey(cfg.APIKey), nc)
	fmt.Printf("  %s│%s  Model      %s%s%s\n", dim, nc, cyan, cfg.Model, nc)
	fmt.Printf("  %s│%s  Listen     %s%s%s  →  %sANTHROPIC_BASE_URL=http://localhost%s%s\n", dim, nc, cyan, cfg.ListenAddr, nc, dim, cfg.ListenAddr, nc)
	fmt.Printf("  %s└────────────────────────────────────────────┘%s\n\n", dim, nc)

	return nil
}

// parseEnvFile reads a .env file and returns a map of key-value pairs.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)
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
		result[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return result, scanner.Err()
}

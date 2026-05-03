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
	"github.com/BurntSushi/toml"
	"github.com/fajarhide/heimsense/internal/config"
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

// NeedsSetup returns true if neither config.toml nor .env exists.
func NeedsSetup() bool {
	home, _ := os.UserHomeDir()
	tomlPath := filepath.Join(home, ".heimsense", "config.toml")
	envPath := filepath.Join(home, ".heimsense", ".env")

	_, errToml := os.Stat(tomlPath)
	_, errEnv := os.Stat(envPath)
	
	return os.IsNotExist(errToml) && os.IsNotExist(errEnv)
}

// RunWizard runs the interactive first-run setup wizard.
func RunWizard() error {
	reader := bufio.NewReader(os.Stdin)

	printHeader()

	// 1. Omni Integration
	omniEnabled := promptYesNo(reader, "Enable Omni token distillation? (Saves 30-90% tokens) [Y/n]")

	// 2. Fallback tier configuration
	useFallback := promptYesNo(reader, "Configure a fallback provider for reliability? [y/N]")

	// 3. Configure Provider 1
	fmt.Printf("\n  %s--- Configure Primary Provider ---%s\n", bold, nc)
	p1URL, err := promptProvider(reader)
	if err != nil { return err }
	p1Key, err := promptAPIKey()
	if err != nil { return err }
	p1Model, err := promptModel(reader)
	if err != nil { return err }

	var p2URL, p2Key, p2Model string
	if useFallback {
		fmt.Printf("\n  %s--- Configure Fallback Provider ---%s\n", bold, nc)
		p2URL, err = promptProvider(reader)
		if err != nil { return err }
		p2Key, err = promptAPIKey()
		if err != nil { return err }
		p2Model, err = promptModel(reader)
		if err != nil { return err }
	}

	// 4. Listen port
	fmt.Println()
	listenAddr, err := promptPort(reader)
	if err != nil {
		return fmt.Errorf("port input: %w", err)
	}

	// Assemble TOML Config
	cfg := config.RootConfig{
		Server: config.ServerConfig{
			ListenAddr:       listenAddr,
			RequestTimeoutMs: 120000,
		},
		Omni: config.OmniConfig{
			Enabled:         omniEnabled,
			MCPURL:          "http://localhost:7070",
			MinContentBytes: 1024,
		},
		ModelMap: config.ModelMapConfig{},
		Providers: []config.ProviderConfig{
			{
				Name:         "primary",
				BaseURL:      p1URL,
				APIKey:       p1Key,
				DefaultModel: p1Model,
				Priority:     1,
				MaxRetries:   3,
			},
		},
	}

	if useFallback {
		cfg.Providers = append(cfg.Providers, config.ProviderConfig{
			Name:         "fallback",
			BaseURL:      p2URL,
			APIKey:       p2Key,
			DefaultModel: p2Model,
			Priority:     2,
			MaxRetries:   3,
		})
	}

	// Show summary
	fmt.Println()
	fmt.Printf("  %s┌─ Summary ─────────────────────────────────┐%s\n", dim, nc)
	fmt.Printf("  %s│%s  Omni Distiller : %s%v%s\n", dim, nc, cyan, omniEnabled, nc)
	fmt.Printf("  %s│%s  Primary        : %s%s%s (%s)\n", dim, nc, cyan, p1URL, nc, p1Model)
	if useFallback {
		fmt.Printf("  %s│%s  Fallback       : %s%s%s (%s)\n", dim, nc, cyan, p2URL, nc, p2Model)
	}
	fmt.Printf("  %s│%s  Listen Addr    : %s%s%s\n", dim, nc, cyan, listenAddr, nc)
	fmt.Printf("  %s└────────────────────────────────────────────┘%s\n", dim, nc)
	fmt.Println()

	// 5. Write config
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".heimsense")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.toml")

	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		f.Close()
		return fmt.Errorf("encoding toml: %w", err)
	}
	f.Close()
	fmt.Printf("  %s✓%s Config saved    %s~/.heimsense/config.toml%s\n", green, nc, dim, nc)

	// 6. Configure Claude Code
	if err := ConfigureClaudeCode(listenAddr, p1Model); err != nil {
		fmt.Printf("  %s!%s Claude Code     %sskipped (%v)%s\n", "\033[1;33m", nc, dim, err, nc)
	} else {
		fmt.Printf("  %s✓%s Claude Code    %s~/.claude/settings.json%s\n", green, nc, dim, nc)
	}

	fmt.Println()
	fmt.Printf("  %s%sSetup complete!%s\n", bold, green, nc)
	fmt.Println()
	fmt.Printf("  %s1.%s Server will start on %s%s%s\n", bold, nc, cyan, listenAddr, nc)
	fmt.Printf("  %s2.%s Open another terminal and run %sclaude%s\n", bold, nc, cyan, nc)
	fmt.Printf("  %s3.%s Inside Claude, run %s/model%s and select the custom model\n", bold, nc, cyan, nc)
	fmt.Println()

	return nil
}

func promptYesNo(reader *bufio.Reader, prompt string) bool {
	defaultYes := strings.Contains(prompt, "[Y/n]")
	for {
		fmt.Printf("  %s%s%s ", bold, prompt, nc)
		input, err := reader.ReadString('\n')
		if err != nil { return defaultYes }
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "" { return defaultYes }
		if input == "y" || input == "yes" { return true }
		if input == "n" || input == "no" { return false }
	}
}

func printHeader() {
	fmt.Println()
	fmt.Printf("  %s%sHEIM·SENSE%s  %ssetup%s\n", bold, cyan, nc, dim, nc)
	fmt.Printf("  %sUnlock Your Claude Code for Any LLM%s\n", "\033[3;36m", nc)
	fmt.Println()
}

func promptProvider(reader *bufio.Reader) (string, error) {
	fmt.Printf("  %sSelect provider type:%s\n\n", bold, nc)
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

	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		keyBytes, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		key := strings.TrimSpace(string(keyBytes))
		return key, nil // Can be empty for local Ollama
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(input)
	return key, nil
}

func promptModel(reader *bufio.Reader) (string, error) {
	fmt.Printf("  %sDefault Model [gpt-4o-mini]: %s", bold, nc)
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
	port, err := strconv.Atoi(input)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid port: %s (must be 1-65535)", input)
	}
	return fmt.Sprintf(":%d", port), nil
}

func maskKey(key string) string {
	if key == "" { return "none" }
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}

func ConfigureClaudeCode(listenAddr, model string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	claudeDir := filepath.Join(home, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")
	os.MkdirAll(claudeDir, 0755)

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

	env, ok := data["env"].(map[string]interface{})
	if !ok {
		env = make(map[string]interface{})
	}
	env["ANTHROPIC_BASE_URL"] = "http://localhost" + listenAddr
	env["ANTHROPIC_CUSTOM_MODEL_OPTION"] = model
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_NAME"] = "Heimsense Custom Model"
	env["ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION"] = "Custom model via Heimsense adapter"
	// Removing ANTHROPIC_AUTH_TOKEN so it's managed entirely by Heimsense config
	delete(env, "ANTHROPIC_AUTH_TOKEN") 
	env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "1"
	data["env"] = env

	out, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(settingsPath, append(out, '\n'), 0644)

	claudeJSON := filepath.Join(home, ".claude.json")
	if raw, err := os.ReadFile(claudeJSON); err == nil {
		var cj map[string]interface{}
		if json.Unmarshal(raw, &cj) == nil {
			cj["hasCompletedOnboarding"] = true
			if out, err := json.MarshalIndent(cj, "", "  "); err == nil {
				os.WriteFile(claudeJSON, append(out, '\n'), 0644)
			}
		}
	}

	return nil
}

func SyncToClaude() error {
	cfg, err := config.LoadTOML()
	if err != nil {
		// Fallback to older dotenv if TOML doesn't exist
		home, _ := os.UserHomeDir()
		envPath := filepath.Join(home, ".heimsense", ".env")
		if _, err := os.Stat(envPath); err == nil {
			config.LoadDotEnv()
			listenAddr := os.Getenv("LISTEN_ADDR")
			if listenAddr == "" { listenAddr = ":8080" }
			model := os.Getenv("ANTHROPIC_CUSTOM_MODEL_OPTION")
			return ConfigureClaudeCode(listenAddr, model)
		}
		return fmt.Errorf("config not found — run 'heimsense setup' first")
	}

	if err := ConfigureClaudeCode(cfg.ListenAddr, cfg.DefaultModel); err != nil {
		return err
	}

	fmt.Printf("\n  %s%sHEIM·SENSE%s  %ssync%s\n\n", bold, cyan, nc, dim, nc)
	fmt.Printf("  %s✓%s Synced to %s~/.claude/settings.json%s\n\n", green, nc, dim, nc)
	return nil
}

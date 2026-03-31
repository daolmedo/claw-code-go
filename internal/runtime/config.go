package runtime

import (
	"os"
	"path/filepath"
)

const (
	DefaultModel     = "claude-sonnet-4-20250514"
	DefaultMaxTokens = 8096
)

// Config holds runtime configuration for the CLI.
type Config struct {
	Model      string
	MaxTokens  int
	SystemPrompt string
	SessionDir string
	APIKey     string
	BaseURL    string
}

// LoadConfig reads configuration from environment variables and applies defaults.
func LoadConfig() *Config {
	cfg := &Config{
		Model:     DefaultModel,
		MaxTokens: DefaultMaxTokens,
	}

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.APIKey = key
	}

	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
		cfg.Model = model
	}

	if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	// Default session dir: ~/.claw-code/sessions
	homeDir, err := os.UserHomeDir()
	if err == nil {
		cfg.SessionDir = filepath.Join(homeDir, ".claw-code", "sessions")
	} else {
		cfg.SessionDir = ".claw-code-sessions"
	}

	return cfg
}

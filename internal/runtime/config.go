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
	Model        string
	MaxTokens    int
	SystemPrompt string
	SessionDir   string
	APIKey       string
	BaseURL      string

	// Provider and auth fields (Phase 3).
	// ProviderName is one of: "anthropic", "bedrock", "vertex", "foundry".
	ProviderName string
	// AuthMethod is one of: "api_key", "oauth", "iam", "adc", "azure_identity".
	AuthMethod string
	// OAuthToken is the resolved OAuth access token (set at startup when using OAuth).
	OAuthToken string
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

	// Detect the active provider from environment variables.
	cfg.ProviderName = detectProvider()

	return cfg
}

// detectProvider reads env vars to determine which provider to use.
func detectProvider() string {
	switch {
	case os.Getenv("CLAUDE_CODE_USE_BEDROCK") == "1":
		return "bedrock"
	case os.Getenv("CLAUDE_CODE_USE_VERTEX") == "1":
		return "vertex"
	case os.Getenv("CLAUDE_CODE_USE_FOUNDRY") == "1":
		return "foundry"
	default:
		return "anthropic"
	}
}

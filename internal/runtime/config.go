package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultModel     = "claude-sonnet-4-20250514"
	DefaultMaxTokens = 8096
)

// MCPServerConfig describes a single MCP server connection.
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio" or "sse"
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

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

	// MCPServers lists MCP server connections (Phase 4).
	MCPServers []MCPServerConfig

	// Compaction settings (Phase 6).
	// CompactionEnabled enables automatic session compaction (default: true).
	CompactionEnabled bool
	// CompactionThreshold is the fraction of MaxTokens at which compaction
	// triggers (e.g., 0.75 triggers at 75% of the token budget).
	CompactionThreshold float64
	// CompactionKeepRecent is the number of most-recent messages retained
	// verbatim after compaction.
	CompactionKeepRecent int
}

// LoadConfig reads configuration from environment variables and applies defaults.
func LoadConfig() *Config {
	cfg := &Config{
		Model:                DefaultModel,
		MaxTokens:            DefaultMaxTokens,
		CompactionEnabled:    true,
		CompactionThreshold:  DefaultCompactionThreshold,
		CompactionKeepRecent: DefaultCompactionKeepRecent,
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

	// Load MCP server configs.
	cfg.MCPServers = loadMCPServers(homeDir)

	return cfg
}

// loadMCPServers reads MCP server configurations from the settings file and
// the CLAUDE_MCP_SERVERS environment variable (JSON override, takes precedence).
func loadMCPServers(homeDir string) []MCPServerConfig {
	// Try env var override first.
	if raw := os.Getenv("CLAUDE_MCP_SERVERS"); raw != "" {
		var servers []MCPServerConfig
		if err := json.Unmarshal([]byte(raw), &servers); err == nil {
			return servers
		}
	}

	// Otherwise read from ~/.claude/settings.json.
	if homeDir == "" {
		return nil
	}
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}

	var settings struct {
		MCPServers []MCPServerConfig `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	return settings.MCPServers
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

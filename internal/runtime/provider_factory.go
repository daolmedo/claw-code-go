package runtime

import (
	"claw-code-go/internal/api"
	anthropicprovider "claw-code-go/internal/api/providers/anthropic"
	bedrockprovider "claw-code-go/internal/api/providers/bedrock"
	foundryprovider "claw-code-go/internal/api/providers/foundry"
	vertexprovider "claw-code-go/internal/api/providers/vertex"
	"os"
)

// SelectProvider returns the Provider to use based on environment variables.
//
// Selection priority (first match wins):
//   - CLAUDE_CODE_USE_BEDROCK=1  → AWS Bedrock
//   - CLAUDE_CODE_USE_VERTEX=1   → Google Cloud Vertex AI
//   - CLAUDE_CODE_USE_FOUNDRY=1  → Azure AI Foundry
//   - (default)                  → Anthropic direct API
func SelectProvider() api.Provider {
	switch {
	case os.Getenv("CLAUDE_CODE_USE_BEDROCK") == "1":
		return bedrockprovider.New()
	case os.Getenv("CLAUDE_CODE_USE_VERTEX") == "1":
		return vertexprovider.New()
	case os.Getenv("CLAUDE_CODE_USE_FOUNDRY") == "1":
		return foundryprovider.New()
	default:
		return anthropicprovider.New()
	}
}

// NewProviderClient creates an API client for the configured provider.
// Returns an error for stub providers that are not yet implemented.
func NewProviderClient(cfg *Config) (api.APIClient, error) {
	provider := SelectProvider()
	return provider.NewClient(api.ProviderConfig{
		APIKey:     cfg.APIKey,
		OAuthToken: cfg.OAuthToken,
		BaseURL:    cfg.BaseURL,
		Model:      cfg.Model,
		MaxTokens:  cfg.MaxTokens,
	})
}

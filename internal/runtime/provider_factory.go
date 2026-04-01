package runtime

import (
	"claw-code-go/internal/api"
	anthropicprovider "claw-code-go/internal/api/providers/anthropic"
	bedrockprovider "claw-code-go/internal/api/providers/bedrock"
	foundryprovider "claw-code-go/internal/api/providers/foundry"
	openaiprovider "claw-code-go/internal/api/providers/openai"
	vertexprovider "claw-code-go/internal/api/providers/vertex"
	"context"
	"fmt"
)

// SelectProvider returns the Provider for the given name.
// Supported names: "anthropic" (default), "openai", "bedrock", "vertex", "foundry".
func SelectProvider(name string) api.Provider {
	switch name {
	case "openai":
		return openaiprovider.New()
	case "bedrock":
		return bedrockprovider.New()
	case "vertex":
		return vertexprovider.New()
	case "foundry":
		return foundryprovider.New()
	default:
		return anthropicprovider.New()
	}
}

// NewProviderClient creates an API client for the provider named in cfg.ProviderName.
// Returns an error for stub providers that are not yet implemented.
func NewProviderClient(cfg *Config) (api.APIClient, error) {
	provider := SelectProvider(cfg.ProviderName)
	return provider.NewClient(api.ProviderConfig{
		APIKey:     cfg.APIKey,
		OAuthToken: cfg.OAuthToken,
		BaseURL:    cfg.BaseURL,
		Model:      cfg.Model,
		MaxTokens:  cfg.MaxTokens,
	})
}

// ----- NoAuthClient ----------------------------------------------------------

// NoAuthClient is a placeholder APIClient used when no credentials are configured.
// Every call returns a friendly error directing the user to /login.
type NoAuthClient struct{}

// NewNoAuthClient returns a NoAuthClient as an api.APIClient interface value.
func NewNoAuthClient() api.APIClient {
	return &NoAuthClient{}
}

func (c *NoAuthClient) StreamResponse(_ context.Context, _ api.CreateMessageRequest) (<-chan api.StreamEvent, error) {
	return nil, fmt.Errorf("not authenticated — type /login to connect to an AI provider")
}

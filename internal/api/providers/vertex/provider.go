// Package vertex is a stub for the Google Cloud Vertex AI provider.
// TODO: implement using google.golang.org/api/option and the Vertex AI SDK.
// Authentication: GCP Application Default Credentials (ADC) — gcloud auth, workload identity, etc.
// Model IDs: "claude-sonnet-4@20250514" (Vertex AI format)
package vertex

import (
	"fmt"

	"claw-code-go/internal/api"
)

// Provider implements api.Provider for Google Cloud Vertex AI.
type Provider struct{}

// New returns a new Vertex AI Provider.
func New() *Provider { return &Provider{} }

// Name returns the provider identifier.
func (p *Provider) Name() string { return "vertex" }

// AuthMethod returns the GCP Application Default Credentials auth method.
func (p *Provider) AuthMethod() api.AuthMethod { return api.AuthMethodADC }

// NewClient is a stub. Vertex AI support is not yet implemented.
// Set CLAUDE_CODE_USE_VERTEX=1 to select this provider.
func (p *Provider) NewClient(_ api.ProviderConfig) (api.APIClient, error) {
	return nil, fmt.Errorf("vertex provider: not yet implemented (requires GCP Application Default Credentials)")
}

// MapModelID converts a canonical Claude model ID to the Vertex AI format.
// Example: "claude-sonnet-4-20250514" → "claude-sonnet-4@20250514"
func MapModelID(model string) string {
	// TODO: implement full model ID mapping table
	// Vertex uses '@' instead of '-' before the date suffix
	return model
}

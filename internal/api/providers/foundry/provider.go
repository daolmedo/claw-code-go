// Package foundry is a stub for the Azure AI Foundry provider.
// TODO: implement using github.com/Azure/azure-sdk-for-go.
// Authentication: Azure Managed Identity or service principal credentials.
// Model IDs: deployed model names as configured in the Azure AI Foundry portal.
package foundry

import (
	"fmt"

	"claw-code-go/internal/api"
)

// Provider implements api.Provider for Azure AI Foundry.
type Provider struct{}

// New returns a new Azure Foundry Provider.
func New() *Provider { return &Provider{} }

// Name returns the provider identifier.
func (p *Provider) Name() string { return "foundry" }

// AuthMethod returns the Azure Identity auth method.
func (p *Provider) AuthMethod() api.AuthMethod { return api.AuthMethodAzureIdentity }

// NewClient is a stub. Azure AI Foundry support is not yet implemented.
// Set CLAUDE_CODE_USE_FOUNDRY=1 to select this provider.
func (p *Provider) NewClient(_ api.ProviderConfig) (api.APIClient, error) {
	return nil, fmt.Errorf("foundry provider: not yet implemented (requires Azure Identity credentials)")
}

// MapModelID returns the model name as deployed in the Azure AI Foundry portal.
// Azure model IDs are typically short names configured at deployment time.
func MapModelID(model string) string {
	// TODO: implement mapping to Azure deployment names
	return model
}

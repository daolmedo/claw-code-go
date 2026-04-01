// Package bedrock is a stub for the AWS Bedrock provider.
// TODO: implement using aws-sdk-go-v2 bedrock-runtime client.
// Authentication: AWS IAM credentials (env vars, instance profile, or ~/.aws/credentials).
// Model IDs: "us.anthropic.claude-sonnet-4-20250514-v1:0" (cross-region inference format)
package bedrock

import (
	"fmt"

	"claw-code-go/internal/api"
)

// Provider implements api.Provider for AWS Bedrock.
type Provider struct{}

// New returns a new Bedrock Provider.
func New() *Provider { return &Provider{} }

// Name returns the provider identifier.
func (p *Provider) Name() string { return "bedrock" }

// AuthMethod returns the AWS IAM auth method used by Bedrock.
func (p *Provider) AuthMethod() api.AuthMethod { return api.AuthMethodIAM }

// NewClient is a stub. AWS Bedrock support is not yet implemented.
// Set CLAUDE_CODE_USE_BEDROCK=1 to select this provider.
func (p *Provider) NewClient(_ api.ProviderConfig) (api.APIClient, error) {
	return nil, fmt.Errorf("bedrock provider: not yet implemented (requires aws-sdk-go-v2 bedrock-runtime)")
}

// MapModelID converts a canonical Claude model ID to the Bedrock cross-region inference format.
// Example: "claude-sonnet-4-20250514" → "us.anthropic.claude-sonnet-4-20250514-v1:0"
func MapModelID(model string) string {
	// TODO: implement full model ID mapping table
	return "us.anthropic." + model + "-v1:0"
}

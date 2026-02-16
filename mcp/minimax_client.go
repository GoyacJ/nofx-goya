package mcp

import (
	"net/http"
)

const (
	ProviderMiniMax = "minimax"
)

type MiniMaxClient struct {
	*Client
}

// NewMiniMaxClient creates MiniMax client (backward compatible).
func NewMiniMaxClient() AIClient {
	return NewMiniMaxClientWithOptions()
}

// NewMiniMaxClientWithOptions creates MiniMax client (supports options pattern).
//
// Note: MiniMax defaults are intentionally left empty. Users must explicitly
// configure BaseURL and Model when enabling this provider.
func NewMiniMaxClientWithOptions(opts ...ClientOption) AIClient {
	miniMaxOpts := []ClientOption{
		WithProvider(ProviderMiniMax),
	}

	allOpts := append(miniMaxOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)

	miniMaxClient := &MiniMaxClient{
		Client: baseClient,
	}

	baseClient.hooks = miniMaxClient
	return miniMaxClient
}

func (c *MiniMaxClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] MiniMax API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] MiniMax using custom BaseURL: %s", customURL)
	} else {
		c.BaseURL = ""
		c.logger.Infof("🔧 [MCP] MiniMax has no default BaseURL, please configure custom BaseURL")
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] MiniMax using custom Model: %s", customModel)
	} else {
		c.Model = ""
		c.logger.Infof("🔧 [MCP] MiniMax has no default Model, please configure custom Model")
	}
}

// MiniMax uses standard OpenAI-compatible API with Bearer auth.
func (c *MiniMaxClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

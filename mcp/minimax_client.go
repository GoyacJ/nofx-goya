package mcp

import (
	"net/http"
)

const (
	ProviderMiniMax       = "minimax"
	DefaultMiniMaxBaseURL = "https://api.minimaxi.com/anthropic"
	DefaultMiniMaxModel   = "MiniMax-M2.5"
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
// Default endpoint/model follow MiniMax OpenAI-compatible quickstart.
func NewMiniMaxClientWithOptions(opts ...ClientOption) AIClient {
	miniMaxOpts := []ClientOption{
		WithProvider(ProviderMiniMax),
		WithBaseURL(DefaultMiniMaxBaseURL),
		WithModel(DefaultMiniMaxModel),
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
		c.BaseURL = DefaultMiniMaxBaseURL
		c.logger.Infof("🔧 [MCP] MiniMax using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] MiniMax using custom Model: %s", customModel)
	} else {
		c.Model = DefaultMiniMaxModel
		c.logger.Infof("🔧 [MCP] MiniMax using default Model: %s", c.Model)
	}
}

// MiniMax uses standard OpenAI-compatible API with Bearer auth.
func (c *MiniMaxClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

package mcp

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	ProviderOpenClaw = "openclaw"
)

// OpenClawClient routes NOFX AI requests through an OpenClaw gateway.
// OpenClaw requires explicit base URL and model from user configuration.
type OpenClawClient struct {
	*Client
}

func NewOpenClawClient() AIClient {
	return NewOpenClawClientWithOptions()
}

func NewOpenClawClientWithOptions(opts ...ClientOption) AIClient {
	openclawOpts := []ClientOption{
		WithProvider(ProviderOpenClaw),
	}

	allOpts := append(openclawOpts, opts...)
	baseClient := NewClient(allOpts...).(*Client)
	ocClient := &OpenClawClient{Client: baseClient}
	baseClient.hooks = ocClient
	return ocClient
}

func (c *OpenClawClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.Provider = ProviderOpenClaw
	c.APIKey = strings.TrimSpace(apiKey)
	c.BaseURL = normalizeOpenClawBaseURL(customURL)
	c.Model = strings.TrimSpace(customModel)
	c.UseFullURL = false

	if strings.HasSuffix(strings.TrimSpace(customURL), "#") {
		c.UseFullURL = true
	}

	if len(c.APIKey) > 8 {
		c.logger.Infof("🔧 [MCP] OpenClaw API Key: %s...%s", c.APIKey[:4], c.APIKey[len(c.APIKey)-4:])
	}
	c.logger.Infof("🔧 [MCP] OpenClaw BaseURL: %s (fullURL=%v)", c.BaseURL, c.UseFullURL)
	c.logger.Infof("🔧 [MCP] OpenClaw Model: %s", c.Model)
}

func normalizeOpenClawBaseURL(raw string) string {
	base := strings.TrimSpace(raw)
	if base == "" {
		return ""
	}

	if strings.HasSuffix(base, "#") {
		return strings.TrimSuffix(base, "#")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/")
	}

	switch strings.TrimRight(parsed.Path, "/") {
	case "", ".":
		parsed.Path = "/v1"
	case "/chat/completions":
		parsed.Path = "/v1"
	case "/v1/chat/completions":
		parsed.Path = "/v1"
	}

	normalized := parsed.String()
	return strings.TrimRight(normalized, "/")
}

func (c *OpenClawClient) setAuthHeader(reqHeaders http.Header) {
	c.Client.setAuthHeader(reqHeaders)
}

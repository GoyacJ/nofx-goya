package backtest

import (
	"strings"
	"testing"

	"nofx/mcp"
)

func TestConfigureMCPClient_MiniMaxRequiresBaseURLAndModel(t *testing.T) {
	cfg := BacktestConfig{
		AICfg: AIConfig{
			Provider: "minimax",
			APIKey:   "test-key",
		},
	}

	_, err := configureMCPClient(cfg, mcp.NewClient())
	if err == nil {
		t.Fatalf("expected minimax validation error")
	}
	if !strings.Contains(err.Error(), "requires api key, base_url and model") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigureMCPClient_MiniMaxSuccess(t *testing.T) {
	cfg := BacktestConfig{
		AICfg: AIConfig{
			Provider: "minimax",
			APIKey:   "test-key",
			BaseURL:  "https://api.minimax.chat/v1",
			Model:    "MiniMax-Text-01",
		},
	}

	client, err := configureMCPClient(cfg, mcp.NewClient())
	if err != nil {
		t.Fatalf("expected minimax client creation success, got %v", err)
	}

	mmClient, ok := client.(*mcp.MiniMaxClient)
	if !ok {
		t.Fatalf("expected *mcp.MiniMaxClient, got %T", client)
	}
	if mmClient.Provider != "minimax" {
		t.Fatalf("unexpected provider: %s", mmClient.Provider)
	}
	if mmClient.BaseURL != cfg.AICfg.BaseURL {
		t.Fatalf("unexpected base URL: %s", mmClient.BaseURL)
	}
	if mmClient.Model != cfg.AICfg.Model {
		t.Fatalf("unexpected model: %s", mmClient.Model)
	}
}

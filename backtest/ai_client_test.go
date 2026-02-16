package backtest

import (
	"testing"

	"nofx/mcp"
)

func TestConfigureMCPClient_MiniMaxUsesDefaultsWhenBaseURLAndModelMissing(t *testing.T) {
	cfg := BacktestConfig{
		AICfg: AIConfig{
			Provider: "minimax",
			APIKey:   "test-key",
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
	if mmClient.BaseURL != mcp.DefaultMiniMaxBaseURL {
		t.Fatalf("expected default base URL %q, got %q", mcp.DefaultMiniMaxBaseURL, mmClient.BaseURL)
	}
	if mmClient.Model != mcp.DefaultMiniMaxModel {
		t.Fatalf("expected default model %q, got %q", mcp.DefaultMiniMaxModel, mmClient.Model)
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

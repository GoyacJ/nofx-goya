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

func TestConfigureMCPClient_OpenClawRequiresURLAndModel(t *testing.T) {
	cfg := BacktestConfig{
		AICfg: AIConfig{
			Provider: "openclaw",
			APIKey:   "test-key",
		},
	}

	_, err := configureMCPClient(cfg, mcp.NewClient())
	if err == nil {
		t.Fatalf("expected error when openclaw base_url/model missing")
	}
}

func TestConfigureMCPClient_OpenClawSuccess(t *testing.T) {
	cfg := BacktestConfig{
		AICfg: AIConfig{
			Provider: "openclaw",
			APIKey:   "test-key",
			BaseURL:  "https://gateway.example.com",
			Model:    "openclaw/model-v1",
		},
	}

	client, err := configureMCPClient(cfg, mcp.NewClient())
	if err != nil {
		t.Fatalf("expected openclaw client creation success, got %v", err)
	}

	ocClient, ok := client.(*mcp.OpenClawClient)
	if !ok {
		t.Fatalf("expected *mcp.OpenClawClient, got %T", client)
	}
	if ocClient.Provider != mcp.ProviderOpenClaw {
		t.Fatalf("unexpected provider: %s", ocClient.Provider)
	}
	if ocClient.BaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("unexpected base URL: %s", ocClient.BaseURL)
	}
	if ocClient.Model != cfg.AICfg.Model {
		t.Fatalf("unexpected model: %s", ocClient.Model)
	}
}

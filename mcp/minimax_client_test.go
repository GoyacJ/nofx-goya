package mcp

import "testing"

func TestNewMiniMaxClient_Default(t *testing.T) {
	client := NewMiniMaxClient()
	mc, ok := client.(*MiniMaxClient)
	if !ok {
		t.Fatalf("expected *MiniMaxClient, got %T", client)
	}

	if mc.Provider != ProviderMiniMax {
		t.Fatalf("expected provider %q, got %q", ProviderMiniMax, mc.Provider)
	}
	if mc.BaseURL != DefaultMiniMaxBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultMiniMaxBaseURL, mc.BaseURL)
	}
	if mc.Model != DefaultMiniMaxModel {
		t.Fatalf("expected default model %q, got %q", DefaultMiniMaxModel, mc.Model)
	}
}

func TestMiniMaxClient_SetAPIKey_CustomConfig(t *testing.T) {
	client := NewMiniMaxClient()
	mc := client.(*MiniMaxClient)

	mc.SetAPIKey("test-minimax-api-key", "https://api.minimax.chat/v1", "MiniMax-Text-01")

	if mc.APIKey != "test-minimax-api-key" {
		t.Fatalf("unexpected api key: %q", mc.APIKey)
	}
	if mc.BaseURL != "https://api.minimax.chat/v1" {
		t.Fatalf("unexpected base URL: %q", mc.BaseURL)
	}
	if mc.Model != "MiniMax-Text-01" {
		t.Fatalf("unexpected model: %q", mc.Model)
	}
}

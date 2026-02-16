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

func TestMiniMaxClient_SetAPIKey_NormalizesLegacyURLs(t *testing.T) {
	testCases := []struct {
		name     string
		inputURL string
		wantURL  string
	}{
		{
			name:     "legacy minimaxi anthropic endpoint",
			inputURL: "https://api.minimaxi.com/anthropic",
			wantURL:  "https://api.minimaxi.com/v1",
		},
		{
			name:     "legacy minimax anthropic endpoint",
			inputURL: "https://api.minimax.io/anthropic",
			wantURL:  "https://api.minimax.io/v1",
		},
		{
			name:     "host only",
			inputURL: "https://api.minimax.io",
			wantURL:  "https://api.minimax.io/v1",
		},
		{
			name:     "accidental full endpoint",
			inputURL: "https://api.minimax.io/v1/chat/completions",
			wantURL:  "https://api.minimax.io/v1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewMiniMaxClient()
			mc := client.(*MiniMaxClient)

			mc.SetAPIKey("test-minimax-api-key", tc.inputURL, "")

			if mc.BaseURL != tc.wantURL {
				t.Fatalf("for input %q, expected %q, got %q", tc.inputURL, tc.wantURL, mc.BaseURL)
			}
		})
	}
}

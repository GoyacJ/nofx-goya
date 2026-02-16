package mcp

import "testing"

func TestNewOpenClawClient_Default(t *testing.T) {
	client := NewOpenClawClient()
	oc, ok := client.(*OpenClawClient)
	if !ok {
		t.Fatalf("expected *OpenClawClient, got %T", client)
	}

	if oc.Provider != ProviderOpenClaw {
		t.Fatalf("expected provider %q, got %q", ProviderOpenClaw, oc.Provider)
	}
	if oc.BaseURL != "" {
		t.Fatalf("expected empty default base URL, got %q", oc.BaseURL)
	}
	if oc.Model != "" {
		t.Fatalf("expected empty default model, got %q", oc.Model)
	}
}

func TestOpenClawClient_SetAPIKey_CustomConfig(t *testing.T) {
	client := NewOpenClawClient()
	oc := client.(*OpenClawClient)

	oc.SetAPIKey("test-openclaw-key", "https://gateway.example.com", "openclaw/model-v1")

	if oc.APIKey != "test-openclaw-key" {
		t.Fatalf("unexpected api key: %q", oc.APIKey)
	}
	if oc.BaseURL != "https://gateway.example.com/v1" {
		t.Fatalf("expected normalized base URL with /v1 suffix, got %q", oc.BaseURL)
	}
	if oc.Model != "openclaw/model-v1" {
		t.Fatalf("unexpected model: %q", oc.Model)
	}
	if oc.UseFullURL {
		t.Fatalf("expected UseFullURL=false for normal base URL")
	}
}

func TestOpenClawClient_SetAPIKey_FullURLMode(t *testing.T) {
	client := NewOpenClawClient()
	oc := client.(*OpenClawClient)

	oc.SetAPIKey("test-openclaw-key", "https://gateway.example.com/v1/chat/completions#", "openclaw/model-v1")

	if oc.BaseURL != "https://gateway.example.com/v1/chat/completions" {
		t.Fatalf("unexpected full URL base: %q", oc.BaseURL)
	}
	if !oc.UseFullURL {
		t.Fatalf("expected UseFullURL=true for '#' full-url mode")
	}
}

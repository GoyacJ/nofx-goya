package api

import (
	"strings"
	"testing"
)

func TestDiagnoseModelTestError_MiniMaxInvalidKey(t *testing.T) {
	hint := diagnoseModelTestError(
		"minimax",
		`AI API call failed: API returned error (status 401): {"error":{"message":"invalid api key (2049)"}}`,
	)
	if !strings.Contains(hint, "2049") {
		t.Fatalf("expected hint to mention 2049, got %q", hint)
	}
}

func TestBuildAIClientForProvider_Unsupported(t *testing.T) {
	client, err := buildAIClientForProvider("unknown-provider", "test-key", "", "")
	if err == nil {
		t.Fatalf("expected error for unsupported provider, got nil and client=%T", client)
	}
}

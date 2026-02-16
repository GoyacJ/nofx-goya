package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/mcp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type TestModelConfigRequest struct {
	ModelID         string `json:"model_id"`
	Provider        string `json:"provider"`
	APIKey          string `json:"api_key"`
	CustomAPIURL    string `json:"custom_api_url"`
	CustomModelName string `json:"custom_model_name"`
}

type TestModelConfigResponse struct {
	OK          bool   `json:"ok"`
	Provider    string `json:"provider,omitempty"`
	BaseURL     string `json:"base_url,omitempty"`
	Model       string `json:"model,omitempty"`
	LatencyMs   int64  `json:"latency_ms,omitempty"`
	Message     string `json:"message,omitempty"`
	ResponseTip string `json:"response_tip,omitempty"`
	Error       string `json:"error,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// handleTestModelConfig tests model connectivity using current form values.
// It supports encrypted payloads and falls back to persisted config for blank fields in edit mode.
func (s *Server) handleTestModelConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	req, err := s.parseTestModelConfigRequest(c, userID)
	if err != nil {
		SafeBadRequest(c, err.Error())
		return
	}

	provider, apiKey, baseURL, modelName, err := s.resolveModelTestInput(userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logger.Infof("🧪 Testing AI model connection: provider=%s, model_id=%s, base_url=%s", provider, req.ModelID, baseURL)

	client, err := buildAIClientForProvider(provider, apiKey, baseURL, modelName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client.SetTimeout(30 * time.Second)

	start := time.Now()
	_, callErr := client.CallWithMessages(
		"You are a connectivity check assistant. Reply with exactly OK.",
		"Return exactly OK.",
	)
	latencyMs := time.Since(start).Milliseconds()
	resolvedBaseURL, resolvedModel := extractAIClientConfig(client, baseURL, modelName)

	if callErr != nil {
		errMsg := callErr.Error()
		c.JSON(http.StatusOK, TestModelConfigResponse{
			OK:        false,
			Provider:  provider,
			BaseURL:   resolvedBaseURL,
			Model:     resolvedModel,
			LatencyMs: latencyMs,
			Error:     errMsg,
			Hint:      diagnoseModelTestError(provider, errMsg),
		})
		return
	}

	c.JSON(http.StatusOK, TestModelConfigResponse{
		OK:          true,
		Provider:    provider,
		BaseURL:     resolvedBaseURL,
		Model:       resolvedModel,
		LatencyMs:   latencyMs,
		Message:     "Connection test succeeded",
		ResponseTip: "Provider responded normally",
	})
}

func (s *Server) parseTestModelConfigRequest(c *gin.Context, userID string) (TestModelConfigRequest, error) {
	var req TestModelConfigRequest
	bodyBytes, err := c.GetRawData()
	if err != nil {
		return req, fmt.Errorf("failed to read request body")
	}

	cfg := config.Get()
	if !cfg.TransportEncryption {
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			return req, fmt.Errorf("invalid request format")
		}
		return req, nil
	}

	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		return req, fmt.Errorf("invalid request format, encrypted transmission required")
	}

	if encryptedPayload.WrappedKey == "" {
		logger.Infof("❌ Detected unencrypted model test request (UserID: %s)", userID)
		return req, fmt.Errorf("encrypted transmission is required")
	}

	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		return req, fmt.Errorf("failed to decrypt request")
	}

	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		return req, fmt.Errorf("failed to parse decrypted payload")
	}

	return req, nil
}

func (s *Server) resolveModelTestInput(userID string, req TestModelConfigRequest) (provider, apiKey, baseURL, modelName string, err error) {
	provider = strings.ToLower(strings.TrimSpace(req.Provider))
	apiKey = strings.TrimSpace(req.APIKey)
	baseURL = strings.TrimSpace(req.CustomAPIURL)
	modelName = strings.TrimSpace(req.CustomModelName)

	modelID := strings.TrimSpace(req.ModelID)
	if modelID != "" {
		existing, getErr := s.store.AIModel().Get(userID, modelID)
		if getErr == nil && existing != nil {
			if provider == "" {
				provider = strings.ToLower(strings.TrimSpace(existing.Provider))
			}
			if apiKey == "" {
				apiKey = strings.TrimSpace(string(existing.APIKey))
			}
			if baseURL == "" {
				baseURL = strings.TrimSpace(existing.CustomAPIURL)
			}
			if modelName == "" {
				modelName = strings.TrimSpace(existing.CustomModelName)
			}
		}
	}

	if provider == "" {
		return "", "", "", "", fmt.Errorf("provider is required")
	}
	if apiKey == "" {
		return "", "", "", "", fmt.Errorf("API key is required for connection test")
	}

	switch provider {
	case "minimax":
		if baseURL == "" {
			baseURL = mcp.DefaultMiniMaxBaseURL
		}
		if modelName == "" {
			modelName = mcp.DefaultMiniMaxModel
		}
	case "openclaw":
		if baseURL == "" {
			return "", "", "", "", fmt.Errorf("OpenClaw requires explicit base URL")
		}
		if modelName == "" {
			return "", "", "", "", fmt.Errorf("OpenClaw requires explicit model name")
		}
	}

	return provider, apiKey, baseURL, modelName, nil
}

func buildAIClientForProvider(provider, apiKey, customURL, customModel string) (mcp.AIClient, error) {
	switch provider {
	case "qwen":
		client := mcp.NewQwenClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "deepseek":
		client := mcp.NewDeepSeekClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "claude":
		client := mcp.NewClaudeClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "kimi":
		client := mcp.NewKimiClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "gemini":
		client := mcp.NewGeminiClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "grok":
		client := mcp.NewGrokClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "openai":
		client := mcp.NewOpenAIClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "minimax":
		client := mcp.NewMiniMaxClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case "openclaw":
		client := mcp.NewOpenClawClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	case mcp.ProviderCustom, "":
		client := mcp.NewClient()
		client.SetAPIKey(apiKey, customURL, customModel)
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func extractAIClientConfig(client mcp.AIClient, fallbackBaseURL, fallbackModel string) (string, string) {
	switch c := client.(type) {
	case *mcp.DeepSeekClient:
		return c.BaseURL, c.Model
	case *mcp.QwenClient:
		return c.BaseURL, c.Model
	case *mcp.OpenAIClient:
		return c.BaseURL, c.Model
	case *mcp.ClaudeClient:
		return c.BaseURL, c.Model
	case *mcp.GeminiClient:
		return c.BaseURL, c.Model
	case *mcp.GrokClient:
		return c.BaseURL, c.Model
	case *mcp.KimiClient:
		return c.BaseURL, c.Model
	case *mcp.MiniMaxClient:
		return c.BaseURL, c.Model
	case *mcp.OpenClawClient:
		return c.BaseURL, c.Model
	case *mcp.Client:
		return c.BaseURL, c.Model
	default:
		return fallbackBaseURL, fallbackModel
	}
}

func diagnoseModelTestError(provider, errMsg string) string {
	lower := strings.ToLower(errMsg)

	if provider == "minimax" {
		if strings.Contains(lower, "invalid api key (2049)") {
			return "MiniMax 返回 401/2049：API Key 无效。请确认使用的是 MiniMax API Key（不是其他产品的 key），并确保控制台域名与 Base URL 同区域（platform.minimaxi.com 对应 api.minimaxi.com）。"
		}
		if strings.Contains(lower, "status 404") {
			return "MiniMax 接口路径返回 404。NOFX 使用 OpenAI 兼容协议，请优先使用 https://api.minimaxi.com/v1（若填写 /anthropic，系统会自动转换为 /v1）。"
		}
		if strings.Contains(lower, "status 401") {
			return "MiniMax 认证失败。请检查 API Key 是否过期/禁用，或是否误用了与当前套餐不兼容的 key。"
		}
	}

	if strings.Contains(lower, "status 401") {
		return "认证失败，请检查 API Key 是否正确，以及是否有调用当前模型的权限。"
	}
	if strings.Contains(lower, "status 404") {
		return "接口地址不可用，请检查 Base URL 与协议路径是否匹配。"
	}
	if strings.Contains(lower, "timeout") {
		return "请求超时，请检查网络连通性、代理设置或稍后重试。"
	}

	return ""
}

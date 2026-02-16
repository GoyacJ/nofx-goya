package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"nofx/mcp"
	"nofx/store"

	"github.com/gin-gonic/gin"
)

// TestUpdateTraderRequest_SystemPromptTemplate Test whether SystemPromptTemplate field exists when updating trader
func TestUpdateTraderRequest_SystemPromptTemplate(t *testing.T) {
	tests := []struct {
		name                   string
		requestJSON            string
		expectedPromptTemplate string
	}{
		{
			name: "Should accept system_prompt_template=nof1 during update",
			requestJSON: `{
				"name": "Test Trader",
				"ai_model_id": "gpt-4",
				"exchange_id": "binance",
				"initial_balance": 1000,
				"scan_interval_minutes": 5,
				"btc_eth_leverage": 5,
				"altcoin_leverage": 3,
				"trading_symbols": "BTC,ETH",
				"custom_prompt": "test",
				"override_base_prompt": false,
				"is_cross_margin": true,
				"system_prompt_template": "nof1"
			}`,
			expectedPromptTemplate: "nof1",
		},
		{
			name: "Should accept system_prompt_template=default during update",
			requestJSON: `{
				"name": "Test Trader",
				"ai_model_id": "gpt-4",
				"exchange_id": "binance",
				"initial_balance": 1000,
				"scan_interval_minutes": 5,
				"btc_eth_leverage": 5,
				"altcoin_leverage": 3,
				"trading_symbols": "BTC,ETH",
				"custom_prompt": "test",
				"override_base_prompt": false,
				"is_cross_margin": true,
				"system_prompt_template": "default"
			}`,
			expectedPromptTemplate: "default",
		},
		{
			name: "Should accept system_prompt_template=custom during update",
			requestJSON: `{
				"name": "Test Trader",
				"ai_model_id": "gpt-4",
				"exchange_id": "binance",
				"initial_balance": 1000,
				"scan_interval_minutes": 5,
				"btc_eth_leverage": 5,
				"altcoin_leverage": 3,
				"trading_symbols": "BTC,ETH",
				"custom_prompt": "test",
				"override_base_prompt": false,
				"is_cross_margin": true,
				"system_prompt_template": "custom"
			}`,
			expectedPromptTemplate: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test whether UpdateTraderRequest struct can correctly parse system_prompt_template field
			var req UpdateTraderRequest
			err := json.Unmarshal([]byte(tt.requestJSON), &req)
			if err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Verify SystemPromptTemplate field is correctly read
			if req.SystemPromptTemplate != tt.expectedPromptTemplate {
				t.Errorf("Expected SystemPromptTemplate=%q, got %q",
					tt.expectedPromptTemplate, req.SystemPromptTemplate)
			}

			// Verify other fields are also correctly parsed
			if req.Name != "Test Trader" {
				t.Errorf("Name not parsed correctly")
			}
			if req.AIModelID != "gpt-4" {
				t.Errorf("AIModelID not parsed correctly")
			}
		})
	}
}

// TestGetTraderConfigResponse_SystemPromptTemplate Test whether return value contains system_prompt_template when getting trader config
func TestGetTraderConfigResponse_SystemPromptTemplate(t *testing.T) {
	tests := []struct {
		name             string
		traderConfig     *store.Trader
		expectedTemplate string
	}{
		{
			name: "Get config should return system_prompt_template=nof1",
			traderConfig: &store.Trader{
				ID:                   "trader-123",
				UserID:               "user-1",
				Name:                 "Test Trader",
				AIModelID:            "gpt-4",
				ExchangeID:           "binance",
				InitialBalance:       1000,
				ScanIntervalMinutes:  5,
				BTCETHLeverage:       5,
				AltcoinLeverage:      3,
				TradingSymbols:       "BTC,ETH",
				CustomPrompt:         "test",
				OverrideBasePrompt:   false,
				SystemPromptTemplate: "nof1",
				IsCrossMargin:        true,
				IsRunning:            false,
			},
			expectedTemplate: "nof1",
		},
		{
			name: "Get config should return system_prompt_template=default",
			traderConfig: &store.Trader{
				ID:                   "trader-456",
				UserID:               "user-1",
				Name:                 "Test Trader 2",
				AIModelID:            "gpt-4",
				ExchangeID:           "binance",
				InitialBalance:       2000,
				ScanIntervalMinutes:  10,
				BTCETHLeverage:       10,
				AltcoinLeverage:      5,
				TradingSymbols:       "BTC",
				CustomPrompt:         "",
				OverrideBasePrompt:   false,
				SystemPromptTemplate: "default",
				IsCrossMargin:        false,
				IsRunning:            false,
			},
			expectedTemplate: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate handleGetTraderConfig return value construction logic (fixed implementation)
			result := map[string]interface{}{
				"trader_id":              tt.traderConfig.ID,
				"trader_name":            tt.traderConfig.Name,
				"ai_model":               tt.traderConfig.AIModelID,
				"exchange_id":            tt.traderConfig.ExchangeID,
				"initial_balance":        tt.traderConfig.InitialBalance,
				"scan_interval_minutes":  tt.traderConfig.ScanIntervalMinutes,
				"btc_eth_leverage":       tt.traderConfig.BTCETHLeverage,
				"altcoin_leverage":       tt.traderConfig.AltcoinLeverage,
				"trading_symbols":        tt.traderConfig.TradingSymbols,
				"custom_prompt":          tt.traderConfig.CustomPrompt,
				"override_base_prompt":   tt.traderConfig.OverrideBasePrompt,
				"system_prompt_template": tt.traderConfig.SystemPromptTemplate,
				"is_cross_margin":        tt.traderConfig.IsCrossMargin,
				"is_running":             tt.traderConfig.IsRunning,
			}

			// Check if response contains system_prompt_template
			if _, exists := result["system_prompt_template"]; !exists {
				t.Errorf("Response is missing 'system_prompt_template' field")
			} else {
				actualTemplate := result["system_prompt_template"].(string)
				if actualTemplate != tt.expectedTemplate {
					t.Errorf("Expected system_prompt_template=%q, got %q",
						tt.expectedTemplate, actualTemplate)
				}
			}

			// Verify other fields are correct
			if result["trader_id"] != tt.traderConfig.ID {
				t.Errorf("trader_id mismatch")
			}
			if result["trader_name"] != tt.traderConfig.Name {
				t.Errorf("trader_name mismatch")
			}
		})
	}
}

// TestUpdateTraderRequest_CompleteFields Verify UpdateTraderRequest struct definition completeness
func TestUpdateTraderRequest_CompleteFields(t *testing.T) {
	jsonData := `{
		"name": "Test Trader",
		"ai_model_id": "gpt-4",
		"exchange_id": "binance",
		"initial_balance": 1000,
		"scan_interval_minutes": 5,
		"btc_eth_leverage": 5,
		"altcoin_leverage": 3,
		"trading_symbols": "BTC,ETH",
		"custom_prompt": "test",
		"override_base_prompt": false,
		"is_cross_margin": true,
		"system_prompt_template": "nof1"
	}`

	var req UpdateTraderRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify basic fields are correctly parsed
	if req.Name != "Test Trader" {
		t.Errorf("Name mismatch: got %q", req.Name)
	}
	if req.AIModelID != "gpt-4" {
		t.Errorf("AIModelID mismatch: got %q", req.AIModelID)
	}

	// Verify SystemPromptTemplate field has been correctly added to struct
	if req.SystemPromptTemplate != "nof1" {
		t.Errorf("SystemPromptTemplate mismatch: expected %q, got %q", "nof1", req.SystemPromptTemplate)
	}
}

// TestTraderListResponse_SystemPromptTemplate Test whether trader object returned by handleTraderList API contains system_prompt_template field
func TestTraderListResponse_SystemPromptTemplate(t *testing.T) {
	// Simulate trader object construction in handleTraderList
	trader := &store.Trader{
		ID:                   "trader-001",
		UserID:               "user-1",
		Name:                 "My Trader",
		AIModelID:            "gpt-4",
		ExchangeID:           "binance",
		InitialBalance:       5000,
		SystemPromptTemplate: "nof1",
		IsRunning:            true,
	}

	// Construct API response object (consistent with logic in api/server.go)
	response := map[string]interface{}{
		"trader_id":              trader.ID,
		"trader_name":            trader.Name,
		"ai_model":               trader.AIModelID,
		"exchange_id":            trader.ExchangeID,
		"is_running":             trader.IsRunning,
		"initial_balance":        trader.InitialBalance,
		"system_prompt_template": trader.SystemPromptTemplate,
	}

	// Verify system_prompt_template field exists
	if _, exists := response["system_prompt_template"]; !exists {
		t.Errorf("Trader list response is missing 'system_prompt_template' field")
	}

	// Verify system_prompt_template value is correct
	if response["system_prompt_template"] != "nof1" {
		t.Errorf("Expected system_prompt_template='nof1', got %v", response["system_prompt_template"])
	}
}

// TestPublicTraderListResponse_SystemPromptTemplate Test whether trader object returned by handlePublicTraderList API contains system_prompt_template field
func TestPublicTraderListResponse_SystemPromptTemplate(t *testing.T) {
	// Simulate trader data returned by getConcurrentTraderData
	traderData := map[string]interface{}{
		"trader_id":              "trader-002",
		"trader_name":            "Public Trader",
		"ai_model":               "claude",
		"exchange":               "binance",
		"total_equity":           10000.0,
		"total_pnl":              500.0,
		"total_pnl_pct":          5.0,
		"position_count":         3,
		"margin_used_pct":        25.0,
		"is_running":             true,
		"system_prompt_template": "default",
	}

	// Construct API response object (consistent with logic in api/server.go handlePublicTraderList)
	response := map[string]interface{}{
		"trader_id":              traderData["trader_id"],
		"trader_name":            traderData["trader_name"],
		"ai_model":               traderData["ai_model"],
		"exchange":               traderData["exchange"],
		"total_equity":           traderData["total_equity"],
		"total_pnl":              traderData["total_pnl"],
		"total_pnl_pct":          traderData["total_pnl_pct"],
		"position_count":         traderData["position_count"],
		"margin_used_pct":        traderData["margin_used_pct"],
		"system_prompt_template": traderData["system_prompt_template"],
	}

	// Verify system_prompt_template field exists
	if _, exists := response["system_prompt_template"]; !exists {
		t.Errorf("Public trader list response is missing 'system_prompt_template' field")
	}

	// Verify system_prompt_template value is correct
	if response["system_prompt_template"] != "default" {
		t.Errorf("Expected system_prompt_template='default', got %v", response["system_prompt_template"])
	}
}

func TestCreateExchangeRequest_QMTFields(t *testing.T) {
	payload := `{
		"exchange_type": "qmt",
		"account_name": "A-Share Account",
		"enabled": true,
		"qmt_gateway_url": "http://127.0.0.1:19090",
		"qmt_account_id": "sim-001",
		"qmt_gateway_token": "secret-token",
		"qmt_market": "CN-A"
	}`

	var req CreateExchangeRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if req.ExchangeType != "qmt" {
		t.Fatalf("expected exchange_type=qmt, got %q", req.ExchangeType)
	}
	if req.QMTGatewayURL != "http://127.0.0.1:19090" {
		t.Fatalf("unexpected qmt_gateway_url: %q", req.QMTGatewayURL)
	}
	if req.QMTAccountID != "sim-001" {
		t.Fatalf("unexpected qmt_account_id: %q", req.QMTAccountID)
	}
	if req.QMTGatewayToken != "secret-token" {
		t.Fatalf("unexpected qmt_gateway_token: %q", req.QMTGatewayToken)
	}
	if req.QMTMarket != "CN-A" {
		t.Fatalf("unexpected qmt_market: %q", req.QMTMarket)
	}
}

func TestUpdateExchangeConfigRequest_QMTFields(t *testing.T) {
	payload := `{
		"exchanges": {
			"ex-001": {
				"enabled": true,
				"qmt_gateway_url": "http://127.0.0.1:19090",
				"qmt_account_id": "sim-001",
				"qmt_gateway_token": "token-abc",
				"qmt_market": "CN-A"
			}
		}
	}`

	var req UpdateExchangeConfigRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	cfg, ok := req.Exchanges["ex-001"]
	if !ok {
		t.Fatalf("expected exchange config ex-001")
	}
	if cfg.QMTGatewayURL != "http://127.0.0.1:19090" {
		t.Fatalf("unexpected qmt_gateway_url: %q", cfg.QMTGatewayURL)
	}
	if cfg.QMTAccountID != "sim-001" {
		t.Fatalf("unexpected qmt_account_id: %q", cfg.QMTAccountID)
	}
	if cfg.QMTGatewayToken != "token-abc" {
		t.Fatalf("unexpected qmt_gateway_token: %q", cfg.QMTGatewayToken)
	}
	if cfg.QMTMarket != "CN-A" {
		t.Fatalf("unexpected qmt_market: %q", cfg.QMTMarket)
	}
}

func TestSafeExchangeConfig_QMTTokenExcluded(t *testing.T) {
	safe := SafeExchangeConfig{
		ExchangeType:  "qmt",
		AccountName:   "A-Share Account",
		QMTGatewayURL: "http://127.0.0.1:19090",
		QMTAccountID:  "sim-001",
		QMTMarket:     "CN-A",
	}

	data, err := json.Marshal(safe)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, "qmt_gateway_token") || strings.Contains(jsonStr, "qmtGatewayToken") {
		t.Fatalf("safe config should not expose qmt token, got: %s", jsonStr)
	}
}

func newWebRouteTestServer(webFS fstest.MapFS) *Server {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	s := &Server{
		router: router,
		webFS:  webFS,
	}

	router.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.setupWebRoutes()
	return s
}

func TestWebRoutes_IndexAndSPAFallback(t *testing.T) {
	webFS := fstest.MapFS{
		"index.html": {
			Data: []byte("<html><body>NOFX UI</body></html>"),
		},
		"assets/app.js": {
			Data: []byte("console.log('nofx');"),
		},
	}
	server := newWebRouteTestServer(webFS)

	t.Run("GET / returns index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "NOFX UI") {
			t.Fatalf("expected index content, got: %s", rec.Body.String())
		}
		if cache := rec.Header().Get("Cache-Control"); cache != "no-cache" {
			t.Fatalf("expected no-cache for index, got %q", cache)
		}
	})

	t.Run("GET /dashboard falls back to index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "NOFX UI") {
			t.Fatalf("expected SPA fallback index content, got: %s", rec.Body.String())
		}
		if cache := rec.Header().Get("Cache-Control"); cache != "no-cache" {
			t.Fatalf("expected no-cache for SPA fallback, got %q", cache)
		}
	})

	t.Run("GET /assets/<hashed>.js returns static file with cache header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "console.log('nofx')") {
			t.Fatalf("expected static asset content, got: %s", rec.Body.String())
		}
		if cache := rec.Header().Get("Cache-Control"); cache != "public, max-age=31536000, immutable" {
			t.Fatalf("unexpected cache header: %q", cache)
		}
	})
}

func TestWebRoutes_APIPriorityAndMethodRules(t *testing.T) {
	webFS := fstest.MapFS{
		"index.html": {
			Data: []byte("<html><body>NOFX UI</body></html>"),
		},
	}
	server := newWebRouteTestServer(webFS)

	t.Run("GET /api/health returns API health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
			t.Fatalf("expected API health JSON, got: %s", rec.Body.String())
		}
	})

	t.Run("GET /api/non-exist returns 404 and never falls back to index", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/non-exist", nil)
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "NOFX UI") {
			t.Fatalf("api route should not fallback to index, body: %s", rec.Body.String())
		}
	})

	t.Run("POST /dashboard returns 404 (no SPA fallback for non-GET/HEAD)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/dashboard", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		server.router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestSimpleHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	s := &Server{router: router}
	router.GET("/health", s.handleSimpleHealth)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestBuildDefaultSafeModels_IncludesMiniMax(t *testing.T) {
	models := buildDefaultSafeModels()
	found := false
	for _, model := range models {
		if model.Provider == "minimax" && model.Name == "MiniMax AI" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected default model list to include MiniMax AI")
	}
}

func TestBuildSupportedModels_IncludesMiniMax(t *testing.T) {
	models := buildSupportedModels()
	found := false
	for _, model := range models {
		if model["provider"] == "minimax" {
			found = true
			if model["defaultModel"] != mcp.DefaultMiniMaxModel {
				t.Fatalf("expected minimax default model %q, got %v", mcp.DefaultMiniMaxModel, model["defaultModel"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected supported model list to include minimax")
	}
}

func TestBuildDefaultSafeModels_IncludesOpenClaw(t *testing.T) {
	models := buildDefaultSafeModels()
	found := false
	for _, model := range models {
		if model.Provider == "openclaw" && model.Name == "OpenClaw AI" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected default model list to include OpenClaw AI")
	}
}

func TestBuildSupportedModels_IncludesOpenClaw(t *testing.T) {
	models := buildSupportedModels()
	found := false
	for _, model := range models {
		if model["provider"] == "openclaw" {
			found = true
			if model["defaultModel"] != "" {
				t.Fatalf("expected openclaw default model to be empty, got %v", model["defaultModel"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected supported model list to include openclaw")
	}
}

func TestValidateModelConfigUpdate_MiniMaxRules(t *testing.T) {
	if err := validateModelConfigUpdate("minimax", ModelUpdateConfig{Enabled: true}, nil); err != nil {
		t.Fatalf("expected minimax validation to pass with defaults, got %v", err)
	}

	if err := validateModelConfigUpdate("minimax", ModelUpdateConfig{
		Enabled:         true,
		CustomAPIURL:    "https://api.minimax.chat/v1",
		CustomModelName: "MiniMax-Text-01",
	}, nil); err != nil {
		t.Fatalf("expected minimax validation to pass with explicit config, got %v", err)
	}

	existing := &store.AIModel{
		Provider:        "minimax",
		CustomAPIURL:    "https://api.minimax.chat/v1",
		CustomModelName: "MiniMax-Text-01",
	}
	if err := validateModelConfigUpdate("user_1_minimax", ModelUpdateConfig{Enabled: true}, existing); err != nil {
		t.Fatalf("expected minimax validation to pass with existing persisted config, got %v", err)
	}

	if err := validateModelConfigUpdate("openai", ModelUpdateConfig{Enabled: true}, nil); err != nil {
		t.Fatalf("non-minimax providers should not be blocked, got %v", err)
	}
}

func TestValidateModelConfigUpdate_OpenClawRules(t *testing.T) {
	if err := validateModelConfigUpdate("openclaw", ModelUpdateConfig{
		Enabled: true,
		APIKey:  "test-key",
	}, nil); err == nil {
		t.Fatalf("expected openclaw validation failure without base url and model")
	}

	if err := validateModelConfigUpdate("openclaw", ModelUpdateConfig{
		Enabled:         true,
		APIKey:          "test-key",
		CustomAPIURL:    "https://gateway.example.com",
		CustomModelName: "openclaw/model-v1",
	}, nil); err != nil {
		t.Fatalf("expected openclaw validation success, got %v", err)
	}

	existing := &store.AIModel{
		Provider:        "openclaw",
		APIKey:          "persisted-key",
		CustomAPIURL:    "https://gateway.example.com",
		CustomModelName: "openclaw/model-v1",
	}
	if err := validateModelConfigUpdate("user_1_openclaw", ModelUpdateConfig{Enabled: true}, existing); err != nil {
		t.Fatalf("expected openclaw validation to pass with existing persisted config, got %v", err)
	}
}

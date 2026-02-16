package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"os"
	"strconv"
	"strings"
	"time"

	"nofx/store"
	"nofx/trader"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type openClawWebhookEvent struct {
	ID       string         `json:"id"`
	EventID  string         `json:"event_id"`
	Type     string         `json:"type"`
	UserID   string         `json:"user_id"`
	TraderID string         `json:"trader_id"`
	Provider string         `json:"provider"`
	Data     map[string]any `json:"data"`
}

type openClawApprovalDecisionRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleOpenClawWebhookEvent(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read webhook payload"})
		return
	}

	var event openClawWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook payload"})
		return
	}

	payloadUserID := strings.TrimSpace(event.UserID)
	if payloadUserID == "" {
		payloadUserID = stringFromAny(event.Data["user_id"])
	}
	userID := normalizeUserID(payloadUserID)

	secret, err := s.resolveOpenClawWebhookSecret(userID)
	if err != nil {
		SafeInternalError(c, "Resolve OpenClaw webhook secret", err)
		return
	}
	signature := c.GetHeader("X-OpenClaw-Signature")
	timestamp := c.GetHeader("X-OpenClaw-Timestamp")
	if err := verifyOpenClawWebhookSignature(secret, body, signature, timestamp); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	eventType := strings.TrimSpace(event.Type)
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing event type"})
		return
	}

	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = strings.TrimSpace(event.ID)
	}
	if eventID == "" {
		sum := sha256.Sum256(body)
		eventID = hex.EncodeToString(sum[:])
	}

	traderID := strings.TrimSpace(event.TraderID)
	if traderID == "" {
		traderID = stringFromAny(event.Data["trader_id"])
	}

	provider := strings.ToLower(strings.TrimSpace(event.Provider))
	if provider == "" {
		provider = store.OpenClawProvider
	}

	created, err := s.store.OpenClaw().CreateEvent(&store.OpenClawEvent{
		EventID:   eventID,
		UserID:    userID,
		TraderID:  traderID,
		Provider:  provider,
		EventType: eventType,
		Payload:   string(body),
		Signature: signature,
		Status:    "received",
	})
	if err != nil {
		SafeInternalError(c, "Save OpenClaw webhook event", err)
		return
	}
	if !created {
		c.JSON(http.StatusOK, gin.H{
			"message":  "duplicate webhook event",
			"event_id": eventID,
		})
		return
	}

	toolName := extractOpenClawToolName(event.Data)
	riskLevel := classifyOpenClawToolRisk(toolName)
	requestedPayload := marshalJSONOrEmpty(event.Data)

	if eventType == "tool.call.requested" {
		if riskLevel == store.OpenClawRiskWriteHigh {
			approval := &store.OpenClawApprovalRequest{
				UserID:           userID,
				TraderID:         traderID,
				EventID:          eventID,
				Provider:         provider,
				ToolName:         toolName,
				RiskLevel:        riskLevel,
				Status:           store.OpenClawApprovalPending,
				RequestedPayload: requestedPayload,
			}
			if err := s.store.OpenClaw().CreateApproval(approval); err != nil {
				SafeInternalError(c, "Create OpenClaw approval request", err)
				return
			}
			c.JSON(http.StatusAccepted, gin.H{
				"message":           "event accepted",
				"event_id":          eventID,
				"approval_required": true,
				"approval_id":       approval.ID,
				"status":            approval.Status,
			})
			return
		}

		if err := s.store.OpenClaw().CreateToolExecution(&store.OpenClawToolExecution{
			UserID:         userID,
			TraderID:       traderID,
			EventID:        eventID,
			Provider:       provider,
			ToolName:       toolName,
			Status:         store.OpenClawExecutionRequested,
			RequestPayload: requestedPayload,
		}); err != nil {
			SafeInternalError(c, "Create OpenClaw tool execution", err)
			return
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message":           "event accepted",
		"event_id":          eventID,
		"approval_required": false,
		"risk_level":        riskLevel,
	})
}

func (s *Server) handleListOpenClawApprovals(c *gin.Context) {
	userID := normalizeUserID(c.GetString("user_id"))
	status := strings.TrimSpace(c.Query("status"))
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	approvals, err := s.store.OpenClaw().ListApprovals(userID, status, limit)
	if err != nil {
		SafeInternalError(c, "List OpenClaw approvals", err)
		return
	}
	c.JSON(http.StatusOK, approvals)
}

func (s *Server) handleApproveOpenClawApproval(c *gin.Context) {
	userID := normalizeUserID(c.GetString("user_id"))
	approvalID := strings.TrimSpace(c.Param("id"))
	if approvalID == "" {
		SafeBadRequest(c, "approval id is required")
		return
	}

	var req openClawApprovalDecisionRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			SafeBadRequest(c, "Invalid request parameters")
			return
		}
	}

	approval, err := s.store.OpenClaw().Approve(userID, approvalID, userID, req.Reason)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Approve OpenClaw request", err)
		return
	}

	execution := &store.OpenClawToolExecution{
		UserID:         approval.UserID,
		TraderID:       approval.TraderID,
		ApprovalID:     approval.ID,
		EventID:        approval.EventID,
		Provider:       approval.Provider,
		ToolName:       approval.ToolName,
		Status:         store.OpenClawExecutionApproved,
		RequestPayload: approval.RequestedPayload,
	}
	if err := s.store.OpenClaw().CreateToolExecution(execution); err != nil {
		SafeInternalError(c, "Create OpenClaw approved execution record", err)
		return
	}

	execResult, execErr := s.executeApprovedOpenClawTool(approval)
	if execErr != nil {
		logger.Warnf("OpenClaw tool execution failed (approval=%s, tool=%s): %v", approval.ID, approval.ToolName, execErr)
		updateErr := s.store.OpenClaw().UpdateToolExecutionResult(
			approval.UserID,
			execution.ID,
			store.OpenClawExecutionFailed,
			"",
			execErr.Error(),
		)
		if updateErr != nil {
			SafeInternalError(c, "Update OpenClaw failed execution record", updateErr)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"approval":         approval,
			"execution_id":     execution.ID,
			"execution_status": store.OpenClawExecutionFailed,
			"error":            execErr.Error(),
		})
		return
	}

	updateErr := s.store.OpenClaw().UpdateToolExecutionResult(
		approval.UserID,
		execution.ID,
		store.OpenClawExecutionExecuted,
		marshalJSONOrEmpty(execResult),
		"",
	)
	if updateErr != nil {
		SafeInternalError(c, "Update OpenClaw executed record", updateErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"approval":         approval,
		"execution_id":     execution.ID,
		"execution_status": store.OpenClawExecutionExecuted,
		"result":           execResult,
	})
}

func (s *Server) handleRejectOpenClawApproval(c *gin.Context) {
	userID := normalizeUserID(c.GetString("user_id"))
	approvalID := strings.TrimSpace(c.Param("id"))
	if approvalID == "" {
		SafeBadRequest(c, "approval id is required")
		return
	}

	var req openClawApprovalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SafeBadRequest(c, "Invalid request parameters")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		SafeBadRequest(c, "reason is required")
		return
	}

	approval, err := s.store.OpenClaw().Reject(userID, approvalID, userID, reason)
	if err != nil {
		SafeError(c, http.StatusBadRequest, "Reject OpenClaw request", err)
		return
	}

	if err := s.store.OpenClaw().CreateToolExecution(&store.OpenClawToolExecution{
		UserID:         approval.UserID,
		TraderID:       approval.TraderID,
		ApprovalID:     approval.ID,
		EventID:        approval.EventID,
		Provider:       approval.Provider,
		ToolName:       approval.ToolName,
		Status:         store.OpenClawExecutionRejected,
		RequestPayload: approval.RequestedPayload,
		ErrorMessage:   reason,
	}); err != nil {
		SafeInternalError(c, "Create OpenClaw rejected execution record", err)
		return
	}

	c.JSON(http.StatusOK, approval)
}

func classifyOpenClawToolRisk(toolName string) string {
	name := strings.ToLower(strings.TrimSpace(toolName))
	if name == "" {
		return store.OpenClawRiskWriteHigh
	}

	readOnlyKeywords := []string{
		"get_", "list_", "query_", "fetch_", "read_", "balance", "position", "order_status", "kline", "symbol", "market_data", "snapshot",
	}
	for _, keyword := range readOnlyKeywords {
		if strings.Contains(name, keyword) {
			return store.OpenClawRiskReadOnly
		}
	}

	if strings.Contains(name, "cancel") {
		return store.OpenClawRiskWriteLow
	}

	writeHighKeywords := []string{
		"open_", "close_", "create_order", "place_order", "set_leverage", "set_margin", "trade", "execute",
	}
	for _, keyword := range writeHighKeywords {
		if strings.Contains(name, keyword) {
			return store.OpenClawRiskWriteHigh
		}
	}

	return store.OpenClawRiskWriteHigh
}

func extractOpenClawToolName(data map[string]any) string {
	if data == nil {
		return "unknown_tool"
	}
	if tool := strings.TrimSpace(stringFromAny(data["tool_name"])); tool != "" {
		return tool
	}
	if tool := strings.TrimSpace(stringFromAny(data["name"])); tool != "" {
		return tool
	}
	if tool, ok := data["tool"].(map[string]any); ok {
		if name := strings.TrimSpace(stringFromAny(tool["name"])); name != "" {
			return name
		}
	}
	return "unknown_tool"
}

func marshalJSONOrEmpty(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func stringFromAny(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func verifyOpenClawWebhookSignature(secret string, body []byte, signatureHeader string, timestampHeader string) error {
	if strings.TrimSpace(secret) == "" {
		return nil
	}

	rawSig := strings.TrimSpace(signatureHeader)
	if rawSig == "" {
		return fmt.Errorf("missing openclaw webhook signature")
	}
	rawSig = strings.TrimPrefix(rawSig, "sha256=")
	rawSig = strings.TrimPrefix(rawSig, "SHA256=")

	payload := body
	if ts := strings.TrimSpace(timestampHeader); ts != "" {
		parsedTs, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid openclaw webhook timestamp")
		}
		now := time.Now().Unix()
		if parsedTs < now-300 || parsedTs > now+300 {
			return fmt.Errorf("openclaw webhook timestamp out of range")
		}
		payload = []byte(ts + "." + string(body))
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(rawSig)), []byte(strings.ToLower(expected))) {
		return fmt.Errorf("invalid openclaw webhook signature")
	}
	return nil
}

func (s *Server) resolveOpenClawWebhookSecret(userID string) (string, error) {
	if s.store != nil && s.store.AIModel() != nil {
		secret, err := s.store.AIModel().GetProviderWebhookSecret(userID, store.OpenClawProvider)
		if err == nil {
			return strings.TrimSpace(secret), nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}

	return strings.TrimSpace(os.Getenv("OPENCLAW_WEBHOOK_SECRET")), nil
}

func (s *Server) executeApprovedOpenClawTool(approval *store.OpenClawApprovalRequest) (map[string]any, error) {
	if approval == nil {
		return nil, fmt.Errorf("approval is nil")
	}
	userID := normalizeUserID(approval.UserID)
	traderID := strings.TrimSpace(approval.TraderID)
	if traderID == "" {
		return nil, fmt.Errorf("trader_id is required for tool execution")
	}

	fullConfig, err := s.store.Trader().GetFullConfig(userID, traderID)
	if err != nil {
		return nil, fmt.Errorf("failed to load trader config: %w", err)
	}
	if fullConfig.Exchange == nil || !fullConfig.Exchange.Enabled {
		return nil, fmt.Errorf("trader exchange is not configured or disabled")
	}

	tempTrader, err := s.buildTempTraderFromExchange(fullConfig.Exchange, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary trader: %w", err)
	}

	toolInput := parseOpenClawToolInput(approval.RequestedPayload)
	toolName := normalizeOpenClawToolName(approval.ToolName)
	symbol := strings.TrimSpace(extractOpenClawString(toolInput, "symbol", "ticker", "instrument", "inst_id", "instId", "pair", "code"))
	normalizedSymbol := ""
	if symbol != "" {
		normalizedSymbol = market.NormalizeByExchange(fullConfig.Exchange.ExchangeType, symbol)
	}

	switch toolName {
	case "open_long", "openlong", "buy":
		if normalizedSymbol == "" {
			return nil, fmt.Errorf("symbol is required for open_long")
		}
		quantity, err := resolveOpenClawOrderQuantity(toolInput, tempTrader, normalizedSymbol)
		if err != nil {
			return nil, err
		}
		leverage := extractOpenClawInt(toolInput, 1, "leverage", "lev")
		if leverage <= 0 {
			leverage = 1
		}
		return tempTrader.OpenLong(normalizedSymbol, quantity, leverage)
	case "open_short", "openshort", "sell_short":
		if normalizedSymbol == "" {
			return nil, fmt.Errorf("symbol is required for open_short")
		}
		quantity, err := resolveOpenClawOrderQuantity(toolInput, tempTrader, normalizedSymbol)
		if err != nil {
			return nil, err
		}
		leverage := extractOpenClawInt(toolInput, 1, "leverage", "lev")
		if leverage <= 0 {
			leverage = 1
		}
		return tempTrader.OpenShort(normalizedSymbol, quantity, leverage)
	case "close_long", "closelong", "sell":
		if normalizedSymbol == "" {
			return nil, fmt.Errorf("symbol is required for close_long")
		}
		quantity := extractOpenClawFloat(toolInput, 0, "quantity", "qty", "size", "amount", "volume")
		if quantity < 0 {
			quantity = 0
		}
		return tempTrader.CloseLong(normalizedSymbol, quantity)
	case "close_short", "closeshort", "buy_to_cover":
		if normalizedSymbol == "" {
			return nil, fmt.Errorf("symbol is required for close_short")
		}
		quantity := extractOpenClawFloat(toolInput, 0, "quantity", "qty", "size", "amount", "volume")
		if quantity < 0 {
			quantity = 0
		}
		return tempTrader.CloseShort(normalizedSymbol, quantity)
	case "set_leverage", "setleverage":
		if normalizedSymbol == "" {
			return nil, fmt.Errorf("symbol is required for set_leverage")
		}
		leverage := extractOpenClawInt(toolInput, 0, "leverage", "lev")
		if leverage <= 0 {
			return nil, fmt.Errorf("valid leverage is required for set_leverage")
		}
		if err := tempTrader.SetLeverage(normalizedSymbol, leverage); err != nil {
			return nil, err
		}
		return map[string]any{
			"status":   "ok",
			"symbol":   normalizedSymbol,
			"leverage": leverage,
		}, nil
	case "set_margin_mode", "setmarginmode":
		if normalizedSymbol == "" {
			return nil, fmt.Errorf("symbol is required for set_margin_mode")
		}
		isCrossMargin, ok := extractOpenClawBool(toolInput, "is_cross_margin", "isCrossMargin", "cross", "cross_margin")
		if !ok {
			mode := strings.ToLower(strings.TrimSpace(extractOpenClawString(toolInput, "margin_mode", "marginMode", "mode")))
			switch mode {
			case "cross", "cross_margin", "crossed":
				isCrossMargin = true
			case "isolated", "isolate", "isolated_margin":
				isCrossMargin = false
			default:
				return nil, fmt.Errorf("margin mode is required for set_margin_mode")
			}
		}
		if err := tempTrader.SetMarginMode(normalizedSymbol, isCrossMargin); err != nil {
			return nil, err
		}
		marginMode := "isolated"
		if isCrossMargin {
			marginMode = "cross"
		}
		return map[string]any{
			"status":      "ok",
			"symbol":      normalizedSymbol,
			"margin_mode": marginMode,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported OpenClaw tool: %s", approval.ToolName)
	}
}

func normalizeOpenClawToolName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func parseOpenClawToolInput(rawPayload string) map[string]any {
	payload := map[string]any{}
	rawPayload = strings.TrimSpace(rawPayload)
	if rawPayload == "" {
		return payload
	}
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return payload
	}

	merged := make(map[string]any, len(payload))
	for k, v := range payload {
		merged[k] = v
	}

	candidates := []string{"arguments", "args", "input", "params", "tool_input", "toolInput", "request", "payload"}
	for _, key := range candidates {
		if val, ok := lookupOpenClawValue(payload, key); ok {
			switch t := val.(type) {
			case map[string]any:
				for k, v := range t {
					merged[k] = v
				}
			case string:
				var nested map[string]any
				if err := json.Unmarshal([]byte(strings.TrimSpace(t)), &nested); err == nil {
					for k, v := range nested {
						merged[k] = v
					}
				}
			}
		}
	}

	if toolRaw, ok := lookupOpenClawValue(payload, "tool"); ok {
		if toolMap, ok := toolRaw.(map[string]any); ok {
			for k, v := range toolMap {
				merged[k] = v
			}
		}
	}

	return merged
}

func resolveOpenClawOrderQuantity(toolInput map[string]any, tempTrader trader.Trader, symbol string) (float64, error) {
	quantity := extractOpenClawFloat(toolInput, 0, "quantity", "qty", "size", "amount", "volume")
	if quantity > 0 {
		return quantity, nil
	}

	notional := extractOpenClawFloat(toolInput, 0, "notional", "position_size_usd", "position_size")
	if notional <= 0 {
		return 0, fmt.Errorf("quantity is required for order execution")
	}

	price, err := tempTrader.GetMarketPrice(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve quantity from notional: %w", err)
	}
	if price <= 0 {
		return 0, fmt.Errorf("invalid market price %.8f for quantity conversion", price)
	}
	quantity = notional / price
	if quantity <= 0 {
		return 0, fmt.Errorf("calculated quantity is invalid")
	}
	return quantity, nil
}

func lookupOpenClawValue(data map[string]any, keys ...string) (any, bool) {
	if len(data) == 0 || len(keys) == 0 {
		return nil, false
	}
	for _, key := range keys {
		needle := strings.ToLower(strings.TrimSpace(key))
		if needle == "" {
			continue
		}
		for k, v := range data {
			if strings.EqualFold(strings.TrimSpace(k), needle) {
				return v, true
			}
		}
	}
	return nil, false
}

func extractOpenClawString(data map[string]any, keys ...string) string {
	val, ok := lookupOpenClawValue(data, keys...)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringFromAny(val))
}

func extractOpenClawFloat(data map[string]any, fallback float64, keys ...string) float64 {
	val, ok := lookupOpenClawValue(data, keys...)
	if !ok {
		return fallback
	}
	switch t := val.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f
		}
	}
	return fallback
}

func extractOpenClawInt(data map[string]any, fallback int, keys ...string) int {
	val := extractOpenClawFloat(data, float64(fallback), keys...)
	return int(val)
}

func extractOpenClawBool(data map[string]any, keys ...string) (bool, bool) {
	val, ok := lookupOpenClawValue(data, keys...)
	if !ok {
		return false, false
	}
	switch t := val.(type) {
	case bool:
		return t, true
	case string:
		raw := strings.ToLower(strings.TrimSpace(t))
		switch raw {
		case "true", "1", "yes", "y", "on", "cross", "crossed":
			return true, true
		case "false", "0", "no", "n", "off", "isolated", "isolate":
			return false, true
		}
	case int:
		return t != 0, true
	case int32:
		return t != 0, true
	case int64:
		return t != 0, true
	case float64:
		return t != 0, true
	}
	return false, false
}

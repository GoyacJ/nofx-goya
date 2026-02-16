package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"nofx/store"

	"github.com/gin-gonic/gin"
)

func TestClassifyOpenClawToolRisk(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		want     string
	}{
		{name: "read only", toolName: "get_positions", want: store.OpenClawRiskReadOnly},
		{name: "low risk", toolName: "cancel_order", want: store.OpenClawRiskWriteLow},
		{name: "high risk", toolName: "open_long", want: store.OpenClawRiskWriteHigh},
		{name: "unknown defaults high", toolName: "do_something", want: store.OpenClawRiskWriteHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyOpenClawToolRisk(tt.toolName)
			if got != tt.want {
				t.Fatalf("expected risk %q, got %q", tt.want, got)
			}
		})
	}
}

func TestVerifyOpenClawWebhookSignature(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"type":"tool.call.requested"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(body)))
	sig := hex.EncodeToString(mac.Sum(nil))

	if err := verifyOpenClawWebhookSignature(secret, body, "sha256="+sig, timestamp); err != nil {
		t.Fatalf("expected signature verify success, got %v", err)
	}

	if err := verifyOpenClawWebhookSignature(secret, body, "sha256=bad", timestamp); err == nil {
		t.Fatalf("expected signature verify failure")
	}
}

func TestVerifyOpenClawWebhookSignature_TimestampOutOfRange(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"type":"tool.call.requested"}`)
	oldTimestamp := "100"

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(oldTimestamp + "." + string(body)))
	sig := hex.EncodeToString(mac.Sum(nil))

	if err := verifyOpenClawWebhookSignature(secret, body, sig, oldTimestamp); err == nil {
		t.Fatalf("expected timestamp out-of-range error")
	}

	nowTs := time.Now().Unix()
	nowHeader := strconv.FormatInt(nowTs, 10)
	macNow := hmac.New(sha256.New, []byte(secret))
	macNow.Write([]byte(nowHeader + "." + string(body)))
	sigNow := hex.EncodeToString(macNow.Sum(nil))
	if err := verifyOpenClawWebhookSignature(secret, body, sigNow, nowHeader); err != nil {
		t.Fatalf("expected valid timestamp signature, got %v", err)
	}
}

func TestHandleOpenClawWebhookEvent_HighRiskCreatesApprovalAndIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPENCLAW_WEBHOOK_SECRET", "test-openclaw-secret")

	st, err := store.NewWithConfig(store.DBConfig{Type: store.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	s := &Server{store: st}
	router := gin.New()
	router.POST("/api/openclaw/webhooks/events", s.handleOpenClawWebhookEvent)

	payload := []byte(`{"event_id":"evt-high-risk-1","type":"tool.call.requested","user_id":"u1","trader_id":"trader-1","provider":"openclaw","data":{"tool_name":"open_long","symbol":"BTCUSDT"}}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := signOpenClawPayload("test-openclaw-secret", timestamp, payload)

	req := httptest.NewRequest(http.MethodPost, "/api/openclaw/webhooks/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenClaw-Timestamp", timestamp)
	req.Header.Set("X-OpenClaw-Signature", "sha256="+signature)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var acceptedResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &acceptedResp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if acceptedResp["approval_required"] != true {
		t.Fatalf("expected approval_required=true, got %v", acceptedResp["approval_required"])
	}

	approvals, err := st.OpenClaw().ListApprovals("u1", "", 10)
	if err != nil {
		t.Fatalf("failed to list approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected 1 approval, got %d", len(approvals))
	}
	if approvals[0].Status != store.OpenClawApprovalPending {
		t.Fatalf("expected pending approval, got %s", approvals[0].Status)
	}

	dupReq := httptest.NewRequest(http.MethodPost, "/api/openclaw/webhooks/events", bytes.NewReader(payload))
	dupReq.Header.Set("Content-Type", "application/json")
	dupReq.Header.Set("X-OpenClaw-Timestamp", timestamp)
	dupReq.Header.Set("X-OpenClaw-Signature", "sha256="+signature)
	dupRec := httptest.NewRecorder()
	router.ServeHTTP(dupRec, dupReq)

	if dupRec.Code != http.StatusOK {
		t.Fatalf("expected duplicate status %d, got %d", http.StatusOK, dupRec.Code)
	}
	if !strings.Contains(dupRec.Body.String(), "duplicate webhook event") {
		t.Fatalf("expected duplicate event message, got %s", dupRec.Body.String())
	}
}

func TestHandleOpenClawWebhookEvent_UsesUserConfiguredSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPENCLAW_WEBHOOK_SECRET", "fallback-secret")

	st, err := store.NewWithConfig(store.DBConfig{Type: store.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.AIModel().Update(
		"u1",
		"openclaw",
		true,
		"oc-api-key",
		"http://127.0.0.1:18789/v1",
		"openclaw:main",
		"user-specific-secret",
	); err != nil {
		t.Fatalf("failed to seed openclaw model config: %v", err)
	}

	s := &Server{store: st}
	router := gin.New()
	router.POST("/api/openclaw/webhooks/events", s.handleOpenClawWebhookEvent)

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	successPayload := []byte(`{"event_id":"evt-user-secret-ok","type":"tool.call.requested","user_id":"u1","trader_id":"trader-1","provider":"openclaw","data":{"tool_name":"open_long","symbol":"BTCUSDT"}}`)
	successSig := signOpenClawPayload("user-specific-secret", timestamp, successPayload)
	successReq := httptest.NewRequest(http.MethodPost, "/api/openclaw/webhooks/events", bytes.NewReader(successPayload))
	successReq.Header.Set("Content-Type", "application/json")
	successReq.Header.Set("X-OpenClaw-Timestamp", timestamp)
	successReq.Header.Set("X-OpenClaw-Signature", "sha256="+successSig)
	successRec := httptest.NewRecorder()
	router.ServeHTTP(successRec, successReq)
	if successRec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d with user secret, got %d, body=%s", http.StatusAccepted, successRec.Code, successRec.Body.String())
	}

	failPayload := []byte(`{"event_id":"evt-user-secret-fail","type":"tool.call.requested","user_id":"u1","trader_id":"trader-1","provider":"openclaw","data":{"tool_name":"open_long","symbol":"ETHUSDT"}}`)
	failSig := signOpenClawPayload("fallback-secret", timestamp, failPayload)
	failReq := httptest.NewRequest(http.MethodPost, "/api/openclaw/webhooks/events", bytes.NewReader(failPayload))
	failReq.Header.Set("Content-Type", "application/json")
	failReq.Header.Set("X-OpenClaw-Timestamp", timestamp)
	failReq.Header.Set("X-OpenClaw-Signature", "sha256="+failSig)
	failRec := httptest.NewRecorder()
	router.ServeHTTP(failRec, failReq)
	if failRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d with wrong fallback secret, got %d, body=%s", http.StatusUnauthorized, failRec.Code, failRec.Body.String())
	}
}

func TestOpenClawApprovalDecisionHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	st, err := store.NewWithConfig(store.DBConfig{Type: store.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	approveReq := &store.OpenClawApprovalRequest{
		UserID:           "u1",
		TraderID:         "trader-1",
		EventID:          "evt-approve-1",
		ToolName:         "open_long",
		RiskLevel:        store.OpenClawRiskWriteHigh,
		Status:           store.OpenClawApprovalPending,
		RequestedPayload: `{"symbol":"BTCUSDT"}`,
	}
	if err := st.OpenClaw().CreateApproval(approveReq); err != nil {
		t.Fatalf("failed to create approval request: %v", err)
	}

	rejectReq := &store.OpenClawApprovalRequest{
		UserID:           "u1",
		TraderID:         "trader-1",
		EventID:          "evt-reject-1",
		ToolName:         "close_long",
		RiskLevel:        store.OpenClawRiskWriteHigh,
		Status:           store.OpenClawApprovalPending,
		RequestedPayload: `{"symbol":"ETHUSDT"}`,
	}
	if err := st.OpenClaw().CreateApproval(rejectReq); err != nil {
		t.Fatalf("failed to create reject request: %v", err)
	}

	s := &Server{store: st}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "u1")
		c.Next()
	})
	router.POST("/api/openclaw/approvals/:id/approve", s.handleApproveOpenClawApproval)
	router.POST("/api/openclaw/approvals/:id/reject", s.handleRejectOpenClawApproval)

	approveBody := strings.NewReader(`{"reason":"approved by test"}`)
	approveHTTPReq := httptest.NewRequest(http.MethodPost, "/api/openclaw/approvals/"+approveReq.ID+"/approve", approveBody)
	approveHTTPReq.Header.Set("Content-Type", "application/json")
	approveRec := httptest.NewRecorder()
	router.ServeHTTP(approveRec, approveHTTPReq)
	if approveRec.Code != http.StatusOK {
		t.Fatalf("expected approve status %d, got %d, body=%s", http.StatusOK, approveRec.Code, approveRec.Body.String())
	}

	approved, err := st.OpenClaw().GetApproval("u1", approveReq.ID)
	if err != nil {
		t.Fatalf("failed to get approved request: %v", err)
	}
	if approved.Status != store.OpenClawApprovalApproved {
		t.Fatalf("expected approved status, got %s", approved.Status)
	}

	rejectBody := strings.NewReader(`{"reason":"risk too high"}`)
	rejectHTTPReq := httptest.NewRequest(http.MethodPost, "/api/openclaw/approvals/"+rejectReq.ID+"/reject", rejectBody)
	rejectHTTPReq.Header.Set("Content-Type", "application/json")
	rejectRec := httptest.NewRecorder()
	router.ServeHTTP(rejectRec, rejectHTTPReq)
	if rejectRec.Code != http.StatusOK {
		t.Fatalf("expected reject status %d, got %d, body=%s", http.StatusOK, rejectRec.Code, rejectRec.Body.String())
	}

	rejected, err := st.OpenClaw().GetApproval("u1", rejectReq.ID)
	if err != nil {
		t.Fatalf("failed to get rejected request: %v", err)
	}
	if rejected.Status != store.OpenClawApprovalRejected {
		t.Fatalf("expected rejected status, got %s", rejected.Status)
	}

	var executionCount int64
	if err := st.GormDB().Model(&store.OpenClawToolExecution{}).
		Where("approval_id IN ?", []string{approveReq.ID, rejectReq.ID}).
		Count(&executionCount).Error; err != nil {
		t.Fatalf("failed to count execution rows: %v", err)
	}
	if executionCount != 2 {
		t.Fatalf("expected 2 execution records, got %d", executionCount)
	}

	var approveExecution store.OpenClawToolExecution
	if err := st.GormDB().
		Where("approval_id = ?", approveReq.ID).
		First(&approveExecution).Error; err != nil {
		t.Fatalf("failed to load approve execution record: %v", err)
	}
	if approveExecution.Status != store.OpenClawExecutionFailed {
		t.Fatalf("expected approve execution status %s, got %s", store.OpenClawExecutionFailed, approveExecution.Status)
	}
	if approveExecution.ErrorMessage == "" {
		t.Fatalf("expected failed approve execution to contain error message")
	}

	var rejectExecution store.OpenClawToolExecution
	if err := st.GormDB().
		Where("approval_id = ?", rejectReq.ID).
		First(&rejectExecution).Error; err != nil {
		t.Fatalf("failed to load reject execution record: %v", err)
	}
	if rejectExecution.Status != store.OpenClawExecutionRejected {
		t.Fatalf("expected reject execution status %s, got %s", store.OpenClawExecutionRejected, rejectExecution.Status)
	}
}

func TestHandleListOpenClawApprovals_ReturnsCurrentUserOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	st, err := store.NewWithConfig(store.DBConfig{Type: store.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.OpenClaw().CreateApproval(&store.OpenClawApprovalRequest{
		UserID:           "u1",
		ToolName:         "open_long",
		RiskLevel:        store.OpenClawRiskWriteHigh,
		Status:           store.OpenClawApprovalPending,
		RequestedPayload: `{"symbol":"BTCUSDT"}`,
	}); err != nil {
		t.Fatalf("failed to seed u1 approval: %v", err)
	}
	if err := st.OpenClaw().CreateApproval(&store.OpenClawApprovalRequest{
		UserID:           "u2",
		ToolName:         "open_long",
		RiskLevel:        store.OpenClawRiskWriteHigh,
		Status:           store.OpenClawApprovalPending,
		RequestedPayload: `{"symbol":"ETHUSDT"}`,
	}); err != nil {
		t.Fatalf("failed to seed u2 approval: %v", err)
	}

	s := &Server{store: st}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "u1")
		c.Next()
	})
	router.GET("/api/openclaw/approvals", s.handleListOpenClawApprovals)

	req := httptest.NewRequest(http.MethodGet, "/api/openclaw/approvals?limit=10", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var approvals []store.OpenClawApprovalRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &approvals); err != nil {
		t.Fatalf("failed to parse approvals response: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("expected only 1 approval for current user, got %d", len(approvals))
	}
	if approvals[0].UserID != "u1" {
		t.Fatalf("expected approval user_id=u1, got %s", approvals[0].UserID)
	}
}

func signOpenClawPayload(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(body)))
	return hex.EncodeToString(mac.Sum(nil))
}

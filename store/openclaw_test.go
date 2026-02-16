package store

import (
	"testing"
)

func TestOpenClawStore_CreateEventIdempotent(t *testing.T) {
	gdb, err := InitGorm(":memory:")
	if err != nil {
		t.Fatalf("init gorm: %v", err)
	}
	st := NewOpenClawStore(gdb)
	if err := st.initTables(); err != nil {
		t.Fatalf("init tables: %v", err)
	}

	created, err := st.CreateEvent(&OpenClawEvent{
		EventID:   "evt-1",
		UserID:    "u1",
		EventType: "tool.call.requested",
		Payload:   `{"hello":"world"}`,
	})
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}
	if !created {
		t.Fatalf("expected first insert created=true")
	}

	created, err = st.CreateEvent(&OpenClawEvent{
		EventID:   "evt-1",
		UserID:    "u1",
		EventType: "tool.call.requested",
		Payload:   `{"hello":"world"}`,
	})
	if err != nil {
		t.Fatalf("duplicate create event failed: %v", err)
	}
	if created {
		t.Fatalf("expected duplicate insert created=false")
	}
}

func TestOpenClawStore_ApprovalLifecycle(t *testing.T) {
	gdb, err := InitGorm(":memory:")
	if err != nil {
		t.Fatalf("init gorm: %v", err)
	}
	st := NewOpenClawStore(gdb)
	if err := st.initTables(); err != nil {
		t.Fatalf("init tables: %v", err)
	}

	req := &OpenClawApprovalRequest{
		UserID:           "u1",
		EventID:          "evt-2",
		ToolName:         "open_long",
		RiskLevel:        OpenClawRiskWriteHigh,
		RequestedPayload: `{"symbol":"BTCUSDT"}`,
	}
	if err := st.CreateApproval(req); err != nil {
		t.Fatalf("create approval failed: %v", err)
	}

	approved, err := st.Approve("u1", req.ID, "u1", "looks good")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if approved.Status != OpenClawApprovalApproved {
		t.Fatalf("expected status approved, got %s", approved.Status)
	}

	if _, err := st.Reject("u1", req.ID, "u1", "should fail"); err == nil {
		t.Fatalf("expected reject to fail after approval")
	}
}

func TestOpenClawStore_UpdateToolExecutionResult(t *testing.T) {
	gdb, err := InitGorm(":memory:")
	if err != nil {
		t.Fatalf("init gorm: %v", err)
	}
	st := NewOpenClawStore(gdb)
	if err := st.initTables(); err != nil {
		t.Fatalf("init tables: %v", err)
	}

	exec := &OpenClawToolExecution{
		UserID:         "u1",
		ApprovalID:     "a1",
		EventID:        "evt-3",
		ToolName:       "open_long",
		Status:         OpenClawExecutionApproved,
		RequestPayload: `{"symbol":"BTCUSDT","quantity":0.1}`,
	}
	if err := st.CreateToolExecution(exec); err != nil {
		t.Fatalf("create execution failed: %v", err)
	}

	resultPayload := `{"status":"FILLED","orderId":"123"}`
	if err := st.UpdateToolExecutionResult("u1", exec.ID, OpenClawExecutionExecuted, resultPayload, ""); err != nil {
		t.Fatalf("update execution result failed: %v", err)
	}

	var persisted OpenClawToolExecution
	if err := gdb.Where("id = ?", exec.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load execution failed: %v", err)
	}
	if persisted.Status != OpenClawExecutionExecuted {
		t.Fatalf("expected status %s, got %s", OpenClawExecutionExecuted, persisted.Status)
	}
	if persisted.ResultPayload != resultPayload {
		t.Fatalf("expected result payload %q, got %q", resultPayload, persisted.ResultPayload)
	}
}

package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	OpenClawProvider = "openclaw"

	OpenClawRiskReadOnly  = "read_only"
	OpenClawRiskWriteLow  = "write_low_risk"
	OpenClawRiskWriteHigh = "write_high_risk"

	OpenClawApprovalPending  = "pending_approval"
	OpenClawApprovalApproved = "approved"
	OpenClawApprovalRejected = "rejected"
	OpenClawApprovalExecuted = "executed"
	OpenClawApprovalFailed   = "failed"

	OpenClawExecutionRequested = "requested"
	OpenClawExecutionApproved  = "approved"
	OpenClawExecutionExecuted  = "executed"
	OpenClawExecutionRejected  = "rejected"
	OpenClawExecutionFailed    = "failed"
)

type OpenClawStore struct {
	db *gorm.DB
}

type OpenClawEvent struct {
	ID        string `gorm:"column:id;primaryKey;size:36" json:"id"`
	EventID   string `gorm:"column:event_id;not null;uniqueIndex;size:128" json:"event_id"`
	UserID    string `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	TraderID  string `gorm:"column:trader_id;default:'';index" json:"trader_id"`
	Provider  string `gorm:"column:provider;not null;default:openclaw;index" json:"provider"`
	EventType string `gorm:"column:event_type;not null;index" json:"event_type"`
	Payload   string `gorm:"column:payload;type:text" json:"payload"`
	Signature string `gorm:"column:signature;type:text" json:"signature"`
	Status    string `gorm:"column:status;not null;default:received;index" json:"status"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (OpenClawEvent) TableName() string {
	return "openclaw_events"
}

type OpenClawApprovalRequest struct {
	ID               string     `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID           string     `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	TraderID         string     `gorm:"column:trader_id;default:'';index" json:"trader_id"`
	EventID          string     `gorm:"column:event_id;default:'';index" json:"event_id"`
	Provider         string     `gorm:"column:provider;not null;default:openclaw;index" json:"provider"`
	ToolName         string     `gorm:"column:tool_name;not null;index" json:"tool_name"`
	RiskLevel        string     `gorm:"column:risk_level;not null;index" json:"risk_level"`
	Status           string     `gorm:"column:status;not null;default:pending_approval;index" json:"status"`
	DecisionReason   string     `gorm:"column:decision_reason;type:text" json:"decision_reason"`
	RequestedPayload string     `gorm:"column:requested_payload;type:text" json:"requested_payload"`
	RequestedAt      time.Time  `gorm:"column:requested_at;not null;autoCreateTime" json:"requested_at"`
	ApprovedAt       *time.Time `gorm:"column:approved_at" json:"approved_at,omitempty"`
	RejectedAt       *time.Time `gorm:"column:rejected_at" json:"rejected_at,omitempty"`
	DecidedBy        string     `gorm:"column:decided_by;default:''" json:"decided_by"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (OpenClawApprovalRequest) TableName() string {
	return "openclaw_approval_requests"
}

type OpenClawToolExecution struct {
	ID             string `gorm:"column:id;primaryKey;size:36" json:"id"`
	UserID         string `gorm:"column:user_id;not null;default:default;index" json:"user_id"`
	TraderID       string `gorm:"column:trader_id;default:'';index" json:"trader_id"`
	ApprovalID     string `gorm:"column:approval_id;default:'';index" json:"approval_id"`
	EventID        string `gorm:"column:event_id;default:'';index" json:"event_id"`
	Provider       string `gorm:"column:provider;not null;default:openclaw;index" json:"provider"`
	ToolName       string `gorm:"column:tool_name;not null;index" json:"tool_name"`
	Status         string `gorm:"column:status;not null;default:requested;index" json:"status"`
	RequestPayload string `gorm:"column:request_payload;type:text" json:"request_payload"`
	ResultPayload  string `gorm:"column:result_payload;type:text" json:"result_payload"`
	ErrorMessage   string `gorm:"column:error_message;type:text" json:"error_message"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (OpenClawToolExecution) TableName() string {
	return "openclaw_tool_executions"
}

func NewOpenClawStore(db *gorm.DB) *OpenClawStore {
	return &OpenClawStore{db: db}
}

func (s *OpenClawStore) initTables() error {
	return s.db.AutoMigrate(
		&OpenClawEvent{},
		&OpenClawApprovalRequest{},
		&OpenClawToolExecution{},
	)
}

func (s *OpenClawStore) CreateEvent(event *OpenClawEvent) (bool, error) {
	if event == nil {
		return false, fmt.Errorf("event is nil")
	}
	if strings.TrimSpace(event.EventID) == "" {
		return false, fmt.Errorf("event_id is required")
	}

	var existing OpenClawEvent
	if err := s.db.Where("event_id = ?", event.EventID).First(&existing).Error; err == nil {
		return false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	now := time.Now().UTC()
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if strings.TrimSpace(event.UserID) == "" {
		event.UserID = "default"
	}
	if strings.TrimSpace(event.Provider) == "" {
		event.Provider = OpenClawProvider
	}
	if strings.TrimSpace(event.Status) == "" {
		event.Status = "received"
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}

	if err := s.db.Create(event).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *OpenClawStore) CreateApproval(req *OpenClawApprovalRequest) error {
	if req == nil {
		return fmt.Errorf("approval request is nil")
	}
	now := time.Now().UTC()
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	if strings.TrimSpace(req.UserID) == "" {
		req.UserID = "default"
	}
	if strings.TrimSpace(req.Provider) == "" {
		req.Provider = OpenClawProvider
	}
	if strings.TrimSpace(req.Status) == "" {
		req.Status = OpenClawApprovalPending
	}
	if req.RequestedAt.IsZero() {
		req.RequestedAt = now
	}
	return s.db.Create(req).Error
}

func (s *OpenClawStore) ListApprovals(userID, status string, limit int) ([]*OpenClawApprovalRequest, error) {
	if strings.TrimSpace(userID) == "" {
		userID = "default"
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var approvals []*OpenClawApprovalRequest
	query := s.db.Where("user_id = ?", userID)
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Order("requested_at DESC").Limit(limit).Find(&approvals).Error; err != nil {
		return nil, err
	}
	return approvals, nil
}

func (s *OpenClawStore) GetApproval(userID, approvalID string) (*OpenClawApprovalRequest, error) {
	if strings.TrimSpace(userID) == "" {
		userID = "default"
	}
	var approval OpenClawApprovalRequest
	if err := s.db.Where("id = ? AND user_id = ?", approvalID, userID).First(&approval).Error; err != nil {
		return nil, err
	}
	return &approval, nil
}

func (s *OpenClawStore) Approve(userID, approvalID, decidedBy, reason string) (*OpenClawApprovalRequest, error) {
	return s.decide(userID, approvalID, decidedBy, reason, OpenClawApprovalApproved)
}

func (s *OpenClawStore) Reject(userID, approvalID, decidedBy, reason string) (*OpenClawApprovalRequest, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, fmt.Errorf("reject reason is required")
	}
	return s.decide(userID, approvalID, decidedBy, reason, OpenClawApprovalRejected)
}

func (s *OpenClawStore) decide(userID, approvalID, decidedBy, reason, targetStatus string) (*OpenClawApprovalRequest, error) {
	if strings.TrimSpace(userID) == "" {
		userID = "default"
	}
	var updated OpenClawApprovalRequest

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var approval OpenClawApprovalRequest
		if err := tx.Where("id = ? AND user_id = ?", approvalID, userID).First(&approval).Error; err != nil {
			return err
		}
		if approval.Status != OpenClawApprovalPending {
			return fmt.Errorf("approval %s already decided with status %s", approvalID, approval.Status)
		}

		now := time.Now().UTC()
		updates := map[string]any{
			"status":          targetStatus,
			"decision_reason": strings.TrimSpace(reason),
			"decided_by":      strings.TrimSpace(decidedBy),
			"updated_at":      now,
		}
		if targetStatus == OpenClawApprovalApproved {
			updates["approved_at"] = now
		}
		if targetStatus == OpenClawApprovalRejected {
			updates["rejected_at"] = now
		}

		if err := tx.Model(&approval).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", approval.ID).First(&updated).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *OpenClawStore) CreateToolExecution(exe *OpenClawToolExecution) error {
	if exe == nil {
		return fmt.Errorf("tool execution is nil")
	}
	if exe.ID == "" {
		exe.ID = uuid.NewString()
	}
	if strings.TrimSpace(exe.UserID) == "" {
		exe.UserID = "default"
	}
	if strings.TrimSpace(exe.Provider) == "" {
		exe.Provider = OpenClawProvider
	}
	if strings.TrimSpace(exe.Status) == "" {
		exe.Status = OpenClawExecutionRequested
	}
	return s.db.Create(exe).Error
}

func (s *OpenClawStore) UpdateToolExecutionResult(userID, executionID, status, resultPayload, errorMessage string) error {
	if strings.TrimSpace(userID) == "" {
		userID = "default"
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return fmt.Errorf("execution id is required")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return fmt.Errorf("execution status is required")
	}

	updates := map[string]any{
		"status":         status,
		"result_payload": strings.TrimSpace(resultPayload),
		"error_message":  strings.TrimSpace(errorMessage),
		"updated_at":     time.Now().UTC(),
	}

	res := s.db.Model(&OpenClawToolExecution{}).
		Where("id = ? AND user_id = ?", executionID, userID).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

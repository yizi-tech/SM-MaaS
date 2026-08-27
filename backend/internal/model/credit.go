package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type CreditApplicationStatus string

const (
	CreditPending  CreditApplicationStatus = "pending"
	CreditApproved CreditApplicationStatus = "approved"
	CreditRejected CreditApplicationStatus = "rejected"
)

// CreditApplication represents a token credit (授信) application.
// Users who have consumed >= CreditThreshold (5000 CNY) may apply;
// admins approve it by granting a token credit quota (token_credits).
type CreditApplication struct {
	ID            uint            `gorm:"primaryKey" json:"id"`
	UserID        uint            `gorm:"index;not null" json:"user_id"`
	Status        string          `gorm:"size:20;not null;default:pending;index" json:"status"`
	GrantedTokens int64           `gorm:"default:0" json:"granted_tokens"`
	RejectReason  string          `gorm:"size:500" json:"reject_reason"`
	ConsumedTotal decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"consumed_total"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	ReviewedAt    *time.Time      `json:"reviewed_at"`
}

// CreditThreshold is the cumulative consumption (CNY) required to apply.
const CreditThreshold = 5000

func (CreditApplication) TableName() string {
	return "credit_applications"
}
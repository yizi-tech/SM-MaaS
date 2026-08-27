package model

import (
	"time"
)

// CreditCollection records a 催账 (credit collection) action taken by an
// admin against a user with outstanding credit (credit_used > 0).
type CreditCollection struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	AdminID   uint      `gorm:"not null" json:"admin_id"`
	TokensDue int64     `gorm:"not null" json:"tokens_due"` // credit_used at collection time
	Note      string    `gorm:"size:500" json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (CreditCollection) TableName() string {
	return "credit_collections"
}
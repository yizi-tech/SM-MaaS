package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// ConversationLog retains a single LLM chat request/response pair for data
// retention and export (JSONL training / review).
type ConversationLog struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	UserID    uint            `gorm:"index;not null" json:"user_id"`
	ApiKeyID  *uint           `gorm:"index" json:"api_key_id"`
	RequestID string          `gorm:"size:64;index" json:"request_id"`
	Model     string          `gorm:"size:100;index" json:"model"`
	Messages  string          `gorm:"type:json" json:"messages"`
	Response  string          `gorm:"type:json" json:"response"`
	TokensIn     int             `json:"tokens_in"`
	TokensOut    int             `json:"tokens_out"`
	TokensCached int             `json:"tokens_cached"`
	Cost         decimal.Decimal `gorm:"type:decimal(20,6);default:0" json:"cost"`
	Stream    bool            `gorm:"default:false" json:"stream"`
	Status    string          `gorm:"size:20;default:success" json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}

// Feedback is a user-submitted issue/suggestion report (bug, feature
// request, other) tracked by admins.
type Feedback struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"user_id"`
	Type       string     `gorm:"size:20;default:other" json:"type"`
	Title      string     `gorm:"size:200;not null" json:"title"`
	Content    string     `gorm:"type:text;not null" json:"content"`
	Contact    string     `gorm:"size:100" json:"contact"`
	Status     string     `gorm:"size:20;default:pending;index" json:"status"`
	AdminNote  string     `gorm:"type:text" json:"admin_note"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

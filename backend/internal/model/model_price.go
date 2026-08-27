package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// PricingEntry mirrors the per-token input/output prices of a ModelPrice,
// used in listings where only the enabled prices are needed.
type PricingEntry struct {
	Input  decimal.Decimal
	Output decimal.Decimal
}

// ModelPrice is the single source of truth for per-model billing prices.
// Prices are stored per token (CNY); admin APIs and UI exchange values in
// per-1M-token units and convert at the handler layer.
// A model without an enabled ModelPrice entry cannot be invoked.
type ModelPrice struct {
	ID             uint                `gorm:"primaryKey" json:"id"`
	Model          string              `gorm:"size:100;uniqueIndex;not null" json:"model"`
	InputPrice     decimal.Decimal     `gorm:"type:decimal(20,9);not null" json:"input_price"`  // CNY per token
	OutputPrice    decimal.Decimal     `gorm:"type:decimal(20,9);not null" json:"output_price"` // CNY per token
	CacheReadPrice decimal.NullDecimal `gorm:"type:decimal(20,9)" json:"cache_read_price"`      // CNY per token; NULL = 默认输入价×10%
	CacheWritePrice decimal.NullDecimal `gorm:"type:decimal(20,9)" json:"cache_write_price"`   // CNY per token; NULL = 默认输入价×125%
	Enabled        bool                `gorm:"default:true" json:"enabled"`
	Remark         string              `gorm:"size:200" json:"remark"`
	// SupportUnlimited marks the model as eligible for the unlimited-firepower
	// promo (set by admin when configuring the price). It is a static property:
	// only when true can UnlimitedEnabled be toggled on.
	SupportUnlimited bool `gorm:"default:false" json:"support_unlimited"`
	// UnlimitedEnabled switches the unlimited-firepower promo on/off for this
	// model. When true and the caller holds a paid active subscription, usage is
	// not billed (no token/balance deduction).
	UnlimitedEnabled bool `gorm:"default:false" json:"unlimited_enabled"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}
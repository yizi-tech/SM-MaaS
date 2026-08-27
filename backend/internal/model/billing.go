package model

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BillingType string

const (
	BillingPayPerUse    BillingType = "pay_per_use"
	BillingSubscription BillingType = "subscription"
	// BillingUnlimited is used for the unlimited-firepower promo: the request is
	// recorded for audit but no balance or subscription quota is deducted.
	BillingUnlimited BillingType = "unlimited_firepower"
)

type TransactionType string
type TransactionStatus string

const (
	TransactionRecharge TransactionType = "recharge"
	TransactionConsume  TransactionType = "consume"
	TransactionRefund   TransactionType = "refund"
	TransactionSubscription TransactionType = "subscription"
	TransactionTokenPackage  TransactionType = "token_package"
	TransactionAdjust        TransactionType = "adjust"

	TransactionPending   TransactionStatus = "pending"
	TransactionSuccess   TransactionStatus = "success"
	TransactionFailed    TransactionStatus = "failed"
	TransactionRefunded  TransactionStatus = "refunded"
	// TransactionCancelled marks an order that was never paid and expired
	// (e.g. unpaid epay orders auto-cancelled after the 30-minute window).
	TransactionCancelled TransactionStatus = "cancelled"
)

type Plan struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	Name           string          `gorm:"size:100;not null" json:"name"`
	Description    string          `gorm:"size:500" json:"description"`
	Price          decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"price"`
	Currency       string          `gorm:"size:10;default:CNY" json:"currency"`
	DurationDays   int             `gorm:"not null" json:"duration_days"`
	RPM            int             `gorm:"default:60" json:"rpm"`
	TPM            int             `gorm:"default:100000" json:"tpm"`
	IncludedTokens int64           `gorm:"default:0" json:"included_tokens"`
	ConcurrentLimit int            `gorm:"default:10" json:"concurrent_limit"`
	ModelAccess    StringSlice     `gorm:"type:text" json:"model_access"`
	Status         string          `gorm:"size:20;default:active" json:"status"`
	SortOrder      int             `gorm:"default:0" json:"sort_order"`
	// MaxPurchase limits how many times a single user may purchase this plan.
	// 0 means unlimited (the default).
	MaxPurchase    int             `gorm:"default:0" json:"max_purchase"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index" json:"-"`
}

type Subscription struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	UserID      uint            `gorm:"index;not null" json:"user_id"`
	PlanID      uint            `gorm:"not null" json:"plan_id"`
	Status      string          `gorm:"size:20;default:active" json:"status"`
	StartAt     time.Time       `json:"start_at"`
	EndAt       time.Time       `json:"end_at"`
	AutoRenew   bool            `gorm:"default:true" json:"auto_renew"`
	Price       decimal.Decimal `gorm:"type:decimal(10,2)" json:"price"`
	IncludedTokens int64        `gorm:"default:0" json:"included_tokens"`
	UsedTokens  int64           `gorm:"default:0" json:"used_tokens"`
	CancelledAt *time.Time      `json:"cancelled_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
	Plan Plan `gorm:"foreignKey:PlanID" json:"-"`
}

// MatchesModel reports whether the plan grants access to the given model name.
// An empty model access list means the plan grants access to every model;
// entries support exact names or prefix wildcards like "gpt-4o*".
func (p *Plan) MatchesModel(modelName string) bool {
	if len(p.ModelAccess) == 0 {
		return true
	}
	modelName = strings.TrimSpace(modelName)
	for _, m := range p.ModelAccess {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if strings.HasSuffix(m, "*") {
			if strings.HasPrefix(modelName, strings.TrimSuffix(m, "*")) {
				return true
			}
		} else if m == modelName {
			return true
		}
	}
	return false
}

type BillingRecord struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	UserID       uint            `gorm:"index;not null" json:"user_id"`
	RequestID    string          `gorm:"size:100;uniqueIndex" json:"request_id"`
	Model        string          `gorm:"size:100;not null" json:"model"`
	Provider     string          `gorm:"size:50" json:"provider"`
	TokensIn     int             `json:"tokens_in"`
	TokensOut    int             `json:"tokens_out"`
	CachedTokens int             `json:"cached_tokens"`
	TokensCacheWrite int         `json:"tokens_cache_write"`
	Cost         decimal.Decimal `gorm:"type:decimal(20,6)" json:"cost"`
	TTFTMs       int64           `gorm:"default:0" json:"ttft_ms"`         // 首 token 延迟（毫秒）
	DurationMs   int64           `gorm:"default:0" json:"duration_ms"`     // 总耗时（毫秒）
	Detail       string          `gorm:"type:text" json:"detail"`          // 计费公式快照（JSON）
	BillingType  BillingType     `gorm:"size:20" json:"billing_type"`
	DeductType   string          `gorm:"size:20;default:''" json:"deduct_type"` // normal | unlimited_promo | credit
	SubscriptionID *uint         `gorm:"index" json:"subscription_id"`
	ApiKeyID     *uint           `gorm:"index" json:"api_key_id"`
	CreatedAt    time.Time       `json:"created_at"`

	User         User          `gorm:"foreignKey:UserID" json:"-"`
	Subscription *Subscription `gorm:"foreignKey:SubscriptionID" json:"-"`
}

type Transaction struct {
	ID             uint              `gorm:"primaryKey" json:"id"`
	UserID         uint              `gorm:"index;not null" json:"user_id"`
	TransactionNo  string            `gorm:"size:100;uniqueIndex;not null" json:"transaction_no"`
	Type           TransactionType   `gorm:"size:20;not null" json:"type"`
	Amount         decimal.Decimal   `gorm:"type:decimal(20,6);not null" json:"amount"`
	BalanceBefore  decimal.Decimal   `gorm:"type:decimal(20,6)" json:"balance_before"`
	BalanceAfter   decimal.Decimal   `gorm:"type:decimal(20,6)" json:"balance_after"`
	PaymentMethod  string            `gorm:"size:50" json:"payment_method"`
	Status         TransactionStatus `gorm:"size:20;default:pending" json:"status"`
	Description    string            `gorm:"size:500" json:"description"`
	BillingRecordID *uint            `gorm:"index" json:"billing_record_id"`
	SubscriptionID  *uint            `gorm:"index" json:"subscription_id"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

type SystemConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Group     string    `gorm:"size:50" json:"group"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TokenPackage represents a purchasable token top-up package (加油包).
// Purchasing one deducts the balance and credits the user's token credits.
type TokenPackage struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	Name        string          `gorm:"size:100;not null" json:"name"`
	Description string          `gorm:"size:500" json:"description"`
	Tokens      int64           `gorm:"not null" json:"tokens"`
	BonusTokens int64           `gorm:"default:0" json:"bonus_tokens"`
	Price       decimal.Decimal `gorm:"type:decimal(10,2);not null" json:"price"`
	Status      string          `gorm:"size:20;default:active" json:"status"`
	SortOrder   int             `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
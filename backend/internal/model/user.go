package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type UserRole string
type UserStatus string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"

	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Email          string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
	PasswordHash   string         `gorm:"size:255;not null" json:"-"`
	Nickname       string         `gorm:"size:100" json:"nickname"`
	Avatar         string         `gorm:"size:500" json:"avatar"`
	Role           UserRole       `gorm:"size:20;default:user" json:"role"`
	Status         UserStatus     `gorm:"size:20;default:active" json:"status"`
	Balance        decimal.Decimal `gorm:"type:decimal(20,6);default:0" json:"balance"`
	TokenCredits   int64           `gorm:"default:0" json:"token_credits"` // tokens owned via token packages (加油包)
	// TokenAlertThreshold: when the user's combined token balance (token
	// credits + remaining subscription quota) drops below this value, a warning
	// email is sent. 0 disables the alert.
	TokenAlertThreshold int64 `gorm:"default:0" json:"token_alert_threshold"`
	// TokenAlertSent prevents repeated emails while the balance stays below the
	// threshold; it is reset to false once the balance rises back above it.
	TokenAlertSent bool `gorm:"default:false" json:"token_alert_sent"`
	CreditLimit    int64           `gorm:"default:0" json:"credit_limit"`  // tokens approved via credit application (授信总额度)
	CreditUsed     int64           `gorm:"default:0" json:"credit_used"`   // tokens consumed on credit, pending repayment (待还)
	RealNameStatus string         `gorm:"size:20;default:unverified" json:"real_name_status"`
	Phone          string         `gorm:"size:20" json:"phone"`
	QQ             string         `gorm:"size:20" json:"qq"`
	OpenIDUID      string         `gorm:"column:open_id_uid;size:64;uniqueIndex" json:"openid_uid,omitempty"`
	OpenIDUsername string         `gorm:"column:open_id_username;size:100" json:"openid_username,omitempty"`
	LastLoginAt    *time.Time     `json:"last_login_at"`
	LastLoginIP    string         `gorm:"size:45" json:"last_login_ip"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	ApiKeys        []ApiKey        `gorm:"foreignKey:UserID" json:"-"`
	Subscriptions  []Subscription  `gorm:"foreignKey:UserID" json:"-"`
	BillingRecords []BillingRecord `gorm:"foreignKey:UserID" json:"-"`
	Transactions   []Transaction   `gorm:"foreignKey:UserID" json:"-"`
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

type ApiKey struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      uint           `gorm:"index;not null" json:"user_id"`
	KeyHash     string         `gorm:"size:64;uniqueIndex;not null" json:"-"`
	KeyPrefix   string         `gorm:"size:10;not null" json:"key_prefix"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	ModelAccess StringSlice    `gorm:"type:text" json:"model_access"`
	RateLimitID *uint          `gorm:"index" json:"rate_limit_id"`
	Status      string         `gorm:"size:20;default:active" json:"status"`
	LastUsedAt  *time.Time     `json:"last_used_at"`
	ExpiresAt   *time.Time     `json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	User      User       `gorm:"foreignKey:UserID" json:"-"`
	RateLimit *RateLimit `gorm:"foreignKey:RateLimitID" json:"-"`
}

type StringSlice []string

func (s StringSlice) GormDataType() string {
	return "text"
}

// Value implements driver.Valuer so GORM can serialize the slice into the
// text column. JSON is used so the order is preserved deterministically.
func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner. It accepts JSON arrays as well as plain
// comma-separated strings (legacy format) for robustness.
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("StringSlice.Scan: unsupported type %T", value)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		*s = StringSlice{}
		return nil
	}

	var out []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return err
		}
	} else {
		// Legacy comma-separated format.
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	*s = out
	return nil
}

type RateLimit struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	UserID          *uint  `gorm:"index" json:"user_id"`
	ApiKeyID        *uint  `gorm:"index" json:"api_key_id"`
	Model           string `gorm:"size:100" json:"model"`
	RPM             int    `gorm:"default:60" json:"rpm"`
	TPM             int    `gorm:"default:100000" json:"tpm"`
	ConcurrentLimit int    `gorm:"default:10" json:"concurrent_limit"`
	Priority        int    `gorm:"default:0" json:"priority"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type IdentityVerification struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"uniqueIndex;not null" json:"user_id"`
	RealName       string     `gorm:"size:100;not null" json:"real_name"`
	IDNumber       string     `gorm:"size:30;not null" json:"id_number"`
	IDCardFront    string     `gorm:"size:500" json:"id_card_front"`
	IDCardBack     string     `gorm:"size:500" json:"id_card_back"`
	Status         string     `gorm:"size:20;default:pending" json:"status"`
	RejectReason   string     `gorm:"size:500" json:"reject_reason"`
	ReviewerID     *uint      `json:"reviewer_id"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	User     User  `gorm:"foreignKey:UserID" json:"-"`
	Reviewer *User `gorm:"foreignKey:ReviewerID" json:"-"`
}
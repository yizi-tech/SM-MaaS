package dto

type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=6,max=100"`
	Nickname     string `json:"nickname" binding:"required,min=2,max=50"`
	VerifyCode   string `json:"verify_code"`
	VerifyMethod string `json:"verify_method"` // email | sms (default email)
	Phone        string `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo struct {
	ID             uint   `json:"id"`
	Email          string `json:"email"`
	Nickname       string `json:"nickname"`
	Avatar         string `json:"avatar"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	Balance        string `json:"balance"`
	TokenCredits   int64  `json:"token_credits"`
	CreditUsed     int64  `json:"credit_used"`
	TokenAlertThreshold int64 `json:"token_alert_threshold"`
	RealNameStatus string `json:"real_name_status"`
	Phone          string `json:"phone"`
	QQ             string `json:"qq"`
	CreatedAt      string `json:"created_at"`
}

type UpdateProfileRequest struct {
	Nickname            string `json:"nickname"`
	Phone               string `json:"phone"`
	QQ                  string `json:"qq"`
	Avatar              string `json:"avatar"`
	TokenAlertThreshold int64  `json:"token_alert_threshold"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=100"`
	// VerifyMethod + VerifyCode: the one-time code proves ownership of the
	// registered email or bound phone before the password can be changed.
	VerifyMethod string `json:"verify_method"` // email | sms (default: email)
	VerifyCode   string `json:"verify_code" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type IdentityVerificationRequest struct {
	RealName    string `json:"real_name" binding:"required"`
	IDNumber    string `json:"id_number" binding:"required"`
	IDCardFront string `json:"id_card_front" binding:"required"`
	IDCardBack  string `json:"id_card_back" binding:"required"`
}

type ApiKeyRequest struct {
	Name        string   `json:"name" binding:"required"`
	ModelAccess []string `json:"model_access"`
}

type ApiKeyResponse struct {
	ID          uint     `json:"id"`
	KeyPrefix   string   `json:"key_prefix"`
	FullKey     string   `json:"full_key,omitempty"`
	Name        string   `json:"name"`
	ModelAccess []string `json:"model_access"`
	Status      string   `json:"status"`
	LastUsedAt  string   `json:"last_used_at"`
	ExpiresAt   string   `json:"expires_at"`
	CreatedAt   string   `json:"created_at"`
}

type RechargeRequest struct {
	Amount        string `json:"amount" binding:"required,numeric"`
	PaymentMethod string `json:"payment_method" binding:"required"`
}

type SubscribeRequest struct {
	PlanID uint `json:"plan_id" binding:"required"`
}

type PlanResponse struct {
	ID              uint     `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Price           string   `json:"price"`
	Currency        string   `json:"currency"`
	DurationDays    int      `json:"duration_days"`
	RPM             int      `json:"rpm"`
	TPM             int      `json:"tpm"`
	IncludedTokens  int64    `json:"included_tokens"`
	ConcurrentLimit int      `json:"concurrent_limit"`
	ModelAccess     []string `json:"model_access"`
	MaxPurchase     int      `json:"max_purchase"`
}

type BillingRecordResponse struct {
	ID           uint   `json:"id"`
	RequestID    string `json:"request_id"`
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	TokensIn     int    `json:"tokens_in"`
	TokensOut    int    `json:"tokens_out"`
	CachedTokens int    `json:"cached_tokens"`
	CacheWrite   int    `json:"tokens_cache_write"`
	Cost         string `json:"cost"`
	TTFTMs       int64  `json:"ttft_ms"`
	DurationMs   int64  `json:"duration_ms"`
	Detail       string `json:"detail"`
	BillingType  string `json:"billing_type"`
	CreatedAt    string `json:"created_at"`
}

type TransactionResponse struct {
	ID            uint   `json:"id"`
	TransactionNo string `json:"transaction_no"`
	Type          string `json:"type"`
	Amount        string `json:"amount"`
	BalanceBefore string `json:"balance_before"`
	BalanceAfter  string `json:"balance_after"`
	PaymentMethod string `json:"payment_method"`
	Status        string `json:"status"`
	Description   string `json:"description"`
	CreatedAt     string `json:"created_at"`
}

type SubscriptionResponse struct {
	ID             uint   `json:"id"`
	PlanName       string `json:"plan_name"`
	Status         string `json:"status"`
	StartAt        string `json:"start_at"`
	EndAt          string `json:"end_at"`
	AutoRenew      bool   `json:"auto_renew"`
	Price          string `json:"price"`
	UsedTokens     int64  `json:"used_tokens"`
	IncludedTokens int64  `json:"included_tokens"`
}

type PaginationRequest struct {
	Page int `form:"page" binding:"min=1"`
	Size int `form:"size" binding:"min=1,max=100"`
}

func DefaultPagination() (int, int) {
	return 1, 20
}

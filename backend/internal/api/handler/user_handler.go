package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/mass-platform/backend/internal/api/dto"
	"github.com/mass-platform/backend/internal/auth"
	"github.com/mass-platform/backend/internal/billing"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/payment"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/internal/service"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/mass-platform/backend/pkg/response"
)

// UserHandler handles user-related API endpoints.
const (
	loginMaxFailures = 5
	loginLockWindow  = 15 * time.Minute
)

type loginFailEntry struct {
	count   int
	blocked time.Time
}

type UserHandler struct {
	authService     *auth.AuthService
	loginFailures   map[string]*loginFailEntry
	loginMu         sync.Mutex
	userRepo        *repository.UserRepository
	apiKeyRepo      *repository.ApiKeyRepository
	billingService  *billing.BillingService
	billingRepo     *repository.BillingRecordRepository
	txRepo          *repository.TransactionRepository
	planRepo        *repository.PlanRepository
	subRepo         *repository.SubscriptionRepository
	identityRepo    *repository.IdentityVerificationRepository
	tokenPkgRepo    *repository.TokenPackageRepository
	resetCouponRepo *repository.ResetCouponRepository
	notifRepo       *repository.NotificationRepository
	configRepo      *repository.SystemConfigRepository
	invoiceRepo     *repository.InvoiceRepository
	creditRepo      *repository.CreditRepository
	channelRepo     *repository.ChannelRepository
	verifySvc       *service.VerifyCodeService
	openidSvc       *service.OpenIDService
	uploadDir       string
	uploadMaxSizeMB int64
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(
	authService *auth.AuthService,
	userRepo *repository.UserRepository,
	apiKeyRepo *repository.ApiKeyRepository,
	billingService *billing.BillingService,
	billingRepo *repository.BillingRecordRepository,
	txRepo *repository.TransactionRepository,
	planRepo *repository.PlanRepository,
	subRepo *repository.SubscriptionRepository,
	identityRepo *repository.IdentityVerificationRepository,
	tokenPkgRepo *repository.TokenPackageRepository,
	resetCouponRepo *repository.ResetCouponRepository,
	notifRepo *repository.NotificationRepository,
	configRepo *repository.SystemConfigRepository,
	invoiceRepo *repository.InvoiceRepository,
	creditRepo *repository.CreditRepository,
	channelRepo *repository.ChannelRepository,
	verifySvc *service.VerifyCodeService,
	openidSvc *service.OpenIDService,
	uploadDir string,
	uploadMaxSizeMB int64,
) *UserHandler {
	return &UserHandler{
		authService:     authService,
		loginFailures:   make(map[string]*loginFailEntry),
		userRepo:        userRepo,
		apiKeyRepo:      apiKeyRepo,
		billingService:  billingService,
		billingRepo:     billingRepo,
		txRepo:          txRepo,
		planRepo:        planRepo,
		subRepo:         subRepo,
		identityRepo:    identityRepo,
		tokenPkgRepo:    tokenPkgRepo,
		resetCouponRepo: resetCouponRepo,
		notifRepo:       notifRepo,
		configRepo:      configRepo,
		invoiceRepo:     invoiceRepo,
		creditRepo:      creditRepo,
		channelRepo:     channelRepo,
		verifySvc:       verifySvc,
		openidSvc:       openidSvc,
		uploadDir:       uploadDir,
		uploadMaxSizeMB: uploadMaxSizeMB,
	}
}

// generateAPIKey creates a random API key with "sk-" prefix and returns
// the full key, its SHA-256 hash, and any error encountered.
func generateAPIKey() (fullKey string, keyHash string, keyPrefix string, err error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate API key: %w", err)
	}

	fullKey = "sk-" + hex.EncodeToString(keyBytes)
	hash := sha256.Sum256([]byte(fullKey))
	keyHash = hex.EncodeToString(hash[:])
	keyPrefix = fullKey[:10] // "sk-" + first 7 hex chars (fits size:10 column)

	return fullKey, keyHash, keyPrefix, nil
}

// randomHex returns a random hex string of n bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// getUserID extracts the authenticated user's ID from the gin context.
func getUserID(c *gin.Context) (uint, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := raw.(uint)
	return id, ok
}

// defaultAvatar is the fallback avatar shown when a user has neither a QQ
// number nor a custom avatar (served from the frontend assets).
const defaultAvatar = "/assets/sm-1.png"

// qqAvatarURL builds the QQ CDN avatar URL for a QQ number. q1.qlogo.cn is the
// official avatar endpoint used by qq.com; s=640 is the largest variant.
func qqAvatarURL(qq string) string {
	if qq == "" {
		return ""
	}
	return "https://q1.qlogo.cn/g?b=qq&nk=" + qq + "&s=640"
}

// validQQ reports whether the string is a plausible QQ number (5-11 digits).
func validQQ(qq string) bool {
	if len(qq) < 5 || len(qq) > 11 {
		return false
	}
	for _, r := range qq {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// resolveAvatar derives the display avatar for a user: the QQ CDN avatar (auto
// fetched from qq.com) when the user has filled in a QQ number; otherwise the
// default logo. Uploaded avatars are no longer used.
func resolveAvatar(user *model.User) string {
	if user != nil && user.QQ != "" {
		if url := qqAvatarURL(user.QQ); url != "" {
			return url
		}
	}
	return defaultAvatar
}

// toUserInfo converts a model.User to a dto.UserInfo.
func toUserInfo(user *model.User) dto.UserInfo {
	return dto.UserInfo{
		ID:             user.ID,
		Email:          user.Email,
		Nickname:       user.Nickname,
		Avatar:         resolveAvatar(user),
		Role:           string(user.Role),
		Status:         string(user.Status),
		Balance:        user.Balance.String(),
		TokenCredits:   user.TokenCredits,
		CreditUsed:     user.CreditUsed,
		TokenAlertThreshold: user.TokenAlertThreshold,
		RealNameStatus: user.RealNameStatus,
		Phone:          user.Phone,
		QQ:             user.QQ,
		CreatedAt:      user.CreatedAt.Format(time.RFC3339),
	}
}

// toApiKeyResponse converts a model.ApiKey to a dto.ApiKeyResponse without the full key.
func toApiKeyResponse(key *model.ApiKey) dto.ApiKeyResponse {
	lastUsedAt := ""
	if key.LastUsedAt != nil {
		lastUsedAt = key.LastUsedAt.Format(time.RFC3339)
	}
	expiresAt := ""
	if key.ExpiresAt != nil {
		expiresAt = key.ExpiresAt.Format(time.RFC3339)
	}
	return dto.ApiKeyResponse{
		ID:          key.ID,
		KeyPrefix:   key.KeyPrefix,
		Name:        key.Name,
		ModelAccess: key.ModelAccess,
		Status:      key.Status,
		LastUsedAt:  lastUsedAt,
		ExpiresAt:   expiresAt,
		CreatedAt:   key.CreatedAt.Format(time.RFC3339),
	}
}

// toBillingRecordResponse converts a model.BillingRecord to a dto.BillingRecordResponse.
func toBillingRecordResponse(r *model.BillingRecord) dto.BillingRecordResponse {
	return dto.BillingRecordResponse{
		ID:           r.ID,
		RequestID:    r.RequestID,
		Model:        r.Model,
		Provider:     r.Provider,
		TokensIn:     r.TokensIn,
		TokensOut:    r.TokensOut,
		CachedTokens: r.CachedTokens,
		CacheWrite:   r.TokensCacheWrite,
		Cost:         r.Cost.String(),
		TTFTMs:       r.TTFTMs,
		DurationMs:   r.DurationMs,
		Detail:       r.Detail,
		BillingType:  string(r.BillingType),
		CreatedAt:    r.CreatedAt.Format(time.RFC3339),
	}
}

// toTransactionResponse converts a model.Transaction to a dto.TransactionResponse.
func toTransactionResponse(t *model.Transaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:            t.ID,
		TransactionNo: t.TransactionNo,
		Type:          string(t.Type),
		Amount:        t.Amount.String(),
		BalanceBefore: t.BalanceBefore.String(),
		BalanceAfter:  t.BalanceAfter.String(),
		PaymentMethod: t.PaymentMethod,
		Status:        string(t.Status),
		Description:   t.Description,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
	}
}

// toPlanResponse converts a model.Plan to a dto.PlanResponse.
func toPlanResponse(p *model.Plan) dto.PlanResponse {
	return dto.PlanResponse{
		ID:              p.ID,
		Name:            p.Name,
		Description:     p.Description,
		Price:           p.Price.String(),
		Currency:        p.Currency,
		DurationDays:    p.DurationDays,
		RPM:             p.RPM,
		TPM:             p.TPM,
		IncludedTokens:  p.IncludedTokens,
		ConcurrentLimit: p.ConcurrentLimit,
		ModelAccess:     p.ModelAccess,
	}
}

// toSubscriptionResponse converts a model.Subscription to a dto.SubscriptionResponse.
func toSubscriptionResponse(s *model.Subscription) dto.SubscriptionResponse {
	return dto.SubscriptionResponse{
		ID:             s.ID,
		PlanName:       s.Plan.Name,
		Status:         s.Status,
		StartAt:        s.StartAt.Format(time.RFC3339),
		EndAt:          s.EndAt.Format(time.RFC3339),
		AutoRenew:      s.AutoRenew,
		Price:          s.Price.String(),
		UsedTokens:     s.UsedTokens,
		IncludedTokens: s.Plan.IncludedTokens,
	}
}

// parsePagination is defined in admin_handler.go (shared within package handler).

// Register handles user registration (POST /api/v1/auth/register).
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Check if email already exists
	if _, err := h.userRepo.FindByEmail(req.Email); err == nil {
		response.Error(c, http.StatusConflict, "email already registered")
		return
	}

	// Verify code (email or SMS) before creating the account
	channel := req.VerifyMethod
	if channel == "" {
		channel = service.ChannelEmail
	}
	if channel != service.ChannelEmail && channel != service.ChannelSMS {
		response.BadRequest(c, "invalid verify_method")
		return
	}
	target := req.Email
	if channel == service.ChannelSMS {
		target = req.Phone
		if target == "" {
			response.BadRequest(c, "phone is required for SMS verification")
			return
		}
	}
	if h.verifySvc != nil {
		if err := h.verifySvc.Verify(c.Request.Context(), channel, target, req.VerifyCode); err != nil {
			switch err {
			case service.ErrInvalidCode, service.ErrCodeExpired:
				response.BadRequest(c, "验证码错误或已过期，请重新获取")
			case service.ErrTooManyAttempts:
				response.BadRequest(c, "尝试次数过多，请重新获取验证码")
			default:
				response.BadRequest(c, "验证失败，请重试")
			}
			return
		}
	}

	// Create user via auth service
	user, err := h.authService.Register(req.Email, req.Password, req.Nickname)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to register: %v", err))
		return
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}

	// Save user to database
	if err := h.userRepo.Create(user); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to save user: %v", err))
		return
	}
	if user.Phone != "" {
		if err := h.userRepo.Update(user); err != nil {
			logging.Warn("auth", "register", "failed to persist phone", map[string]interface{}{"email": req.Email})
		}
	}

	// Generate token
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to generate token: %v", err))
		return
	}

	response.Success(c, gin.H{
		"token": token,
		"user":  toUserInfo(user),
	})
}

// SendVerifyCode sends a one-time verification code for registration
// (POST /api/v1/auth/send-code, public, no auth).
func (h *UserHandler) SendVerifyCode(c *gin.Context) {
	var req struct {
		Method string `json:"method"` // email | sms
		Email  string `json:"email"`
		Phone  string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	method := req.Method
	if method == "" {
		method = service.ChannelEmail
	}
	var target string
	switch method {
	case service.ChannelEmail:
		target = req.Email
	case service.ChannelSMS:
		target = req.Phone
	default:
		response.BadRequest(c, "invalid method")
		return
	}
	if target == "" {
		response.BadRequest(c, "请填写接收验证码的邮箱或手机号")
		return
	}
	if h.verifySvc == nil {
		response.InternalError(c, "验证码服务未配置")
		return
	}
	brand := ""
	if h.configRepo != nil {
		if configs, err := h.configRepo.GetAll(); err == nil {
			for _, cfg := range configs {
				if cfg.Key == "site_name" {
					brand = cfg.Value
					break
				}
			}
		}
	}
	if err := h.verifySvc.SendWithBrand(c.Request.Context(), method, target, brand); err != nil {
		switch err {
		case service.ErrResendTooSoon:
			response.BadRequest(c, "发送过于频繁，请 60 秒后再试")
		case service.ErrChannelDisabled:
			if method == service.ChannelSMS {
				response.BadRequest(c, "短信验证码服务未开通，请使用邮箱验证")
			} else {
				response.InternalError(c, "邮件服务未配置，请联系管理员")
			}
		default:
			response.InternalError(c, "验证码发送失败，请稍后重试")
		}
		return
	}
	response.Success(c, gin.H{"message": "验证码已发送"})
}

// Login handles user login (POST /api/v1/auth/login).
// loginLocked reports whether the account is currently locked out after too
// many consecutive failed login attempts.
func (h *UserHandler) loginLocked(email string) (bool, time.Duration) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	e, ok := h.loginFailures[email]
	if !ok || e.count < loginMaxFailures {
		return false, 0
	}
	if remain := time.Until(e.blocked); remain > 0 {
		return true, remain
	}
	delete(h.loginFailures, email)
	return false, 0
}

// recordLoginFailure increments the failure counter for an account and
// activates the lockout window once the threshold is reached.
func (h *UserHandler) recordLoginFailure(email string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	e, ok := h.loginFailures[email]
	if !ok {
		e = &loginFailEntry{}
		h.loginFailures[email] = e
	}
	e.count++
	if e.count >= loginMaxFailures {
		e.blocked = time.Now().Add(loginLockWindow)
	}
}

func (h *UserHandler) clearLoginFailures(email string) {
	h.loginMu.Lock()
	defer h.loginMu.Unlock()
	delete(h.loginFailures, email)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if locked, remain := h.loginLocked(email); locked {
		response.Error(c, http.StatusTooManyRequests,
			fmt.Sprintf("too many failed attempts, try again in %d minutes", int(remain.Minutes())+1))
		return
	}

	user, token, err := h.authService.Login(email, req.Password)
	if err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			h.recordLoginFailure(email)
			response.Unauthorized(c, "invalid email or password")
		case auth.ErrUserDisabled:
			response.Forbidden(c, "account is disabled")
		default:
			response.InternalError(c, fmt.Sprintf("login failed: %v", err))
		}
		return
	}

	h.clearLoginFailures(email)
	response.Success(c, dto.LoginResponse{
		Token: token,
		User:  toUserInfo(user),
	})
}

// GetProfile returns the current user's profile (GET /api/v1/user/profile).
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	response.Success(c, toUserInfo(user))
}

// UpdateProfile updates the current user's profile (PUT /api/v1/user/profile).
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.QQ != "" {
		if !validQQ(req.QQ) {
			response.BadRequest(c, "invalid qq number")
			return
		}
		user.QQ = req.QQ
	}
	if req.Avatar != "" {
		if !strings.HasPrefix(req.Avatar, "http://") && !strings.HasPrefix(req.Avatar, "https://") && !strings.HasPrefix(req.Avatar, "/uploads/") {
			response.BadRequest(c, "invalid avatar url")
			return
		}
		user.Avatar = req.Avatar
	}

	if err := h.userRepo.Update(user); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to update profile: %v", err))
		return
	}

	response.Success(c, toUserInfo(user))
}

// UpdateTokenAlert sets the current user's low-token-balance alert threshold
// (PUT /api/v1/user/token-alert). A threshold of 0 disables the alert.
func (h *UserHandler) UpdateTokenAlert(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Threshold int64 `json:"threshold" binding:"gte=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: threshold must be >= 0")
		return
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	user.TokenAlertThreshold = req.Threshold
	// Reset the sent flag whenever the threshold changes so a fresh alert can
	// fire if the balance is already below the new value.
	user.TokenAlertSent = false
	if err := h.userRepo.Update(user); err != nil {
		response.InternalError(c, "failed to update alert threshold")
		return
	}
	response.Success(c, gin.H{"token_alert_threshold": user.TokenAlertThreshold})
}

// SendPasswordVerifyCode sends a one-time code (email or SMS) to the
// current user's registered email / bound phone for password changes
// (POST /api/v1/user/password/send-code, JWT auth).
func (h *UserHandler) SendPasswordVerifyCode(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Method string `json:"method"` // email | sms (default: email)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	method := req.Method
	if method == "" {
		method = service.ChannelEmail
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		response.InternalError(c, "failed to load user")
		return
	}

	var target string
	switch method {
	case service.ChannelEmail:
		target = user.Email
	case service.ChannelSMS:
		target = user.Phone
		if target == "" {
			response.BadRequest(c, "请先在个人设置中绑定手机号，或改用邮箱验证")
			return
		}
	default:
		response.BadRequest(c, "invalid method")
		return
	}
	if target == "" {
		response.BadRequest(c, "账号未绑定可用的邮箱或手机号")
		return
	}

	if h.verifySvc == nil {
		response.InternalError(c, "验证码服务未配置")
		return
	}

	brand := ""
	if h.configRepo != nil {
		if configs, err := h.configRepo.GetAll(); err == nil {
			for _, cfg := range configs {
				if cfg.Key == "site_name" {
					brand = cfg.Value
					break
				}
			}
		}
	}

	if err := h.verifySvc.SendForPurpose(c.Request.Context(), method, target, "修改密码", brand); err != nil {
		switch err {
		case service.ErrResendTooSoon:
			response.BadRequest(c, "发送过于频繁，请 60 秒后再试")
		case service.ErrChannelDisabled:
			if method == service.ChannelSMS {
				response.BadRequest(c, "短信验证码服务未开通，请改用邮箱验证")
			} else {
				response.InternalError(c, "邮件服务未配置，请联系管理员")
			}
		default:
			response.InternalError(c, "验证码发送失败，请稍后重试")
		}
		return
	}

	response.Success(c, gin.H{"target": maskTarget(target)})
}

// maskTarget hides most of an email or phone number for display purposes.
func maskTarget(target string) string {
	if strings.Contains(target, "@") {
		parts := strings.SplitN(target, "@", 2)
		name := parts[0]
		if len(name) <= 1 {
			return target
		}
		return name[:1] + strings.Repeat("*", len(name)-1) + "@" + parts[1]
	}
	if len(target) >= 8 {
		return target[:3] + strings.Repeat("*", len(target)-7) + target[len(target)-4:]
	}
	return strings.Repeat("*", len(target))
}

// siteOrigin returns the platform's base URL (e.g. "https://mass.yiziyun.com")
// from system_configs.site_url, so that OpenID callbacks arriving on a
// different host (maas.yiziyun.com) can redirect back to the primary console.
func (h *UserHandler) siteOrigin() string {
	if h.configRepo != nil {
		configs, err := h.configRepo.GetAll()
		if err == nil {
			for _, cfg := range configs {
				if cfg.Key == "site_url" {
					origin := strings.TrimRight(cfg.Value, "/")
					if origin != "" {
						return origin
					}
				}
			}
		}
	}
	return "https://mass.yiziyun.com"
}

// ChangePassword changes the current user's password (PUT /api/v1/user/password).
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}

	method := req.VerifyMethod
	if method == "" {
		method = service.ChannelEmail
	}
	if method != service.ChannelEmail && method != service.ChannelSMS {
		response.BadRequest(c, "invalid verify_method")
		return
	}

	// Verify the code (email or SMS) before changing the password. The old
	// password is checked first so a consumed code is not wasted on a wrong
	// current password.
	if err := h.authService.VerifyOldPassword(userID, req.OldPassword); err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			response.BadRequest(c, "current password is incorrect")
		default:
			response.InternalError(c, fmt.Sprintf("failed to change password: %v", err))
		}
		return
	}

	target := ""
	if h.verifySvc != nil {
		user, uerr := h.userRepo.FindByID(userID)
		if uerr != nil {
			response.InternalError(c, "failed to load user")
			return
		}
		if method == service.ChannelSMS {
			if user.Phone == "" {
				response.BadRequest(c, "账号未绑定手机号，无法使用短信验证")
				return
			}
			target = user.Phone
		} else {
			target = user.Email
		}
		if err := h.verifySvc.Verify(c.Request.Context(), method, target, req.VerifyCode); err != nil {
			switch err {
			case service.ErrInvalidCode, service.ErrCodeExpired:
				response.BadRequest(c, "验证码错误或已过期，请重新获取")
			case service.ErrTooManyAttempts:
				response.BadRequest(c, "尝试次数过多，请重新获取验证码")
			default:
				response.BadRequest(c, "验证失败，请重试")
			}
			return
		}
	}

	if err := h.authService.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		switch err {
		case auth.ErrInvalidCredentials:
			response.BadRequest(c, "current password is incorrect")
		default:
			response.InternalError(c, fmt.Sprintf("failed to change password: %v", err))
		}
		return
	}

	response.SuccessWithMessage(c, "password changed successfully", nil)
}

// GetApiKeys lists all API keys for the current user (GET /api/v1/user/api-keys).
func (h *UserHandler) GetApiKeys(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	keys, err := h.apiKeyRepo.FindByUserID(userID)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to list API keys: %v", err))
		return
	}

	items := make([]dto.ApiKeyResponse, 0, len(keys))
	for i := range keys {
		items = append(items, toApiKeyResponse(&keys[i]))
	}

	response.Success(c, items)
}

// CreateApiKey creates a new API key for the current user (POST /api/v1/user/api-keys).
func (h *UserHandler) CreateApiKey(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.ApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}

	fullKey, keyHash, keyPrefix, err := generateAPIKey()
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to generate API key: %v", err))
		return
	}

	apiKey := &model.ApiKey{
		UserID:      userID,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Name:        req.Name,
		ModelAccess: req.ModelAccess,
		Status:      "active",
	}

	if err := h.apiKeyRepo.Create(apiKey); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to save API key: %v", err))
		return
	}

	resp := toApiKeyResponse(apiKey)
	resp.FullKey = fullKey

	response.Success(c, resp)
}

// DeleteApiKey deletes an API key by ID (DELETE /api/v1/user/api-keys/:id).
func (h *UserHandler) DeleteApiKey(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid API key ID")
		return
	}

	// Verify the key belongs to the current user
	keys, err := h.apiKeyRepo.FindByUserID(userID)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to find API keys: %v", err))
		return
	}

	found := false
	for _, key := range keys {
		if key.ID == uint(id) {
			found = true
			break
		}
	}

	if !found {
		response.NotFound(c, "API key not found")
		return
	}

	if err := h.apiKeyRepo.Delete(uint(id)); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to delete API key: %v", err))
		return
	}

	response.SuccessWithMessage(c, "API key deleted successfully", nil)
}

// GetUsage returns usage summary for the current user (GET /api/v1/user/usage).
func (h *UserHandler) GetUsage(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	// Parse time range from query params, default to last 30 days
	now := time.Now()
	start := now.AddDate(0, 0, -30)
	end := now

	if startStr := c.Query("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = t
		}
	}
	if endStr := c.Query("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = t
		}
	}

	totalTokens, totalCost, err := h.billingService.GetUserUsage(userID, start, end)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get usage: %v", err))
		return
	}

	dailyItems, err := h.billingService.GetUserUsageDaily(userID, start, end)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get daily usage: %v", err))
		return
	}
	daily := make([]gin.H, len(dailyItems))
	for i, it := range dailyItems {
		daily[i] = gin.H{
			"date":   it.Date,
			"tokens": it.Tokens,
			"cost":   it.Cost.String(),
		}
	}

	response.Success(c, gin.H{
		"start":        start.Format(time.RFC3339),
		"end":          end.Format(time.RFC3339),
		"total_tokens": totalTokens,
		"total_cost":   totalCost.String(),
		"daily":        daily,
	})
}

// GetBillingRecords returns paginated billing records (GET /api/v1/user/billing-records).
func (h *UserHandler) GetBillingRecords(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	page, size := parsePagination(c)

	records, total, err := h.billingService.GetUserBillingRecords(userID, page, size)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get billing records: %v", err))
		return
	}

	items := make([]dto.BillingRecordResponse, 0, len(records))
	for i := range records {
		items = append(items, toBillingRecordResponse(&records[i]))
	}

	response.Page(c, items, total, page, size)
}

// GetBillingRecord returns a single billing record owned by the caller
// (GET /api/v1/user/billing-records/:id). Used by the usage-detail modal.
func (h *UserHandler) GetBillingRecord(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, "invalid billing record id")
		return
	}
	var rec model.BillingRecord
	if err := h.billingRepo.FindByIDAndUser(uint(id), userID, &rec); err != nil {
		response.NotFound(c, "账单记录不存在")
		return
	}
	response.Success(c, toBillingRecordResponse(&rec))
}

// GetTransactions returns paginated transactions (GET /api/v1/user/transactions).
func (h *UserHandler) GetTransactions(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	page, size := parsePagination(c)

	transactions, total, err := h.billingService.GetUserTransactions(userID, page, size)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get transactions: %v", err))
		return
	}

	items := make([]dto.TransactionResponse, 0, len(transactions))
	for i := range transactions {
		items = append(items, toTransactionResponse(&transactions[i]))
	}

	response.Page(c, items, total, page, size)
}

// GetPlans lists all active plans (GET /api/v1/plans).
func (h *UserHandler) GetPlans(c *gin.Context) {
	plans, err := h.planRepo.FindActive()
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get plans: %v", err))
		return
	}

	items := make([]dto.PlanResponse, 0, len(plans))
	for i := range plans {
		items = append(items, toPlanResponse(&plans[i]))
	}

	response.Success(c, items)
}

// Subscribe creates a new subscription for the current user (POST /api/v1/user/subscribe).
func (h *UserHandler) Subscribe(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}

	sub, err := h.billingService.Subscribe(userID, req.PlanID)
	if err != nil {
		switch {
		case errors.Is(err, billing.ErrCannotDowngrade):
			response.BadRequest(c, "无法降级：仅支持升级到 token 额度不低于当前套餐的套餐")
		case errors.Is(err, billing.ErrAlreadySubscribed):
			response.BadRequest(c, "你已订阅该套餐")
		case errors.Is(err, billing.ErrPurchaseLimitExceeded):
			response.BadRequest(c, "该套餐已达购买次数上限")
		case errors.Is(err, billing.ErrInsufficientBalance):
			// Report the exact shortage (need) so the frontend can offer an
			// immediate payment flow for the missing amount.
			var insuf *billing.InsufficientBalanceError
			need := "0"
			if errors.As(err, &insuf) && insuf.Need.GreaterThan(decimal.Zero) {
				need = insuf.Need.StringFixed(2)
			}
			response.ErrorWithData(c, http.StatusPaymentRequired, "insufficient balance", gin.H{"need": need})
		default:
			response.InternalError(c, fmt.Sprintf("failed to subscribe: %v", err))
		}
		return
	}

	// Reload with plan info for response
	sub, err = h.subRepo.FindByID(sub.ID)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to load subscription: %v", err))
		return
	}

	response.Success(c, toSubscriptionResponse(sub))
}

// GetSubscriptions lists active subscriptions for the current user (GET /api/v1/user/subscriptions).
func (h *UserHandler) GetSubscriptions(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	subs, err := h.subRepo.FindActiveByUserID(userID)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("failed to get subscriptions: %v", err))
		return
	}

	items := make([]dto.SubscriptionResponse, 0, len(subs))
	for i := range subs {
		items = append(items, toSubscriptionResponse(&subs[i]))
	}

	response.Success(c, items)
}

// CancelSubscription cancels a subscription by ID (POST /api/v1/user/subscriptions/:id/cancel).
func (h *UserHandler) CancelSubscription(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid subscription ID")
		return
	}

	// Verify the subscription belongs to the current user
	sub, err := h.subRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "subscription not found")
		return
	}

	if sub.UserID != userID {
		response.Forbidden(c, "subscription does not belong to the current user")
		return
	}

	if err := h.billingService.CancelSubscription(sub.ID); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to cancel subscription: %v", err))
		return
	}

	response.SuccessWithMessage(c, "subscription cancelled successfully", nil)
}

// SetAutoRenew toggles the auto-renew flag of a subscription without cancelling
// it (POST /api/v1/user/subscriptions/:id/auto-renew).
func (h *UserHandler) SetAutoRenew(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid subscription ID")
		return
	}

	var req struct {
		AutoRenew bool `json:"auto_renew"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	sub, err := h.subRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "subscription not found")
		return
	}

	if sub.UserID != userID {
		response.Forbidden(c, "subscription does not belong to the current user")
		return
	}

	if err := h.billingService.SetAutoRenew(sub.ID, req.AutoRenew); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to update auto-renew: %v", err))
		return
	}

	response.SuccessWithMessage(c, "auto-renew updated successfully", nil)
}

// GetModelCatalog returns the model catalog for the model marketplace.
// (GET /api/v1/models). It is public and does not require authentication.
//
// Only models with an enabled price entry in model_prices are exposed (a
// model without a configured price cannot be billed and stays hidden).
// Prices shown are the live admin-configured prices (CNY per 1M tokens);
// static built-in catalog prices are display metadata only.
func (h *UserHandler) GetModelCatalog(c *gin.Context) {
	response.Success(c, buildModelMarketplace(h.channelRepo, h.billingService))
}

// ListTokenPackages returns all active token packages (GET /api/v1/user/token-packages).
func (h *UserHandler) ListTokenPackages(c *gin.Context) {
	list, err := h.tokenPkgRepo.ListActive()
	if err != nil {
		response.InternalError(c, "failed to list token packages")
		return
	}
	response.Success(c, list)
}

// PurchaseTokenPackage buys a token package using the balance (POST /api/v1/user/token-packages/:id/purchase).
func (h *UserHandler) PurchaseTokenPackage(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	pkgID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid package id")
		return
	}

	tx, credits, err := h.billingService.PurchaseTokenPackage(userID, uint(pkgID))
	if err != nil {
		if errors.Is(err, billing.ErrInsufficientBalance) {
			response.Error(c, http.StatusPaymentRequired, "insufficient balance, please recharge first")
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "token package not found")
			return
		}
		response.InternalError(c, fmt.Sprintf("failed to purchase token package: %v", err))
		return
	}

	response.Success(c, gin.H{
		"transaction":   toTransactionResponse(tx),
		"token_credits": credits,
	})
}

// UploadFile uploads an image (e.g. ID card photo) and returns its URL
// (POST /api/v1/user/upload).
func (h *UserHandler) UploadFile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file field is required")
		return
	}

	maxBytes := h.uploadMaxSizeMB * 1024 * 1024
	if file.Size > maxBytes {
		response.BadRequest(c, fmt.Sprintf("file too large, max size is %dMB", h.uploadMaxSizeMB))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		response.BadRequest(c, "unsupported image format, only JPG/PNG/WebP allowed")
		return
	}

	// Verify the file content matches the claimed image format (magic bytes).
	src, err := file.Open()
	if err != nil {
		response.InternalError(c, "failed to read uploaded file")
		return
	}
	head := make([]byte, 512)
	n, _ := src.Read(head)
	src.Close()
	detected := http.DetectContentType(head[:n])
	allowedMime := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	if !allowedMime[detected] {
		response.BadRequest(c, "file content does not match an allowed image format")
		return
	}

	sub := path.Join("identity", strconv.FormatUint(uint64(userID), 10))
	dir := filepath.Join(h.uploadDir, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		response.InternalError(c, "failed to create upload directory")
		return
	}

	name := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), randomHex(8), ext)
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		response.InternalError(c, "failed to save uploaded file")
		return
	}

	url := path.Join("/uploads", sub, name)
	response.Success(c, gin.H{
		"url":  url,
		"size": file.Size,
	})
}

// SubmitIdentityVerification submits real-name verification (POST /api/v1/user/identity-verification).
func (h *UserHandler) SubmitIdentityVerification(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req dto.IdentityVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Check if already submitted
	existing, err := h.identityRepo.FindByUserID(userID)
	if err == nil && existing != nil {
		response.Error(c, http.StatusConflict, "identity verification already submitted")
		return
	}

	verification := &model.IdentityVerification{
		UserID:      userID,
		RealName:    req.RealName,
		IDNumber:    req.IDNumber,
		IDCardFront: req.IDCardFront,
		IDCardBack:  req.IDCardBack,
		Status:      "pending",
	}

	if err := h.identityRepo.Create(verification); err != nil {
		response.InternalError(c, fmt.Sprintf("failed to submit verification: %v", err))
		return
	}

	// Update user's real name status
	user, err := h.userRepo.FindByID(userID)
	if err == nil {
		user.RealNameStatus = "pending"
		_ = h.userRepo.Update(user)
	}

	response.Success(c, gin.H{
		"id":     verification.ID,
		"status": verification.Status,
	})
}

// GetIdentityVerificationStatus returns the current identity verification status (GET /api/v1/user/identity-verification).
func (h *UserHandler) GetIdentityVerificationStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	verification, err := h.identityRepo.FindByUserID(userID)
	if err != nil {
		response.Success(c, gin.H{
			"status": "unverified",
		})
		return
	}

	response.Success(c, gin.H{
		"id":            verification.ID,
		"real_name":     verification.RealName,
		"status":        verification.Status,
		"reject_reason": verification.RejectReason,
		"created_at":    verification.CreatedAt.Format(time.RFC3339),
		"updated_at":    verification.UpdatedAt.Format(time.RFC3339),
	})
}

// ListMyResetCoupons returns the reset coupons bound to the current user
// (GET /api/v1/user/reset-coupons).
func (h *UserHandler) ListMyResetCoupons(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	coupons, err := h.resetCouponRepo.ListByUserID(userID)
	if err != nil {
		response.InternalError(c, "failed to load reset coupons")
		return
	}
	items := make([]gin.H, len(coupons))
	for i, cp := range coupons {
		items[i] = gin.H{
			"id":         cp.ID,
			"code":       cp.Code,
			"status":     cp.Status,
			"note":       cp.Note,
			"used_at":    cp.UsedAt,
			"created_at": cp.CreatedAt,
		}
	}
	response.Success(c, items)
}

// RedeemResetCoupon redeems a reset coupon, resetting the used token quota of
// all of the user's active subscriptions to zero (POST /api/v1/user/reset-coupons/:id/redeem).
func (h *UserHandler) RedeemResetCoupon(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid coupon id")
		return
	}

	coupon, err := h.resetCouponRepo.FindByID(uint(id))
	if err != nil {
		response.BadRequest(c, "coupon not found")
		return
	}
	if coupon.UserID != userID {
		response.Forbidden(c, "coupon does not belong to this user")
		return
	}
	if coupon.Status != "unused" {
		response.BadRequest(c, "coupon already used")
		return
	}

	reset, err := h.resetCouponRepo.RedeemWithReset(coupon.ID, userID, time.Now())
	if err != nil {
		if errors.Is(err, repository.ErrCouponAlreadyUsed) {
			response.BadRequest(c, "coupon already used")
			return
		}
		response.InternalError(c, "redeem failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"coupon_id":   coupon.ID,
		"reset_count": reset,
	})
}

// ListNotifications returns the current user's notifications
// (GET /api/v1/user/notifications).
func (h *UserHandler) ListNotifications(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	items, total, err := h.notifRepo.ListByUserID(userID, page, size)
	if err != nil {
		response.InternalError(c, "failed to load notifications")
		return
	}
	out := make([]gin.H, len(items))
	for i, n := range items {
		out[i] = gin.H{
			"id":         n.ID,
			"title":      n.Title,
			"content":    n.Content,
			"type":       n.Type,
			"is_read":    n.IsRead,
			"read_at":    n.ReadAt,
			"created_at": n.CreatedAt,
		}
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": page, "size": size})
}

// GetNotificationUnread returns the unread notification count
// (GET /api/v1/user/notifications/unread-count).
func (h *UserHandler) GetNotificationUnread(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	count, err := h.notifRepo.UnreadCount(userID)
	if err != nil {
		response.InternalError(c, "failed to load unread count")
		return
	}
	response.Success(c, gin.H{"unread": count})
}

// MarkNotificationRead marks a single notification as read
// (PUT /api/v1/user/notifications/:id/read).
func (h *UserHandler) MarkNotificationRead(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid notification id")
		return
	}
	rows, err := h.notifRepo.MarkRead(uint(id), userID, time.Now())
	if err != nil {
		response.InternalError(c, "failed to mark read")
		return
	}
	response.Success(c, gin.H{"affected": rows})
}

// MarkAllNotificationsRead marks all notifications of the user as read
// (PUT /api/v1/user/notifications/read-all).
func (h *UserHandler) MarkAllNotificationsRead(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	rows, err := h.notifRepo.MarkAllRead(userID, time.Now())
	if err != nil {
		response.InternalError(c, "failed to mark all read")
		return
	}
	response.Success(c, gin.H{"affected": rows})
}

// ---------------------------------------------------------------------------
// 易支付（epay）对接
// ---------------------------------------------------------------------------

// epayConfigFromDB loads the epay gateway configuration from system_configs.
// It returns ok=false when epay is not configured or disabled.
func (h *UserHandler) epayConfigFromDB() (payment.EpayConfig, bool) {
	cfg := payment.EpayConfig{}
	if h.configRepo == nil {
		return cfg, false
	}
	all, err := h.configRepo.GetAll()
	if err != nil {
		return cfg, false
	}
	values := map[string]string{}
	for _, c := range all {
		values[c.Key] = c.Value
	}
	if values["pay_epay_enabled"] != "true" && values["pay_epay_enabled"] != "1" {
		return cfg, false
	}
	cfg.Gateway = strings.TrimSpace(values["pay_epay_gateway"])
	cfg.PID = strings.TrimSpace(values["pay_epay_pid"])
	cfg.Key = strings.TrimSpace(values["pay_epay_key"])
	cfg.SignUpper = values["pay_epay_sign_upper"] == "true" || values["pay_epay_sign_upper"] == "1"
	cfg.Enabled = true
	if cfg.Gateway == "" || cfg.PID == "" || cfg.Key == "" {
		return cfg, false
	}
	return cfg, true
}

// wechatConfigFromDB loads the native WeChat Pay configuration from
// system_configs. It returns ok=false when not configured or disabled.
func (h *UserHandler) wechatConfigFromDB() (payment.WechatPayConfig, bool) {
	cfg := payment.WechatPayConfig{}
	if h.configRepo == nil {
		return cfg, false
	}
	all, err := h.configRepo.GetAll()
	if err != nil {
		return cfg, false
	}
	values := map[string]string{}
	for _, c := range all {
		values[c.Key] = c.Value
	}
	if values["pay_wechat_enabled"] != "true" && values["pay_wechat_enabled"] != "1" {
		return cfg, false
	}
	cfg.Enabled = true
	cfg.AppID = strings.TrimSpace(values["pay_wechat_appid"])
	cfg.MchID = strings.TrimSpace(values["pay_wechat_mchid"])
	cfg.APIKey = strings.TrimSpace(values["pay_wechat_api_key"])
	cfg.SerialNo = strings.TrimSpace(values["pay_wechat_serial"])
	cfg.PrivateKey = strings.TrimSpace(values["pay_wechat_private_key"])
	cfg.NotifyURL = strings.TrimSpace(values["pay_wechat_notify_url"])
	if cfg.AppID == "" || cfg.MchID == "" || cfg.APIKey == "" || cfg.PrivateKey == "" {
		return cfg, false
	}
	return cfg, true
}

// alipayConfigFromDB loads the native Alipay configuration from system_configs.
func (h *UserHandler) alipayConfigFromDB() (payment.AlipayConfig, bool) {
	cfg := payment.AlipayConfig{}
	if h.configRepo == nil {
		return cfg, false
	}
	all, err := h.configRepo.GetAll()
	if err != nil {
		return cfg, false
	}
	values := map[string]string{}
	for _, c := range all {
		values[c.Key] = c.Value
	}
	if values["pay_alipay_enabled"] != "true" && values["pay_alipay_enabled"] != "1" {
		return cfg, false
	}
	cfg.Enabled = true
	cfg.AppID = strings.TrimSpace(values["pay_alipay_appid"])
	cfg.PrivateKey = strings.TrimSpace(values["pay_alipay_private_key"])
	cfg.PublicKey = strings.TrimSpace(values["pay_alipay_public_key"])
	cfg.NotifyURL = strings.TrimSpace(values["pay_alipay_notify_url"])
	cfg.ReturnURL = strings.TrimSpace(values["pay_alipay_return_url"])
	cfg.Gateway = strings.TrimSpace(values["pay_alipay_gateway"])
	if cfg.AppID == "" || cfg.PrivateKey == "" || cfg.PublicKey == "" {
		return cfg, false
	}
	return cfg, true
}

// siteURL resolves the public site base URL (used for notify/return URLs).
func (h *UserHandler) siteURL() string {
	siteURL := "https://mass.yiziyun.com"
	if h.configRepo != nil {
		if all, err := h.configRepo.GetAll(); err == nil {
			for _, c := range all {
				if c.Key == "site_url" && strings.TrimSpace(c.Value) != "" {
					siteURL = strings.TrimRight(strings.TrimSpace(c.Value), "/")
					break
				}
			}
		}
	}
	return siteURL
}

// GetPaymentConfig returns the available payment methods for the current user
// (GET /api/v1/user/payment-config).
func (h *UserHandler) GetPaymentConfig(c *gin.Context) {
	methods := []string{"balance"}
	if _, ok := h.epayConfigFromDB(); ok {
		methods = append(methods, "epay")
	}
	if _, ok := h.wechatConfigFromDB(); ok {
		methods = append(methods, "wechat")
	}
	if _, ok := h.alipayConfigFromDB(); ok {
		methods = append(methods, "alipay")
	}
	response.Success(c, gin.H{
		"methods": methods,
	})
}

// EpayNotify handles the epay async payment notification
// (POST /api/v1/pay/epay/notify, public, no auth).
func (h *UserHandler) EpayNotify(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		response.BadRequest(c, "bad request")
		return
	}
	params := map[string]string{}
	for k, v := range c.Request.Form {
		params[k] = v[0]
	}

	cfg, ok := h.epayConfigFromDB()
	if !ok {
		c.String(http.StatusOK, "fail")
		return
	}
	client := payment.NewEpayClient(cfg)
	if !client.VerifyNotify(params) {
		logging.Warn("pay", "epay_notify", "signature verification failed", map[string]interface{}{"params": params})
		c.String(http.StatusOK, "fail")
		return
	}
	outTradeNo := params["out_trade_no"]
	tradeStatus := params["trade_status"]
	if tradeStatus != "TRADE_SUCCESS" {
		// 未支付完成的回调：直接应答 success 避免重复推送
		c.String(http.StatusOK, "success")
		return
	}
	// Reconcile: the gateway-reported amount must match the local order.
	tx, err := h.billingService.FindTransactionByNo(outTradeNo)
	if err != nil || tx == nil {
		logging.Warn("pay", "epay_notify", "transaction not found for callback", map[string]interface{}{"out_trade_no": outTradeNo})
		c.String(http.StatusOK, "fail")
		return
	}
	if params["money"] != tx.Amount.StringFixed(2) {
		logging.Warn("pay", "epay_notify", "amount mismatch, refusing to settle",
			map[string]interface{}{"out_trade_no": outTradeNo, "gateway_money": params["money"], "local_amount": tx.Amount.String()})
		c.String(http.StatusOK, "fail")
		return
	}
	if _, err := h.billingService.CompleteEpayPayment(outTradeNo); err != nil {
		logging.Error("pay", "epay_notify", "failed to settle payment", err,
			map[string]interface{}{"out_trade_no": outTradeNo})
		c.String(http.StatusOK, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}

// CreateEpayOrder creates a recharge order and returns the epay payment URL
// (POST /api/v1/user/recharge/epay).
func (h *UserHandler) CreateEpayOrder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Amount  string `json:"amount" binding:"required"`
		PayType string `json:"pay_type"` // alipay | wxpay | qqpay
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.NewFromInt(1)) || amount.GreaterThan(decimal.NewFromInt(50000)) {
		response.BadRequest(c, "金额需在 ¥1.00 - ¥50000.00 之间")
		return
	}
	cfg, ok := h.epayConfigFromDB()
	if !ok {
		response.BadRequest(c, "易支付未启用，请联系管理员")
		return
	}
	payType := req.PayType
	if payType == "" {
		payType = "alipay"
	}
	if payType != "alipay" && payType != "wxpay" && payType != "qqpay" {
		response.BadRequest(c, "不支持的支付方式")
		return
	}

	txNo := h.billingService.GenerateTransactionNo()
	tx := &model.Transaction{
		UserID:        userID,
		TransactionNo: txNo,
		Type:          model.TransactionRecharge,
		Amount:        amount,
		PaymentMethod: "epay",
		Status:        model.TransactionPending,
		Description:   fmt.Sprintf("epay %s order", payType),
	}
	if err := h.txRepo.Create(tx); err != nil {
		response.InternalError(c, "failed to create order")
		return
	}

	siteURL := "https://mass.piteyun.com"
	if h.configRepo != nil {
		if all, err := h.configRepo.GetAll(); err == nil {
			for _, c := range all {
				if c.Key == "site_url" && strings.TrimSpace(c.Value) != "" {
					siteURL = strings.TrimRight(strings.TrimSpace(c.Value), "/")
					break
				}
			}
		}
	}
	notifyURL := siteURL + "/api/v1/pay/epay/notify"
	returnURL := siteURL + "/user/recharge"

	client := payment.NewEpayClient(cfg)
	payURL, err := client.BuildOrder(amount.StringFixed(2), txNo,
		"平台余额充值", notifyURL, returnURL, payType, c.ClientIP())
	if err != nil {
		response.InternalError(c, "failed to build payment url")
		return
	}

	response.Success(c, gin.H{
		"out_trade_no": txNo,
		"pay_url":      payURL,
		"amount":       amount.StringFixed(2),
	})
}

// GetEpayOrderStatus returns the local status of an epay order
// (GET /api/v1/user/recharge/epay/status?out_trade_no=xxx).
func (h *UserHandler) GetEpayOrderStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	txNo := c.Query("out_trade_no")
	if txNo == "" {
		response.BadRequest(c, "out_trade_no is required")
		return
	}
	tx, err := h.txRepo.FindByTransactionNo(txNo)
	if err != nil || tx.UserID != userID {
		response.BadRequest(c, "订单不存在")
		return
	}
	response.Success(c, gin.H{
		"out_trade_no": tx.TransactionNo,
		"status":       string(tx.Status),
		"amount":       tx.Amount.StringFixed(2),
	})
}

// QueryEpayOrder actively queries the epay gateway for the order status and
// settles it locally if the gateway reports the order as paid
// (POST /api/v1/user/recharge/epay/query).
func (h *UserHandler) QueryEpayOrder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		OutTradeNo string `json:"out_trade_no"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.OutTradeNo == "" {
		response.BadRequest(c, "out_trade_no is required")
		return
	}
	tx, err := h.txRepo.FindByTransactionNo(req.OutTradeNo)
	if err != nil || tx.UserID != userID {
		response.BadRequest(c, "订单不存在")
		return
	}
	if tx.Status == model.TransactionSuccess {
		response.Success(c, gin.H{"status": string(tx.Status)})
		return
	}
	cfg, ok := h.epayConfigFromDB()
	if !ok {
		response.BadRequest(c, "易支付未启用，请联系管理员")
		return
	}
	paid, _, err := payment.NewEpayClient(cfg).QueryOrder(req.OutTradeNo)
	if err != nil {
		response.InternalError(c, "查询网关失败: "+err.Error())
		return
	}
	if paid {
		if _, err := h.billingService.CompleteEpayPayment(req.OutTradeNo); err != nil {
			response.InternalError(c, "入账失败: "+err.Error())
			return
		}
		response.Success(c, gin.H{"status": string(model.TransactionSuccess)})
		return
	}
	response.Success(c, gin.H{"status": string(model.TransactionPending)})
}

// ---------------------------------------------------------------------------
// 原生微信支付（WeChat Pay v3 Native）
// ---------------------------------------------------------------------------

// CreateWechatOrder creates a WeChat Pay native order and returns the code_url
// the user scans with WeChat (POST /api/v1/user/recharge/wechat).
func (h *UserHandler) CreateWechatOrder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Amount string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.NewFromInt(1)) || amount.GreaterThan(decimal.NewFromInt(50000)) {
		response.BadRequest(c, "金额需在 ¥1.00 - ¥50000.00 之间")
		return
	}
	cfg, ok := h.wechatConfigFromDB()
	if !ok {
		response.BadRequest(c, "微信支付未启用，请联系管理员")
		return
	}
	txNo := h.billingService.GenerateTransactionNo()
	tx := &model.Transaction{
		UserID:        userID,
		TransactionNo: txNo,
		Type:          model.TransactionRecharge,
		Amount:        amount,
		PaymentMethod: "wechat",
		Status:        model.TransactionPending,
		Description:   "微信支付充值",
	}
	if err := h.txRepo.Create(tx); err != nil {
		response.InternalError(c, "failed to create order")
		return
	}
	notifyURL := h.siteURL() + "/api/v1/pay/wechat/notify"
	amountCents := amount.Mul(decimal.NewFromInt(100)).IntPart()
	codeURL, err := payment.NewWechatPayClient(cfg).CreateNativeOrder(txNo, "平台余额充值", notifyURL, amountCents)
	if err != nil {
		response.InternalError(c, "微信下单失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"out_trade_no": txNo,
		"code_url":     codeURL,
		"amount":       amount.StringFixed(2),
	})
}

// WechatNotify handles the WeChat Pay async callback (public, no auth).
// Authenticity is established by AES-GCM-decrypting the resource with the
// APIv3 key; only WeChat knows that key, so a successful decryption probing
// our own out_trade_no is sufficient.
func (h *UserHandler) WechatNotify(c *gin.Context) {
	cfg, ok := h.wechatConfigFromDB()
	if !ok {
		c.String(http.StatusOK, "fail")
		return
	}
	var payload struct {
		Resource struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.String(http.StatusOK, "fail")
		return
	}
  plain, err := payment.NewWechatPayClient(cfg).DecryptResource(payload.Resource)
  if err != nil {
    c.String(http.StatusOK, "fail")
    return
  }
  outTradeNo, _ := plain["out_trade_no"].(string)
  tradeState, _ := plain["trade_state"].(string)
  if tradeState != "SUCCESS" {
    c.String(http.StatusOK, "success")
    return
  }
  tx, err := h.txRepo.FindByTransactionNo(outTradeNo)
  if err != nil || tx == nil {
    c.String(http.StatusOK, "fail")
    return
  }
  if !wechatAmountMatches(plain, tx.Amount) {
    logging.Warn("pay", "wechat_notify", "amount mismatch, refusing to settle",
      map[string]interface{}{"out_trade_no": outTradeNo})
    c.String(http.StatusOK, "fail")
    return
  }
  if _, err := h.billingService.CompleteEpayPayment(outTradeNo); err != nil {
    c.String(http.StatusOK, "fail")
    return
  }
  c.String(http.StatusOK, "success")
}

// wechatAmountMatches verifies the amount reported by WeChat (cents, under
// amount.total) equals the local order amount. When the amount field is absent
// it returns true as a defensive default — callback authenticity is already
// established by AES-GCM decryption with the APIv3 key.
func wechatAmountMatches(plain map[string]interface{}, local decimal.Decimal) bool {
  amt, ok := plain["amount"].(map[string]interface{})
  if !ok {
    return true
  }
  total, ok := amt["total"].(float64)
  if !ok {
    return true
  }
  return int64(total) == local.Mul(decimal.NewFromInt(100)).IntPart()
}

// ---------------------------------------------------------------------------
// 原生支付宝（Alipay precreate / QR）
// ---------------------------------------------------------------------------

// CreateAlipayOrder creates an Alipay precreate order and returns the qr_code
// the user scans with Alipay (POST /api/v1/user/recharge/alipay).
func (h *UserHandler) CreateAlipayOrder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Amount string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.NewFromInt(1)) || amount.GreaterThan(decimal.NewFromInt(50000)) {
		response.BadRequest(c, "金额需在 ¥1.00 - ¥50000.00 之间")
		return
	}
	cfg, ok := h.alipayConfigFromDB()
	if !ok {
		response.BadRequest(c, "支付宝未启用，请联系管理员")
		return
	}
	txNo := h.billingService.GenerateTransactionNo()
	tx := &model.Transaction{
		UserID:        userID,
		TransactionNo: txNo,
		Type:          model.TransactionRecharge,
		Amount:        amount,
		PaymentMethod: "alipay",
		Status:        model.TransactionPending,
		Description:   "支付宝充值",
	}
	if err := h.txRepo.Create(tx); err != nil {
		response.InternalError(c, "failed to create order")
		return
	}
	qrCode, err := payment.NewAlipayClient(cfg).CreatePrecreateOrder(txNo, "平台余额充值", amount.StringFixed(2))
	if err != nil {
		response.InternalError(c, "支付宝下单失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"out_trade_no": txNo,
		"qr_code":      qrCode,
		"amount":       amount.StringFixed(2),
	})
}

// AlipayNotify handles the Alipay async callback (public, no auth).
func (h *UserHandler) AlipayNotify(c *gin.Context) {
	cfg, ok := h.alipayConfigFromDB()
	if !ok {
		c.String(http.StatusOK, "failure")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, "failure")
		return
	}
	params := map[string]string{}
	for k, v := range c.Request.PostForm {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			if _, exists := params[k]; !exists {
				params[k] = v[0]
			}
		}
	}
  outTradeNo, tradeStatus, totalAmount, valid := payment.NewAlipayClient(cfg).VerifyNotify(params)
  if !valid {
    c.String(http.StatusOK, "failure")
    return
  }
  if tradeStatus != "TRADE_SUCCESS" {
    c.String(http.StatusOK, "success")
    return
  }
  tx, err := h.txRepo.FindByTransactionNo(outTradeNo)
  if err != nil || tx == nil {
    c.String(http.StatusOK, "failure")
    return
  }
  if totalAmount != "" && totalAmount != tx.Amount.StringFixed(2) {
    logging.Warn("pay", "alipay_notify", "amount mismatch, refusing to settle",
      map[string]interface{}{"out_trade_no": outTradeNo, "gateway": totalAmount, "local": tx.Amount.StringFixed(2)})
    c.String(http.StatusOK, "failure")
    return
  }
  if _, err := h.billingService.CompleteEpayPayment(outTradeNo); err != nil {
    c.String(http.StatusOK, "failure")
    return
  }
  c.String(http.StatusOK, "success")
}

// GetOrderStatus returns the local status of any recharge order by trade no
// (GET /api/v1/user/recharge/status?out_trade_no=xxx). Used to poll native
// and epay orders after redirecting to the payment app.
func (h *UserHandler) GetOrderStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	txNo := c.Query("out_trade_no")
	if txNo == "" {
		response.BadRequest(c, "out_trade_no is required")
		return
	}
	tx, err := h.txRepo.FindByTransactionNo(txNo)
	if err != nil || tx.UserID != userID {
		response.BadRequest(c, "订单不存在")
		return
	}
	response.Success(c, gin.H{"status": string(tx.Status)})
}

// ---------------------------------------------------------------------------
// 发票（Invoice）
// ---------------------------------------------------------------------------

// GetInvoiceQuota returns the user's invoice quota: total successfully
// recharged amount minus the amount occupied by pending/issued invoices
// (GET /api/v1/user/invoice-quota).
func (h *UserHandler) GetInvoiceQuota(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	recharged, err := h.invoiceRepo.RechargeTotal(userID)
	if err != nil {
		response.InternalError(c, "failed to compute quota")
		return
	}
	occupied, err := h.invoiceRepo.IssuedAmount(userID)
	if err != nil {
		response.InternalError(c, "failed to compute quota")
		return
	}
	quota := recharged.Sub(occupied)
	if quota.IsNegative() {
		quota = decimal.Zero
	}
	response.Success(c, gin.H{
		"recharged": recharged.StringFixed(2),
		"occupied":  occupied.StringFixed(2),
		"quota":     quota.StringFixed(2),
	})
}

// CreateInvoice submits an invoice application
// (POST /api/v1/user/invoices).
func (h *UserHandler) CreateInvoice(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Amount      string `json:"amount" binding:"required"`
		TitleType   string `json:"title_type"`   // company | personal
		InvoiceType string `json:"invoice_type"` // normal | vat
		Title       string `json:"title" binding:"required"`
		TaxNo       string `json:"tax_no"`
		BankName    string `json:"bank_name"`
		BankAccount string `json:"bank_account"`
		Address     string `json:"address"`
		Phone       string `json:"phone"`
		Email       string `json:"email"`
		Remark      string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		response.BadRequest(c, "开票金额必须大于 0")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		response.BadRequest(c, "发票抬头不能为空")
		return
	}
	titleType := req.TitleType
	if titleType == "" {
		titleType = "company"
	}
	if titleType != "company" && titleType != "personal" {
		response.BadRequest(c, "抬头类型不合法")
		return
	}
	invoiceType := req.InvoiceType
	if invoiceType == "" {
		invoiceType = "normal"
	}
	if invoiceType != "normal" && invoiceType != "vat" {
		response.BadRequest(c, "发票类型不合法")
		return
	}
	if titleType == "company" && strings.TrimSpace(req.TaxNo) == "" {
		response.BadRequest(c, "企业抬头必须填写税号")
		return
	}
	if invoiceType == "vat" {
		if strings.TrimSpace(req.BankName) == "" || strings.TrimSpace(req.BankAccount) == "" {
			response.BadRequest(c, "增值税专用发票必须填写开户行与银行账号")
			return
		}
	}
	email := strings.TrimSpace(req.Email)
	if email != "" && !strings.Contains(email, "@") {
		response.BadRequest(c, "发票接收邮箱格式不正确")
		return
	}

	// 额度校验：已充值 - 已占用
	recharged, err := h.invoiceRepo.RechargeTotal(userID)
	if err != nil {
		response.InternalError(c, "failed to compute quota")
		return
	}
	occupied, err := h.invoiceRepo.IssuedAmount(userID)
	if err != nil {
		response.InternalError(c, "failed to compute quota")
		return
	}
	quota := recharged.Sub(occupied)
	if quota.IsNegative() {
		quota = decimal.Zero
	}
	if amount.GreaterThan(quota) {
		response.BadRequest(c, "开票金额超出可开票额度（可开 ¥"+quota.StringFixed(2)+"）")
		return
	}

	inv := &model.Invoice{
		UserID:      userID,
		Amount:      amount,
		TitleType:   titleType,
		InvoiceType: invoiceType,
		Title:       title,
		TaxNo:       strings.TrimSpace(req.TaxNo),
		BankName:    strings.TrimSpace(req.BankName),
		BankAccount: strings.TrimSpace(req.BankAccount),
		Address:     strings.TrimSpace(req.Address),
		Phone:       strings.TrimSpace(req.Phone),
		Email:       email,
		Remark:      strings.TrimSpace(req.Remark),
		Status:      "pending",
	}
	if err := h.invoiceRepo.Create(inv); err != nil {
		response.InternalError(c, "failed to create invoice application")
		return
	}
	response.Success(c, toInvoiceResponse(inv))
}

// ListMyInvoices returns the current user's invoices
// (GET /api/v1/user/invoices).
func (h *UserHandler) ListMyInvoices(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	items, total, err := h.invoiceRepo.ListByUserID(userID, page, size)
	if err != nil {
		response.InternalError(c, "failed to load invoices")
		return
	}
	out := make([]gin.H, len(items))
	for i := range items {
		out[i] = toInvoiceResponse(&items[i])
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": page, "size": size})
}

func toInvoiceResponse(inv *model.Invoice) gin.H {
	return gin.H{
		"id":            inv.ID,
		"amount":        inv.Amount.StringFixed(2),
		"title_type":    inv.TitleType,
		"invoice_type":  inv.InvoiceType,
		"title":         inv.Title,
		"tax_no":        inv.TaxNo,
		"bank_name":     inv.BankName,
		"bank_account":  inv.BankAccount,
		"address":       inv.Address,
		"phone":         inv.Phone,
		"email":         inv.Email,
		"status":        inv.Status,
		"invoice_no":    inv.InvoiceNo,
		"reject_reason": inv.RejectReason,
		"remark":        inv.Remark,
		"issued_at":     inv.IssuedAt,
		"created_at":    inv.CreatedAt,
	}
}

// ---------------------------------------------------------------------------
// Token 授信（Credit）
// ---------------------------------------------------------------------------

// GetCreditStatus returns the user's cumulative consumption, the apply
// threshold, whether they may apply, and their latest application
// (GET /api/v1/user/credit/status).
func (h *UserHandler) GetCreditStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	consumed, err := h.creditRepo.ConsumedTotal(userID)
	if err != nil {
		response.InternalError(c, "failed to load credit status")
		return
	}
	hasActive, err := h.creditRepo.HasActive(userID)
	if err != nil {
		response.InternalError(c, "failed to load credit status")
		return
	}
	latest, err := h.creditRepo.Latest(userID)
	if err != nil {
		response.InternalError(c, "failed to load credit status")
		return
	}
	limit, used, err := h.creditRepo.CreditState(userID)
	if err != nil {
		response.InternalError(c, "failed to load credit status")
		return
	}
	canApply := consumed.GreaterThanOrEqual(decimal.NewFromInt(model.CreditThreshold)) && !hasActive
	var app gin.H
	if latest != nil {
		app = toCreditApplicationResponse(latest)
	}
	available := limit - used
	if available < 0 {
		available = 0
	}
	response.Success(c, gin.H{
		"consumed_total":   consumed.StringFixed(2),
		"threshold":        model.CreditThreshold,
		"can_apply":        canApply,
		"application":      app,
		"credit_limit":     limit,
		"credit_used":      used,
		"credit_available": available,
	})
}

// ApplyCredit submits a token credit application
// (POST /api/v1/user/credit/apply).
func (h *UserHandler) ApplyCredit(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	consumed, err := h.creditRepo.ConsumedTotal(userID)
	if err != nil {
		response.InternalError(c, "failed to load credit status")
		return
	}
	if consumed.LessThan(decimal.NewFromInt(model.CreditThreshold)) {
		response.BadRequest(c, fmt.Sprintf("累计消费满 ¥%d 才可申请授信", model.CreditThreshold))
		return
	}
	hasActive, err := h.creditRepo.HasActive(userID)
	if err != nil {
		response.InternalError(c, "failed to load credit status")
		return
	}
	if hasActive {
		response.BadRequest(c, "已存在待审核或已生效的授信申请，无需重复提交")
		return
	}
	app := &model.CreditApplication{
		UserID:        userID,
		Status:        string(model.CreditPending),
		ConsumedTotal: consumed,
	}
	if err := h.creditRepo.Create(app); err != nil {
		response.InternalError(c, "failed to submit credit application")
		return
	}
	response.Success(c, toCreditApplicationResponse(app))
}

func toCreditApplicationResponse(app *model.CreditApplication) gin.H {
	return gin.H{
		"id":             app.ID,
		"status":         app.Status,
		"granted_tokens": app.GrantedTokens,
		"reject_reason":  app.RejectReason,
		"consumed_total": app.ConsumedTotal.StringFixed(2),
		"created_at":     app.CreatedAt,
		"reviewed_at":    app.ReviewedAt,
	}
}

// RepayCredit repays outstanding credit (待还额度) using the user's purchased
// token credits (POST /api/v1/user/credit/repay).
func (h *UserHandler) RepayCredit(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		Tokens int64 `json:"tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if req.Tokens <= 0 {
		response.BadRequest(c, "还款数量必须大于 0")
		return
	}
	if err := h.creditRepo.RepayCredit(userID, req.Tokens); err != nil {
		if errors.Is(err, repository.ErrCreditRepayInsufficient) {
			response.BadRequest(c, "Token 额度不足或待还额度不足，请先购买加油包")
			return
		}
		response.InternalError(c, "failed to repay credit")
		return
	}
	limit, used, _ := h.creditRepo.CreditState(userID)
	response.Success(c, gin.H{
		"credit_used":      used,
		"credit_available": limit - used,
		"token_credits":    h.getTokenCredits(userID),
	})
}

func (h *UserHandler) getTokenCredits(userID uint) int64 {
	u, err := h.userRepo.FindByID(userID)
	if err != nil {
		return 0
	}
	return u.TokenCredits
}

// ---------------------------------------------------------------------------
// 亦 OpenID 快捷登录 / 账号绑定
// ---------------------------------------------------------------------------

// openIDRandomState generates a cryptographically random OAuth state token.
func openIDRandomState() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("st%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// OpenIDConfig returns whether 亦 OpenID quick login is enabled, so the
// login page can show/hide the shortcut button accordingly.
// GET /api/v1/auth/openid/config
func (h *UserHandler) OpenIDConfig(c *gin.Context) {
	if h.openidSvc == nil {
		response.Success(c, gin.H{"enabled": false})
		return
	}
	cfg, err := h.openidSvc.LoadConfig(c.Request.Context())
	if err != nil {
		response.Success(c, gin.H{"enabled": false})
		return
	}
	response.Success(c, gin.H{
		"enabled":      cfg.Enabled && cfg.ClientID != "",
		"server":       cfg.Server,
		"redirect_uri": cfg.RedirectURI,
	})
}

// OpenIDAuthorize redirects the browser to the 亦 OpenID authorization page
// (intent=login for sign-in, intent=bind for binding to the current account).
// GET /api/v1/auth/openid/authorize?intent=login
func (h *UserHandler) OpenIDAuthorize(c *gin.Context) {
	intent := c.Query("intent")
	if intent == "" {
		intent = "login"
	}
	if intent != "login" && intent != "bind" {
		response.BadRequest(c, "invalid intent")
		return
	}
	if h.openidSvc == nil {
		response.InternalError(c, "openid service not configured")
		return
	}
	cfg, err := h.openidSvc.LoadConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to load oauth config")
		return
	}
	if !cfg.Enabled || cfg.ClientID == "" {
		response.Error(c, http.StatusBadRequest, "亦 OpenID 登录未启用，请联系管理员")
		return
	}

	var userID uint
	if intent == "bind" {
		id, ok := getUserID(c)
		if !ok || id == 0 {
			response.Unauthorized(c, "请先登录后再绑定")
			return
		}
		userID = id
	}

	state := openIDRandomState()
	if err := h.openidSvc.StoreState(c.Request.Context(), state, intent, userID); err != nil {
		response.InternalError(c, "failed to store oauth state")
		return
	}
	authURL, err := h.openidSvc.AuthorizeURL(cfg, state)
	if err != nil {
		response.InternalError(c, "failed to build authorize url")
		return
	}
	c.Redirect(http.StatusFound, authURL)
}

// OpenIDCallback handles the OAuth redirect back from 亦 OpenID. For
// intent=login it signs the user in (auto-creating an account on first
// login); for intent=bind it attaches the identity to the bound user.
// GET /api/v1/auth/openid/callback?code=..&state=..
func (h *UserHandler) OpenIDCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		response.BadRequest(c, "missing code or state")
		return
	}
	if h.openidSvc == nil {
		response.InternalError(c, "openid service not configured")
		return
	}
	ctx := c.Request.Context()

	intent, bindUserID, ok := h.openidSvc.TakeState(ctx, state)
	if !ok {
		response.BadRequest(c, "state 无效或已过期，请重新发起登录")
		return
	}

	cfg, err := h.openidSvc.LoadConfig(ctx)
	if err != nil || !cfg.Enabled || cfg.ClientID == "" {
		response.InternalError(c, "openid oauth not configured")
		return
	}

	token, err := h.openidSvc.ExchangeCode(ctx, cfg, code)
	if err != nil {
		logging.Error("user_handler", "openid_callback", "exchange code failed", err, nil)
		response.BadRequest(c, "亦 OpenID 授权失败，请重试")
		return
	}
	ou, err := h.openidSvc.GetUserInfo(ctx, cfg, token)
	if err != nil {
		logging.Error("user_handler", "openid_callback", "userinfo failed", err, nil)
		response.BadRequest(c, "获取亦 OpenID 用户信息失败，请重试")
		return
	}

	if intent == "bind" {
		if bindUserID == 0 {
			response.Unauthorized(c, "请先登录后再绑定")
			return
		}
		existing, err := h.userRepo.FindByOpenIDUID(ou.UID)
		if err == nil && existing != nil && existing.ID != bindUserID {
			response.Error(c, http.StatusConflict, "该亦 OpenID 已绑定其他账号，请先解绑")
			return
		}
		if err := h.userRepo.BindOpenID(bindUserID, ou.UID, ou.Username); err != nil {
			logging.Error("user_handler", "openid_bind", "bind failed", err,
				map[string]interface{}{"user_id": bindUserID, "uid": ou.UID})
			response.InternalError(c, "绑定失败，请重试")
			return
		}
		c.Redirect(http.StatusFound, h.siteOrigin()+"/user/settings?oauth_bound=1")
		return
	}

	// intent = login
	user, err := h.userRepo.FindByOpenIDUID(ou.UID)
	if err != nil {
		// First sign-in with this identity: auto-create a platform account.
		passwordHash, perr := h.authService.HashPassword(openIDRandomState() + hex.EncodeToString([]byte(ou.UID)))
		if perr != nil {
			response.InternalError(c, "failed to prepare account")
			return
		}
		email := "openid_" + strings.TrimSpace(ou.UID) + "@openid.yiziyun.com"
		nickname := ou.Nickname
		if nickname == "" {
			nickname = ou.Username
		}
		if nickname == "" {
			nickname = "OpenID 用户"
		}
		user = &model.User{
			Email:          email,
			PasswordHash:   passwordHash,
			Nickname:       nickname,
			Role:           model.RoleUser,
			Status:         model.UserStatusActive,
			OpenIDUID:      strings.TrimSpace(ou.UID),
			OpenIDUsername: ou.Username,
		}
		if cerr := h.userRepo.Create(user); cerr != nil {
			logging.Error("user_handler", "openid_callback", "auto create failed", cerr,
				map[string]interface{}{"uid": ou.UID})
			response.InternalError(c, "自动创建账号失败，请稍后重试")
			return
		}
	}

	if user.Status != model.UserStatusActive {
		response.Forbidden(c, "账号已被禁用")
		return
	}

	jwt, err := h.authService.GenerateToken(user)
	if err != nil {
		response.InternalError(c, "failed to generate token")
		return
	}
	now := time.Now()
	_ = h.userRepo.Update(&model.User{
		ID:          user.ID,
		LastLoginAt: &now,
	})
	// Redirect to the login route: the SPA reads oauth_token there
	// (Login page useEffect) and completes the sign-in. Redirecting to
	// /user directly would hit RequireAuth with no stored token and bounce
	// back to /user/login, dropping the query string and breaking login.
	c.Redirect(http.StatusFound, h.siteOrigin()+"/user/login?oauth_token="+jwt)
}

// OpenIDStatus returns the 亦 OpenID binding state of the current user.
// GET /api/v1/user/openid/status
func (h *UserHandler) OpenIDStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok || userID == 0 {
		response.Unauthorized(c, "user not authenticated")
		return
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		response.InternalError(c, "failed to load user")
		return
	}
	response.Success(c, gin.H{
		"bound":    user.OpenIDUID != "",
		"username": user.OpenIDUsername,
	})
}

// OpenIDBind returns the authorization URL for binding 亦 OpenID to the
// current account. POST /api/v1/user/openid/bind
func (h *UserHandler) OpenIDBind(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok || userID == 0 {
		response.Unauthorized(c, "user not authenticated")
		return
	}
	if h.openidSvc == nil {
		response.InternalError(c, "openid service not configured")
		return
	}
	cfg, err := h.openidSvc.LoadConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to load oauth config")
		return
	}
	if !cfg.Enabled || cfg.ClientID == "" {
		response.Error(c, http.StatusBadRequest, "亦 OpenID 登录未启用，请联系管理员")
		return
	}
	state := openIDRandomState()
	if err := h.openidSvc.StoreState(c.Request.Context(), state, "bind", userID); err != nil {
		response.InternalError(c, "failed to store oauth state")
		return
	}
	authURL, err := h.openidSvc.AuthorizeURL(cfg, state)
	if err != nil {
		response.InternalError(c, "failed to build authorize url")
		return
	}
	response.Success(c, gin.H{"authorize_url": authURL})
}

// OpenIDUnbind removes the 亦 OpenID binding from the current account.
// POST /api/v1/user/openid/unbind
func (h *UserHandler) OpenIDUnbind(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok || userID == 0 {
		response.Unauthorized(c, "user not authenticated")
		return
	}
	if err := h.userRepo.UnbindOpenID(userID); err != nil {
		response.InternalError(c, "解绑失败，请重试")
		return
	}
	response.Success(c, gin.H{"message": "已解绑"})
}

// GetUnlimitedModels returns the model IDs the current user is eligible for
// under the unlimited-firepower promo: the model's promo must be enabled AND the
// user must hold a paid active subscription covering that model. The user UI
// uses this to render "已激活" vs "订阅解锁" states instead of showing the
// perk to everyone whenever an admin toggles it on.
// GET /api/v1/user/unlimited-firepower
func (h *UserHandler) GetUnlimitedModels(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	eligible := []string{}
	for _, p := range h.billingService.ListEnabledPrices() {
		if !p.UnlimitedEnabled {
			continue
		}
		if _, ok := h.billingService.IsUnlimitedFirepower(userID, p.Model); ok {
			eligible = append(eligible, p.Model)
		}
	}
	response.Success(c, gin.H{"models": eligible})
}

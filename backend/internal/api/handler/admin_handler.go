package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/mass-platform/backend/internal/api/dto"
	"github.com/mass-platform/backend/internal/auth"
	"github.com/mass-platform/backend/internal/billing"
	"github.com/mass-platform/backend/internal/llm/provider"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/monitor"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/mass-platform/backend/pkg/response"
)

// AdminHandler handles admin-related API endpoints.
type AdminHandler struct {
	userRepo         *repository.UserRepository
	apiKeyRepo       *repository.ApiKeyRepository
	billingService   *billing.BillingService
	billingRepo      *repository.BillingRecordRepository
	txRepo           *repository.TransactionRepository
	planRepo         *repository.PlanRepository
	subRepo          *repository.SubscriptionRepository
	identityRepo     *repository.IdentityVerificationRepository
	monitorService   *monitor.MonitorService
	configRepo       *repository.SystemConfigRepository
	metricsRepo      *repository.SystemMetricsRepository
	logRepo          *repository.SystemLogRepository
	channelRepo      *repository.ChannelRepository
	pricingGroupRepo *repository.PricingGroupRepository
	modelPriceRepo   *repository.ModelPriceRepository
	resetCouponRepo  *repository.ResetCouponRepository
	notifRepo        *repository.NotificationRepository
	tokenPkgRepo     *repository.TokenPackageRepository
	invoiceRepo      *repository.InvoiceRepository
	creditRepo       *repository.CreditRepository
	authService      *auth.AuthService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(
	userRepo *repository.UserRepository,
	apiKeyRepo *repository.ApiKeyRepository,
	billingService *billing.BillingService,
	billingRepo *repository.BillingRecordRepository,
	txRepo *repository.TransactionRepository,
	planRepo *repository.PlanRepository,
	subRepo *repository.SubscriptionRepository,
	identityRepo *repository.IdentityVerificationRepository,
	monitorService *monitor.MonitorService,
	configRepo *repository.SystemConfigRepository,
	metricsRepo *repository.SystemMetricsRepository,
	logRepo *repository.SystemLogRepository,
	channelRepo *repository.ChannelRepository,
	pricingGroupRepo *repository.PricingGroupRepository,
	modelPriceRepo *repository.ModelPriceRepository,
	resetCouponRepo *repository.ResetCouponRepository,
	notifRepo *repository.NotificationRepository,
	tokenPkgRepo *repository.TokenPackageRepository,
	invoiceRepo *repository.InvoiceRepository,
	creditRepo *repository.CreditRepository,
	authService *auth.AuthService,
) *AdminHandler {
	return &AdminHandler{
		userRepo:         userRepo,
		apiKeyRepo:       apiKeyRepo,
		billingService:   billingService,
		billingRepo:      billingRepo,
		txRepo:           txRepo,
		planRepo:         planRepo,
		subRepo:          subRepo,
		identityRepo:     identityRepo,
		monitorService:   monitorService,
		configRepo:       configRepo,
		metricsRepo:      metricsRepo,
		logRepo:          logRepo,
		channelRepo:      channelRepo,
		pricingGroupRepo: pricingGroupRepo,
		modelPriceRepo:   modelPriceRepo,
		resetCouponRepo:  resetCouponRepo,
		notifRepo:        notifRepo,
		tokenPkgRepo:     tokenPkgRepo,
		invoiceRepo:      invoiceRepo,
		creditRepo:       creditRepo,
		authService:      authService,
	}
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

type updateUserStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active disabled suspended"`
	Reason string `json:"reason"`
}

type updateUserInfoRequest struct {
	Nickname       string   `json:"nickname"`
	Phone          string   `json:"phone"`
	Role           string   `json:"role"`
	Status         string   `json:"status"`
	RealNameStatus string   `json:"real_name_status"`
	Balance        *float64 `json:"balance"`
	BalanceAdjust  string   `json:"balance_adjust"`
	BalanceNote    string   `json:"balance_note"`
}

type reviewIdentityRequest struct {
	Action string `json:"action" binding:"required,oneof=approve reject"`
	Reason string `json:"reason"`
}

type createPlanRequest struct {
	Name            string   `json:"name" binding:"required"`
	Description     string   `json:"description"`
	Price           string   `json:"price" binding:"required,numeric"`
	Currency        string   `json:"currency"`
	DurationDays    int      `json:"duration_days" binding:"required,min=1"`
	RPM             int      `json:"rpm"`
	TPM             int      `json:"tpm"`
	IncludedTokens  int64    `json:"included_tokens"`
	ConcurrentLimit int      `json:"concurrent_limit"`
	ModelAccess     []string `json:"model_access"`
	SortOrder       int      `json:"sort_order"`
	MaxPurchase     int      `json:"max_purchase"`
}

type updatePlanRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Price           string   `json:"price" binding:"omitempty,numeric"`
	Currency        string   `json:"currency"`
	DurationDays    int      `json:"duration_days" binding:"omitempty,min=1"`
	RPM             int      `json:"rpm"`
	TPM             int      `json:"tpm"`
	IncludedTokens  int64    `json:"included_tokens"`
	ConcurrentLimit int      `json:"concurrent_limit"`
	ModelAccess     []string `json:"model_access"`
	Status          string   `json:"status" binding:"omitempty,oneof=active inactive"`
	SortOrder       int      `json:"sort_order"`
	MaxPurchase     int      `json:"max_purchase"`
}

type updateSystemConfigRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
	Group string `json:"group"`
}

type updateSystemConfigsRequest struct {
	Group string `json:"group" binding:"required"`
	Items []struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value"`
	} `json:"items" binding:"required"`
}

type userDetailResponse struct {
	ID             uint                       `json:"id"`
	Email          string                     `json:"email"`
	Nickname       string                     `json:"nickname"`
	Avatar         string                     `json:"avatar"`
	Role           string                     `json:"role"`
	Status         string                     `json:"status"`
	Balance        string                     `json:"balance"`
	RealNameStatus string                     `json:"real_name_status"`
	Phone          string                     `json:"phone"`
	LastLoginAt    *time.Time                 `json:"last_login_at"`
	LastLoginIP    string                     `json:"last_login_ip"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
	Subscriptions  []dto.SubscriptionResponse `json:"subscriptions"`
	APIKeys        []dto.ApiKeyResponse       `json:"api_keys"`
}

type analyticsOverviewResponse struct {
	TotalUsers        int64  `json:"total_users"`
	ActiveUsers       int64  `json:"active_users"`
	TotalRevenue      string `json:"total_revenue"`
	TodayRequests     int64  `json:"today_requests"`
	ActiveSubs        int64  `json:"active_subscriptions"`
	TodayRevenue      string `json:"today_revenue"`
	TodayNewUsers     int64  `json:"today_new_users"`
	PendingVerify     int64  `json:"pending_verifications"`
	TotalRequests     int64  `json:"total_requests"`
}

type revenueAnalyticsItem struct {
	Date   string `json:"date"`
	Amount string `json:"amount"`
	Count  int64  `json:"count"`
}

type systemHealthResponse struct {
	Database bool `json:"database"`
	Redis    bool `json:"redis"`
	API      bool `json:"api"`
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// getAdminID extracts the admin user_id from the Gin context.
func getAdminID(c *gin.Context) uint {
	id, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	uid, ok := id.(uint)
	if !ok {
		return 0
	}
	return uid
}

// parsePagination extracts page and size from query params, falling back to defaults.
func parsePagination(c *gin.Context) (page, size int) {
	page = 1
	size = 20

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 1 && v <= 100 {
			size = v
		}
	}
	return
}

// ---------------------------------------------------------------------------
// 1. ListUsers
// ---------------------------------------------------------------------------

// ListUsers returns a paginated list of users with optional search by email or nickname.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page, size := parsePagination(c)
	keyword := c.Query("keyword")

	var users []model.User
	var total int64
	var err error

	if keyword != "" {
		users, total, err = h.userRepo.Search(page, size, keyword)
	} else {
		users, total, err = h.userRepo.List(page, size, nil)
	}

	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}

	items := make([]dto.UserInfo, len(users))
	for i, u := range users {
		items[i] = dto.UserInfo{
			ID:             u.ID,
			Email:          u.Email,
			Nickname:       u.Nickname,
			Avatar:         u.Avatar,
			Role:           string(u.Role),
			Status:         string(u.Status),
			Balance:        u.Balance.String(),
			RealNameStatus: u.RealNameStatus,
			Phone:          u.Phone,
			CreatedAt:      u.CreatedAt.Format(time.RFC3339),
		}
	}

	response.Page(c, items, total, page, size)
}

// ---------------------------------------------------------------------------
// 2. GetUserDetail
// ---------------------------------------------------------------------------

// GetUserDetail returns detailed information about a specific user,
// including their subscriptions and API keys.
func (h *AdminHandler) GetUserDetail(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	// Fetch subscriptions
	subs, err := h.subRepo.FindActiveByUserID(user.ID)
	if err != nil {
		subs = nil
	}

	subscriptionItems := make([]dto.SubscriptionResponse, 0)
	for _, s := range subs {
		planName := ""
		if s.Plan.Name != "" {
			planName = s.Plan.Name
		}
		subscriptionItems = append(subscriptionItems, dto.SubscriptionResponse{
			ID:             s.ID,
			PlanName:       planName,
			Status:         s.Status,
			StartAt:        s.StartAt.Format(time.RFC3339),
			EndAt:          s.EndAt.Format(time.RFC3339),
			AutoRenew:      s.AutoRenew,
			Price:          s.Price.String(),
			UsedTokens:     s.UsedTokens,
			IncludedTokens: s.Plan.IncludedTokens,
		})
	}

	// Fetch API keys
	apiKeys, err := h.apiKeyRepo.FindByUserID(user.ID)
	if err != nil {
		apiKeys = nil
	}

	apiKeyItems := make([]dto.ApiKeyResponse, 0)
	for _, k := range apiKeys {
		lastUsed := ""
		if k.LastUsedAt != nil {
			lastUsed = k.LastUsedAt.Format(time.RFC3339)
		}
		expires := ""
		if k.ExpiresAt != nil {
			expires = k.ExpiresAt.Format(time.RFC3339)
		}
		apiKeyItems = append(apiKeyItems, dto.ApiKeyResponse{
			ID:          k.ID,
			KeyPrefix:   k.KeyPrefix,
			Name:        k.Name,
			ModelAccess: k.ModelAccess,
			Status:      k.Status,
			LastUsedAt:  lastUsed,
			ExpiresAt:   expires,
			CreatedAt:   k.CreatedAt.Format(time.RFC3339),
		})
	}

	resp := userDetailResponse{
		ID:             user.ID,
		Email:          user.Email,
		Nickname:       user.Nickname,
		Avatar:         user.Avatar,
		Role:           string(user.Role),
		Status:         string(user.Status),
		Balance:        user.Balance.String(),
		RealNameStatus: user.RealNameStatus,
		Phone:          user.Phone,
		LastLoginAt:    user.LastLoginAt,
		LastLoginIP:    user.LastLoginIP,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
		Subscriptions:  subscriptionItems,
		APIKeys:        apiKeyItems,
	}

	response.Success(c, resp)
}

// ---------------------------------------------------------------------------
// 3. UpdateUserStatus
// ---------------------------------------------------------------------------

// UpdateUserStatus updates the status of a user account (active/disabled/suspended).
func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	var req updateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	adminID, _ := getUserID(c)
	if userID == uint64(adminID) && req.Status != "active" {
		response.BadRequest(c, "不能禁用当前登录的管理员账号")
		return
	}

	user.Status = model.UserStatus(req.Status)

	if err := h.userRepo.Update(user); err != nil {
		response.InternalError(c, "failed to update user status")
		return
	}

	response.SuccessWithMessage(c, "user status updated successfully", gin.H{
		"user_id": user.ID,
		"status":  req.Status,
	})
}

// UpdateUserInfo edits profile fields of a user, optionally adjusting the
// balance (PUT /api/v1/admin/users/:id). Only non-empty fields are applied;
// a balance value triggers a balance adjustment recorded as an "adjust"
// transaction.
func (h *AdminHandler) UpdateUserInfo(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	var req updateUserInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.userRepo.FindByID(uint(userID))
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	adminID, _ := getUserID(c)
	isSelf := userID == uint64(adminID)

	if req.Nickname != "" {
		if len([]rune(req.Nickname)) > 50 {
			response.BadRequest(c, "昵称不能超过 50 个字符")
			return
		}
		user.Nickname = req.Nickname
	}
	if req.Phone != "" {
		if len(req.Phone) > 20 {
			response.BadRequest(c, "手机号格式不正确")
			return
		}
		user.Phone = req.Phone
	}
	if req.Role != "" {
		if req.Role != "user" && req.Role != "admin" {
			response.BadRequest(c, "invalid role, must be user or admin")
			return
		}
		if isSelf && req.Role != "admin" {
			response.BadRequest(c, "不能修改当前登录管理员的角色")
			return
		}
		user.Role = model.UserRole(req.Role)
	}
	if req.Status != "" {
		switch req.Status {
		case "active", "disabled", "suspended":
		default:
			response.BadRequest(c, "invalid status")
			return
		}
		if isSelf && req.Status != "active" {
			response.BadRequest(c, "不能禁用当前登录的管理员账号")
			return
		}
		user.Status = model.UserStatus(req.Status)
	}
	if req.RealNameStatus != "" {
		switch req.RealNameStatus {
		case "unverified", "pending", "verified", "rejected":
		default:
			response.BadRequest(c, "invalid real_name_status")
			return
		}
		user.RealNameStatus = req.RealNameStatus
	}

	if err := h.userRepo.Update(user); err != nil {
		response.InternalError(c, "failed to update user")
		return
	}

	// Balance adjustment: diff = target - current. Positive adds, negative
	// deducts; a matching transaction record is always written.
	var diff decimal.Decimal
	if req.BalanceAdjust != "" {
		raw := strings.TrimSpace(req.BalanceAdjust)
		raw = strings.Replace(raw, "＋", "+", 1)
		raw = strings.Replace(raw, "－", "-", 1)
		adj, perr := decimal.NewFromString(raw)
		if perr != nil || adj.IsZero() {
			response.BadRequest(c, "调整金额格式不正确，例如 +100 或 -50")
			return
		}
		diff = adj
		if user.Balance.Add(diff).LessThan(decimal.Zero) {
			response.BadRequest(c, "调整后余额不能为负数")
			return
		}
	} else if req.Balance != nil {
		target := decimal.NewFromFloat(*req.Balance)
		if target.LessThan(decimal.Zero) {
			response.BadRequest(c, "余额不能为负数")
			return
		}
		diff = target.Sub(user.Balance)
	}
	if !diff.IsZero() {
		note := req.BalanceNote
		if note == "" {
			note = "admin adjustment"
		}
		desc := fmt.Sprintf("Admin balance adjustment (%s): %s", diff.String(), note)
		if err := h.billingService.AdjustBalance(user.ID, diff, desc); err != nil {
			if strings.Contains(err.Error(), "adjusted balance cannot be negative") {
				response.BadRequest(c, "调整后余额不能为负数")
				return
			}
			response.InternalError(c, "failed to adjust balance")
			return
		}
	}

	response.SuccessWithMessage(c, "user updated successfully", gin.H{
		"user_id": user.ID,
	})
}

// ---------------------------------------------------------------------------
// 4. ListIdentityVerifications
// ---------------------------------------------------------------------------

// ListIdentityVerifications returns a paginated list of identity verification
// records, optionally filtered by status.
func (h *AdminHandler) ListIdentityVerifications(c *gin.Context) {
	page, size := parsePagination(c)
	status := c.Query("status")

	verifications, total, err := h.identityRepo.List(page, size, status)
	if err != nil {
		response.InternalError(c, "failed to list identity verifications")
		return
	}

	response.Page(c, verifications, total, page, size)
}

// ---------------------------------------------------------------------------
// 5. ReviewIdentityVerification
// ---------------------------------------------------------------------------

// ReviewIdentityVerification approves or rejects an identity verification request.
func (h *AdminHandler) ReviewIdentityVerification(c *gin.Context) {
	verificationID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid verification id")
		return
	}

	var req reviewIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	verification, err := h.identityRepo.FindByID(uint(verificationID))
	if err != nil {
		response.NotFound(c, "verification record not found")
		return
	}

	if verification.Status != "pending" {
		response.BadRequest(c, "verification has already been reviewed")
		return
	}

	adminID := getAdminID(c)
	now := time.Now()

	switch req.Action {
	case "approve":
		verification.Status = "approved"
		verification.RejectReason = ""
		verification.ReviewerID = &adminID
		verification.ReviewedAt = &now

		// Update user's real name status
		user, err := h.userRepo.FindByID(verification.UserID)
		if err == nil {
			user.RealNameStatus = "verified"
			_ = h.userRepo.Update(user)
		}

	case "reject":
		verification.Status = "rejected"
		verification.RejectReason = req.Reason
		verification.ReviewerID = &adminID
		verification.ReviewedAt = &now

		// Update user's real name status
		user, err := h.userRepo.FindByID(verification.UserID)
		if err == nil {
			user.RealNameStatus = "rejected"
			_ = h.userRepo.Update(user)
		}
	}

	if err := h.identityRepo.Update(verification); err != nil {
		response.InternalError(c, "failed to update verification record")
		return
	}

	response.SuccessWithMessage(c, "verification reviewed successfully", gin.H{
		"verification_id": verification.ID,
		"status":          verification.Status,
	})
}

// ---------------------------------------------------------------------------
// 6. ListPlans
// ---------------------------------------------------------------------------

// ListPlans returns all plans, including inactive ones.
func (h *AdminHandler) ListPlans(c *gin.Context) {
	plans, err := h.planRepo.FindAll()
	if err != nil {
		response.InternalError(c, "failed to list plans")
		return
	}

	items := make([]dto.PlanResponse, len(plans))
	for i, p := range plans {
		items[i] = dto.PlanResponse{
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
			MaxPurchase:     p.MaxPurchase,
		}
	}

	response.Success(c, items)
}

// ---------------------------------------------------------------------------
// 7. CreatePlan
// ---------------------------------------------------------------------------

// CreatePlan creates a new subscription plan.
func (h *AdminHandler) CreatePlan(c *gin.Context) {
	var req createPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	price, err := decimal.NewFromString(req.Price)
	if err != nil || price.LessThan(decimal.Zero) {
		response.BadRequest(c, "invalid price")
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "CNY"
	}

	plan := &model.Plan{
		Name:            req.Name,
		Description:     req.Description,
		Price:           price,
		Currency:        currency,
		DurationDays:    req.DurationDays,
		RPM:             req.RPM,
		TPM:             req.TPM,
		IncludedTokens:  req.IncludedTokens,
		ConcurrentLimit: req.ConcurrentLimit,
		ModelAccess:     req.ModelAccess,
		Status:          "active",
		SortOrder:       req.SortOrder,
		MaxPurchase:     req.MaxPurchase,
	}

	if err := h.planRepo.Create(plan); err != nil {
		response.InternalError(c, "failed to create plan")
		return
	}

	response.SuccessWithMessage(c, "plan created successfully", gin.H{
		"plan_id": plan.ID,
	})
}

// ---------------------------------------------------------------------------
// 8. UpdatePlan
// ---------------------------------------------------------------------------

// UpdatePlan updates an existing subscription plan.
func (h *AdminHandler) UpdatePlan(c *gin.Context) {
	planID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	var req updatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	plan, err := h.planRepo.FindByID(uint(planID))
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}

	if req.Name != "" {
		plan.Name = req.Name
	}
	if req.Description != "" {
		plan.Description = req.Description
	}
	if req.Price != "" {
		price, err := decimal.NewFromString(req.Price)
		if err != nil || price.LessThan(decimal.Zero) {
			response.BadRequest(c, "invalid price")
			return
		}
		plan.Price = price
	}
	if req.Currency != "" {
		plan.Currency = req.Currency
	}
	if req.DurationDays > 0 {
		plan.DurationDays = req.DurationDays
	}
	if req.RPM > 0 {
		plan.RPM = req.RPM
	}
	if req.TPM > 0 {
		plan.TPM = req.TPM
	}
	if req.IncludedTokens > 0 {
		plan.IncludedTokens = req.IncludedTokens
	}
	if req.ConcurrentLimit > 0 {
		plan.ConcurrentLimit = req.ConcurrentLimit
	}
	if req.ModelAccess != nil {
		plan.ModelAccess = req.ModelAccess
	}
	if req.Status != "" {
		plan.Status = req.Status
	}
	plan.MaxPurchase = req.MaxPurchase
	if req.SortOrder != 0 || req.SortOrder != plan.SortOrder {
		plan.SortOrder = req.SortOrder
	}

	if err := h.planRepo.Update(plan); err != nil {
		response.InternalError(c, "failed to update plan")
		return
	}

	response.SuccessWithMessage(c, "plan updated successfully", gin.H{
		"plan_id": plan.ID,
	})
}

// ---------------------------------------------------------------------------
// 9. DeletePlan
// ---------------------------------------------------------------------------

// DeletePlan deletes a subscription plan by ID.
func (h *AdminHandler) DeletePlan(c *gin.Context) {
	planID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	if err := h.planRepo.Delete(uint(planID)); err != nil {
		response.InternalError(c, "failed to delete plan")
		return
	}

	response.SuccessWithMessage(c, "plan deleted successfully", nil)
}

// ---------------------------------------------------------------------------
// 10. ListOrders
// ---------------------------------------------------------------------------

// ListOrders returns a paginated list of all transactions/recharge orders.
func (h *AdminHandler) ListOrders(c *gin.Context) {
	page, size := parsePagination(c)

	filters := make(map[string]interface{})
	if t := c.Query("type"); t != "" {
		filters["type = ?"] = t
	}
	if status := c.Query("status"); status != "" {
		filters["status = ?"] = status
	}
	if userID := c.Query("user_id"); userID != "" {
		if uid, err := strconv.ParseUint(userID, 10, 64); err == nil {
			filters["user_id = ?"] = uid
		}
	}

	orders, total, err := h.txRepo.FindAll(page, size, filters)
	if err != nil {
		response.InternalError(c, "failed to list orders")
		return
	}

	items := make([]dto.TransactionResponse, len(orders))
	for i, o := range orders {
		items[i] = dto.TransactionResponse{
			ID:            o.ID,
			TransactionNo: o.TransactionNo,
			Type:          string(o.Type),
			Amount:        o.Amount.String(),
			BalanceBefore: o.BalanceBefore.String(),
			BalanceAfter:  o.BalanceAfter.String(),
			PaymentMethod: o.PaymentMethod,
			Status:        string(o.Status),
			Description:   o.Description,
			CreatedAt:     o.CreatedAt.Format(time.RFC3339),
		}
	}

	response.Page(c, items, total, page, size)
}

// ---------------------------------------------------------------------------
// 11. GetAnalyticsOverview
// ---------------------------------------------------------------------------

// GetAnalyticsOverview returns overview statistics for the admin dashboard.
func (h *AdminHandler) GetAnalyticsOverview(c *gin.Context) {
	totalUsers, err := h.userRepo.Count()
	if err != nil {
		response.InternalError(c, "failed to count users")
		return
	}

	activeUsers, err := h.userRepo.CountByStatus(model.UserStatusActive)
	if err != nil {
		response.InternalError(c, "failed to count active users")
		return
	}

	totalRevenue, err := h.txRepo.SumRevenue()
	if err != nil {
		response.InternalError(c, "failed to calculate revenue")
		return
	}

	todayRequests, err := h.billingRepo.CountToday()
	if err != nil {
		response.InternalError(c, "failed to count today's requests")
		return
	}

	totalRequests, err := h.billingRepo.CountAll()
	if err != nil {
		response.InternalError(c, "failed to count requests")
		return
	}

	activeSubs, err := h.subRepo.CountActive()
	if err != nil {
		response.InternalError(c, "failed to count active subscriptions")
		return
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayRevenueItems, err := h.txRepo.SumByDateRange(startOfDay, now)
	if err != nil {
		response.InternalError(c, "failed to calculate today's revenue")
		return
	}
	var todayRevenue decimal.Decimal
	for _, item := range todayRevenueItems {
		todayRevenue = todayRevenue.Add(item.Amount)
	}

	todayNewUsers, err := h.userRepo.CountCreatedAfter(startOfDay)
	if err != nil {
		response.InternalError(c, "failed to count new users")
		return
	}

	pendingVerify, err := h.identityRepo.CountByStatus("pending")
	if err != nil {
		response.InternalError(c, "failed to count pending verifications")
		return
	}

	resp := analyticsOverviewResponse{
		TotalUsers:    totalUsers,
		ActiveUsers:   activeUsers,
		TotalRevenue:  totalRevenue.String(),
		TodayRequests: todayRequests,
		ActiveSubs:    activeSubs,
		TodayRevenue:  todayRevenue.String(),
		TodayNewUsers: todayNewUsers,
		PendingVerify: pendingVerify,
		TotalRequests: totalRequests,
	}

	response.Success(c, resp)
}

// ---------------------------------------------------------------------------
// 12. GetRevenueAnalytics
// ---------------------------------------------------------------------------

// GetRevenueAnalytics returns revenue breakdown by date range.
func (h *AdminHandler) GetRevenueAnalytics(c *gin.Context) {
	startStr := c.Query("start_date")
	endStr := c.Query("end_date")

	start := time.Now().AddDate(0, -1, 0) // default: last 30 days
	end := time.Now()

	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			end = t.Add(24*time.Hour - time.Second) // end of day
		}
	}

	items, err := h.txRepo.SumByDateRange(start, end)
	if err != nil {
		response.InternalError(c, "failed to get revenue analytics")
		return
	}

	result := make([]revenueAnalyticsItem, len(items))
	for i, item := range items {
		result[i] = revenueAnalyticsItem{
			Date:   item.Date,
			Amount: item.Amount.String(),
			Count:  item.Count,
		}
	}

	response.Success(c, result)
}

// ---------------------------------------------------------------------------
// 12b. GetDailyAnalytics
// ---------------------------------------------------------------------------

type dailyAnalyticsItem struct {
	Date     string `json:"date"`
	Revenue  string `json:"revenue"`
	Requests int64  `json:"requests"`
	NewUsers int64  `json:"new_users"`
	NewSubs  int64  `json:"new_subs"`
}

// GetDailyAnalytics returns daily trends (revenue, requests, new users, new
// subscriptions) within the given date range, filling every day with zeros.
func (h *AdminHandler) GetDailyAnalytics(c *gin.Context) {
	layout := "2006-01-02"
	start, err1 := time.Parse(layout, c.Query("start_date"))
	end, err2 := time.Parse(layout, c.Query("end_date"))
	if err1 != nil || err2 != nil || end.Before(start) {
		response.BadRequest(c, "invalid date range")
		return
	}
	end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, end.Location())

	revItems, err := h.txRepo.SumByDateRange(start, end)
	if err != nil {
		response.InternalError(c, "failed to get revenue analytics")
		return
	}
	reqItems, err := h.billingRepo.CountByDateRange(start, end)
	if err != nil {
		response.InternalError(c, "failed to count requests")
		return
	}
	userItems, err := h.userRepo.CountCreatedByDateRange(start, end)
	if err != nil {
		response.InternalError(c, "failed to count new users")
		return
	}
	subItems, err := h.subRepo.CountCreatedByDateRange(start, end)
	if err != nil {
		response.InternalError(c, "failed to count new subscriptions")
		return
	}

	revMap := make(map[string]decimal.Decimal)
	for _, it := range revItems {
		revMap[it.Date] = it.Amount
	}
	reqMap := make(map[string]int64)
	for _, it := range reqItems {
		reqMap[it.Date] = it.Count
	}
	userMap := make(map[string]int64)
	for _, it := range userItems {
		userMap[it.Date] = it.Count
	}
	subMap := make(map[string]int64)
	for _, it := range subItems {
		subMap[it.Date] = it.Count
	}

	var result []dailyAnalyticsItem
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format(layout)
		result = append(result, dailyAnalyticsItem{
			Date:     ds,
			Revenue:  revMap[ds].String(),
			Requests: reqMap[ds],
			NewUsers: userMap[ds],
			NewSubs:  subMap[ds],
		})
	}

	response.Success(c, result)
}

// ---------------------------------------------------------------------------
// 13. GetSystemConfig
// ---------------------------------------------------------------------------

// GetSystemConfig returns all system configuration entries.
func (h *AdminHandler) GetSystemConfig(c *gin.Context) {
	configs, err := h.configRepo.GetAll()
	if err != nil {
		response.InternalError(c, "failed to get system config")
		return
	}

	response.Success(c, configs)
}

// GetPublicSiteConfig returns the public branding settings (site name, logo,
// description, footer, ICP number, etc.) consumed by the user/admin consoles.
// It is public and does not require authentication; only the "site" group keys
// are exposed to avoid leaking credentials of other config groups.
func (h *AdminHandler) GetPublicSiteConfig(c *gin.Context) {
	configs, err := h.configRepo.GetAll()
	if err != nil {
		response.InternalError(c, "failed to get site config")
		return
	}

	publicKeys := map[string]bool{
		"site_name":        true,
		"site_url":         true,
		"site_logo":        true,
		"site_description": true,
		"site_icp":         true,
		"site_footer":      true,
		"legal_terms":      true,
		"legal_privacy":    true,
	}
	out := make(map[string]string, len(publicKeys))
	for _, cfg := range configs {
		if publicKeys[cfg.Key] {
			out[cfg.Key] = cfg.Value
		}
	}

	response.Success(c, out)
}

// ---------------------------------------------------------------------------
// 14. UpdateSystemConfig
// ---------------------------------------------------------------------------

// UpdateSystemConfig updates a specific system configuration entry.
func (h *AdminHandler) UpdateSystemConfig(c *gin.Context) {
	var req updateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.configRepo.Set(req.Key, req.Value, req.Group); err != nil {
		response.InternalError(c, "failed to update system config")
		return
	}

	response.SuccessWithMessage(c, "system config updated successfully", gin.H{
		"key": req.Key,
	})
}

// UpdateSystemConfigs batch-saves the config entries of one group in a single
// transaction, backing the category forms on the admin settings page
// (site / contact / notify / payment).
func (h *AdminHandler) UpdateSystemConfigs(c *gin.Context) {
	var req updateSystemConfigsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	items := make([]model.SystemConfig, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, model.SystemConfig{Key: it.Key, Value: it.Value})
	}
	if err := h.configRepo.SetBatch(req.Group, items); err != nil {
		response.InternalError(c, "failed to update system configs")
		return
	}

	response.SuccessWithMessage(c, "system configs updated successfully", gin.H{
		"group": req.Group,
		"count": len(req.Items),
	})
}

// ---------------------------------------------------------------------------
// 15. GetSystemLogs
// ---------------------------------------------------------------------------

// GetSystemLogs returns paginated system logs with optional filters
// (level, module, date range).
func (h *AdminHandler) GetSystemLogs(c *gin.Context) {
	page, size := parsePagination(c)

	filters := make(map[string]interface{})

	if level := c.Query("level"); level != "" {
		filters["level = ?"] = level
	}
	if module := c.Query("module"); module != "" {
		filters["module = ?"] = module
	}
	if startStr := c.Query("start_date"); startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			filters["created_at >= ?"] = t
		}
	}
	if endStr := c.Query("end_date"); endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			filters["created_at <= ?"] = t.Add(24*time.Hour - time.Second)
		}
	}

	logs, total, err := h.logRepo.List(page, size, filters)
	if err != nil {
		response.InternalError(c, "failed to get system logs")
		return
	}

	response.Page(c, logs, total, page, size)
}

// ---------------------------------------------------------------------------
// 16. GetSystemMetrics
// ---------------------------------------------------------------------------

// GetSystemMetrics returns system metrics data within a date range.
func (h *AdminHandler) GetSystemMetrics(c *gin.Context) {
	startStr := c.Query("start_date")
	endStr := c.Query("end_date")

	start := time.Now().AddDate(0, 0, -7) // default: last 7 days
	end := time.Now()

	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			end = t.Add(24*time.Hour - time.Second)
		}
	}

	// 实时聚合数据库，按日返回（不依赖定时采集表）
	metrics, err := h.metricsRepo.AggregateRange(start, end)
	if err != nil {
		response.InternalError(c, "failed to get system metrics")
		return
	}

	response.Success(c, metrics)
}

// ---------------------------------------------------------------------------
// 17. GetSystemHealth
// ---------------------------------------------------------------------------

// GetSystemHealth returns the health status of system components.
func (h *AdminHandler) GetSystemHealth(c *gin.Context) {
	health := h.monitorService.GetSystemHealth()

	resp := systemHealthResponse{
		Database: health["database"],
		Redis:    health["redis"],
		API:      health["api"],
	}

	response.Success(c, resp)
}

// ---------------------------------------------------------------------------
// 18. LLM channel management (模型渠道)
// ---------------------------------------------------------------------------

type channelRequest struct {
	Name     string   `json:"name" binding:"required"`
	Type     string   `json:"type" binding:"required,oneof=openai anthropic"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	Models   []string `json:"models"`
	Priority int      `json:"priority"`
	Enabled  bool     `json:"enabled"`
	Remark   string   `json:"remark"`
}

func sanitizeChannelModels(ms []string) model.StringSlice {
	var out model.StringSlice
	seen := make(map[string]bool)
	for _, m := range ms {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// maskSecret shows only the first 6 and last 4 chars of a secret.
func maskSecret(secret string) string {
	if len(secret) <= 10 {
		if secret == "" {
			return ""
		}
		return "******"
	}
	return secret[:6] + "****" + secret[len(secret)-4:]
}

// ListLLMChannels returns all LLM channels (GET /api/v1/admin/channels).
// Upstream API keys are masked; full keys are never returned to clients.
func (h *AdminHandler) ListLLMChannels(c *gin.Context) {
	list, err := h.channelRepo.List()
	if err != nil {
		response.InternalError(c, "failed to list channels")
		return
	}
	for i := range list {
		if list[i].APIKey != "" {
			list[i].APIKey = maskSecret(list[i].APIKey)
		}
	}
	response.Success(c, list)
}

// AddLLMChannel creates a new LLM channel (POST /api/v1/admin/channels).
func (h *AdminHandler) AddLLMChannel(c *gin.Context) {
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: name and type (openai/anthropic) are required")
		return
	}
	ch := &model.LLMChannel{
		Name:     strings.TrimSpace(req.Name),
		Type:     req.Type,
		BaseURL:  strings.TrimSpace(req.BaseURL),
		APIKey:   strings.TrimSpace(req.APIKey),
		Models:   sanitizeChannelModels(req.Models),
		Priority: req.Priority,
		Enabled:  req.Enabled,
		Remark:   strings.TrimSpace(req.Remark),
	}
	if err := h.channelRepo.Create(ch); err != nil {
		response.InternalError(c, "failed to create channel")
		return
	}
	response.Success(c, ch)
}

// UpdateLLMChannel updates an existing LLM channel (PUT /api/v1/admin/channels/:id).
func (h *AdminHandler) UpdateLLMChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid channel id")
		return
	}
	ch, err := h.channelRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "channel not found")
		return
	}
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: name and type (openai/anthropic) are required")
		return
	}
	ch.Name = strings.TrimSpace(req.Name)
	ch.Type = req.Type
	ch.BaseURL = strings.TrimSpace(req.BaseURL)
	if req.APIKey != "" {
		ch.APIKey = strings.TrimSpace(req.APIKey)
	}
	ch.Models = sanitizeChannelModels(req.Models)
	ch.Priority = req.Priority
	ch.Enabled = req.Enabled
	ch.Remark = strings.TrimSpace(req.Remark)
	if err := h.channelRepo.Update(ch); err != nil {
		response.InternalError(c, "failed to update channel")
		return
	}
	ch.APIKey = maskSecret(ch.APIKey)
	response.Success(c, ch)
}

// DeleteLLMChannel deletes an LLM channel (DELETE /api/v1/admin/channels/:id).
func (h *AdminHandler) DeleteLLMChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid channel id")
		return
	}
	if err := h.channelRepo.Delete(uint(id)); err != nil {
		response.InternalError(c, "failed to delete channel")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

type testChannelRequest struct {
	Type    string `json:"type" binding:"required,oneof=openai anthropic"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// fetchModelsFromProvider probes an upstream provider's /models endpoint and
// returns the available model ids. errMsg is empty on success.
func fetchModelsFromProvider(baseURL, apiKey, typ string) (ids []string, latencyMs int64, errMsg string) {
	client := &http.Client{Timeout: 20 * time.Second}
	base := provider.NormalizeBaseURL(baseURL)

	var headers = map[string]string{}
	switch typ {
	case "anthropic":
		headers["x-api-key"] = strings.TrimSpace(apiKey)
		headers["anthropic-version"] = "2023-06-01"
	default:
		headers["Authorization"] = "Bearer " + strings.TrimSpace(apiKey)
	}

	// 统一在归一化后的基址后追加 /v1/models（与 chat 路由保持一致：
	// NormalizeBaseURL 已剔除用户误填的 /v1，此处始终补回 /v1）。
	candidates := []string{base + "/v1/models"}
	var lastStatus int
	var lastMsg string
	for _, url := range candidates {
		httpReq, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		for k, v := range headers {
			if v != "" {
				httpReq.Header.Set(k, v)
			}
		}

		start := time.Now()
		resp, err := client.Do(httpReq)
		latencyMs = time.Since(start).Milliseconds()
		if err != nil {
			lastMsg = "无法连接上游：" + err.Error()
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastStatus = resp.StatusCode
			msg := strings.TrimSpace(string(body))
			if len(msg) > 300 {
				msg = msg[:300]
			}
			lastMsg = fmt.Sprintf("上游返回 HTTP %d: %s", resp.StatusCode, msg)
			continue
		}
		if parsed := extractModelIDs(body); len(parsed) > 0 {
			return parsed, latencyMs, ""
		}
		lastMsg = "上游响应格式异常（未解析到 model 列表）"
	}
	if lastStatus != 0 {
		return nil, latencyMs, lastMsg
	}
	if lastMsg == "" {
		lastMsg = "无法连接上游"
	}
	return nil, latencyMs, lastMsg
}

// extractModelIDs 从上游 /models 响应中尽力解析模型 ID，兼容多种结构：
// {"data":[{"id":..}]} / {"models":[..]} / 顶层数组，字段支持 id / name / model。
func extractModelIDs(body []byte) []string {
	var generic struct {
		Data   []map[string]interface{} `json:"data"`
		Models []map[string]interface{} `json:"models"`
	}
	_ = json.Unmarshal(body, &generic)
	src := append(generic.Data, generic.Models...)
	if len(src) == 0 {
		var arr []map[string]interface{}
		if json.Unmarshal(body, &arr) == nil {
			src = arr
		}
	}

	seen := map[string]bool{}
	ids := make([]string, 0, len(src))
	pick := func(m map[string]interface{}) string {
		for _, k := range []string{"id", "name", "model"} {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}
	for _, m := range src {
		if id := pick(m); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func channelModelsResult(ids []string, latencyMs int64, errMsg string) gin.H {
	if errMsg != "" {
		return gin.H{
			"ok":         false,
			"latency_ms": latencyMs,
			"count":      0,
			"models":     []string{},
			"error":      errMsg,
		}
	}
	return gin.H{
		"ok":         true,
		"latency_ms": latencyMs,
		"count":      len(ids),
		"models":     ids,
	}
}

// TestLLMChannel probes an upstream provider's /models endpoint with the given
// credentials and returns the available model ids (POST /api/v1/admin/channels/test).
// It always returns HTTP 200; use the ok flag to distinguish success/failure so
// the frontend can render the result inline.
func (h *AdminHandler) TestLLMChannel(c *gin.Context) {
	var req testChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: type (openai/anthropic) is required")
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		response.BadRequest(c, "base_url is required for testing")
		return
	}
	ids, latency, errMsg := fetchModelsFromProvider(req.BaseURL, req.APIKey, req.Type)
	res := channelModelsResult(ids, latency, errMsg)
	if errMsg == "" {
		res["provider"] = req.Type
	}
	response.Success(c, res)
}

// FetchChannelModels loads a stored channel's credentials and probes its
// upstream /models endpoint (POST /api/v1/admin/channels/:id/fetch-models).
// Used by the admin UI to import a channel's model list into the price table.
func (h *AdminHandler) FetchChannelModels(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid channel id")
		return
	}
	ch, err := h.channelRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "channel not found")
		return
	}
	ids, latency, errMsg := fetchModelsFromProvider(ch.BaseURL, ch.APIKey, ch.Type)
	response.Success(c, channelModelsResult(ids, latency, errMsg))
}

// ---------------------------------------------------------------------------
// 19. Pricing group management (模型定价 / 分组倍率)
// ---------------------------------------------------------------------------

type pricingGroupRequest struct {
	Name       string   `json:"name" binding:"required"`
	Multiplier string   `json:"multiplier" binding:"required"`
	Models     []string `json:"models"`
	Enabled    bool     `json:"enabled"`
	Remark     string   `json:"remark"`
}

func (h *AdminHandler) applyPricingGroup(ch *model.PricingGroup, req pricingGroupRequest) error {
	mult, err := decimal.NewFromString(strings.TrimSpace(req.Multiplier))
	if err != nil || mult.LessThanOrEqual(decimal.Zero) || mult.GreaterThan(decimal.NewFromInt(1000)) {
		return errors.New("multiplier must be a positive number (max 1000)")
	}
	ch.Name = strings.TrimSpace(req.Name)
	ch.Multiplier = mult
	ch.Models = sanitizeChannelModels(req.Models)
	ch.Enabled = req.Enabled
	ch.Remark = strings.TrimSpace(req.Remark)
	return nil
}

// ListPricingGroups returns all pricing groups (GET /api/v1/admin/pricing-groups).
func (h *AdminHandler) ListPricingGroups(c *gin.Context) {
	list, err := h.pricingGroupRepo.List()
	if err != nil {
		response.InternalError(c, "failed to list pricing groups")
		return
	}
	response.Success(c, list)
}

// AddPricingGroup creates a new pricing group (POST /api/v1/admin/pricing-groups).
func (h *AdminHandler) AddPricingGroup(c *gin.Context) {
	var req pricingGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: name and multiplier are required")
		return
	}
	g := &model.PricingGroup{}
	if err := h.applyPricingGroup(g, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.pricingGroupRepo.Create(g); err != nil {
		response.InternalError(c, "failed to create pricing group")
		return
	}
	response.Success(c, g)
}

// UpdatePricingGroup updates an existing pricing group (PUT /api/v1/admin/pricing-groups/:id).
func (h *AdminHandler) UpdatePricingGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid pricing group id")
		return
	}
	g, err := h.pricingGroupRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "pricing group not found")
		return
	}
	var req pricingGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: name and multiplier are required")
		return
	}
	if err := h.applyPricingGroup(g, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.pricingGroupRepo.Update(g); err != nil {
		response.InternalError(c, "failed to update pricing group")
		return
	}
	response.Success(c, g)
}

// DeletePricingGroup deletes a pricing group (DELETE /api/v1/admin/pricing-groups/:id).
func (h *AdminHandler) DeletePricingGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid pricing group id")
		return
	}
	if err := h.pricingGroupRepo.Delete(uint(id)); err != nil {
		response.InternalError(c, "failed to delete pricing group")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ---------------------------------------------------------------------------
// Model price management
// ---------------------------------------------------------------------------

const millionTokens = 1_000_000

// modelPriceRequest uses per-1M-tokens units (CNY) for human-friendly entry.
// Cache prices are optional: an empty cache_read_price / cache_write_price
// falls back to the built-in defaults (input×10% / input×125%).
type modelPriceRequest struct {
	Model           string `json:"model" binding:"required"`
	InputPrice      string `json:"input_price" binding:"required"`  // ¥ per 1M tokens
	OutputPrice     string `json:"output_price" binding:"required"` // ¥ per 1M tokens
	CacheReadPrice  string `json:"cache_read_price"`                // ¥ per 1M tokens, optional
	CacheWritePrice string `json:"cache_write_price"`               // ¥ per 1M tokens, optional
	Enabled         bool   `json:"enabled"`
	Remark          string `json:"remark"`
	SupportUnlimited bool  `json:"support_unlimited"`               // 是否支持无限火力活动
}

// modelPriceResponse exposes per-1M-tokens units to the admin UI.
type modelPriceResponse struct {
	ID              uint   `json:"id"`
	Model           string `json:"model"`
	InputPrice      string `json:"input_price"`       // ¥ per 1M tokens
	OutputPrice     string `json:"output_price"`      // ¥ per 1M tokens
	CacheReadPrice  string `json:"cache_read_price"`  // ¥ per 1M tokens; 空 = 默认(输入×10%)
	CacheWritePrice string `json:"cache_write_price"` // ¥ per 1M tokens; 空 = 默认(输入×125%)
	Enabled         bool   `json:"enabled"`
	Remark          string `json:"remark"`
	SupportUnlimited bool  `json:"support_unlimited"`
	UnlimitedEnabled bool  `json:"unlimited_enabled"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// perTokenFromMillion parses a per-1M-tokens price into the stored per-token value.
func perTokenFromMillion(v string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(v))
	if err != nil || d.IsNegative() {
		return decimal.Zero, errors.New("price must be a non-negative number")
	}
	if d.GreaterThan(decimal.NewFromInt(millionTokens)) {
		return decimal.Zero, errors.New("price too large (max ¥1,000,000 per 1M tokens)")
	}
	return d.Div(decimal.NewFromInt(millionTokens)), nil
}

// perMillionString converts a stored per-token price back to per-1M-tokens.
func perMillionString(d decimal.Decimal) string {
	return d.Mul(decimal.NewFromInt(millionTokens)).Round(6).String()
}

func toModelPriceResponse(p *model.ModelPrice) modelPriceResponse {
	get := func(v decimal.NullDecimal) string {
		if !v.Valid {
			return ""
		}
		return perMillionString(v.Decimal)
	}
	return modelPriceResponse{
		ID:              p.ID,
		Model:           p.Model,
		InputPrice:      perMillionString(p.InputPrice),
		OutputPrice:     perMillionString(p.OutputPrice),
		CacheReadPrice:  get(p.CacheReadPrice),
		CacheWritePrice: get(p.CacheWritePrice),
		Enabled:         p.Enabled,
		Remark:          p.Remark,
		SupportUnlimited: p.SupportUnlimited,
		UnlimitedEnabled: p.UnlimitedEnabled,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *AdminHandler) applyModelPrice(p *model.ModelPrice, req modelPriceRequest) error {
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" || len(modelName) > 100 {
		return errors.New("model must be a non-empty name (max 100 chars)")
	}
	in, err := perTokenFromMillion(req.InputPrice)
	if err != nil {
		return fmt.Errorf("input price: %w", err)
	}
	out, err := perTokenFromMillion(req.OutputPrice)
	if err != nil {
		return fmt.Errorf("output price: %w", err)
	}
	// Optional cache prices: empty string resets to the built-in defaults.
	cacheRead, err := optionalCachePrice(req.CacheReadPrice)
	if err != nil {
		return fmt.Errorf("cache read price: %w", err)
	}
	cacheWrite, err := optionalCachePrice(req.CacheWritePrice)
	if err != nil {
		return fmt.Errorf("cache write price: %w", err)
	}
	p.Model = modelName
	p.InputPrice = in
	p.OutputPrice = out
	p.CacheReadPrice = cacheRead
	p.CacheWritePrice = cacheWrite
	p.Enabled = req.Enabled
	p.Remark = strings.TrimSpace(req.Remark)
	p.SupportUnlimited = req.SupportUnlimited
	return nil
}

// optionalCachePrice parses an optional per-1M-tokens cache price. An empty
// string maps to NULL (use the built-in default factor).
func optionalCachePrice(v string) (decimal.NullDecimal, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return decimal.NullDecimal{}, nil
	}
	d, err := perTokenFromMillion(v)
	if err != nil {
		return decimal.NullDecimal{}, err
	}
	return decimal.NullDecimal{Decimal: d, Valid: true}, nil
}

// ListModelPrices returns all model prices (GET /api/v1/admin/model-prices).
func (h *AdminHandler) ListModelPrices(c *gin.Context) {
	list, err := h.modelPriceRepo.List()
	if err != nil {
		response.InternalError(c, "failed to list model prices")
		return
	}
	out := make([]modelPriceResponse, 0, len(list))
	for i := range list {
		out = append(out, toModelPriceResponse(&list[i]))
	}
	response.Success(c, out)
}

// AddModelPrice creates a model price entry (POST /api/v1/admin/model-prices).
func (h *AdminHandler) AddModelPrice(c *gin.Context) {
	var req modelPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: model and prices are required")
		return
	}
	modelName := strings.TrimSpace(req.Model)
	if existing, err := h.modelPriceRepo.FindByModel(modelName); err == nil && existing != nil {
		response.BadRequest(c, fmt.Sprintf("model price already exists for %s", modelName))
		return
	}
	p := &model.ModelPrice{}
	if err := h.applyModelPrice(p, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.modelPriceRepo.Create(p); err != nil {
		response.InternalError(c, "failed to create model price")
		return
	}
	response.Success(c, toModelPriceResponse(p))
}

// UpdateModelPrice updates a model price entry (PUT /api/v1/admin/model-prices/:id).
func (h *AdminHandler) UpdateModelPrice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid model price id")
		return
	}
	p, err := h.modelPriceRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "model price not found")
		return
	}
	var req modelPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: model and prices are required")
		return
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName != p.Model {
		if existing, err := h.modelPriceRepo.FindByModel(modelName); err == nil && existing != nil {
			response.BadRequest(c, fmt.Sprintf("model price already exists for %s", modelName))
			return
		}
	}
	if err := h.applyModelPrice(p, req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.modelPriceRepo.Update(p); err != nil {
		response.InternalError(c, "failed to update model price")
		return
	}
	response.Success(c, toModelPriceResponse(p))
}

// DeleteModelPrice deletes a model price entry (DELETE /api/v1/admin/model-prices/:id).
func (h *AdminHandler) DeleteModelPrice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid model price id")
		return
	}
	if err := h.modelPriceRepo.Delete(uint(id)); err != nil {
		response.InternalError(c, "failed to delete model price")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// SyncModelPrices imports models into the price table. Models are taken from a
// channel's stored model list (channel_id) or an explicit list, and an entry is
// created for each model that does not already exist. Existing entries are left
// untouched. (POST /api/v1/admin/model-prices/sync)
func (h *AdminHandler) SyncModelPrices(c *gin.Context) {
	var req struct {
		ChannelID     *uint    `json:"channel_id"`
		Models        []string `json:"models"`
		DefaultInput  string   `json:"default_input"`
		DefaultOutput string   `json:"default_output"`
		Enabled       bool     `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	models := req.Models
	if req.ChannelID != nil {
		ch, err := h.channelRepo.FindByID(*req.ChannelID)
		if err != nil {
			response.NotFound(c, "channel not found")
			return
		}
		models = []string(ch.Models)
	}

	// 去重（大小写不敏感）
	lowerSeen := map[string]bool{}
	uniq := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || lowerSeen[strings.ToLower(m)] {
			continue
		}
		lowerSeen[strings.ToLower(m)] = true
		uniq = append(uniq, m)
	}
	if len(uniq) == 0 {
		response.Success(c, gin.H{"created": 0, "skipped": 0, "total": 0})
		return
	}

	existing, err := h.modelPriceRepo.List()
	if err != nil {
		response.InternalError(c, "failed to load price table")
		return
	}
	have := map[string]bool{}
	for _, p := range existing {
		have[strings.ToLower(p.Model)] = true
	}

	in := decimal.Zero
	if v, err := perTokenFromMillion(req.DefaultInput); err == nil {
		in = v
	}
	out := decimal.Zero
	if v, err := perTokenFromMillion(req.DefaultOutput); err == nil {
		out = v
	}

	created, skipped := 0, 0
	for _, m := range uniq {
		if have[strings.ToLower(m)] {
			skipped++
			continue
		}
		p := &model.ModelPrice{
			Model:       m,
			InputPrice:  in,
			OutputPrice: out,
			Enabled:     req.Enabled,
		}
		if err := h.modelPriceRepo.Create(p); err != nil {
			continue
		}
		created++
	}
	response.Success(c, gin.H{"created": created, "skipped": skipped, "total": len(uniq)})
}

// ToggleModelUnlimited flips the unlimited-firepower switch for a model price
// (POST /api/v1/admin/model-prices/:id/unlimited). Only models whose
// support_unlimited flag is true can be enabled; the handler enforces this so an
// operator cannot turn the promo on for an unsupported model.
func (h *AdminHandler) ToggleModelUnlimited(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid model price id")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	p, err := h.modelPriceRepo.FindByID(uint(id))
	if err != nil {
		response.NotFound(c, "model price not found")
		return
	}
	if req.Enabled && !p.SupportUnlimited {
		response.BadRequest(c, "该模型未启用无限火力支持，无法开启")
		return
	}
	p.UnlimitedEnabled = req.Enabled
	if err := h.modelPriceRepo.Update(p); err != nil {
		response.InternalError(c, "failed to update model price")
		return
	}
	response.Success(c, toModelPriceResponse(p))
}

// resetCouponRequest is the payload for issuing reset coupons.
type resetCouponRequest struct {
	UserID uint   `json:"user_id"` // 0 = issue to all active users
	Count  int    `json:"count"`   // number of coupons per user (default 1, max 10)
	Note   string `json:"note"`
}

// IssueResetCoupons issues reset coupons to a specific user or to all users
// (POST /api/v1/admin/reset-coupons).
func (h *AdminHandler) IssueResetCoupons(c *gin.Context) {
	adminID := getAdminID(c)
	if adminID == 0 {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req resetCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	gen := func(userID uint) model.ResetCoupon {
		return model.ResetCoupon{
			Code:     generateCouponCode(),
			UserID:   userID,
			Status:   "unused",
			Note:     strings.TrimSpace(req.Note),
			IssuedBy: adminID,
		}
	}

	var targetIDs []uint
	if req.UserID > 0 {
		user, err := h.userRepo.FindByID(req.UserID)
		if err != nil {
			response.BadRequest(c, "user not found")
			return
		}
		targetIDs = []uint{user.ID}
	} else {
		users, err := h.userRepo.ListActiveIDs()
		if err != nil {
			response.InternalError(c, "failed to load users")
			return
		}
		if len(users) == 0 {
			response.BadRequest(c, "no active users")
			return
		}
		targetIDs = users
	}

	issued := 0
	for _, uid := range targetIDs {
		var coupons []model.ResetCoupon
		for i := 0; i < count; i++ {
			coupons = append(coupons, gen(uid))
		}
		if err := h.resetCouponRepo.CreateBatch(coupons); err != nil {
			logging.Error("admin", "issue_reset_coupons", "failed to create coupons", err,
				map[string]interface{}{"user_id": uid})
			continue
		}
		issued += len(coupons)
	}

	target := "specific"
	if req.UserID == 0 {
		target = "all"
	}
	response.Success(c, gin.H{
		"issued": issued,
		"target": target,
	})
}

// ListResetCoupons returns paginated reset coupons (GET /api/v1/admin/reset-coupons).
func (h *AdminHandler) ListResetCoupons(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if size < 1 || size > 100 {
		size = 20
	}

	var userID *uint
	if v := c.Query("user_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil && id > 0 {
			uid := uint(id)
			userID = &uid
		}
	}

	coupons, total, err := h.resetCouponRepo.ListPaginated(page, size, userID)
	if err != nil {
		response.InternalError(c, "failed to load reset coupons")
		return
	}

	items := make([]gin.H, len(coupons))
	userCache := map[uint]string{}
	for i, cp := range coupons {
		email := ""
		if name, ok := userCache[cp.UserID]; ok {
			email = name
		} else if u, err := h.userRepo.FindByID(cp.UserID); err == nil {
			email = u.Email
			userCache[cp.UserID] = email
		}
		items[i] = gin.H{
			"id":         cp.ID,
			"code":       cp.Code,
			"user_id":    cp.UserID,
			"user_email": email,
			"status":     cp.Status,
			"note":       cp.Note,
			"issued_by":  cp.IssuedBy,
			"used_at":    cp.UsedAt,
			"created_at": cp.CreatedAt,
		}
	}

	response.Success(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// generateCouponCode creates a unique redemption code: RC + timestamp + random hex.
func generateCouponCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("RC%d%s", time.Now().UnixMilli(), hex.EncodeToString(b))
}

// SendNotification sends a notification to a specific user or all active
// users (POST /api/v1/admin/notifications).
func (h *AdminHandler) SendNotification(c *gin.Context) {
	var req struct {
		UserID  uint   `json:"user_id"` // 0 = all active users
		Title   string `json:"title"`
		Content string `json:"content"`
		Type    string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" {
		response.BadRequest(c, "title is required")
		return
	}
	if req.Content == "" {
		response.BadRequest(c, "content is required")
		return
	}
	ntype := req.Type
	if ntype == "" {
		ntype = "system"
	}

	adminID := getAdminID(c)
	if adminID == 0 {
		response.Unauthorized(c, "unauthorized")
		return
	}

	issued := 0
	target := "specific"
	if req.UserID == 0 {
		ids, err := h.userRepo.ListActiveIDs()
		if err != nil {
			response.InternalError(c, "failed to load users")
			return
		}
		if len(ids) == 0 {
			response.BadRequest(c, "no active users")
			return
		}
		n, err := h.notifRepo.CreateBatch(ids, req.Title, req.Content, ntype, adminID)
		if err != nil {
			response.InternalError(c, "failed to send notifications")
			return
		}
		issued = n
		target = "all"
	} else {
		if err := h.notifRepo.Create(&model.Notification{
			UserID:   req.UserID,
			Title:    req.Title,
			Content:  req.Content,
			Type:     ntype,
			IssuedBy: adminID,
		}); err != nil {
			response.InternalError(c, "failed to send notification")
			return
		}
		issued = 1
	}

	response.Success(c, gin.H{
		"issued": issued,
		"target": target,
	})
}

// ListNotifications returns all notifications with pagination
// (GET /api/v1/admin/notifications).
func (h *AdminHandler) ListNotifications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	items, total, err := h.notifRepo.ListPaginated(page, size)
	if err != nil {
		response.InternalError(c, "failed to load notifications")
		return
	}
	userIDs := map[uint]string{}
	for _, n := range items {
		if _, ok := userIDs[n.UserID]; !ok {
			if u, err := h.userRepo.FindByID(n.UserID); err == nil {
				userIDs[n.UserID] = u.Email
			}
		}
	}
	out := make([]gin.H, len(items))
	for i, n := range items {
		out[i] = gin.H{
			"id":         n.ID,
			"user_id":    n.UserID,
			"user_email": userIDs[n.UserID],
			"title":      n.Title,
			"content":    n.Content,
			"type":       n.Type,
			"is_read":    n.IsRead,
			"created_at": n.CreatedAt,
		}
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": page, "size": size})
}

// ListTokenPackages returns all token packages including inactive ones
// (GET /api/v1/admin/token-packages).
func (h *AdminHandler) ListTokenPackages(c *gin.Context) {
	items, err := h.tokenPkgRepo.ListAll()
	if err != nil {
		response.InternalError(c, "failed to load token packages")
		return
	}
	response.Success(c, items)
}

// CreateTokenPackage creates a new token package (加油包)
// (POST /api/v1/admin/token-packages).
func (h *AdminHandler) CreateTokenPackage(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description string  `json:"description"`
		Tokens      int64   `json:"tokens" binding:"required"`
		BonusTokens int64   `json:"bonus_tokens"`
		Price       float64 `json:"price" binding:"required"`
		SortOrder   int     `json:"sort_order"`
		Status      string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Tokens <= 0 || req.Price <= 0 {
		response.BadRequest(c, "name, tokens and price are required")
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	pkg := &model.TokenPackage{
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		Tokens:      req.Tokens,
		BonusTokens: req.BonusTokens,
		Price:       decimal.RequireFromString(fmt.Sprintf("%.2f", req.Price)),
		Status:      status,
		SortOrder:   req.SortOrder,
	}
	if err := h.tokenPkgRepo.Create(pkg); err != nil {
		response.InternalError(c, "failed to create token package")
		return
	}
	response.Success(c, pkg)
}

// UpdateTokenPackage updates an existing token package
// (PUT /api/v1/admin/token-packages/:id).
func (h *AdminHandler) UpdateTokenPackage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid package id")
		return
	}
	pkg, err := h.tokenPkgRepo.FindByID(uint(id))
	if err != nil {
		response.BadRequest(c, "token package not found")
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Tokens      int64   `json:"tokens"`
		BonusTokens int64   `json:"bonus_tokens"`
		Price       float64 `json:"price"`
		SortOrder   int     `json:"sort_order"`
		Status      string  `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		pkg.Name = strings.TrimSpace(req.Name)
	}
	if req.Tokens > 0 {
		pkg.Tokens = req.Tokens
	}
	if req.Price > 0 {
		pkg.Price = decimal.RequireFromString(fmt.Sprintf("%.2f", req.Price))
	}
	pkg.Description = strings.TrimSpace(req.Description)
	pkg.BonusTokens = req.BonusTokens
	pkg.SortOrder = req.SortOrder
	if req.Status == "active" || req.Status == "inactive" {
		pkg.Status = req.Status
	}
	if err := h.tokenPkgRepo.Update(pkg); err != nil {
		response.InternalError(c, "failed to update token package")
		return
	}
	response.Success(c, pkg)
}

// DeleteTokenPackage removes a token package
// (DELETE /api/v1/admin/token-packages/:id).
func (h *AdminHandler) DeleteTokenPackage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid package id")
		return
	}
	if err := h.tokenPkgRepo.Delete(uint(id)); err != nil {
		response.InternalError(c, "failed to delete token package")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// UpdateTokenPackageStatus enables/disables a token package
// (PUT /api/v1/admin/token-packages/:id/status).
func (h *AdminHandler) UpdateTokenPackageStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid package id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if req.Status != "active" && req.Status != "inactive" {
		response.BadRequest(c, "status must be active or inactive")
		return
	}
	if err := h.tokenPkgRepo.SetStatus(uint(id), req.Status); err != nil {
		response.InternalError(c, "failed to update status")
		return
	}
	response.Success(c, gin.H{"status": req.Status})
}

// ---------------------------------------------------------------------------
// 发票审核（Invoice）
// ---------------------------------------------------------------------------

// ListInvoices returns all invoice applications with optional status filter
// (GET /api/v1/admin/invoices?status=pending&page=1&size=20).
func (h *AdminHandler) ListInvoices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := strings.TrimSpace(c.Query("status"))
	items, total, err := h.invoiceRepo.ListPaginated(page, size, status)
	if err != nil {
		response.InternalError(c, "failed to load invoices")
		return
	}
	userEmails := map[uint]string{}
	out := make([]gin.H, len(items))
	for i, inv := range items {
		if _, ok := userEmails[inv.UserID]; !ok {
			if u, err := h.userRepo.FindByID(inv.UserID); err == nil {
				userEmails[inv.UserID] = u.Email
			}
		}
		out[i] = gin.H{
			"id":            inv.ID,
			"user_id":       inv.UserID,
			"user_email":    userEmails[inv.UserID],
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
	response.Success(c, gin.H{"items": out, "total": total, "page": page, "size": size})
}

// IssueInvoice marks a pending invoice as issued with an invoice number
// (POST /api/v1/admin/invoices/:id/issue).
func (h *AdminHandler) IssueInvoice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid invoice id")
		return
	}
	var req struct {
		InvoiceNo string `json:"invoice_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "发票号码不能为空")
		return
	}
	req.InvoiceNo = strings.TrimSpace(req.InvoiceNo)
	if req.InvoiceNo == "" {
		response.BadRequest(c, "发票号码不能为空")
		return
	}
	inv, err := h.invoiceRepo.FindByID(uint(id))
	if err != nil {
		response.BadRequest(c, "发票申请不存在")
		return
	}
	if inv.Status != "pending" {
		response.BadRequest(c, "该申请已处理，无法重复操作")
		return
	}
	now := time.Now()
	if err := h.invoiceRepo.Issue(inv.ID, req.InvoiceNo, now); err != nil {
		response.InternalError(c, "failed to issue invoice")
		return
	}
	response.Success(c, gin.H{"id": inv.ID, "status": "issued", "invoice_no": req.InvoiceNo})
}

// RejectInvoice rejects a pending invoice application
// (POST /api/v1/admin/invoices/:id/reject).
func (h *AdminHandler) RejectInvoice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid invoice id")
		return
	}
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "驳回原因不能为空")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		response.BadRequest(c, "驳回原因不能为空")
		return
	}
	inv, err := h.invoiceRepo.FindByID(uint(id))
	if err != nil {
		response.BadRequest(c, "发票申请不存在")
		return
	}
	if inv.Status != "pending" {
		response.BadRequest(c, "该申请已处理，无法重复操作")
		return
	}
	if err := h.invoiceRepo.Reject(inv.ID, req.Reason); err != nil {
		response.InternalError(c, "failed to reject invoice")
		return
	}
	response.Success(c, gin.H{"id": inv.ID, "status": "rejected"})
}

// ---------------------------------------------------------------------------
// Token 授信审核（Credit）
// ---------------------------------------------------------------------------

// ListCreditApplications returns credit applications with optional status
// filter (GET /api/v1/admin/credit-applications?status=pending).
func (h *AdminHandler) ListCreditApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	status := strings.TrimSpace(c.Query("status"))
	items, total, err := h.creditRepo.ListPaginated(page, size, status)
	if err != nil {
		response.InternalError(c, "failed to load credit applications")
		return
	}
	out := make([]gin.H, len(items))
	for i, app := range items {
		email := ""
		if u, err := h.userRepo.FindByID(app.UserID); err == nil {
			email = u.Email
		}
		limit, used, _ := h.creditRepo.CreditState(app.UserID)
		available := limit - used
		if available < 0 {
			available = 0
		}
		out[i] = gin.H{
			"id":               app.ID,
			"user_id":          app.UserID,
			"user_email":       email,
			"status":           app.Status,
			"granted_tokens":   app.GrantedTokens,
			"reject_reason":    app.RejectReason,
			"consumed_total":   app.ConsumedTotal.StringFixed(2),
			"credit_limit":     limit,
			"credit_used":      used,
			"credit_available": available,
			"created_at":       app.CreatedAt,
			"reviewed_at":      app.ReviewedAt,
		}
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": page, "size": size})
}

// ApproveCreditApplication approves a pending application and grants the
// admin-specified token credit quota
// (POST /api/v1/admin/credit-applications/:id/approve).
func (h *AdminHandler) ApproveCreditApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid application id")
		return
	}
	var req struct {
		GrantedTokens int64 `json:"granted_tokens"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if req.GrantedTokens <= 0 {
		response.BadRequest(c, "授信 Token 额度必须大于 0")
		return
	}
	if err := h.creditRepo.ApproveAndGrant(uint(id), req.GrantedTokens); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.BadRequest(c, "该申请不存在或已处理")
			return
		}
		response.InternalError(c, "failed to approve credit application")
		return
	}
	response.Success(c, gin.H{"id": id, "status": "approved", "granted_tokens": req.GrantedTokens})
}

// RejectCreditApplication rejects a pending application with a reason
// (POST /api/v1/admin/credit-applications/:id/reject).
func (h *AdminHandler) RejectCreditApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid application id")
		return
	}
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "驳回原因不能为空")
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		response.BadRequest(c, "驳回原因不能为空")
		return
	}
	if err := h.creditRepo.Reject(uint(id), req.Reason); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.BadRequest(c, "该申请不存在或已处理")
			return
		}
		response.InternalError(c, "failed to reject credit application")
		return
	}
	response.Success(c, gin.H{"id": id, "status": "rejected"})
}

// ---------------------------------------------------------------------------
// 催账（Credit Collection）
// ---------------------------------------------------------------------------

// CollectCredit sends a collection notice to a user with outstanding credit
// (POST /api/v1/admin/credit-collect).
func (h *AdminHandler) CollectCredit(c *gin.Context) {
	adminID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}
	var req struct {
		UserID uint   `json:"user_id"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	user, err := h.userRepo.FindByID(req.UserID)
	if err != nil {
		response.BadRequest(c, "用户不存在")
		return
	}
	_ = user
	limit, used, err := h.creditRepo.CreditState(req.UserID)
	if err != nil {
		response.InternalError(c, "failed to load credit state")
		return
	}
	if used <= 0 {
		response.BadRequest(c, "该用户没有待还授信额度")
		return
	}
	note := strings.TrimSpace(req.Note)
	if note == "" {
		note = "请尽快购买加油包并在充值页归还授信额度"
	}
	content := fmt.Sprintf("您有 %d Tokens 授信额度待还（总额度 %d，已用 %d）。%s。逾期未还可能影响账户正常使用。",
		used, limit, used, note)
	if _, err := h.notifRepo.CreateBatch([]uint{req.UserID}, "授信催款通知", content, "credit", adminID); err != nil {
		response.InternalError(c, "failed to send collection notice")
		return
	}
	col := &model.CreditCollection{
		UserID:    req.UserID,
		AdminID:   adminID,
		TokensDue: used,
		Note:      note,
	}
	if err := h.creditRepo.AddCollection(col); err != nil {
		response.InternalError(c, "failed to record collection")
		return
	}
	response.Success(c, gin.H{"id": col.ID, "user_id": req.UserID, "tokens_due": used})
}

// ListCreditCollections returns collection records
// (GET /api/v1/admin/credit-collections?page=&size=).
func (h *AdminHandler) ListCreditCollections(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	items, total, err := h.creditRepo.ListCollections(page, size)
	if err != nil {
		response.InternalError(c, "failed to load collections")
		return
	}
	out := make([]gin.H, len(items))
	for i, col := range items {
		email := ""
		if u, err := h.userRepo.FindByID(col.UserID); err == nil {
			email = u.Email
		}
		out[i] = gin.H{
			"id":         col.ID,
			"user_id":    col.UserID,
			"user_email": email,
			"tokens_due": col.TokensDue,
			"note":       col.Note,
			"created_at": col.CreatedAt,
		}
	}
	response.Success(c, gin.H{"items": out, "total": total, "page": page, "size": size})
}

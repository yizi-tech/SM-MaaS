package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/mass-platform/backend/internal/api/dto"
	"github.com/mass-platform/backend/internal/auth"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/pkg/response"
)

// 桌面 GUI 开放接口：登录联动、套餐/加量包余量毫秒级同步、模型选择。
// 契约文档见 docs/api-desktop-gui.md。

// guiSubscriptionView is the GUI-facing snapshot of the user's active
// subscription quota (真实 token 口径，含缓存命中，与扣费口径一致).
type guiSubscriptionView struct {
	ID              uint     `json:"id"`
	PlanName        string   `json:"plan_name"`
	Status          string   `json:"status"`
	StartAt         string   `json:"start_at"`
	EndAt           string   `json:"end_at"`
	AutoRenew       bool     `json:"auto_renew"`
	Price           string   `json:"price"`
	IncludedTokens  int64    `json:"included_tokens"`
	UsedTokens      int64    `json:"used_tokens"`
	RemainingTokens int64    `json:"remaining_tokens"`
	RPM             int      `json:"rpm"`
	TPM             int      `json:"tpm"`
	ConcurrentLimit int      `json:"concurrent_limit"`
	ModelAccess     []string `json:"model_access"`
}

// guiModelView is one user-accessible model entry (prices in CNY per 1M).
type guiModelView struct {
	ID                 string   `json:"id"`
	Provider           string   `json:"provider"`
	Name               string   `json:"name"`
	Context            string   `json:"context"`
	InputPricePerM     string   `json:"input_price_per_m"`
	OutputPricePerM    string   `json:"output_price_per_m"`
	CacheReadPricePerM string   `json:"cache_read_price_per_m"`
	Features           []string `json:"features"`
	Status             string   `json:"status"`
}

// activeSubscription returns the first unexpired active subscription.
func (h *UserHandler) activeSubscription(userID uint) *model.Subscription {
	if h.subRepo == nil {
		return nil
	}
	subs, err := h.subRepo.FindActiveByUserID(userID)
	if err != nil {
		return nil
	}
	now := time.Now()
	for i := range subs {
		if subs[i].EndAt.After(now) {
			return &subs[i]
		}
	}
	return nil
}

// guiQuotaSnapshot assembles the millisecond-level quota sync payload.
// Only indexed reads (subscriptions by user + users row), no aggregation.
func (h *UserHandler) guiQuotaSnapshot(user *model.User) gin.H {
	var subView *guiSubscriptionView
	if sub := h.activeSubscription(user.ID); sub != nil {
		remaining := sub.IncludedTokens - sub.UsedTokens
		if remaining < 0 {
			remaining = 0
		}
		access := sub.Plan.ModelAccess
		if access == nil {
			access = []string{}
		}
		subView = &guiSubscriptionView{
			ID:              sub.ID,
			PlanName:        sub.Plan.Name,
			Status:          sub.Status,
			StartAt:         sub.StartAt.Format(time.RFC3339),
			EndAt:           sub.EndAt.Format(time.RFC3339),
			AutoRenew:       sub.AutoRenew,
			Price:           sub.Price.String(),
			IncludedTokens:  sub.IncludedTokens,
			UsedTokens:      sub.UsedTokens,
			RemainingTokens: remaining,
			RPM:             sub.Plan.RPM,
			TPM:             sub.Plan.TPM,
			ConcurrentLimit: sub.Plan.ConcurrentLimit,
			ModelAccess:     access,
		}
	}

	creditLimit, creditUsed := int64(0), int64(0)
	if h.creditRepo != nil {
		if l, u, err := h.creditRepo.CreditState(user.ID); err == nil {
			creditLimit, creditUsed = l, u
		}
	}

	return gin.H{
		"server_time":   time.Now().UnixMilli(),
		"subscription":  subView,
		"token_credits": user.TokenCredits,
		"balance":       user.Balance.String(),
		"credit": gin.H{
			"limit":     creditLimit,
			"used":      creditUsed,
			"available": creditLimit - creditUsed,
		},
	}
}

// guiModelCatalog builds the user-scoped model list: priced AND channel-available
// AND (if the user has a subscription with a non-empty model_access allowlist)
// covered by that allowlist (wildcard "*" supported). Only 1-2 indexed queries.
func (h *UserHandler) guiModelCatalog(user *model.User) ([]guiModelView, string) {
	perMillion := decimal.NewFromInt(1_000_000)
	formatPrice := func(v decimal.Decimal) string {
		return "¥" + v.Mul(perMillion).Round(4).String()
	}

	// Channel availability.
	var enabled []model.LLMChannel
	if h.channelRepo != nil {
		list, err := h.channelRepo.ListEnabled()
		if err == nil {
			enabled = list
		}
	}

	// Active subscription allowlist (empty = all models).
	var allowlist []string
	if sub := h.activeSubscription(user.ID); sub != nil {
		allowlist = sub.Plan.ModelAccess
	}
	allowed := func(name string) bool {
		if len(allowlist) == 0 {
			return true
		}
		for _, m := range allowlist {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if strings.HasSuffix(m, "*") {
				if strings.HasPrefix(name, strings.TrimSuffix(m, "*")) {
					return true
				}
			} else if m == name {
				return true
			}
		}
		return false
	}

	priced := make(map[string]model.ModelPrice)
	if h.billingService != nil {
		for _, p := range h.billingService.ListEnabledPrices() {
			if _, dup := priced[p.Model]; !dup {
				priced[p.Model] = p
			}
		}
	}

	served := make(map[string]bool)
	var out []guiModelView
	appendModel := func(id string, m model.ModelPrice, catalogEntry model.ModelCatalogEntry) {
		if served[id] || !allowed(id) {
			return
		}
		channelOK := len(enabled) == 0
		for i := range enabled {
			if enabled[i].MatchesModel(id) {
				channelOK = true
				break
			}
		}
		if !channelOK {
			return
		}
		cacheRead := m.InputPrice.Mul(decimal.RequireFromString("0.1"))
		if m.CacheReadPrice.Valid {
			cacheRead = m.CacheReadPrice.Decimal
		}
		features := catalogEntry.Features
		if features == nil {
			features = []string{}
		}
		served[id] = true
		out = append(out, guiModelView{
			ID:                 id,
			Provider:           catalogEntry.Provider,
			Name:               catalogEntry.Name,
			Context:            catalogEntry.Context,
			InputPricePerM:     formatPrice(m.InputPrice),
			OutputPricePerM:    formatPrice(m.OutputPrice),
			CacheReadPricePerM: formatPrice(cacheRead),
			Features:           features,
			Status:             "available",
		})
	}

	for _, e := range model.GetModelCatalog() {
		p, ok := priced[e.ID]
		if !ok {
			continue
		}
		appendModel(e.ID, p, e)
	}
	// Concrete channel models missing from the built-in catalog.
	for i := range enabled {
		ch := &enabled[i]
		for _, mm := range ch.Models {
			name := strings.TrimSpace(mm)
			if name == "" || strings.HasSuffix(name, "*") || served[name] {
				continue
			}
			p, ok := priced[name]
			if !ok {
				continue
			}
			appendModel(name, p, model.ModelCatalogEntry{
				ID:       name,
				Provider: ch.Type,
				Name:     name,
				Context:  "-",
				Features: []string{"按量计费"},
			})
		}
	}

	defaultModel := ""
	if len(out) > 0 {
		defaultModel = out[0].ID
	}
	// Prefer the first concrete (non-wildcard) allowlist entry as default.
	for _, m := range allowlist {
		if m == "" || strings.HasSuffix(m, "*") {
			continue
		}
		if served[m] {
			defaultModel = m
			break
		}
	}
	return out, defaultModel
}

// GUILogin signs in a user and returns the full session snapshot
// (token + user + quota + models) in one round trip.
// POST /api/v1/gui/login
func (h *UserHandler) GUILogin(c *gin.Context) {
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

	models, defaultModel := h.guiModelCatalog(user)
	response.Success(c, gin.H{
		"token": token,
		"user":  toUserInfo(user),
		"quota": h.guiQuotaSnapshot(user),
		"models": gin.H{
			"default_model": defaultModel,
			"models":        models,
		},
	})
}

// GUISession refreshes the full session snapshot for an existing token.
// GET /api/v1/gui/session
func (h *UserHandler) GUISession(c *gin.Context) {
	user, ok := h.guiCurrentUser(c)
	if !ok {
		return
	}
	models, defaultModel := h.guiModelCatalog(user)
	response.Success(c, gin.H{
		"user":  toUserInfo(user),
		"quota": h.guiQuotaSnapshot(user),
		"models": gin.H{
			"default_model": defaultModel,
			"models":        models,
		},
	})
}

// GUISync returns the lightweight quota snapshot for millisecond-level sync.
// GET /api/v1/gui/sync
func (h *UserHandler) GUISync(c *gin.Context) {
	user, ok := h.guiCurrentUser(c)
	if !ok {
		return
	}
	quota := h.guiQuotaSnapshot(user)
	quota["server_time"] = time.Now().UnixMilli()
	response.Success(c, quota)
}

// GUIModels returns the models the current user can actually invoke.
// GET /api/v1/gui/models
func (h *UserHandler) GUIModels(c *gin.Context) {
	user, ok := h.guiCurrentUser(c)
	if !ok {
		return
	}
	models, defaultModel := h.guiModelCatalog(user)
	response.Success(c, gin.H{
		"default_model": defaultModel,
		"models":        models,
	})
}

// guiCurrentUser loads the authenticated user or writes the error response.
func (h *UserHandler) guiCurrentUser(c *gin.Context) (*model.User, bool) {
	userID, ok := getUserID(c)
	if !ok {
		response.Unauthorized(c, "user not authenticated")
		return nil, false
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		response.Unauthorized(c, "user not found")
		return nil, false
	}
	return user, true
}

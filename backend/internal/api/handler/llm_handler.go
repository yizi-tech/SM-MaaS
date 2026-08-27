package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/mass-platform/backend/internal/billing"
	"github.com/mass-platform/backend/internal/llm/provider"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/monitor"
	"github.com/mass-platform/backend/internal/rate"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/mass-platform/backend/pkg/response"
)

// LLMHandler handles LLM API gateway requests.
type LLMHandler struct {
	providerFactory *provider.ProviderFactory
	channelRepo     *repository.ChannelRepository
	rateLimiter     *rate.RateLimiter
	billingService  *billing.BillingService
	apiKeyRepo      *repository.ApiKeyRepository
	userRepo        *repository.UserRepository
	convoRepo       *repository.ConversationLogRepository
}

// NewLLMHandler creates a new LLMHandler.
func NewLLMHandler(
	pf *provider.ProviderFactory,
	channelRepo *repository.ChannelRepository,
	rl *rate.RateLimiter,
	bs *billing.BillingService,
	akr *repository.ApiKeyRepository,
	ur *repository.UserRepository,
	cr *repository.ConversationLogRepository,
) *LLMHandler {
	return &LLMHandler{
		providerFactory: pf,
		channelRepo:     channelRepo,
		rateLimiter:     rl,
		billingService:  bs,
		apiKeyRepo:      akr,
		userRepo:        ur,
		convoRepo:       cr,
	}
}

// resolveBilling determines how a request should be billed: if the user has an
// active subscription whose plan grants access to the model, usage is charged
// to the subscription; otherwise it is pay-per-use.
func (h *LLMHandler) resolveBilling(userID uint, modelName string) (model.BillingType, *uint) {
	sub := h.billingService.ResolveSubscriptionForModel(userID, modelName)
	if sub == nil {
		return model.BillingPayPerUse, nil
	}
	return model.BillingSubscription, &sub.ID
}

// resolveBillingUnlimited resolves billing the same way as resolveBilling but
// upgrades to the unlimited-firepower perk when the model has the promo enabled
// and the user holds a paid active subscription. When the user is not eligible
// (or the promo is off) it falls back to the normal billing resolution, so the
// model remains fully usable and billed as usual.
func (h *LLMHandler) resolveBillingUnlimited(userID uint, modelName string) (model.BillingType, *uint) {
	if subID, ok := h.billingService.IsUnlimitedFirepower(userID, modelName); ok {
		return model.BillingUnlimited, subID
	}
	return h.resolveBilling(userID, modelName)
}

// resolveProviderForModel picks the provider for a model. Enabled DB channels
// are preferred (highest priority first); the static env-configured providers
// are used as a fallback when no channel matches.
func (h *LLMHandler) resolveProviderForModel(ctx context.Context, model string) (provider.Provider, string) {
	if h.channelRepo != nil {
		channels, err := h.channelRepo.ListEnabled()
		if err == nil {
			for i := range channels {
				ch := channels[i]
				if !ch.MatchesModel(model) {
					continue
				}
				switch ch.Type {
				case "anthropic":
					return provider.NewAnthropicProvider(ch.BaseURL, ch.APIKey), ch.Name
				default:
					return provider.NewOpenAIProvider(ch.BaseURL, ch.APIKey), ch.Name
				}
			}
		}
	}
	providerName := resolveProvider(model)
	p, ok := h.providerFactory.Get(providerName)
	if !ok {
		return nil, ""
	}
	return p, providerName
}

// resolveProvider maps a model name to its provider name.
func resolveProvider(model string) string {
	model = strings.TrimSpace(model)
	switch {
	case strings.HasPrefix(model, "gpt"):
		return "openai"
	case strings.HasPrefix(model, "text-"):
		return "openai"
	case strings.HasPrefix(model, "davinci"):
		return "openai"
	case strings.HasPrefix(model, "o1"):
		return "openai"
	case strings.HasPrefix(model, "claude"):
		return "anthropic"
	default:
		return "openai"
	}
}

// getRequestID extracts the request ID from the gin context.
func getRequestID(c *gin.Context) string {
	if rid, exists := c.Get("request_id"); exists {
		if s, ok := rid.(string); ok {
			return s
		}
	}
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

// getAPIKeyFromContext extracts the API key info from the gin context.
func getAPIKeyFromContext(c *gin.Context) (*model.ApiKey, bool) {
	raw, exists := c.Get("api_key")
	if !exists {
		return nil, false
	}
	key, ok := raw.(*model.ApiKey)
	return key, ok
}

// getUserIDFromContext extracts the user ID from the gin context.
func getUserIDFromContext(c *gin.Context) (uint, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := raw.(uint)
	return id, ok
}

// getAPIKeyIDFromContext extracts the API key ID from the gin context.
func getAPIKeyIDFromContext(c *gin.Context) (*uint, bool) {
	raw, exists := c.Get("api_key_id")
	if !exists {
		return nil, false
	}
	id, ok := raw.(uint)
	if !ok {
		return nil, false
	}
	return &id, true
}

// ChatCompletions handles chat completion requests (POST /v1/chat/completions).

// hasFundsFor reports whether the user can pay for a pay-per-use chat:
// an active subscription covering the model, a positive balance, token
// credits, or remaining credit limit.
func (h *LLMHandler) hasFundsFor(userID uint, modelName string) bool {
	if h.billingService.ResolveSubscriptionForModel(userID, modelName) != nil {
		return true
	}
	if h.userRepo == nil {
		return false
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return false
	}
	return user.Balance.GreaterThan(decimal.Zero) ||
		user.TokenCredits > 0 ||
		user.CreditLimit-user.CreditUsed > 0
}

// blockIfUnfunded rejects a request the caller cannot pay for:
//   - pay-per-use callers need balance, token credits or remaining credit limit;
//   - subscription callers need remaining quota, or a balance/credit fallback so
//     any quota overflow is still billed (otherwise an exhausted subscription with
//     zero balance would roll the billing transaction back and ride for free).
//
// Returns true after writing a 402 response; the caller must not proceed.
func (h *LLMHandler) blockIfUnfunded(c *gin.Context, userID uint, modelName, providerName string, billingType model.BillingType, subID *uint) bool {
	switch billingType {
	case model.BillingPayPerUse:
		if h.hasFundsFor(userID, modelName) {
			return false
		}
	case model.BillingSubscription:
		if subID != nil && h.subscriptionCovers(userID, *subID) {
			return false
		}
	case model.BillingUnlimited:
		// Perk covers the cost; never block.
		return false
	default:
		return false
	}
	monitor.LLMRequestTotal.WithLabelValues(modelName, providerName, "402").Inc()
	response.ErrorWithData(c, http.StatusPaymentRequired, "余额不足或订阅额度已用完，请充值/续费后再试", gin.H{"need": 1})
	return true
}

// subscriptionCovers reports whether the subscription can pay for this request:
// it still has quota remaining, or the user has a balance/credit fallback so a
// quota overflow is billed instead of silently rolling back for free.
func (h *LLMHandler) subscriptionCovers(userID uint, subID uint) bool {
	sub := h.billingService.GetSubscription(subID)
	if sub == nil {
		return false
	}
	if sub.IncludedTokens-sub.UsedTokens > 0 {
		return true
	}
	// Quota exhausted: only safe if a balance/credit fallback exists. Check the
	// balance/credit directly (NOT hasFundsFor, which would re-detect this very
	// subscription and erroneously report "covered").
	return h.hasBalanceOrCredit(userID)
}

// hasBalanceOrCredit reports whether the user can pay via balance, token
// credits or remaining credit limit — without considering any subscription.
func (h *LLMHandler) hasBalanceOrCredit(userID uint) bool {
	if h.userRepo == nil {
		return false
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return false
	}
	return user.Balance.GreaterThan(decimal.Zero) ||
		user.TokenCredits > 0 ||
		user.CreditLimit-user.CreditUsed > 0
}

// saveConversation persists a chat request/response pair for data retention
// and JSONL export. Failures are logged but never block the API response.
func (h *LLMHandler) saveConversation(
	userID uint, apiKeyID *uint, requestID, modelName string,
	messages []provider.Message, choices []provider.Choice,
	cost decimal.Decimal, stream bool, tokensIn, tokensOut, tokensCached int,
) {
	if h.convoRepo == nil {
		return
	}
	msgsJSON, err := json.Marshal(messages)
	if err != nil {
		return
	}
	var content, finishReason string
	if len(choices) > 0 {
		content = choices[0].Message.Content.Text()
		finishReason = choices[0].FinishReason
	}
	respJSON, _ := json.Marshal(map[string]interface{}{
		"content":       content,
		"finish_reason": finishReason,
		"stream":        stream,
		"tokens_in":     tokensIn,
		"tokens_out":    tokensOut,
		"tokens_cached": tokensCached,
	})
	log := &model.ConversationLog{
		UserID:       userID,
		ApiKeyID:     apiKeyID,
		RequestID:    requestID,
		Model:        modelName,
		Messages:     string(msgsJSON),
		Response:     string(respJSON),
		TokensIn:     tokensIn,
		TokensOut:    tokensOut,
		TokensCached: tokensCached,
		Cost:         cost,
		Stream:       stream,
		Status:       "success",
	}
	if err := h.convoRepo.Create(log); err != nil {
		logging.Error("llm_handler", "conversation_log", "failed to save conversation", err,
			map[string]interface{}{"request_id": requestID, "user_id": userID})
	}
}

// checkKeyModelAccess enforces the API key's model allowlist. An empty
// ModelAccess means the key may access all models.
func (h *LLMHandler) checkKeyModelAccess(apiKey *model.ApiKey, modelName string) bool {
	if apiKey == nil || len(apiKey.ModelAccess) == 0 {
		return true
	}
	for _, m := range apiKey.ModelAccess {
		if m == modelName || m == "*" {
			return true
		}
	}
	return false
}

// Platform-wide default limits applied to every request that has no more
// specific configuration (API key rate limit or an active plan covering the
// model), so no request is ever unthrottled.
const (
	defaultRateLimitRPM        = 60
	defaultRateLimitTPM        = 100000
	defaultRateLimitConcurrent = 10
)

// effectiveRateLimits resolves the rate limits for a request. Priority:
// 1) API key's attached RateLimit; 2) the active subscription plan covering
// the model; 3) platform defaults. Returns zeroed-out limits only when every
// source is disabled.
func (h *LLMHandler) effectiveRateLimits(apiKey *model.ApiKey, userID uint, modelName string) rate.RateLimitConfig {
	cfg := rate.RateLimitConfig{
		RPM:             defaultRateLimitRPM,
		TPM:             defaultRateLimitTPM,
		ConcurrentLimit: defaultRateLimitConcurrent,
	}
	if apiKey != nil && apiKey.RateLimit != nil {
		rl := apiKey.RateLimit
		cfg = rate.RateLimitConfig{RPM: rl.RPM, TPM: rl.TPM, ConcurrentLimit: rl.ConcurrentLimit}
		return cfg
	}
	if h.billingService != nil {
		if sub := h.billingService.ResolveSubscriptionForModel(userID, modelName); sub != nil {
			cfg = rate.RateLimitConfig{
				RPM:             sub.Plan.RPM,
				TPM:             sub.Plan.TPM,
				ConcurrentLimit: sub.Plan.ConcurrentLimit,
			}
		}
	}
	return cfg
}

// enforceRateLimits applies RPM / TPM / concurrent limits for one request.
// On success it returns the release function that MUST be deferred so the
// concurrent slot is held for the whole call (streaming included). On limit
// violation it reports the 429 through onLimit and returns ok=false.
func (h *LLMHandler) enforceRateLimits(
	c *gin.Context,
	apiKey *model.ApiKey,
	userID uint,
	modelName, providerName string,
	promptTokens int,
	onLimit func(msg string),
) (release func(), ok bool) {
	if h.rateLimiter == nil {
		return func() {}, true
	}

	rl := h.effectiveRateLimits(apiKey, userID, modelName)
	rlKey := rate.GetRateLimitKey(userID, modelName)
	ctx := c.Request.Context()

	if rl.RPM > 0 {
		allowed, err := h.rateLimiter.CheckRPM(ctx, rlKey, rl.RPM)
		if err != nil {
			logging.Error("llm_handler", "rate_limit", "RPM check failed", err,
				map[string]interface{}{"request_id": getRequestID(c), "user_id": userID, "model": modelName})
			// Fail closed: when the limiter cannot make a decision (e.g. Redis
			// unreachable) we reject rather than let traffic through unbounded.
			onLimit("rate limiter unavailable, request rejected")
			return nil, false
		} else if !allowed {
			monitor.LLMRequestTotal.WithLabelValues(modelName, providerName, "429").Inc()
			logging.Warn("llm_handler", "rate_limit", "RPM limit exceeded",
				map[string]interface{}{
					"request_id": getRequestID(c), "user_id": userID, "model": modelName, "limit": rl.RPM,
				})
			onLimit("rate limit exceeded: too many requests per minute")
			return nil, false
		}
	}

	if rl.TPM > 0 && promptTokens > 0 {
		allowed, err := h.rateLimiter.CheckTPM(ctx, rlKey, rl.TPM, promptTokens)
		if err != nil {
			logging.Error("llm_handler", "rate_limit", "TPM check failed", err,
				map[string]interface{}{"request_id": getRequestID(c), "user_id": userID, "model": modelName})
			onLimit("rate limiter unavailable, request rejected")
			return nil, false
		} else if !allowed {
			monitor.LLMRequestTotal.WithLabelValues(modelName, providerName, "429").Inc()
			logging.Warn("llm_handler", "rate_limit", "TPM limit exceeded",
				map[string]interface{}{
					"request_id": getRequestID(c), "user_id": userID, "model": modelName,
					"limit": rl.TPM, "estimated_tokens": promptTokens,
				})
			onLimit("rate limit exceeded: too many tokens per minute")
			return nil, false
		}
	}

	if rl.ConcurrentLimit > 0 {
		allowed, err := h.rateLimiter.AcquireConcurrent(ctx, rlKey, rl.ConcurrentLimit)
		if err != nil {
			logging.Error("llm_handler", "rate_limit", "concurrent check failed", err,
				map[string]interface{}{"request_id": getRequestID(c), "user_id": userID, "model": modelName})
			onLimit("rate limiter unavailable, request rejected")
			return nil, false
		} else if !allowed {
			monitor.LLMRequestTotal.WithLabelValues(modelName, providerName, "429").Inc()
			logging.Warn("llm_handler", "rate_limit", "concurrent limit exceeded",
				map[string]interface{}{
					"request_id": getRequestID(c), "user_id": userID, "model": modelName, "limit": rl.ConcurrentLimit,
				})
			onLimit("rate limit exceeded: too many concurrent requests")
			return nil, false
		} else {
			return func() { h.rateLimiter.ReleaseConcurrent(context.Background(), rlKey) }, true
		}
	}

	return func() {}, true
}

// estimateChatPromptTokens is a cheap, deterministic estimate of the prompt
// tokens a chat request will consume, used for TPM accounting before the
// upstream call returns real usage figures. The configured completion budget
// (max_tokens) is counted up-front so the TPM window also bounds output.
func estimateChatPromptTokens(messages []provider.Message, maxTokens int) int {
	toks := 4 // role / template overhead
	for _, m := range messages {
		toks += len(m.Content.Text())/4 + 4
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	return toks + maxTokens
}

// estimateCompletionPromptTokens is the CompletionRequest (text-in/text-out)
// equivalent of estimateChatPromptTokens.
func estimateCompletionPromptTokens(prompt string, maxTokens int) int {
	toks := 4 + len(prompt)/4
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	return toks + maxTokens
}

func (h *LLMHandler) ChatCompletions(c *gin.Context) {
	requestID := getRequestID(c)

	// Get API key and user info from context
	apiKey, keyExists := getAPIKeyFromContext(c)
	userID, userExists := getUserIDFromContext(c)
	if !userExists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	// Parse request body
	var req provider.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.Error("llm_handler", "chat_completions", "failed to parse request body", err,
			map[string]interface{}{"request_id": requestID})
		response.BadRequest(c, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Model == "" {
		response.BadRequest(c, "model is required")
		return
	}
	if len(req.Model) > 100 {
		response.BadRequest(c, "model name too long")
		return
	}

	if keyExists && !h.checkKeyModelAccess(apiKey, req.Model) {
		response.Error(c, http.StatusForbidden, "model not allowed for this api key")
		return
	}

	// Determine provider (DB channel preferred, env-configured fallback)
	llmProvider, providerName := h.resolveProviderForModel(c.Request.Context(), req.Model)
	if llmProvider == nil {
		logging.Error("llm_handler", "chat_completions", "provider not found", nil,
			map[string]interface{}{
				"request_id": requestID,
				"model":      req.Model,
			})
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("no provider available for model: %s", req.Model))
		return
	}

	// Model price allowlist: reject unpriced models before forwarding/billing.
	if err := h.billingService.EnsureModelPriced(req.Model); err != nil {
		logging.Warn("llm_handler", "chat_completions", "unpriced model rejected",
			map[string]interface{}{"request_id": requestID, "model": req.Model})
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("model pricing not configured: %s", req.Model))
		return
	}

	logging.Info("llm_handler", "chat_completions", "processing request",
		map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"model":      req.Model,
			"provider":   providerName,
			"stream":     req.Stream,
		})

	// Rate limiting (API key limits take precedence; otherwise the active
	// plan's limits apply; unsubscribed callers get platform defaults).
	release, ok := h.enforceRateLimits(c, apiKey, userID, req.Model, providerName,
		estimateChatPromptTokens(req.Messages, req.MaxTokens),
		func(msg string) {
			response.Error(c, http.StatusTooManyRequests, msg)
		})
	if !ok {
		return
	}
	defer release()

	// Determine billing type
	var apiKeyID *uint
	if keyExists {
		id, _ := getAPIKeyIDFromContext(c)
		apiKeyID = id
	}
	billingType, subID := h.resolveBillingUnlimited(userID, req.Model)

	// Pay-per-use callers need funds on hand; block before touching upstream.
	if h.blockIfUnfunded(c, userID, req.Model, providerName, billingType, subID) {
		return
	}

	// Handle streaming
	if req.Stream {
		h.handleChatStream(c, llmProvider, &req, providerName, userID, requestID, apiKeyID, billingType, subID)
		return
	}

	// Non-streaming
	callStarted := time.Now()
	resp, err := llmProvider.ChatCompletion(c.Request.Context(), &req)
	if err != nil {
		monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
		logging.Error("llm_handler", "chat_completions", "provider request failed", err,
			map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"model":      req.Model,
				"provider":   providerName,
			})
		// Do not forward the raw upstream error to the client: it may reveal
		// upstream vendor identity, error details or internal URLs. Return a
		// generic message carrying only the request_id for tracing.
		response.Error(c, http.StatusBadGateway, fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID))
		return
	}

	// Record billing
	if resp.Usage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			c.Request.Context(),
			userID,
			requestID,
			req.Model,
			providerName,
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			resp.Usage.CachedTokenCount(),
			resp.Usage.CacheWriteTokenCount(),
			billingType,
			subID,
			apiKeyID,
			0,
			time.Since(callStarted).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record usage", err,
				map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"model":      req.Model,
				})
		} else {
			h.saveConversation(userID, apiKeyID, requestID, req.Model, req.Messages,
				resp.Choices,
				rec.Cost, false, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokenCount())
		}
	}

	// Track metrics
	monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "success").Inc()
	monitor.LLMTokenTotal.WithLabelValues(req.Model, "prompt").Add(float64(resp.Usage.PromptTokens))
	monitor.LLMTokenTotal.WithLabelValues(req.Model, "completion").Add(float64(resp.Usage.CompletionTokens))

	response.Success(c, resp)
}

// UserChatCompletions serves chat completions for the console 对话测试 page.
// The caller is authenticated by JWT (not an API key) and is billed directly:
// active subscription quota first, then balance / token credits. Streaming
// responses reuse the same SSE pipeline as the gateway.
// (POST /api/v1/user/chat/completions).
func (h *LLMHandler) UserChatCompletions(c *gin.Context) {
	requestID := getRequestID(c)
	userID, userExists := getUserIDFromContext(c)
	if !userExists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var req provider.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	if req.Model == "" {
		response.BadRequest(c, "model is required")
		return
	}
	if len(req.Model) > 100 {
		response.BadRequest(c, "model name too long")
		return
	}
	if len(req.Messages) == 0 {
		response.BadRequest(c, "messages is required")
		return
	}

	llmProvider, providerName := h.resolveProviderForModel(c.Request.Context(), req.Model)
	if llmProvider == nil {
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("no provider available for model: %s", req.Model))
		return
	}
	if err := h.billingService.EnsureModelPriced(req.Model); err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("model pricing not configured: %s", req.Model))
		return
	}

	logging.Info("llm_handler", "user_chat_completions", "processing console chat request",
		map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"model":      req.Model,
			"provider":   providerName,
			"stream":     req.Stream,
		})

	// Rate limiting (JWT console calls: plan limits or platform defaults).
	release, ok := h.enforceRateLimits(c, nil, userID, req.Model, providerName,
		estimateChatPromptTokens(req.Messages, req.MaxTokens),
		func(msg string) {
			response.Error(c, http.StatusTooManyRequests, msg)
		})
	if !ok {
		return
	}
	defer release()

	billingType, subID := h.resolveBillingUnlimited(userID, req.Model)

	if h.blockIfUnfunded(c, userID, req.Model, providerName, billingType, subID) {
		return
	}

	if req.Stream {
		h.handleChatStream(c, llmProvider, &req, providerName, userID, requestID, nil, billingType, subID)
		return
	}

	// Non-streaming
	callStarted := time.Now()
	resp, err := llmProvider.ChatCompletion(c.Request.Context(), &req)
	if err != nil {
		logging.Error("llm_handler", "user_chat_completions", "provider request failed", err,
			map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model})
		response.Error(c, http.StatusBadGateway, "upstream service error, please try again later")
		return
	}

	if resp.Usage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			c.Request.Context(), userID, requestID, req.Model, providerName,
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens,
			resp.Usage.CachedTokenCount(), resp.Usage.CacheWriteTokenCount(),
			billingType, subID, nil,
			0, time.Since(callStarted).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record console chat usage", err,
				map[string]interface{}{"request_id": requestID, "user_id": userID, "model": req.Model})
		} else {
			h.saveConversation(userID, nil, requestID, req.Model, req.Messages, resp.Choices,
				rec.Cost, false, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokenCount())
		}
	}

	response.Success(c, resp)
}

// handleChatStream handles streaming chat completion requests via SSE.
func (h *LLMHandler) handleChatStream(
	c *gin.Context,
	llmProvider provider.Provider,
	req *provider.ChatCompletionRequest,
	providerName string,
	userID uint,
	requestID string,
	apiKeyID *uint,
	billingType model.BillingType,
	subID *uint,
) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Flush headers immediately
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	startedAt := time.Now()
	streamCh, err := llmProvider.ChatCompletionStream(c.Request.Context(), req)
	if err != nil {
		monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
		logging.Error("llm_handler", "chat_stream", "failed to start stream", err,
			map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"model":      req.Model,
				"provider":   providerName,
			})

		errData, _ := json.Marshal(map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID),
				"type":    "internal_error",
			},
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
		c.Writer.Flush()
		return
	}

	// Track last usage for billing
	// Track last usage for billing and accumulate streamed content
	var lastUsage *provider.Usage
	var streamContent strings.Builder
	var ttftMs int64

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logging.Error("llm_handler", "chat_stream", "response writer does not support flushing", nil,
			map[string]interface{}{"request_id": requestID})
		return
	}

	for event := range streamCh {
		// Time to first upstream chunk (TTFT): only count chunks carrying data
		// (skips empty/keepalive events emitted right after connect).
		if ttftMs == 0 && len(event.Data) > 0 {
			ttftMs = time.Since(startedAt).Milliseconds()
		}
		// Check for client disconnect
		select {
		case <-c.Request.Context().Done():
			logging.Info("llm_handler", "chat_stream", "client disconnected",
				map[string]interface{}{"request_id": requestID, "user_id": userID})
			return
		default:
		}

		if event.Error != nil {
			monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
			logging.Error("llm_handler", "chat_stream", "stream error", event.Error,
				map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"model":      req.Model,
				})
			errData, _ := json.Marshal(map[string]interface{}{
				"error": map[string]string{
					"message": fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID),
					"type":    "stream_error",
				},
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
			flusher.Flush()
			return
		}

		if event.Done {
			break
		}

		// Parse the chunk to extract usage info from the last data event
		if len(event.Data) > 0 {
			var chunk struct {
				Usage   *provider.Usage `json:"usage,omitempty"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(event.Data, &chunk); err == nil {
				if chunk.Usage != nil {
					lastUsage = chunk.Usage
				}
				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta
					if delta.Content != "" {
						streamContent.WriteString(delta.Content)
					} else if delta.ReasoningContent != "" {
						streamContent.WriteString(delta.ReasoningContent)
					}
				}
			}
		}

		// Write SSE data event
		fmt.Fprintf(c.Writer, "data: %s\n\n", event.Data)
		flusher.Flush()
	}

	// Send stream done signal
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()

	// Record billing after stream completes
	if lastUsage != nil && lastUsage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			context.Background(),
			userID,
			requestID,
			req.Model,
			providerName,
			lastUsage.PromptTokens,
			lastUsage.CompletionTokens,
			lastUsage.CachedTokenCount(),
			lastUsage.CacheWriteTokenCount(),
			billingType,
			subID,
			apiKeyID,
			ttftMs,
			time.Since(startedAt).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record stream usage", err,
				map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"model":      req.Model,
				})
		} else {
			content := streamContent.String()
			h.saveConversation(userID, apiKeyID, requestID, req.Model, req.Messages,
				[]provider.Choice{{Message: provider.Message{Role: "assistant", Content: provider.ContentString(content)}}},
				rec.Cost, true, lastUsage.PromptTokens, lastUsage.CompletionTokens, lastUsage.CachedTokenCount())
		}
		monitor.LLMTokenTotal.WithLabelValues(req.Model, "prompt").Add(float64(lastUsage.PromptTokens))
		monitor.LLMTokenTotal.WithLabelValues(req.Model, "completion").Add(float64(lastUsage.CompletionTokens))
	}

	monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "success").Inc()
}

// Completions handles text completion requests (POST /v1/completions).
func (h *LLMHandler) Completions(c *gin.Context) {
	requestID := getRequestID(c)

	// Get API key and user info from context
	apiKey, keyExists := getAPIKeyFromContext(c)
	userID, userExists := getUserIDFromContext(c)
	if !userExists {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	// Parse request body
	var req provider.CompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.Error("llm_handler", "completions", "failed to parse request body", err,
			map[string]interface{}{"request_id": requestID})
		response.BadRequest(c, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Model == "" {
		response.BadRequest(c, "model is required")
		return
	}
	if len(req.Model) > 100 {
		response.BadRequest(c, "model name too long")
		return
	}

	if keyExists && !h.checkKeyModelAccess(apiKey, req.Model) {
		response.Error(c, http.StatusForbidden, "model not allowed for this api key")
		return
	}

	// Determine provider (DB channel preferred, env-configured fallback)
	llmProvider, providerName := h.resolveProviderForModel(c.Request.Context(), req.Model)
	if llmProvider == nil {
		logging.Error("llm_handler", "completions", "provider not found", nil,
			map[string]interface{}{
				"request_id": requestID,
				"model":      req.Model,
			})
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("no provider available for model: %s", req.Model))
		return
	}

	// Model price allowlist: reject unpriced models before forwarding/billing.
	if err := h.billingService.EnsureModelPriced(req.Model); err != nil {
		logging.Warn("llm_handler", "completions", "unpriced model rejected",
			map[string]interface{}{"request_id": requestID, "model": req.Model})
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("model pricing not configured: %s", req.Model))
		return
	}

	logging.Info("llm_handler", "completions", "processing request",
		map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"model":      req.Model,
			"provider":   providerName,
			"stream":     req.Stream,
		})

	// Rate limiting (API key limits take precedence; otherwise the active
	// plan's limits apply; unsubscribed callers get platform defaults).
	release, ok := h.enforceRateLimits(c, apiKey, userID, req.Model, providerName,
		estimateCompletionPromptTokens(req.Prompt, req.MaxTokens),
		func(msg string) {
			response.Error(c, http.StatusTooManyRequests, msg)
		})
	if !ok {
		return
	}
	defer release()

	// Determine billing type
	var apiKeyID *uint
	if keyExists {
		id, _ := getAPIKeyIDFromContext(c)
		apiKeyID = id
	}
	billingType, subID := h.resolveBillingUnlimited(userID, req.Model)

	// Pay-per-use callers need funds on hand; block before touching upstream.
	if h.blockIfUnfunded(c, userID, req.Model, providerName, billingType, subID) {
		return
	}

	// Handle streaming
	if req.Stream {
		h.handleCompletionStream(c, llmProvider, &req, providerName, userID, requestID, apiKeyID, billingType, subID)
		return
	}

	// Non-streaming
	callStarted := time.Now()
	resp, err := llmProvider.Completion(c.Request.Context(), &req)
	if err != nil {
		monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
		logging.Error("llm_handler", "completions", "provider request failed", err,
			map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"model":      req.Model,
				"provider":   providerName,
			})
		response.Error(c, http.StatusBadGateway, fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID))
		return
	}

	// Record billing
	if resp.Usage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			c.Request.Context(),
			userID,
			requestID,
			req.Model,
			providerName,
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			resp.Usage.CachedTokenCount(),
			resp.Usage.CacheWriteTokenCount(),
			billingType,
			subID,
			apiKeyID,
			0,
			time.Since(callStarted).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record usage", err,
				map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"model":      req.Model,
				})
		} else {
			h.saveConversation(userID, apiKeyID, requestID, req.Model,
				[]provider.Message{{Role: "user", Content: provider.ContentString(req.Prompt)}},
				resp.Choices,
				rec.Cost, false, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CachedTokenCount())
		}
	}

	// Track metrics
	monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "success").Inc()
	monitor.LLMTokenTotal.WithLabelValues(req.Model, "prompt").Add(float64(resp.Usage.PromptTokens))
	monitor.LLMTokenTotal.WithLabelValues(req.Model, "completion").Add(float64(resp.Usage.CompletionTokens))

	response.Success(c, resp)
}

// handleCompletionStream handles streaming text completion requests via SSE.
func (h *LLMHandler) handleCompletionStream(
	c *gin.Context,
	llmProvider provider.Provider,
	req *provider.CompletionRequest,
	providerName string,
	userID uint,
	requestID string,
	apiKeyID *uint,
	billingType model.BillingType,
	subID *uint,
) {
	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Flush headers immediately
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	startedAt := time.Now()
	streamCh, err := llmProvider.CompletionStream(c.Request.Context(), req)
	if err != nil {
		monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
		logging.Error("llm_handler", "completion_stream", "failed to start stream", err,
			map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"model":      req.Model,
				"provider":   providerName,
			})

		errData, _ := json.Marshal(map[string]interface{}{
			"error": map[string]string{
				"message": fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID),
				"type":    "internal_error",
			},
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
		c.Writer.Flush()
		return
	}

	// Track last usage for billing
	var lastUsage *provider.Usage
	var ttftMs int64

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logging.Error("llm_handler", "completion_stream", "response writer does not support flushing", nil,
			map[string]interface{}{"request_id": requestID})
		return
	}

	for event := range streamCh {
		// Time to first upstream chunk (TTFT): only count chunks carrying data
		// (skips empty/keepalive events emitted right after connect).
		if ttftMs == 0 && len(event.Data) > 0 {
			ttftMs = time.Since(startedAt).Milliseconds()
		}
		// Check for client disconnect
		select {
		case <-c.Request.Context().Done():
			logging.Info("llm_handler", "completion_stream", "client disconnected",
				map[string]interface{}{"request_id": requestID, "user_id": userID})
			return
		default:
		}

		if event.Error != nil {
			monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "error").Inc()
			logging.Error("llm_handler", "completion_stream", "stream error", event.Error,
				map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"model":      req.Model,
				})
			errData, _ := json.Marshal(map[string]interface{}{
				"error": map[string]string{
					"message": fmt.Sprintf("upstream service error (request_id: %s), please try again later", requestID),
					"type":    "stream_error",
				},
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
			flusher.Flush()
			return
		}

		if event.Done {
			break
		}

		// Parse the chunk to extract usage info from the last data event
		if len(event.Data) > 0 {
			var chunk struct {
				Usage *provider.Usage `json:"usage,omitempty"`
			}
			if err := json.Unmarshal(event.Data, &chunk); err == nil && chunk.Usage != nil {
				lastUsage = chunk.Usage
			}
		}

		// Write SSE data event
		fmt.Fprintf(c.Writer, "data: %s\n\n", event.Data)
		flusher.Flush()
	}

	// Record billing after stream completes
	if lastUsage != nil && lastUsage.TotalTokens > 0 {
		rec, err := h.billingService.RecordUsage(
			context.Background(),
			userID,
			requestID,
			req.Model,
			providerName,
			lastUsage.PromptTokens,
			lastUsage.CompletionTokens,
			lastUsage.CachedTokenCount(),
			lastUsage.CacheWriteTokenCount(),
			billingType,
			subID,
			apiKeyID,
			ttftMs,
			time.Since(startedAt).Milliseconds(),
		)
		if err != nil {
			logging.Error("llm_handler", "billing", "failed to record stream usage", err,
				map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"model":      req.Model,
				})
		} else {
			h.saveConversation(userID, apiKeyID, requestID, req.Model,
				[]provider.Message{{Role: "user", Content: provider.ContentString(req.Prompt)}},
				[]provider.Choice{{Message: provider.Message{Role: "assistant", Content: provider.ContentString("")}}},
				rec.Cost, true, lastUsage.PromptTokens, lastUsage.CompletionTokens, lastUsage.CachedTokenCount())
		}

		monitor.LLMTokenTotal.WithLabelValues(req.Model, "prompt").Add(float64(lastUsage.PromptTokens))
		monitor.LLMTokenTotal.WithLabelValues(req.Model, "completion").Add(float64(lastUsage.CompletionTokens))
	}

	monitor.LLMRequestTotal.WithLabelValues(req.Model, providerName, "success").Inc()
}

// HealthCheck reports service liveness (GET /health).
func (h *LLMHandler) HealthCheck(c *gin.Context) {
	response.Success(c, gin.H{
		"status":  "ok",
		"service": "maas-platform-api",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// ListModels returns the model catalog visible to API key callers
// (GET /api/v1/llm/models). Mirrors the public /api/v1/models catalog.
func (h *LLMHandler) ListModels(c *gin.Context) {
	response.Success(c, buildModelMarketplace(h.channelRepo, h.billingService))
}

package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/payment"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Pricing holds per-token pricing for input, output and cache tokens.
// CacheReadPrice / CacheWritePrice are absolute per-token prices: when a model
// has no explicit cache price configured they default to input×10% (read) and
// input×125% (write).
type Pricing struct {
	Input           decimal.Decimal
	Output          decimal.Decimal
	CacheReadPrice  decimal.Decimal
	CacheWritePrice decimal.Decimal
}

// ErrModelNotPriced is returned when a model has no enabled price entry and
// therefore may not be invoked (it is not billed).
var ErrModelNotPriced = errors.New("model pricing not configured")

// ErrInsufficientBalance is returned when the user does not have enough balance.
var ErrInsufficientBalance = errors.New("insufficient balance")

// InsufficientBalanceError carries the exact top-up amount required, so the
// frontend can offer an immediate payment flow for the shortage.
type InsufficientBalanceError struct {
	Need decimal.Decimal
}

func (e *InsufficientBalanceError) Error() string {
	return fmt.Sprintf("insufficient balance, need %s more", e.Need.StringFixed(2))
}

func (e *InsufficientBalanceError) Unwrap() error { return ErrInsufficientBalance }

// BillingService handles all billing-related operations.
type BillingService struct {
	db          *gorm.DB
	userRepo    *UserRepository
	billingRepo *BillingRecordRepository
	txRepo      *TransactionRepository
	planRepo    *PlanRepository
	subRepo     *SubscriptionRepository
	pricingRepo *repository.ModelPriceRepository
	// alertSender delivers low-token-balance warning emails. Optional; set via
	// SetAlertSender. When nil, the threshold check is a no-op.
	alertSender LowTokenAlertSender
}

// LowTokenAlertSender delivers a low-token-balance warning email to a user.
type LowTokenAlertSender interface {
	SendTokenLowAlert(ctx context.Context, to, nickname string, remaining, threshold int64) error
}

// SetAlertSender wires the low-token-balance email sender.
func (s *BillingService) SetAlertSender(a LowTokenAlertSender) {
	s.alertSender = a
}

// computeTokenBalance returns the user's combined token balance: token credits
// (加油包) plus the remaining quota of all active subscriptions.
func (s *BillingService) computeTokenBalance(user *model.User) int64 {
	total := user.TokenCredits
	if subs, err := s.subRepo.FindActiveByUserID(user.ID); err == nil {
		for _, sub := range subs {
			remaining := sub.Plan.IncludedTokens - sub.UsedTokens
			if remaining > 0 {
				total += remaining
			}
		}
	}
	return total
}

// maybeAlertLowToken sends a warning email when the user's combined token
// balance drops below their configured threshold. It only sends once per
// crossing (tracked via token_alert_sent) and resets when the balance recovers.
func (s *BillingService) maybeAlertLowToken(ctx context.Context, userID uint) {
	if s.alertSender == nil {
		return
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return
	}
	threshold := user.TokenAlertThreshold
	if threshold <= 0 {
		if user.TokenAlertSent {
			s.db.Model(&model.User{}).Where("id = ?", userID).Update("token_alert_sent", false)
		}
		return
	}
	total := s.computeTokenBalance(user)
	if total < threshold {
		if !user.TokenAlertSent {
			sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = s.alertSender.SendTokenLowAlert(sendCtx, user.Email, user.Nickname, total, threshold)
			s.db.Model(&model.User{}).Where("id = ?", userID).Update("token_alert_sent", true)
		}
	} else if user.TokenAlertSent {
		s.db.Model(&model.User{}).Where("id = ?", userID).Update("token_alert_sent", false)
	}
}

// UserRepository wraps the billing-specific user data access.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID retrieves a user by ID.
func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	return &user, err
}

// UpdateBalance updates the user's balance.
func (r *UserRepository) UpdateBalance(id uint, balance decimal.Decimal) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("balance", balance).Error
}

// Update saves the user changes.
func (r *UserRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// BillingRecordRepository wraps billing record data access.
type BillingRecordRepository struct {
	db *gorm.DB
}

// NewBillingRecordRepository creates a new BillingRecordRepository.
func NewBillingRecordRepository(db *gorm.DB) *BillingRecordRepository {
	return &BillingRecordRepository{db: db}
}

// Create inserts a billing record.
func (r *BillingRecordRepository) Create(record *model.BillingRecord) error {
	return r.db.Create(record).Error
}

// FindByUserID retrieves paginated billing records for a user.
func (r *BillingRecordRepository) FindByUserID(userID uint, page, size int) ([]model.BillingRecord, int64, error) {
	var records []model.BillingRecord
	var total int64
	query := r.db.Model(&model.BillingRecord{}).Where("user_id = ?", userID)
	query.Count(&total)
	err := query.Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&records).Error
	return records, total, err
}

// SumUsageByUserIDAndDate returns total token usage and cost for a user within a date range.
func (r *BillingRecordRepository) SumUsageByUserIDAndDate(userID uint, start, end time.Time) (int64, decimal.Decimal, error) {
	var totalTokens int64
	var totalCost decimal.Decimal
	err := r.db.Model(&model.BillingRecord{}).
		Select("COALESCE(SUM(tokens_in + tokens_out), 0), COALESCE(SUM(cost), 0)").
		Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, start, end).
		Row().Scan(&totalTokens, &totalCost)
	return totalTokens, totalCost, err
}

// UsageDailyItem represents token usage and cost aggregated for a single day.
type UsageDailyItem struct {
	Date   string          `json:"date"`
	Tokens int64           `json:"tokens"`
	Cost   decimal.Decimal `json:"cost"`
}

// SumUsageDailyByUserIDAndDate returns per-day token usage and cost for a user within a date range,
// ordered by date ascending (only days with records are included).
func (r *BillingRecordRepository) SumUsageDailyByUserIDAndDate(userID uint, start, end time.Time) ([]UsageDailyItem, error) {
	var items []UsageDailyItem
	err := r.db.Model(&model.BillingRecord{}).
		Select("DATE(created_at) AS date, COALESCE(SUM(tokens_in + tokens_out), 0) AS tokens, COALESCE(SUM(cost), 0) AS cost").
		Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, start, end).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

// TransactionRepository wraps transaction data access.
type TransactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository creates a new TransactionRepository.
func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create inserts a transaction.
func (r *TransactionRepository) Create(tx *model.Transaction) error {
	return r.db.Create(tx).Error
}

// FindByUserID retrieves paginated transactions for a user.
func (r *TransactionRepository) FindByUserID(userID uint, page, size int) ([]model.Transaction, int64, error) {
	var txs []model.Transaction
	var total int64
	query := r.db.Model(&model.Transaction{}).Where("user_id = ?", userID)
	query.Count(&total)
	err := query.Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&txs).Error
	return txs, total, err
}

// FindByTransactionNo finds a transaction by its unique number.
func (r *TransactionRepository) FindByTransactionNo(no string) (*model.Transaction, error) {
	var tx model.Transaction
	err := r.db.Where("transaction_no = ?", no).First(&tx).Error
	return &tx, err
}

// Update saves transaction changes.
func (r *TransactionRepository) Update(tx *model.Transaction) error {
	return r.db.Save(tx).Error
}

// PlanRepository wraps plan data access.
type PlanRepository struct {
	db *gorm.DB
}

// NewPlanRepository creates a new PlanRepository.
func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

// FindByID retrieves a plan by ID.
func (r *PlanRepository) FindByID(id uint) (*model.Plan, error) {
	var plan model.Plan
	err := r.db.First(&plan, id).Error
	return &plan, err
}

// SubscriptionRepository wraps subscription data access.
type SubscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new SubscriptionRepository.
func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// Create inserts a subscription.
func (r *SubscriptionRepository) Create(sub *model.Subscription) error {
	return r.db.Create(sub).Error
}

// FindByID retrieves a subscription by ID with the associated plan.
func (r *SubscriptionRepository) FindByID(id uint) (*model.Subscription, error) {
	var sub model.Subscription
	err := r.db.Preload("Plan").First(&sub, id).Error
	return &sub, err
}

// FindActiveByUserID retrieves all active subscriptions for a user.
func (r *SubscriptionRepository) FindActiveByUserID(userID uint) ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.Where("user_id = ? AND status = ?", userID, "active").
		Preload("Plan").Order("end_at DESC").Find(&subs).Error
	return subs, err
}

// Update saves subscription changes.
func (r *SubscriptionRepository) Update(sub *model.Subscription) error {
	return r.db.Save(sub).Error
}

// CountByUserAndPlan returns the total number of subscriptions a user has ever
// purchased for the given plan (across all statuses, including cancelled and
// expired). It is used to enforce per-plan purchase limits.
func (r *SubscriptionRepository) CountByUserAndPlan(userID, planID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Subscription{}).
		Where("user_id = ? AND plan_id = ?", userID, planID).
		Count(&count).Error
	return count, err
}

// FindExpiring retrieves active subscriptions with auto-renew that are expiring within one day.
func (r *SubscriptionRepository) FindExpiring() ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.Where("status = ? AND auto_renew = ? AND end_at <= DATE_ADD(NOW(), INTERVAL 1 DAY)", "active", true).
		Preload("User").Preload("Plan").Find(&subs).Error
	return subs, err
}

// NewBillingService creates a new BillingService.
func NewBillingService(
	db *gorm.DB,
	userRepo *UserRepository,
	billingRepo *BillingRecordRepository,
	txRepo *TransactionRepository,
	planRepo *PlanRepository,
	subRepo *SubscriptionRepository,
	pricingRepo *repository.ModelPriceRepository,
) *BillingService {
	return &BillingService{
		db:          db,
		userRepo:    userRepo,
		billingRepo: billingRepo,
		txRepo:      txRepo,
		planRepo:    planRepo,
		subRepo:     subRepo,
		pricingRepo: pricingRepo,
	}
}

// GetSubscription returns the subscription by ID (with its plan preloaded), or
// nil on error. Used to decide whether a subscription caller can still pay.
func (s *BillingService) GetSubscription(subID uint) *model.Subscription {
	if s.subRepo == nil {
		return nil
	}
	sub, err := s.subRepo.FindByID(subID)
	if err != nil {
		return nil
	}
	return sub
}

// ResolveSubscriptionForModel returns the first active, unexpired subscription
// whose plan grants access to the given model. It returns nil when the user has
// no such subscription, in which case usage is billed pay-per-use.
func (s *BillingService) ResolveSubscriptionForModel(userID uint, modelName string) *model.Subscription {
	if s.subRepo == nil {
		return nil
	}
	subs, err := s.subRepo.FindActiveByUserID(userID)
	if err != nil {
		logging.Error("billing", "resolve_subscription", "failed to load active subscriptions", err,
			map[string]interface{}{"user_id": userID})
		return nil
	}
	now := time.Now()
	for i := range subs {
		sub := &subs[i]
		if now.After(sub.EndAt) {
			continue
		}
		if sub.Plan.MatchesModel(modelName) {
			return sub
		}
	}
	return nil
}

// ResolvePaidSubscriptionForModel is like ResolveSubscriptionForModel but only
// matches an active subscription whose price is at least 0.01 (i.e. a paid
// subscription). It is used to decide unlimited-firepower eligibility so that
// free / pay-per-use-only users do not get the perk.
func (s *BillingService) ResolvePaidSubscriptionForModel(userID uint, modelName string) *model.Subscription {
	if s.subRepo == nil {
		return nil
	}
	subs, err := s.subRepo.FindActiveByUserID(userID)
	if err != nil {
		logging.Error("billing", "resolve_paid_subscription", "failed to load active subscriptions", err,
			map[string]interface{}{"user_id": userID})
		return nil
	}
	now := time.Now()
	for i := range subs {
		sub := &subs[i]
		if now.After(sub.EndAt) {
			continue
		}
		if sub.Plan.MatchesModel(modelName) && sub.Price.GreaterThan(decimal.NewFromFloat(0.01)) {
			return sub
		}
	}
	return nil
}

// IsUnlimitedEnabled reports whether the model has the unlimited-firepower promo
// switched on (requires the price entry to be enabled and UnlimitedEnabled).
func (s *BillingService) IsUnlimitedEnabled(modelName string) bool {
	if s.pricingRepo == nil {
		return false
	}
	p, err := s.pricingRepo.FindByModel(modelName)
	if err != nil {
		return false
	}
	return p.Enabled && p.UnlimitedEnabled
}

// IsUnlimitedFirepower reports whether the user is entitled to the
// unlimited-firepower perk for the given model: the promo must be enabled and the
// user must hold a paid active subscription covering the model. It returns the
// matching subscription id for attribution when eligible.
func (s *BillingService) IsUnlimitedFirepower(userID uint, modelName string) (*uint, bool) {
	if !s.IsUnlimitedEnabled(modelName) {
		return nil, false
	}
	sub := s.ResolvePaidSubscriptionForModel(userID, modelName)
	if sub == nil {
		return nil, false
	}
	id := sub.ID
	return &id, true
}

// PricingForModel returns the enabled per-token pricing for a model.
// A model without an enabled ModelPrice entry yields ErrModelNotPriced.
func (s *BillingService) PricingForModel(modelName string) (Pricing, error) {
	if s.pricingRepo == nil {
		return Pricing{}, ErrModelNotPriced
	}
	p, err := s.pricingRepo.FindByModel(modelName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Pricing{}, ErrModelNotPriced
		}
		return Pricing{}, err
	}
	if !p.Enabled {
		return Pricing{}, ErrModelNotPriced
	}
	readPrice := p.InputPrice.Mul(decimal.RequireFromString(CachedInputDiscountFactor))
	writePrice := p.InputPrice.Mul(decimal.RequireFromString(CacheWriteMarkupFactor))
	if p.CacheReadPrice.Valid {
		readPrice = p.CacheReadPrice.Decimal
	}
	if p.CacheWritePrice.Valid {
		writePrice = p.CacheWritePrice.Decimal
	}
	return Pricing{
		Input:           p.InputPrice,
		Output:          p.OutputPrice,
		CacheReadPrice:  readPrice,
		CacheWritePrice: writePrice,
	}, nil
}

// EnsureModelPriced verifies that a model has an enabled price entry. The LLM
// gateway calls this before forwarding, so unpriced models are rejected
// (allowlist effect) and never reach billing.
func (s *BillingService) EnsureModelPriced(modelName string) error {
	_, err := s.PricingForModel(modelName)
	return err
}

// ListEnabledPrices returns the enabled model price entries, for listings.
func (s *BillingService) ListEnabledPrices() []model.ModelPrice {
	if s.pricingRepo == nil {
		return nil
	}
	list, err := s.pricingRepo.List()
	if err != nil {
		logging.Error("billing", "list_enabled_prices", "failed to list model prices", err, nil)
		return nil
	}
	var out []model.ModelPrice
	for _, p := range list {
		if p.Enabled {
			out = append(out, p)
		}
	}
	return out
}

// getMultiplierForModel returns the pricing-group multiplier for a model.
// The first enabled group (by id) whose model list matches wins; a group never
// matches is ignored; fallback multiplier is 1.
func (s *BillingService) getMultiplierForModel(modelName string) decimal.Decimal {
	one := decimal.NewFromInt(1)

	if s.db == nil {
		return one
	}
	var groups []model.PricingGroup
	if err := s.db.Where("enabled = ?", true).Order("id ASC").Find(&groups).Error; err != nil {
		return one
	}
	for _, g := range groups {
		if g.MatchesModel(modelName) {
			if g.Multiplier.IsZero() || g.Multiplier.IsNegative() {
				return one
			}
			return g.Multiplier
		}
	}
	return one
}

// CalculateCost calculates the cost for a given model and token counts.
// The model price (ModelPrice table) is multiplied by the model's pricing-group
// multiplier, so pay-per-use cost = table price * multiplier.
// Unpriced models yield zero here; the gateway rejects them before billing.
// cachedInputDiscountFactor is the fraction of the input price applied to
// cached (prompt cache hit) tokens. Cache hits are billed at 10% of the
// normal input rate.
const CachedInputDiscountFactor = "0.1"

// cacheWriteMarkupFactor is the multiple of the input price applied to
// cache-creation (cache write) tokens, following the Anthropic convention
// (e.g. Claude pricing charges 1.25x input price for writes).
const CacheWriteMarkupFactor = "1.25"

// UsageLine is one line of the billing formula (input / cache read / cache
// write / output), priced per token.
type UsageLine struct {
	Key    string          `json:"key"`
	Tokens int             `json:"tokens"`
	Rate   decimal.Decimal `json:"rate"` // CNY per token
	Amount decimal.Decimal `json:"amount"`
}

// UsageBreakdown is the full billing calculation snapshot for one request.
type UsageBreakdown struct {
	Lines      []UsageLine     `json:"lines"`
	Subtotal   decimal.Decimal `json:"subtotal"`
	Multiplier decimal.Decimal `json:"multiplier"`
	Discount   decimal.Decimal `json:"discount"` // subtotal - final (≥0)
	Final      decimal.Decimal `json:"final"`
}

// pricingRate returns the per-token rate of a line key ("input", "output",
// "cache_read"), or zero when that line is absent from the breakdown.
func pricingRate(bk *UsageBreakdown, key string) decimal.Decimal {
	for i := range bk.Lines {
		if bk.Lines[i].Key == key {
			return bk.Lines[i].Rate
		}
	}
	return decimal.Zero
}

// userFacingRate converts a per-token price to the per-1M-token figure used
// in user-facing formula strings ("101 tokens × 20/M Tokens").
func userFacingRate(rate decimal.Decimal) decimal.Decimal {
	return rate.Mul(decimal.NewFromInt(1000000))
}

// breakdownFor computes the itemized billing calculation for a request.
// The math is identical to CalculateCost; callers use it to snapshot the
// formula at billing time so later price edits cannot distort history.
func (s *BillingService) breakdownFor(model string, tokensIn, tokensOut, tokensCached, tokensCachedWrite int) (*UsageBreakdown, error) {
	pricing, err := s.PricingForModel(model)
	if err != nil {
		return nil, err
	}
	if tokensCached > tokensIn {
		tokensCached = tokensIn
	}
	if tokensCachedWrite > tokensIn {
		tokensCachedWrite = tokensIn
	}
	if tokensCachedWrite > tokensIn-tokensCached {
		tokensCachedWrite = tokensIn - tokensCached
	}
	uncachedIn := tokensIn - tokensCached - tokensCachedWrite

	cacheReadPrice := pricing.CacheReadPrice
	cacheWritePrice := pricing.CacheWritePrice

	bk := &UsageBreakdown{}
	if uncachedIn > 0 {
		amt := pricing.Input.Mul(decimal.NewFromInt(int64(uncachedIn)))
		bk.Lines = append(bk.Lines, UsageLine{Key: "input", Tokens: uncachedIn, Rate: pricing.Input, Amount: amt})
	}
	if tokensCached > 0 {
		amt := cacheReadPrice.Mul(decimal.NewFromInt(int64(tokensCached)))
		bk.Lines = append(bk.Lines, UsageLine{Key: "cache_read", Tokens: tokensCached, Rate: cacheReadPrice, Amount: amt})
	}
	if tokensCachedWrite > 0 {
		amt := cacheWritePrice.Mul(decimal.NewFromInt(int64(tokensCachedWrite)))
		bk.Lines = append(bk.Lines, UsageLine{Key: "cache_write", Tokens: tokensCachedWrite, Rate: cacheWritePrice, Amount: amt})
	}
	if tokensOut > 0 {
		amt := pricing.Output.Mul(decimal.NewFromInt(int64(tokensOut)))
		bk.Lines = append(bk.Lines, UsageLine{Key: "output", Tokens: tokensOut, Rate: pricing.Output, Amount: amt})
	}
	for _, l := range bk.Lines {
		bk.Subtotal = bk.Subtotal.Add(l.Amount)
	}
	multiplier := s.getMultiplierForModel(model)
	bk.Multiplier = multiplier
	bk.Final = bk.Subtotal
	if !multiplier.IsZero() {
		bk.Final = bk.Subtotal.Mul(multiplier)
	}
	bk.Discount = bk.Subtotal.Sub(bk.Final)
	if bk.Discount.IsNegative() {
		bk.Discount = decimal.Zero
	}
	return bk, nil
}

func (s *BillingService) CalculateCost(model string, tokensIn, tokensOut, tokensCached, tokensCachedWrite int) decimal.Decimal {
	bk, err := s.breakdownFor(model, tokensIn, tokensOut, tokensCached, tokensCachedWrite)
	if err != nil {
		if errors.Is(err, ErrModelNotPriced) {
			logging.Warn("billing", "calculate_cost", "attempt to bill unpriced model",
				map[string]interface{}{"model": model})
		} else {
			logging.Error("billing", "calculate_cost", "failed to load model pricing", err,
				map[string]interface{}{"model": model})
		}
		return decimal.Zero
	}

	logging.Info("billing", "calculate_cost", "cost calculated",
		map[string]interface{}{
			"model":              model,
			"tokens_in":          tokensIn,
			"tokens_out":         tokensOut,
			"tokens_cache":       tokensCached,
			"tokens_cache_write": tokensCachedWrite,
			"multiplier":         bk.Multiplier.String(),
			"cost":               bk.Final.String(),
		})

	return bk.Final
}

// RecordUsage records a billing record and deducts from the user's balance or subscription.
// If subID is provided, the usage is charged to the subscription; otherwise it is pay-per-use.
func (s *BillingService) RecordUsage(
	ctx context.Context,
	userID uint,
	requestID string,
	modelName string,
	provider string,
	tokensIn int,
	tokensOut int,
	tokensCached int,
	tokensCachedWrite int,
	billingType model.BillingType,
	subID *uint,
	apiKeyID *uint,
	ttftMs int64,
	durationMs int64,
) (*model.BillingRecord, error) {
	// Refuse to bill unpriced models even if the gateway check was bypassed.
	if err := s.EnsureModelPriced(modelName); err != nil {
		return nil, err
	}

	// Unlimited-firepower normalization. If the perk was granted at request start
	// but has since been revoked (e.g. admin toggled the switch off mid-stream),
	// fall back to normal billing so the toggle takes effect immediately. When the
	// perk stands, no balance or subscription quota is deducted.
	unlimited := billingType == model.BillingUnlimited
	if unlimited {
		if _, stillEligible := s.IsUnlimitedFirepower(userID, modelName); !stillEligible {
			if subID != nil {
				billingType = model.BillingSubscription
			} else {
				billingType = model.BillingPayPerUse
			}
			unlimited = false
		}
	}

	deductType := "normal"
	if unlimited {
		deductType = "unlimited_promo"
	}

	cost := decimal.Zero
	if !unlimited {
		cost = s.CalculateCost(modelName, tokensIn, tokensOut, tokensCached, tokensCachedWrite)
	}

	// Snapshot the itemized formula at billing time so later price edits
	// cannot distort historical records. Unlimited-firepower records are stored
	// with a zeroed breakdown (the perk means no charge).
	detailJSON := ""
	if unlimited {
		if b, err := json.Marshal(map[string]interface{}{
			"model":      modelName,
			"deduct_type": "unlimited_promo",
			"final":      "0",
		}); err == nil {
			detailJSON = string(b)
		}
	} else if bk, err := s.breakdownFor(modelName, tokensIn, tokensOut, tokensCached, tokensCachedWrite); err == nil {
		type detailDoc struct {
			Model      string          `json:"model"`
			InputRate  string          `json:"input_rate_per_m"` // 元/1M tokens
			OutputRate string          `json:"output_rate_per_m"`
			CacheRate  string          `json:"cache_read_rate_per_m"`
			Lines      []UsageLine     `json:"lines"`
			Subtotal   decimal.Decimal `json:"subtotal"`
			Multiplier decimal.Decimal `json:"multiplier"`
			Discount   decimal.Decimal `json:"discount"`
			Final      decimal.Decimal `json:"final"`
		}
		doc := detailDoc{
			Model:      modelName,
			InputRate:  userFacingRate(pricingRate(bk, "input")).String(),
			OutputRate: userFacingRate(pricingRate(bk, "output")).String(),
			CacheRate:  userFacingRate(pricingRate(bk, "cache_read")).String(),
			Lines:      bk.Lines,
			Subtotal:   bk.Subtotal,
			Multiplier: bk.Multiplier,
			Discount:   bk.Discount,
			Final:      bk.Final,
		}
		if b, err := json.Marshal(doc); err == nil {
			detailJSON = string(b)
		}
	}

	record := &model.BillingRecord{
		UserID:           userID,
		RequestID:        requestID,
		Model:            modelName,
		Provider:         provider,
		TokensIn:         tokensIn,
		TokensOut:        tokensOut,
		CachedTokens:     tokensCached,
		TokensCacheWrite: tokensCachedWrite,
		Cost:             cost,
		TTFTMs:           ttftMs,
		DurationMs:       durationMs,
		Detail:           detailJSON,
		BillingType:      billingType,
		DeductType:       deductType,
		SubscriptionID:   subID,
		ApiKeyID:         apiKeyID,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create billing record
		if err := tx.Create(record).Error; err != nil {
			logging.Error("billing", "record_usage", "failed to create billing record", err,
				map[string]interface{}{"request_id": requestID, "user_id": userID})
			return err
		}

		// If pay-per-use, deduct from user balance
		if billingType == model.BillingPayPerUse && subID == nil {
			paymentMethod := "balance"
			if err := s.deductBalanceTx(tx, userID, cost); err != nil {
				// Fall back to token credit (授信) when the balance is
				// insufficient: consume credit_used tokens instead.
				if errors.Is(err, ErrInsufficientBalance) {
					weighted := s.weightedTokensFor(tokensIn, tokensOut, tokensCached, modelName)
					if creditErr := s.deductCreditTx(tx, userID, weighted); creditErr == nil {
						paymentMethod = "credit"
					} else {
						return err
					}
				} else {
					return err
				}
			}

			// Create consume transaction
			user, err := s.getUserTx(tx, userID)
			if err != nil {
				return err
			}

			txRecord := &model.Transaction{
				UserID:          userID,
				TransactionNo:   s.GenerateTransactionNo(),
				Type:            model.TransactionConsume,
				Amount:          cost,
				BalanceBefore:   user.Balance.Add(cost),
				BalanceAfter:    user.Balance,
				PaymentMethod:   paymentMethod,
				Status:          model.TransactionSuccess,
				Description:     fmt.Sprintf("Usage: %s - %s (%d in / %d out)", provider, modelName, tokensIn, tokensOut),
				BillingRecordID: &record.ID,
			}
			if err := tx.Create(txRecord).Error; err != nil {
				logging.Error("billing", "record_usage", "failed to create transaction", err,
					map[string]interface{}{"request_id": requestID})
				return err
			}
		}

		// If subscription, update used tokens (weighted by the model multiplier
		// so subscription credits are debited as tokens * multiplier). When the
		// remaining quota is insufficient the update matches no row and we fall
		// back to balance billing for this request. Unlimited-firepower requests
		// skip this entirely (no quota is consumed).
		if subID != nil && billingType == model.BillingSubscription {
			weightedTokens := s.weightedTokensFor(tokensIn, tokensOut, tokensCached, modelName)
			ok, err := s.updateSubscriptionUsageTx(tx, *subID, weightedTokens)
			if err != nil {
				return err
			}
			if !ok {
				subID = nil
				if err := s.deductBalanceTx(tx, userID, cost); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		logging.Error("billing", "record_usage", "failed to record usage", err,
			map[string]interface{}{
				"user_id":    userID,
				"request_id": requestID,
				"model":      modelName,
			})
		return nil, err
	}

	logging.Info("billing", "record_usage", "usage recorded successfully",
		map[string]interface{}{
			"user_id":      userID,
			"request_id":   requestID,
			"cost":         cost.String(),
			"billing_type": billingType,
		})

	// Warn the user by email when their combined token balance drops below the
	// configured threshold. Runs after the transaction commits.
	s.maybeAlertLowToken(ctx, userID)

	return record, nil
}

// deductBalanceTx deducts an amount from the user's balance within a transaction.
// It checks for sufficient balance before deducting.
func (s *BillingService) deductBalanceTx(tx *gorm.DB, userID uint, amount decimal.Decimal) error {
	var user model.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
		return err
	}

	if user.Balance.LessThan(amount) {
		logging.Warn("billing", "deduct_balance", "insufficient balance",
			map[string]interface{}{
				"user_id":  userID,
				"balance":  user.Balance.String(),
				"required": amount.String(),
			})
		return ErrInsufficientBalance
	}

	newBalance := user.Balance.Sub(amount)
	if err := tx.Model(&user).Update("balance", newBalance).Error; err != nil {
		return err
	}

	return nil
}

// getUserTx retrieves a user within a transaction.
func (s *BillingService) getUserTx(tx *gorm.DB, userID uint) (*model.User, error) {
	var user model.User
	if err := tx.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// deductCreditTx consumes tokens from the user's available credit
// (credit_used += tokens) within a transaction, when the balance is
// insufficient. Tokens are debited on a first-consumed-first-repaid basis.
func (s *BillingService) deductCreditTx(tx *gorm.DB, userID uint, weightedTokens int64) error {
	if weightedTokens <= 0 {
		return ErrInsufficientBalance
	}
	res := tx.Exec(
		"UPDATE users SET credit_used = credit_used + ? WHERE id = ? AND credit_limit - credit_used >= ?",
		weightedTokens, userID, weightedTokens)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInsufficientBalance
	}
	return nil
}

// updateSubscriptionUsageTx increments the used tokens for a subscription.
// weightedTokens is the usage already multiplied by the pricing-group multiplier.
// updateSubscriptionUsageTx debits subscription quota atomically. If the
// quota is exhausted the update matches no row (RowsAffected == 0) and the
// caller must fall back to per-token billing.
// weightedTokensFor converts raw token counts into subscription-credit token
// units, applying only the pricing-group multiplier. Cached input tokens are
// counted at full weight (no cache discount): the quota is billed on real
// tokens used.
func (s *BillingService) weightedTokensFor(tokensIn, tokensOut, tokensCached int, modelName string) int64 {
	return decimal.NewFromInt(int64(tokensIn + tokensOut)).
		Mul(s.getMultiplierForModel(modelName)).Round(0).IntPart()
}

func (s *BillingService) updateSubscriptionUsageTx(tx *gorm.DB, subID uint, weightedTokens int64) (bool, error) {
	res := tx.Model(&model.Subscription{}).
		Where("id = ? AND (used_tokens + ?) <= included_tokens", subID, weightedTokens).
		UpdateColumn("used_tokens", gorm.Expr("used_tokens + ?", weightedTokens))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeductBalance deducts an amount from the user's balance.
func (s *BillingService) DeductBalance(userID uint, amount decimal.Decimal) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.deductBalanceTx(tx, userID, amount)
	})

	if err != nil {
		logging.Error("billing", "deduct_balance", "failed to deduct balance", err,
			map[string]interface{}{"user_id": userID, "amount": amount.String()})
		return err
	}

	logging.Info("billing", "deduct_balance", "balance deducted successfully",
		map[string]interface{}{"user_id": userID, "amount": amount.String()})
	return nil
}

// AddBalance adds an amount to the user's balance.
func (s *BillingService) AddBalance(userID uint, amount decimal.Decimal) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}

		newBalance := user.Balance.Add(amount)
		if err := tx.Model(&user).Update("balance", newBalance).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		logging.Error("billing", "add_balance", "failed to add balance", err,
			map[string]interface{}{"user_id": userID, "amount": amount.String()})
		return err
	}

	logging.Info("billing", "add_balance", "balance added successfully",
		map[string]interface{}{"user_id": userID, "amount": amount.String()})
	return nil
}

// AdjustBalance applies a signed admin balance adjustment (delta may be
// negative) and records it as a settled transaction with balance snapshots,
// all atomically. The resulting balance must not go negative.
func (s *BillingService) AdjustBalance(userID uint, delta decimal.Decimal, description string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		newBalance := user.Balance.Add(delta)
		if newBalance.LessThan(decimal.Zero) {
			return errors.New("adjusted balance cannot be negative")
		}
		balanceBefore := user.Balance
		if err := tx.Model(&user).Update("balance", newBalance).Error; err != nil {
			return err
		}
		r := &model.Transaction{
			UserID:        userID,
			TransactionNo: s.GenerateTransactionNo(),
			Type:          model.TransactionAdjust,
			Amount:        delta.Abs(),
			PaymentMethod: "admin",
			Status:        model.TransactionSuccess,
			Description:   description,
			BalanceBefore: balanceBefore,
			BalanceAfter:  newBalance,
		}
		if err := tx.Create(r).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logging.Error("billing", "adjust_balance", "failed to adjust balance", err,
			map[string]interface{}{"user_id": userID, "delta": delta.String()})
		return err
	}
	logging.Info("billing", "adjust_balance", "balance adjusted successfully",
		map[string]interface{}{"user_id": userID, "delta": delta.String()})
	return nil
}

// CreateTransaction creates a new transaction record.
func (s *BillingService) CreateTransaction(
	userID uint,
	txType model.TransactionType,
	amount decimal.Decimal,
	paymentMethod string,
	description string,
) (*model.Transaction, error) {
	transaction := &model.Transaction{
		UserID:        userID,
		TransactionNo: s.GenerateTransactionNo(),
		Type:          txType,
		Amount:        amount,
		PaymentMethod: paymentMethod,
		Status:        model.TransactionPending,
		Description:   description,
	}

	if err := s.txRepo.Create(transaction); err != nil {
		logging.Error("billing", "create_transaction", "failed to create transaction", err,
			map[string]interface{}{
				"user_id": userID,
				"type":    txType,
				"amount":  amount.String(),
			})
		return nil, err
	}

	logging.Info("billing", "create_transaction", "transaction created",
		map[string]interface{}{
			"transaction_no": transaction.TransactionNo,
			"user_id":        userID,
			"type":           txType,
			"amount":         amount.String(),
		})

	return transaction, nil
}

// GenerateTransactionNo generates a unique transaction number.
// Format: TX + timestamp (13 digits) + 8 random hex characters.
// FindTransactionByNo returns a transaction by its number (nil-safe).
func (s *BillingService) FindTransactionByNo(no string) (*model.Transaction, error) {
	tx, err := s.txRepo.FindByTransactionNo(no)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (s *BillingService) GenerateTransactionNo() string {
	now := time.Now().UnixMilli()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	return fmt.Sprintf("TX%d%s", now, randomHex)
}

// CompleteEpayPayment settles a pending recharge transaction after the
// payment gateway confirms the order. It is idempotent: transactions already
// marked success are ignored. It returns the settled transaction.
func (s *BillingService) CompleteEpayPayment(txNo string) (*model.Transaction, error) {
	var result *model.Transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Atomic claim: only one concurrent caller may move the transaction
		// from pending to success (prevents double crediting on duplicate /
		// racing gateway callbacks).
		claim := tx.Model(&model.Transaction{}).
			Where("transaction_no = ? AND status = ?", txNo, model.TransactionPending).
			Update("status", model.TransactionSuccess)
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected == 0 {
			// Already settled or not in a claimable state.
			var t model.Transaction
			if err := tx.Where("transaction_no = ?", txNo).First(&t).Error; err != nil {
				return err
			}
			if t.Status == model.TransactionSuccess {
				result = &t
				return nil
			}
			return errors.New("transaction is not pending")
		}
		var t model.Transaction
		if err := tx.Where("transaction_no = ?", txNo).First(&t).Error; err != nil {
			return err
		}
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, t.UserID).Error; err != nil {
			return err
		}
		balanceBefore := user.Balance
		newBalance := user.Balance.Add(t.Amount)
		if err := tx.Model(&user).Update("balance", newBalance).Error; err != nil {
			return err
		}
		now := time.Now()
		updates := map[string]interface{}{
			"status":         model.TransactionSuccess,
			"balance_before": balanceBefore,
			"balance_after":  newBalance,
			"payment_method": "epay",
			"description":    "Recharge via epay",
			"updated_at":     now,
		}
		if err := tx.Model(&t).Updates(updates).Error; err != nil {
			return err
		}
		t.Status = model.TransactionSuccess
		t.BalanceBefore = balanceBefore
		t.BalanceAfter = newBalance
		result = &t
		return nil
	})
	if err != nil {
		return nil, err
	}
	logging.Info("billing", "epay_settle", "epay payment settled",
		map[string]interface{}{"transaction_no": txNo, "amount": result.Amount.String()})
	return result, nil
}

// ExpirePendingOrders auto-cancels gateway orders (epay / wechat / alipay)
// that were created more than ttl ago and never paid, so stale pending orders
// cannot block the user's quota/flow. Returns the number of orders cancelled.
func (s *BillingService) ExpirePendingOrders(ttl time.Duration) (int64, error) {
	cutoff := time.Now().Add(-ttl)
	res := s.db.Model(&model.Transaction{}).
		Where("status = ? AND payment_method IN ? AND created_at < ?",
			model.TransactionPending, []string{"epay", "wechat", "alipay"}, cutoff).
		Update("status", model.TransactionCancelled)
	if res.Error != nil {
		logging.Error("billing", "expire_orders", "failed to expire pending orders", res.Error, nil)
		return 0, res.Error
	}
	if res.RowsAffected > 0 {
		logging.Info("billing", "expire_orders", "expired stale pending orders",
			map[string]interface{}{"cancelled": res.RowsAffected, "cutoff": cutoff})
	}
	return res.RowsAffected, nil
}

// loadConfigValues reads all system_configs rows into a key→value map. It is
// used by the reconciliation task to resolve gateway credentials without a
// repository dependency.
func (s *BillingService) loadConfigValues() map[string]string {
	values := map[string]string{}
	var cfgs []model.SystemConfig
	if err := s.db.Find(&cfgs).Error; err != nil {
		return values
	}
	for _, c := range cfgs {
		values[c.Key] = c.Value
	}
	return values
}

func epayConfigFromValues(v map[string]string) (payment.EpayConfig, bool) {
	if v["pay_epay_enabled"] != "true" && v["pay_epay_enabled"] != "1" {
		return payment.EpayConfig{}, false
	}
	cfg := payment.EpayConfig{
		Gateway:   strings.TrimSpace(v["pay_epay_gateway"]),
		PID:       strings.TrimSpace(v["pay_epay_pid"]),
		Key:       strings.TrimSpace(v["pay_epay_key"]),
		SignUpper: v["pay_epay_sign_upper"] == "true" || v["pay_epay_sign_upper"] == "1",
		Enabled:   true,
	}
	if cfg.Gateway == "" || cfg.PID == "" || cfg.Key == "" {
		return payment.EpayConfig{}, false
	}
	return cfg, true
}

func wechatConfigFromValues(v map[string]string) (payment.WechatPayConfig, bool) {
	if v["pay_wechat_enabled"] != "true" && v["pay_wechat_enabled"] != "1" {
		return payment.WechatPayConfig{}, false
	}
	cfg := payment.WechatPayConfig{
		Enabled:    true,
		AppID:      strings.TrimSpace(v["pay_wechat_appid"]),
		MchID:      strings.TrimSpace(v["pay_wechat_mchid"]),
		APIKey:     strings.TrimSpace(v["pay_wechat_api_key"]),
		SerialNo:   strings.TrimSpace(v["pay_wechat_serial"]),
		PrivateKey: strings.TrimSpace(v["pay_wechat_private_key"]),
		NotifyURL:  strings.TrimSpace(v["pay_wechat_notify_url"]),
	}
	if cfg.AppID == "" || cfg.MchID == "" || cfg.APIKey == "" || cfg.PrivateKey == "" {
		return payment.WechatPayConfig{}, false
	}
	return cfg, true
}

func alipayConfigFromValues(v map[string]string) (payment.AlipayConfig, bool) {
	if v["pay_alipay_enabled"] != "true" && v["pay_alipay_enabled"] != "1" {
		return payment.AlipayConfig{}, false
	}
	cfg := payment.AlipayConfig{
		Enabled:    true,
		AppID:      strings.TrimSpace(v["pay_alipay_appid"]),
		PrivateKey: strings.TrimSpace(v["pay_alipay_private_key"]),
		PublicKey:  strings.TrimSpace(v["pay_alipay_public_key"]),
		NotifyURL:  strings.TrimSpace(v["pay_alipay_notify_url"]),
		ReturnURL:  strings.TrimSpace(v["pay_alipay_return_url"]),
		Gateway:    strings.TrimSpace(v["pay_alipay_gateway"]),
	}
	if cfg.AppID == "" || cfg.PrivateKey == "" || cfg.PublicKey == "" {
		return payment.AlipayConfig{}, false
	}
	return cfg, true
}

// ReconcilePendingOrders actively queries each gateway for pending orders that
// may have been paid but whose async callback was missed (network timeout,
// downtime, etc.). Confirmed orders are settled; the gateway-reported amount is
// cross-checked against the local order and a mismatch is NOT settled (logged
// for manual review). lookback bounds how far back pending orders are
// considered; keep it inside the pending-order expiry window so that orders
// about to be cancelled are not touched.
func (s *BillingService) ReconcilePendingOrders(lookback time.Duration) (int64, error) {
	cutoff := time.Now().Add(-lookback)
	var orders []model.Transaction
	if err := s.db.Where("status = ? AND payment_method IN ? AND created_at >= ?",
		model.TransactionPending, []string{"epay", "wechat", "alipay"}, cutoff).
		Find(&orders).Error; err != nil {
		logging.Error("billing", "reconcile", "failed to load pending orders", err, nil)
		return 0, err
	}
	if len(orders) == 0 {
		return 0, nil
	}

	values := s.loadConfigValues()
	settled := int64(0)
	for _, tx := range orders {
		fields := map[string]interface{}{"out_trade_no": tx.TransactionNo, "payment_method": tx.PaymentMethod}
		paid, amountMismatch := false, false
		switch tx.PaymentMethod {
		case "epay":
			cfg, ok := epayConfigFromValues(values)
			if !ok {
				continue
			}
			p, _, err := payment.NewEpayClient(cfg).QueryOrder(tx.TransactionNo)
			if err != nil {
				logging.Warn("billing", "reconcile", "epay query failed", fields)
				continue
			}
			paid = p
		case "wechat":
			cfg, ok := wechatConfigFromValues(values)
			if !ok {
				continue
			}
			p, amountCents, err := payment.NewWechatPayClient(cfg).QueryOrder(tx.TransactionNo)
			if err != nil {
				logging.Warn("billing", "reconcile", "wechat query failed", fields)
				continue
			}
			paid = p
			if paid {
				localCents := tx.Amount.Mul(decimal.NewFromInt(100)).IntPart()
				amountMismatch = amountCents > 0 && amountCents != localCents
			}
		case "alipay":
			cfg, ok := alipayConfigFromValues(values)
			if !ok {
				continue
			}
			p, amountYuan, err := payment.NewAlipayClient(cfg).QueryOrder(tx.TransactionNo)
			if err != nil {
				logging.Warn("billing", "reconcile", "alipay query failed", fields)
				continue
			}
			paid = p
			if paid {
				amountMismatch = amountYuan != "" && amountYuan != tx.Amount.StringFixed(2)
			}
		}
		if !paid {
			continue
		}
		if amountMismatch {
			logging.Error("billing", "reconcile", "gateway amount mismatch; order not settled", nil, fields)
			continue
		}
		if _, err := s.CompleteEpayPayment(tx.TransactionNo); err != nil {
			logging.Error("billing", "reconcile", "settle failed", err, fields)
			continue
		}
		settled++
	}
	if settled > 0 {
		logging.Info("billing", "reconcile", "reconciled pending orders",
			map[string]interface{}{"settled": settled, "checked": len(orders)})
	}
	return settled, nil
}

// Subscribe creates a new subscription for a user.
// ErrCannotDowngrade is returned when a subscription upgrade would move the
// user to a plan with fewer included tokens than their current plan.
var ErrCannotDowngrade = errors.New("cannot downgrade subscription")

// ErrAlreadySubscribed is returned when the user tries to buy the plan they
// are already on.
var ErrAlreadySubscribed = errors.New("already subscribed to this plan")

// ErrPurchaseLimitExceeded is returned when the user has already purchased the
// plan the maximum number of times allowed.
var ErrPurchaseLimitExceeded = errors.New("purchase limit exceeded for this plan")

// Subscribe purchases a plan. If the user already has an active subscription
// this acts as an upgrade: the unused token quota of the current plan is
// converted into a credit (at the current plan's per-token price) and
// deducted from the new plan's price. Downgrades to plans with fewer
// included tokens are rejected.
func (s *BillingService) Subscribe(userID, planID uint) (*model.Subscription, error) {
	plan, err := s.planRepo.FindByID(planID)
	if err != nil {
		logging.Error("billing", "subscribe", "plan not found", err,
			map[string]interface{}{"plan_id": planID})
		return nil, err
	}

	if plan.Status != "active" {
		return nil, errors.New("plan is not active")
	}

	// Enforce per-plan purchase limit (0 = unlimited).
	if plan.MaxPurchase > 0 {
		owned, err := s.subRepo.CountByUserAndPlan(userID, planID)
		if err != nil {
			logging.Error("billing", "subscribe", "failed to count plan purchases", err,
				map[string]interface{}{"user_id": userID, "plan_id": planID})
			return nil, err
		}
		if owned >= int64(plan.MaxPurchase) {
			return nil, ErrPurchaseLimitExceeded
		}
	}

	now := time.Now()
	sub := &model.Subscription{
		UserID:         userID,
		PlanID:         planID,
		Status:         "active",
		StartAt:        now,
		EndAt:          now.AddDate(0, 0, plan.DurationDays),
		AutoRenew:      true,
		Price:          plan.Price,
		IncludedTokens: plan.IncludedTokens,
	}

	// Upgrade path: an active subscription exists.
	activeSubs, err := s.subRepo.FindActiveByUserID(userID)
	if err != nil {
		logging.Error("billing", "subscribe", "failed to load active subscriptions", err,
			map[string]interface{}{"user_id": userID})
		return nil, err
	}

	var old *model.Subscription
	var deduct decimal.Decimal
	var credit decimal.Decimal
	upgrade := false
	if len(activeSubs) > 0 {
		old = &activeSubs[0]
		if old.PlanID == planID {
			return nil, ErrAlreadySubscribed
		}
		if plan.IncludedTokens < old.Plan.IncludedTokens {
			return nil, ErrCannotDowngrade
		}
		upgrade = true
		// Credit = remaining tokens at the current plan's per-token price.
		remaining := old.Plan.IncludedTokens - old.UsedTokens
		if remaining < 0 {
			remaining = 0
		}
		if old.Plan.IncludedTokens > 0 && remaining > 0 {
			unit := old.Plan.Price.Div(decimal.NewFromInt(old.Plan.IncludedTokens))
			credit = unit.Mul(decimal.NewFromInt(remaining)).Round(2)
			if credit.GreaterThan(plan.Price) {
				credit = plan.Price
			}
		}
		deduct = plan.Price.Sub(credit)
		if deduct.LessThan(decimal.Zero) {
			deduct = decimal.Zero
		}
	} else {
		deduct = plan.Price
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Charge the (discounted) upgrade price. When the balance is
		// insufficient, report the exact shortage so the caller can trigger
		// a payment flow for the missing amount.
		user, err := s.getUserTx(tx, userID)
		if err != nil {
			return err
		}
		if user.Balance.LessThan(deduct) {
			return &InsufficientBalanceError{Need: deduct.Sub(user.Balance)}
		}
		if err := s.deductBalanceTx(tx, userID, deduct); err != nil {
			return err
		}

		// Cancel the old subscription in the same transaction
		if upgrade && old != nil {
			if err := tx.Model(&model.Subscription{}).Where("id = ?", old.ID).
				Updates(map[string]interface{}{
					"status":       "cancelled",
					"cancelled_at": now,
				}).Error; err != nil {
				return err
			}
		}

		// Create the new subscription
		if err := tx.Create(sub).Error; err != nil {
			return err
		}

		// Create subscription transaction
		description := fmt.Sprintf("Subscription: %s (%d days)", plan.Name, plan.DurationDays)
		if upgrade && old != nil {
			description = fmt.Sprintf("Subscription upgrade: %s -> %s (credit %.2f applied)",
				old.Plan.Name, plan.Name, credit.InexactFloat64())
		}

		txRecord := &model.Transaction{
			UserID:         userID,
			TransactionNo:  s.GenerateTransactionNo(),
			Type:           model.TransactionSubscription,
			Amount:         deduct,
			BalanceBefore:  user.Balance.Add(deduct),
			BalanceAfter:   user.Balance,
			PaymentMethod:  "balance",
			Status:         model.TransactionSuccess,
			Description:    description,
			SubscriptionID: &sub.ID,
		}
		return tx.Create(txRecord).Error
	})

	if err != nil {
		logging.Error("billing", "subscribe", "failed to create subscription", err,
			map[string]interface{}{"user_id": userID, "plan_id": planID})
		return nil, err
	}

	logging.Info("billing", "subscribe", "subscription created successfully",
		map[string]interface{}{
			"user_id": userID,
			"plan_id": planID,
			"sub_id":  sub.ID,
			"upgrade": upgrade,
			"credit":  credit.String(),
		})

	return sub, nil
}

// CancelSubscription cancels an active subscription.
func (s *BillingService) CancelSubscription(subID uint) error {
	sub, err := s.subRepo.FindByID(subID)
	if err != nil {
		logging.Error("billing", "cancel_subscription", "subscription not found", err,
			map[string]interface{}{"sub_id": subID})
		return err
	}

	if sub.Status != "active" {
		return errors.New("subscription is not active")
	}

	now := time.Now()
	sub.Status = "cancelled"
	sub.AutoRenew = false
	sub.CancelledAt = &now

	if err := s.subRepo.Update(sub); err != nil {
		logging.Error("billing", "cancel_subscription", "failed to update subscription", err,
			map[string]interface{}{"sub_id": subID})
		return err
	}

	logging.Info("billing", "cancel_subscription", "subscription cancelled",
		map[string]interface{}{"sub_id": subID, "user_id": sub.UserID})

	return nil
}

// SetAutoRenew toggles the auto-renew flag of an active subscription without
// cancelling it. Disabling auto-renew keeps the subscription active until its
// EndAt, after which it simply expires instead of renewing.
func (s *BillingService) SetAutoRenew(subID uint, enabled bool) error {
	sub, err := s.subRepo.FindByID(subID)
	if err != nil {
		logging.Error("billing", "set_auto_renew", "subscription not found", err,
			map[string]interface{}{"sub_id": subID})
		return err
	}

	if sub.Status != "active" {
		return errors.New("subscription is not active")
	}

	sub.AutoRenew = enabled
	if err := s.subRepo.Update(sub); err != nil {
		logging.Error("billing", "set_auto_renew", "failed to update subscription", err,
			map[string]interface{}{"sub_id": subID})
		return err
	}

	logging.Info("billing", "set_auto_renew", "auto-renew updated",
		map[string]interface{}{"sub_id": subID, "user_id": sub.UserID, "auto_renew": enabled})

	return nil
}

// ExpireSubscriptions marks active subscriptions whose end date has passed and
// whose auto-renew is disabled as "expired". This ensures that turning off
// auto-renew lets the user keep the subscription until EndAt, after which it
// stops granting access instead of renewing.
func (s *BillingService) ExpireSubscriptions(ctx context.Context) (int64, error) {
	res := s.db.Model(&model.Subscription{}).
		Where("status = ? AND auto_renew = ? AND end_at <= ?", "active", false, time.Now()).
		Update("status", "expired")
	if res.Error != nil {
		logging.Error("billing", "expire_subs", "failed to expire subscriptions", res.Error, nil)
		return 0, res.Error
	}
	if res.RowsAffected > 0 {
		logging.Info("billing", "expire_subs", "expired subscriptions",
			map[string]interface{}{"count": res.RowsAffected})
	}
	return res.RowsAffected, nil
}

// AutoRenewSubscription processes subscription auto-renewals.
// It finds all active subscriptions with auto-renew enabled that are expiring soon
// and attempts to renew them by charging the user's balance.
func (s *BillingService) AutoRenewSubscription(ctx context.Context) {
	subs, err := s.subRepo.FindExpiring()
	if err != nil {
		logging.Error("billing", "auto_renew", "failed to find expiring subscriptions", err, nil)
		return
	}

	for _, sub := range subs {
		select {
		case <-ctx.Done():
			logging.Info("billing", "auto_renew", "auto-renew cancelled by context", nil)
			return
		default:
		}

		s.renewOneSubscription(sub)
	}
}

// renewOneSubscription attempts to renew a single subscription.
func (s *BillingService) renewOneSubscription(sub model.Subscription) {
	newStart := sub.EndAt
	newEnd := newStart.AddDate(0, 0, sub.Plan.DurationDays)

	newSub := &model.Subscription{
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
		Status:         "active",
		StartAt:        newStart,
		EndAt:          newEnd,
		AutoRenew:      true,
		Price:          sub.Price,
		IncludedTokens: sub.Plan.IncludedTokens,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Charge the user for the renewal
		if err := s.deductBalanceTx(tx, sub.UserID, sub.Price); err != nil {
			return err
		}

		// Mark old subscription as expired
		if err := tx.Model(&sub).Update("status", "expired").Error; err != nil {
			return err
		}

		// Create new subscription
		if err := tx.Create(newSub).Error; err != nil {
			return err
		}

		// Create transaction record
		user, err := s.getUserTx(tx, sub.UserID)
		if err != nil {
			return err
		}

		txRecord := &model.Transaction{
			UserID:         sub.UserID,
			TransactionNo:  s.GenerateTransactionNo(),
			Type:           model.TransactionSubscription,
			Amount:         sub.Price,
			BalanceBefore:  user.Balance.Add(sub.Price),
			BalanceAfter:   user.Balance,
			PaymentMethod:  "balance",
			Status:         model.TransactionSuccess,
			Description:    fmt.Sprintf("Auto-renew: %s", sub.Plan.Name),
			SubscriptionID: &newSub.ID,
		}
		return tx.Create(txRecord).Error
	})

	if err != nil {
		logging.Error("billing", "auto_renew", "failed to renew subscription", err,
			map[string]interface{}{
				"sub_id":  sub.ID,
				"user_id": sub.UserID,
				"plan_id": sub.PlanID,
			})
		// Disable auto-renew to prevent repeated failures
		sub.AutoRenew = false
		if updateErr := s.subRepo.Update(&sub); updateErr != nil {
			logging.Error("billing", "auto_renew", "failed to disable auto-renew after failure", updateErr,
				map[string]interface{}{"sub_id": sub.ID})
		}
		return
	}

	logging.Info("billing", "auto_renew", "subscription renewed successfully",
		map[string]interface{}{
			"old_sub_id": sub.ID,
			"new_sub_id": newSub.ID,
			"user_id":    sub.UserID,
		})
}

// GetUserUsage returns a summary of the user's total token usage and cost within a date range.
func (s *BillingService) GetUserUsage(userID uint, start, end time.Time) (totalTokens int64, totalCost decimal.Decimal, err error) {
	totalTokens, totalCost, err = s.billingRepo.SumUsageByUserIDAndDate(userID, start, end)
	if err != nil {
		logging.Error("billing", "get_user_usage", "failed to get user usage", err,
			map[string]interface{}{"user_id": userID, "start": start, "end": end})
		return 0, decimal.Zero, err
	}

	logging.Info("billing", "get_user_usage", "user usage retrieved",
		map[string]interface{}{
			"user_id":      userID,
			"total_tokens": totalTokens,
			"total_cost":   totalCost.String(),
		})

	return totalTokens, totalCost, nil
}

// GetUserUsageDaily returns per-day usage items within a date range.
func (s *BillingService) GetUserUsageDaily(userID uint, start, end time.Time) ([]UsageDailyItem, error) {
	items, err := s.billingRepo.SumUsageDailyByUserIDAndDate(userID, start, end)
	if err != nil {
		logging.Error("billing", "get_user_usage_daily", "failed to get daily user usage", err,
			map[string]interface{}{"user_id": userID, "start": start, "end": end})
		return nil, err
	}
	return items, nil
}

// GetUserBillingRecords returns paginated billing records for a user.
func (s *BillingService) GetUserBillingRecords(userID uint, page, size int) ([]model.BillingRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	records, total, err := s.billingRepo.FindByUserID(userID, page, size)
	if err != nil {
		logging.Error("billing", "get_user_billing_records", "failed to get billing records", err,
			map[string]interface{}{"user_id": userID, "page": page, "size": size})
		return nil, 0, err
	}

	return records, total, nil
}

// GetUserTransactions returns paginated transactions for a user.
func (s *BillingService) GetUserTransactions(userID uint, page, size int) ([]model.Transaction, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	transactions, total, err := s.txRepo.FindByUserID(userID, page, size)
	if err != nil {
		logging.Error("billing", "get_user_transactions", "failed to get transactions", err,
			map[string]interface{}{"user_id": userID, "page": page, "size": size})
		return nil, 0, err
	}

	return transactions, total, nil
}

// Recharge adds balance to a user's account and creates a recharge transaction.
func (s *BillingService) Recharge(userID uint, amount decimal.Decimal, paymentMethod string) (*model.Transaction, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("recharge amount must be positive")
	}

	var transaction *model.Transaction

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}

		balanceBefore := user.Balance
		newBalance := user.Balance.Add(amount)
		if err := tx.Model(&user).Update("balance", newBalance).Error; err != nil {
			return err
		}

		transaction = &model.Transaction{
			UserID:        userID,
			TransactionNo: s.GenerateTransactionNo(),
			Type:          model.TransactionRecharge,
			Amount:        amount,
			BalanceBefore: balanceBefore,
			BalanceAfter:  newBalance,
			PaymentMethod: paymentMethod,
			Status:        model.TransactionSuccess,
			Description:   fmt.Sprintf("Recharge via %s", paymentMethod),
		}

		return tx.Create(transaction).Error
	})

	if err != nil {
		logging.Error("billing", "recharge", "failed to recharge", err,
			map[string]interface{}{"user_id": userID, "amount": amount.String()})
		return nil, err
	}

	logging.Info("billing", "recharge", "recharge successful",
		map[string]interface{}{
			"user_id":        userID,
			"amount":         amount.String(),
			"transaction_no": transaction.TransactionNo,
		})

	return transaction, nil
}

// PurchaseTokenPackage buys a token package (加油包) using the user's balance.
// It deducts the price from the balance and credits the user's token credits.
// The cash balance is reduced, but the balance-based transaction is recorded
// so the purchase is fully auditable.
func (s *BillingService) PurchaseTokenPackage(userID uint, packageID uint) (*model.Transaction, int64, error) {
	var transaction *model.Transaction
	var newCredits int64

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var pkg model.TokenPackage
		if err := tx.First(&pkg, packageID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("token package not found")
			}
			return err
		}
		if pkg.Status != "active" {
			return errors.New("token package is not available")
		}

		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, userID).Error; err != nil {
			return err
		}
		if user.Balance.LessThan(pkg.Price) {
			return ErrInsufficientBalance
		}

		balanceBefore := user.Balance
		newBalance := balanceBefore.Sub(pkg.Price)
		newCredits = user.TokenCredits + pkg.Tokens + pkg.BonusTokens

		if err := tx.Model(&user).Updates(map[string]interface{}{
			"balance":       newBalance,
			"token_credits": newCredits,
		}).Error; err != nil {
			return err
		}

		desc := fmt.Sprintf("Token package: %s (+%d tokens)", pkg.Name, pkg.Tokens+pkg.BonusTokens)
		transaction = &model.Transaction{
			UserID:        userID,
			TransactionNo: s.GenerateTransactionNo(),
			Type:          model.TransactionTokenPackage,
			Amount:        pkg.Price,
			BalanceBefore: balanceBefore,
			BalanceAfter:  newBalance,
			PaymentMethod: "balance",
			Status:        model.TransactionSuccess,
			Description:   desc,
		}
		return tx.Create(transaction).Error
	})

	if err != nil {
		logging.Error("billing", "purchase_token_package", "failed to purchase token package", err,
			map[string]interface{}{"user_id": userID, "package_id": packageID})
		return nil, 0, err
	}

	logging.Info("billing", "purchase_token_package", "token package purchased",
		map[string]interface{}{
			"user_id":        userID,
			"package_id":     packageID,
			"transaction_no": transaction.TransactionNo,
			"token_credits":  newCredits,
		})

	return transaction, newCredits, nil
}

// ProcessSubscriptionUsage tracks token usage against a subscription.
// It updates the subscription's used_tokens counter.
func (s *BillingService) ProcessSubscriptionUsage(userID uint, subID uint, tokens int) error {
	sub, err := s.subRepo.FindByID(subID)
	if err != nil {
		logging.Error("billing", "process_subscription_usage", "subscription not found", err,
			map[string]interface{}{"sub_id": subID, "user_id": userID})
		return err
	}

	if sub.UserID != userID {
		return errors.New("subscription does not belong to user")
	}

	if sub.Status != "active" {
		return errors.New("subscription is not active")
	}

	now := time.Now()
	if now.After(sub.EndAt) {
		return errors.New("subscription has expired")
	}

	if err := s.subRepo.Update(sub); err != nil {
		logging.Error("billing", "process_subscription_usage", "failed to update subscription usage", err,
			map[string]interface{}{"sub_id": subID, "tokens": tokens})
		return err
	}

	logging.Info("billing", "process_subscription_usage", "subscription usage updated",
		map[string]interface{}{
			"sub_id":  subID,
			"tokens":  tokens,
			"user_id": userID,
		})

	return nil
}

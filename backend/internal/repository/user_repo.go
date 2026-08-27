package repository

import (
	"time"

	"github.com/mass-platform/backend/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *model.User) error {
	// open_id_uid has a unique index; a Go zero-value "" would collide with
	// every other non-OpenID user. Skip the column so it lands as NULL
	// (MySQL allows many NULLs on a unique index).
	if user.OpenIDUID == "" {
		return r.db.Omit("open_id_uid").Create(user).Error
	}
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	return &user, err
}

// ListActiveIDs returns the IDs of all active users.
func (r *UserRepository) ListActiveIDs() ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.User{}).
		Where("status = ?", "active").
		Pluck("id", &ids).Error
	return ids, err
}

func (r *UserRepository) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ?", email).First(&user).Error
	return &user, err
}

// FindByOpenIDUID looks up a user bound to the given 亦 OpenID uid.
func (r *UserRepository) FindByOpenIDUID(uid string) (*model.User, error) {
	var user model.User
	err := r.db.Where("open_id_uid = ?", uid).First(&user).Error
	return &user, err
}

// BindOpenID attaches a 亦 OpenID identity to a user account.
func (r *UserRepository) BindOpenID(userID uint, uid, username string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"open_id_uid": uid, "open_id_username": username}).Error
}

// UnbindOpenID removes the 亦 OpenID identity from a user account.
func (r *UserRepository) UnbindOpenID(userID uint) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"open_id_uid": "", "open_id_username": ""}).Error
}

func (r *UserRepository) Update(user *model.User) error {
	// Updates(struct) only writes non-zero fields, so a partially filled
	// struct (e.g. ID + LastLoginAt from the OpenID login path) can never
	// wipe the rest of the row. open_id_uid is skipped when empty, which
	// also avoids unique-index collisions with legacy "" rows; use
	// BindOpenID/UnbindOpenID for binding changes.
	return r.db.Model(user).Updates(user).Error
}

func (r *UserRepository) UpdateBalance(id uint, balance interface{}) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("balance", balance).Error
}

func (r *UserRepository) List(page, size int, filters map[string]interface{}) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	query := r.db.Model(&model.User{})
	for k, v := range filters {
		query = query.Where(k, v)
	}
	query.Count(&total)
	err := query.Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&users).Error
	return users, total, err
}

// Search searches users by email or nickname (LIKE match).
func (r *UserRepository) Search(page, size int, keyword string) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	like := "%" + keyword + "%"
	query := r.db.Model(&model.User{}).Where("email LIKE ? OR nickname LIKE ?", like, like)
	query.Count(&total)
	err := query.Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&users).Error
	return users, total, err
}

// Count returns the total number of users.
func (r *UserRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Count(&count).Error
	return count, err
}

// CountByStatus returns the number of users with the given status.
func (r *UserRepository) CountByStatus(status model.UserStatus) (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// CountCreatedAfter returns the number of users created after the given time
// (used for "new users today" style metrics).
func (r *UserRepository) CountCreatedAfter(t time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&model.User{}).Where("created_at >= ?", t).Count(&count).Error
	return count, err
}

type ApiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository(db *gorm.DB) *ApiKeyRepository {
	return &ApiKeyRepository{db: db}
}

func (r *ApiKeyRepository) Create(key *model.ApiKey) error {
	return r.db.Create(key).Error
}

func (r *ApiKeyRepository) FindByKeyHash(keyHash string) (*model.ApiKey, error) {
	var key model.ApiKey
	err := r.db.Where("key_hash = ?", keyHash).Preload("User").Preload("RateLimit").First(&key).Error
	return &key, err
}

func (r *ApiKeyRepository) FindByUserID(userID uint) ([]model.ApiKey, error) {
	var keys []model.ApiKey
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

func (r *ApiKeyRepository) Update(key *model.ApiKey) error {
	return r.db.Save(key).Error
}

func (r *ApiKeyRepository) Delete(id uint) error {
	return r.db.Delete(&model.ApiKey{}, id).Error
}

type BillingRecordRepository struct {
	db *gorm.DB
}

func NewBillingRecordRepository(db *gorm.DB) *BillingRecordRepository {
	return &BillingRecordRepository{db: db}
}

func (r *BillingRecordRepository) Create(record *model.BillingRecord) error {
	return r.db.Create(record).Error
}

func (r *BillingRecordRepository) FindByUserID(userID uint, page, size int) ([]model.BillingRecord, int64, error) {
	var records []model.BillingRecord
	var total int64
	r.db.Model(&model.BillingRecord{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&records).Error
	return records, total, err
}

// FindByIDAndUser loads a single billing record, scoped to the owner.
func (r *BillingRecordRepository) FindByIDAndUser(id uint, userID uint, out *model.BillingRecord) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).First(out).Error
}

func (r *BillingRecordRepository) SumByUserIDAndDate(userID uint, start, end string) (float64, error) {
	var total float64
	err := r.db.Model(&model.BillingRecord{}).
		Select("COALESCE(SUM(cost), 0)").
		Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, start, end).
		Scan(&total).Error
	return total, err
}

// CountToday returns the number of billing records created today.
func (r *BillingRecordRepository) CountToday() (int64, error) {
	var count int64
	today := time.Now().Truncate(24 * time.Hour)
	err := r.db.Model(&model.BillingRecord{}).
		Where("created_at >= ?", today).
		Count(&count).Error
	return count, err
}

// CountAll returns the total number of billing records (total requests served).
func (r *BillingRecordRepository) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&model.BillingRecord{}).Count(&count).Error
	return count, err
}

type TransactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(tx *model.Transaction) error {
	return r.db.Create(tx).Error
}

func (r *TransactionRepository) FindByUserID(userID uint, page, size int) ([]model.Transaction, int64, error) {
	var txs []model.Transaction
	var total int64
	r.db.Model(&model.Transaction{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&txs).Error
	return txs, total, err
}

func (r *TransactionRepository) FindByTransactionNo(no string) (*model.Transaction, error) {
	var tx model.Transaction
	err := r.db.Where("transaction_no = ?", no).First(&tx).Error
	return &tx, err
}

func (r *TransactionRepository) Update(tx *model.Transaction) error {
	return r.db.Save(tx).Error
}

// FindAll returns all transactions with optional filters, paginated.
func (r *TransactionRepository) FindAll(page, size int, filters map[string]interface{}) ([]model.Transaction, int64, error) {
	var txs []model.Transaction
	var total int64
	query := r.db.Model(&model.Transaction{})
	for k, v := range filters {
		query = query.Where(k, v)
	}
	query.Count(&total)
	err := query.Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&txs).Error
	return txs, total, err
}

// RevenueSumItem represents revenue aggregation for a single date.
type RevenueSumItem struct {
	Date   string          `json:"date"`
	Amount decimal.Decimal `json:"amount"`
	Count  int64           `json:"count"`
}

// DateCountItem represents a count aggregated for a single date.
type DateCountItem struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// SumRevenue returns the total revenue from all successful recharge transactions.
func (r *TransactionRepository) SumRevenue() (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.Model(&model.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("type = ? AND status = ?", model.TransactionRecharge, model.TransactionSuccess).
		Scan(&total).Error
	return total, err
}

// SumByDateRange returns revenue breakdown grouped by date within the given range.
func (r *TransactionRepository) SumByDateRange(start, end time.Time) ([]RevenueSumItem, error) {
	var items []RevenueSumItem
	err := r.db.Model(&model.Transaction{}).
		Select("DATE(created_at) AS date, COALESCE(SUM(amount), 0) AS amount, COUNT(*) AS count").
		Where("type = ? AND status = ? AND created_at BETWEEN ? AND ?", model.TransactionRecharge, model.TransactionSuccess, start, end).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

// CountByDateRange returns the number of billing records grouped by date.
func (r *BillingRecordRepository) CountByDateRange(start, end time.Time) ([]DateCountItem, error) {
	var items []DateCountItem
	err := r.db.Model(&model.BillingRecord{}).
		Select("DATE(created_at) AS date, COUNT(*) AS count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

// CountCreatedByDateRange returns the number of users created grouped by date.
func (r *UserRepository) CountCreatedByDateRange(start, end time.Time) ([]DateCountItem, error) {
	var items []DateCountItem
	err := r.db.Model(&model.User{}).
		Select("DATE(created_at) AS date, COUNT(*) AS count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

// CountCreatedByDateRange returns the number of subscriptions created grouped by date.
func (r *SubscriptionRepository) CountCreatedByDateRange(start, end time.Time) ([]DateCountItem, error) {
	var items []DateCountItem
	err := r.db.Model(&model.Subscription{}).
		Select("DATE(created_at) AS date, COUNT(*) AS count").
		Where("created_at BETWEEN ? AND ?", start, end).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&items).Error
	return items, err
}

type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) FindByID(id uint) (*model.Plan, error) {
	var plan model.Plan
	err := r.db.First(&plan, id).Error
	return &plan, err
}

func (r *PlanRepository) FindActive() ([]model.Plan, error) {
	var plans []model.Plan
	err := r.db.Where("status = ?", "active").Order("sort_order ASC").Find(&plans).Error
	return plans, err
}

func (r *PlanRepository) Create(plan *model.Plan) error {
	return r.db.Create(plan).Error
}

func (r *PlanRepository) Update(plan *model.Plan) error {
	return r.db.Save(plan).Error
}

func (r *PlanRepository) Delete(id uint) error {
	return r.db.Delete(&model.Plan{}, id).Error
}

// FindAll returns all plans including inactive ones, ordered by sort_order.
func (r *PlanRepository) FindAll() ([]model.Plan, error) {
	var plans []model.Plan
	err := r.db.Order("sort_order ASC").Find(&plans).Error
	return plans, err
}

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Create(sub *model.Subscription) error {
	return r.db.Create(sub).Error
}

func (r *SubscriptionRepository) FindByID(id uint) (*model.Subscription, error) {
	var sub model.Subscription
	err := r.db.Preload("Plan").First(&sub, id).Error
	return &sub, err
}

func (r *SubscriptionRepository) FindActiveByUserID(userID uint) ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.Where("user_id = ? AND status = ?", userID, "active").
		Preload("Plan").Order("end_at DESC").Find(&subs).Error
	return subs, err
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

// CountActive returns the number of currently active subscriptions (paid users).
func (r *SubscriptionRepository) CountActive() (int64, error) {
	var count int64
	err := r.db.Model(&model.Subscription{}).
		Where("status = ?", "active").
		Count(&count).Error
	return count, err
}

func (r *SubscriptionRepository) Update(sub *model.Subscription) error {
	return r.db.Save(sub).Error
}

func (r *SubscriptionRepository) FindExpiring() ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.Where("status = ? AND auto_renew = ? AND end_at <= NOW() + INTERVAL '1 day'", "active", true).
		Preload("User").Find(&subs).Error
	return subs, err
}

type IdentityVerificationRepository struct {
	db *gorm.DB
}

func NewIdentityVerificationRepository(db *gorm.DB) *IdentityVerificationRepository {
	return &IdentityVerificationRepository{db: db}
}

func (r *IdentityVerificationRepository) Create(v *model.IdentityVerification) error {
	return r.db.Create(v).Error
}

// CountByStatus returns the number of identity verifications with the given status.
func (r *IdentityVerificationRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&model.IdentityVerification{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *IdentityVerificationRepository) FindByUserID(userID uint) (*model.IdentityVerification, error) {
	var v model.IdentityVerification
	err := r.db.Where("user_id = ?", userID).First(&v).Error
	return &v, err
}

func (r *IdentityVerificationRepository) List(page, size int, status string) ([]model.IdentityVerification, int64, error) {
	var list []model.IdentityVerification
	var total int64
	query := r.db.Model(&model.IdentityVerification{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query.Count(&total)
	err := query.Preload("User").Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&list).Error
	return list, total, err
}

func (r *IdentityVerificationRepository) Update(v *model.IdentityVerification) error {
	return r.db.Save(v).Error
}

// FindByID retrieves an identity verification record by its ID.
func (r *IdentityVerificationRepository) FindByID(id uint) (*model.IdentityVerification, error) {
	var v model.IdentityVerification
	err := r.db.First(&v, id).Error
	return &v, err
}

type SystemLogRepository struct {
	db *gorm.DB
}

func NewSystemLogRepository(db *gorm.DB) *SystemLogRepository {
	return &SystemLogRepository{db: db}
}

func (r *SystemLogRepository) Create(log *model.SystemLog) error {
	return r.db.Create(log).Error
}

func (r *SystemLogRepository) List(page, size int, filters map[string]interface{}) ([]model.SystemLog, int64, error) {
	var logs []model.SystemLog
	var total int64
	query := r.db.Model(&model.SystemLog{})
	for k, v := range filters {
		query = query.Where(k, v)
	}
	query.Count(&total)
	err := query.Offset((page - 1) * size).Limit(size).Order("created_at DESC").Find(&logs).Error
	return logs, total, err
}

type RateLimitRepository struct {
	db *gorm.DB
}

func NewRateLimitRepository(db *gorm.DB) *RateLimitRepository {
	return &RateLimitRepository{db: db}
}

func (r *RateLimitRepository) FindByUserID(userID uint) (*model.RateLimit, error) {
	var rl model.RateLimit
	err := r.db.Where("user_id = ?", userID).First(&rl).Error
	return &rl, err
}

func (r *RateLimitRepository) FindByModel(modelName string) ([]model.RateLimit, error) {
	var rls []model.RateLimit
	err := r.db.Where("model = ? OR model = ?", modelName, "*").Find(&rls).Error
	return rls, err
}

func (r *RateLimitRepository) Upsert(rl *model.RateLimit) error {
	return r.db.Save(rl).Error
}

type SystemConfigRepository struct {
	db *gorm.DB
}

func NewSystemConfigRepository(db *gorm.DB) *SystemConfigRepository {
	return &SystemConfigRepository{db: db}
}

func (r *SystemConfigRepository) Get(key string) (*model.SystemConfig, error) {
	var cfg model.SystemConfig
	err := r.db.Where("`key` = ?", key).First(&cfg).Error
	return &cfg, err
}

func (r *SystemConfigRepository) Set(key, value, group string) error {
	var cfg model.SystemConfig
	result := r.db.Where("`key` = ?", key).First(&cfg)
	if result.Error != nil {
		cfg = model.SystemConfig{Key: key, Value: value, Group: group}
		return r.db.Create(&cfg).Error
	}
	cfg.Value = value
	cfg.Group = group
	return r.db.Save(&cfg).Error
}

// SetBatch upserts multiple config entries under the same group in one
// transaction, so a category form (site/contact/notify/payment) is saved
// atomically. Empty values are allowed so fields can be cleared.
func (r *SystemConfigRepository) SetBatch(group string, items []model.SystemConfig) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			var cfg model.SystemConfig
			result := tx.Where("`key` = ?", items[i].Key).First(&cfg)
			if result.Error != nil {
				if err := tx.Create(&model.SystemConfig{Key: items[i].Key, Value: items[i].Value, Group: group}).Error; err != nil {
					return err
				}
				continue
			}
			cfg.Value = items[i].Value
			cfg.Group = group
			if err := tx.Save(&cfg).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SystemConfigRepository) GetAll() ([]model.SystemConfig, error) {
	var cfgs []model.SystemConfig
	// "group" and "key" are reserved words in MySQL, so they must be quoted.
	err := r.db.Order("`group` ASC, `key` ASC").Find(&cfgs).Error
	return cfgs, err
}

type SystemMetricsRepository struct {
	db *gorm.DB
}

func NewSystemMetricsRepository(db *gorm.DB) *SystemMetricsRepository {
	return &SystemMetricsRepository{db: db}
}

func (r *SystemMetricsRepository) Create(m *model.SystemMetrics) error {
	return r.db.Create(m).Error
}

func (r *SystemMetricsRepository) GetLatest() (*model.SystemMetrics, error) {
	var m model.SystemMetrics
	err := r.db.Order("timestamp DESC").First(&m).Error
	return &m, err
}

func (r *SystemMetricsRepository) GetRange(start, end time.Time) ([]model.SystemMetrics, error) {
	var metrics []model.SystemMetrics
	err := r.db.Where("timestamp BETWEEN ? AND ?", start, end).Order("timestamp ASC").Find(&metrics).Error
	return metrics, err
}

type dailyAgg struct {
	Date     string
	Requests int64
	Tokens   int64
	Revenue  float64
	Users    int
	Keys     int
}

// AggregateRange 实时聚合 billing_records / transactions / users / api_keys，
// 按自然日返回指标（无需依赖定时采集任务）。
func (r *SystemMetricsRepository) AggregateRange(start, end time.Time) ([]model.SystemMetrics, error) {
	type row struct {
		Date     string
		Requests int64
		Tokens   int64
		Revenue  float64
	}
	var rows []row
	err := r.db.Raw(`
		SELECT DATE_FORMAT(b.created_at, '%Y-%m-%d') AS date,
		       COUNT(*) AS requests,
		       COALESCE(SUM(b.tokens_in + b.tokens_out), 0) AS tokens,
		       0 AS revenue
		FROM billing_records b
		WHERE b.created_at BETWEEN ? AND ?
		GROUP BY date
		ORDER BY date ASC`, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	var txRows []row
	err = r.db.Raw(`
		SELECT DATE_FORMAT(t.created_at, '%Y-%m-%d') AS date,
		       0 AS requests,
		       0 AS tokens,
		       COALESCE(SUM(t.amount), 0) AS revenue
		FROM transactions t
		WHERE t.status = 'success' AND t.created_at BETWEEN ? AND ?
		GROUP BY date`, start, end).Scan(&txRows).Error
	if err != nil {
		return nil, err
	}

	byDate := map[string]*model.SystemMetrics{}
	for i := start; i.Before(end) || i.Equal(end); i = i.AddDate(0, 0, 1) {
		key := i.Format("2006-01-02")
		byDate[key] = &model.SystemMetrics{
			Timestamp: i,
		}
	}
	for _, row := range rows {
		if m, ok := byDate[row.Date]; ok {
			m.TotalRequests = row.Requests
			m.SuccessRequests = row.Requests
			m.TotalTokens = row.Tokens
		}
	}
	for _, row := range txRows {
		if m, ok := byDate[row.Date]; ok {
			m.Revenue = row.Revenue
		}
	}

	out := make([]model.SystemMetrics, 0, len(byDate))
	for i := start; i.Before(end) || i.Equal(end); i = i.AddDate(0, 0, 1) {
		key := i.Format("2006-01-02")
		out = append(out, *byDate[key])
	}

	var activeUsers int64
	var activeKeys int64
	_ = r.db.Model(&model.User{}).Where("status = ?", "active").Count(&activeUsers).Error
	_ = r.db.Model(&model.ApiKey{}).Where("status = ?", "active").Count(&activeKeys).Error
	for i := range out {
		out[i].ActiveUsers = int(activeUsers)
		out[i].ActiveKeys = int(activeKeys)
	}
	return out, nil
}

package repository

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mass-platform/backend/internal/model"
)

var (
	ErrCreditHasActive      = errors.New("already has a pending or approved credit application")
	ErrCreditInsufficient   = errors.New("insufficient credit")
	ErrCreditRepayInsufficient = errors.New("insufficient token credits to repay")
)

// CreditRepository handles token credit (授信) applications.
type CreditRepository struct {
	db *gorm.DB
}

func NewCreditRepository(db *gorm.DB) *CreditRepository {
	return &CreditRepository{db: db}
}

// ConsumedTotal returns the user's cumulative consumption in CNY,
// i.e. the sum of successful consume / subscription / token_package
// transactions.
func (r *CreditRepository) ConsumedTotal(userID uint) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.db.Model(&model.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND type IN ? AND status = ?",
			userID,
			[]string{string(model.TransactionConsume), string(model.TransactionSubscription), string(model.TransactionTokenPackage)},
			string(model.TransactionSuccess)).
		Scan(&total).Error
	if err != nil {
		return decimal.Zero, err
	}
	return total, nil
}

// HasActive returns whether the user already has a pending or approved
// application (they cannot apply again in that case).
func (r *CreditRepository) HasActive(userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.CreditApplication{}).
		Where("user_id = ? AND status IN ?", userID, []string{string(model.CreditPending), string(model.CreditApproved)}).
		Count(&count).Error
	return count > 0, err
}

// Latest returns the user's latest credit application (any status).
func (r *CreditRepository) Latest(userID uint) (*model.CreditApplication, error) {
	var app model.CreditApplication
	err := r.db.Where("user_id = ?", userID).Order("id DESC").First(&app).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// Create inserts a new application (must be validated by the caller).
func (r *CreditRepository) Create(app *model.CreditApplication) error {
	return r.db.Create(app).Error
}

func (r *CreditRepository) FindByID(id uint) (*model.CreditApplication, error) {
	var app model.CreditApplication
	err := r.db.First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// ListPaginated returns applications with optional status filter,
// newest first.
func (r *CreditRepository) ListPaginated(page, size int, status string) ([]model.CreditApplication, int64, error) {
	q := r.db.Model(&model.CreditApplication{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.CreditApplication, 0, size)
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ApproveAndGrant atomically marks the application approved and sets the
// user's credit limit (授信总额度).
func (r *CreditRepository) ApproveAndGrant(id uint, grantedTokens int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var app model.CreditApplication
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&app, id).Error; err != nil {
			return err
		}
		if app.Status != string(model.CreditPending) {
			return gorm.ErrRecordNotFound
		}
		now := time.Now()
		if err := tx.Model(&app).Updates(map[string]interface{}{
			"status":         string(model.CreditApproved),
			"granted_tokens": grantedTokens,
			"reviewed_at":    now,
		}).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE users SET credit_limit = credit_limit + ? WHERE id = ?",
			grantedTokens, app.UserID).Error
	})
}

// Reject marks a pending application as rejected with a reason.
func (r *CreditRepository) Reject(id uint, reason string) error {
	res := r.db.Model(&model.CreditApplication{}).
		Where("id = ? AND status = ?", id, string(model.CreditPending)).
		Updates(map[string]interface{}{
			"status":        string(model.CreditRejected),
			"reject_reason": reason,
			"reviewed_at":   time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
// CreditState returns the user's credit limit / used / available.
func (r *CreditRepository) CreditState(userID uint) (limit, used int64, err error) {
	var u model.User
	if err := r.db.Select("credit_limit", "credit_used").First(&u, userID).Error; err != nil {
		return 0, 0, err
	}
	return u.CreditLimit, u.CreditUsed, nil
}

// DeductCredit atomically consumes tokens from the user's available credit
// (credit_used += tokens). Returns ErrCreditInsufficient when the available
// credit is not enough.
func (r *CreditRepository) DeductCredit(userID uint, tokens int64) error {
	if tokens <= 0 {
		return nil
	}
	res := r.db.Exec(
		"UPDATE users SET credit_used = credit_used + ? WHERE id = ? AND credit_limit - credit_used >= ?",
		tokens, userID, tokens)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCreditInsufficient
	}
	return nil
}

// RepayCredit atomically repays outstanding credit (credit_used -= tokens)
// using the user's purchased token credits (token_credits). Both must be
// sufficient, otherwise ErrCreditRepayInsufficient is returned.
func (r *CreditRepository) RepayCredit(userID uint, tokens int64) error {
	if tokens <= 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(
			"UPDATE users SET token_credits = token_credits - ? WHERE id = ? AND token_credits >= ?",
			tokens, userID, tokens)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrCreditRepayInsufficient
		}
		res = tx.Exec(
			"UPDATE users SET credit_used = credit_used - ? WHERE id = ? AND credit_used >= ?",
			tokens, userID, tokens)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrCreditRepayInsufficient
		}
		return nil
	})
}

// AddCollection records a 催账 (credit collection) action.
func (r *CreditRepository) AddCollection(col *model.CreditCollection) error {
	return r.db.Create(col).Error
}

// ListCollections returns collection records newest first.
func (r *CreditRepository) ListCollections(page, size int) ([]model.CreditCollection, int64, error) {
	var total int64
	if err := r.db.Model(&model.CreditCollection{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.CreditCollection, 0, size)
	err := r.db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

package repository

import (
	"errors"
	"time"

	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

// ErrCouponAlreadyUsed is returned when a coupon has already been redeemed.
var ErrCouponAlreadyUsed = errors.New("coupon already used")

type ResetCouponRepository struct {
	db *gorm.DB
}

func NewResetCouponRepository(db *gorm.DB) *ResetCouponRepository {
	return &ResetCouponRepository{db: db}
}

// Create inserts a reset coupon.
func (r *ResetCouponRepository) Create(coupon *model.ResetCoupon) error {
	return r.db.Create(coupon).Error
}

// CreateBatch inserts multiple reset coupons in one statement.
func (r *ResetCouponRepository) CreateBatch(coupons []model.ResetCoupon) error {
	return r.db.Create(&coupons).Error
}

// FindByID retrieves a coupon by ID.
func (r *ResetCouponRepository) FindByID(id uint) (*model.ResetCoupon, error) {
	var coupon model.ResetCoupon
	err := r.db.First(&coupon, id).Error
	return &coupon, err
}

// ListByUserID returns the coupons bound to a user, newest first.
func (r *ResetCouponRepository) ListByUserID(userID uint) ([]model.ResetCoupon, error) {
	var coupons []model.ResetCoupon
	err := r.db.Where("user_id = ?", userID).
		Order("id DESC").
		Find(&coupons).Error
	return coupons, err
}

// ListPaginated returns coupons with an optional user filter, newest first.
func (r *ResetCouponRepository) ListPaginated(page, size int, userID *uint) ([]model.ResetCoupon, int64, error) {
	var coupons []model.ResetCoupon
	var total int64
	query := r.db.Model(&model.ResetCoupon{})
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	query.Count(&total)
	err := query.Order("id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&coupons).Error
	return coupons, total, err
}

// RedeemWithReset atomically marks a coupon as used and resets the used token
// quota of all of the user's active subscriptions to zero. It returns the
// number of subscriptions reset. Concurrent redemptions are safe: only one
// wins, the loser gets ErrCouponAlreadyUsed.
func (r *ResetCouponRepository) RedeemWithReset(couponID, userID uint, usedAt time.Time) (int, error) {
	resetCount := 0
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.ResetCoupon{}).
			Where("id = ? AND user_id = ? AND status = ?", couponID, userID, "unused").
			Updates(map[string]interface{}{
				"status":  "used",
				"used_at": usedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrCouponAlreadyUsed
		}
		subRes := tx.Model(&model.Subscription{}).
			Where("user_id = ? AND status = ?", userID, "active").
			Update("used_tokens", 0)
		if subRes.Error != nil {
			return subRes.Error
		}
		resetCount = int(subRes.RowsAffected)
		return nil
	})
	return resetCount, err
}

// MarkUsed atomically marks a coupon as used if it is still unused.
// It returns rows affected so callers can detect double redemption.
func (r *ResetCouponRepository) MarkUsed(id uint, usedAt interface{}) (int64, error) {
	res := r.db.Model(&model.ResetCoupon{}).
		Where("id = ? AND status = ?", id, "unused").
		Updates(map[string]interface{}{
			"status":  "used",
			"used_at": usedAt,
		})
	return res.RowsAffected, res.Error
}

// CountUnusedByUserID counts unused coupons for a user.
func (r *ResetCouponRepository) CountUnusedByUserID(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.ResetCoupon{}).
		Where("user_id = ? AND status = ?", userID, "unused").
		Count(&count).Error
	return count, err
}
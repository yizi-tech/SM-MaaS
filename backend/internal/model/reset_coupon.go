package model

import "time"

// ResetCoupon is an admin-issued coupon that resets a user's used subscription
// token quota to zero when redeemed. Coupons are bound to a specific user.
type ResetCoupon struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Code      string     `gorm:"size:64;uniqueIndex;not null" json:"code"`
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	Status    string     `gorm:"size:20;default:unused" json:"status"` // unused | used
	Note      string     `gorm:"size:200" json:"note"`
	IssuedBy  uint       `json:"issued_by"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
}
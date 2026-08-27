package repository

import (
	"time"

	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create inserts a single notification.
func (r *NotificationRepository) Create(n *model.Notification) error {
	return r.db.Create(n).Error
}

// CreateBatch inserts notifications for multiple users in one transaction.
func (r *NotificationRepository) CreateBatch(userIDs []uint, title, content, ntype string, issuedBy uint) (int, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, uid := range userIDs {
			n := model.Notification{
				UserID:   uid,
				Title:    title,
				Content:  content,
				Type:     ntype,
				IssuedBy: issuedBy,
			}
			if err := tx.Create(&n).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return len(userIDs), err
}

// ListByUserID returns the notifications of a user, newest first.
func (r *NotificationRepository) ListByUserID(userID uint, page, size int) ([]model.Notification, int64, error) {
	var items []model.Notification
	var total int64
	q := r.db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&items).Error
	return items, total, err
}

// UnreadCount returns the number of unread notifications for a user.
func (r *NotificationRepository) UnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	return count, err
}

// MarkRead marks a single notification as read (only if it belongs to the user).
func (r *NotificationRepository) MarkRead(id, userID uint, at time.Time) (int64, error) {
	res := r.db.Model(&model.Notification{}).
		Where("id = ? AND user_id = ? AND is_read = ?", id, userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": at,
		})
	return res.RowsAffected, res.Error
}

// MarkAllRead marks all notifications of a user as read.
func (r *NotificationRepository) MarkAllRead(userID uint, at time.Time) (int64, error) {
	res := r.db.Model(&model.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": at,
		})
	return res.RowsAffected, res.Error
}

// ListPaginated returns all notifications (admin view), newest first.
func (r *NotificationRepository) ListPaginated(page, size int) ([]model.Notification, int64, error) {
	var items []model.Notification
	var total int64
	if err := r.db.Model(&model.Notification{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	err := r.db.Order("created_at DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&items).Error
	return items, total, err
}
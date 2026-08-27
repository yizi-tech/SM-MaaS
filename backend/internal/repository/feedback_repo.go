package repository

import (
	"errors"

	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

type ConversationLogRepository struct {
	db *gorm.DB
}

func NewConversationLogRepository(db *gorm.DB) *ConversationLogRepository {
	return &ConversationLogRepository{db: db}
}

func (r *ConversationLogRepository) Create(log *model.ConversationLog) error {
	return r.db.Create(log).Error
}

// ListByUser returns paginated conversation logs for a user, ordered newest
// first. model and status filters are optional (empty = all).
func (r *ConversationLogRepository) ListByUser(userID uint, page, size int, modelName string) ([]model.ConversationLog, int64, error) {
	q := r.db.Model(&model.ConversationLog{}).Where("user_id = ?", userID)
	if modelName != "" {
		q = q.Where("model = ?", modelName)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.ConversationLog
	err := q.Order("id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&logs).Error
	return logs, total, err
}

func (r *ConversationLogRepository) FindByUserAndID(userID, id uint) (*model.ConversationLog, error) {
	var log model.ConversationLog
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// ListExport returns all logs for a user (capped at 10000) for JSONL export.
func (r *ConversationLogRepository) ListExport(userID uint) ([]model.ConversationLog, error) {
	var logs []model.ConversationLog
	err := r.db.Where("user_id = ?", userID).
		Order("id ASC").
		Limit(10000).
		Find(&logs).Error
	return logs, err
}

func (r *ConversationLogRepository) DistinctModels(userID uint) ([]string, error) {
	var models []string
	err := r.db.Model(&model.ConversationLog{}).
		Where("user_id = ?", userID).
		Distinct().
		Order("model ASC").
		Pluck("model", &models).Error
	return models, err
}

// ConversationWithUser is a conversation log joined with the owner's email,
// used by the admin "all calls" view.
type ConversationWithUser struct {
	model.ConversationLog
	UserEmail string `json:"user_email" gorm:"column:user_email"`
}

// ListAll returns paginated conversation logs across ALL users with optional
// filters. userID, model and status are optional (zero/empty = no filter).
func (r *ConversationLogRepository) ListAll(page, size int, userID uint, modelName, status string) ([]ConversationWithUser, int64, error) {
	q := r.db.Model(&model.ConversationLog{}).
		Select("conversation_logs.*, users.email as user_email").
		Joins("LEFT JOIN users ON users.id = conversation_logs.user_id")
	if userID > 0 {
		q = q.Where("conversation_logs.user_id = ?", userID)
	}
	if modelName != "" {
		q = q.Where("conversation_logs.model = ?", modelName)
	}
	if status != "" {
		q = q.Where("conversation_logs.status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []ConversationWithUser
	err := q.Order("conversation_logs.id DESC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&logs).Error
	return logs, total, err
}

// FindByID returns a single conversation log by id regardless of owner.
func (r *ConversationLogRepository) FindByID(id uint) (*model.ConversationLog, error) {
	var log model.ConversationLog
	if err := r.db.Where("id = ?", id).First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

// ExportAll returns all conversation logs (capped at 10000) for JSONL export,
// optionally filtered. Used by the admin export endpoint.
func (r *ConversationLogRepository) ExportAll(userID uint, modelName, status string) ([]model.ConversationLog, error) {
	q := r.db.Model(&model.ConversationLog{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if modelName != "" {
		q = q.Where("model = ?", modelName)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var logs []model.ConversationLog
	err := q.Order("id ASC").Limit(10000).Find(&logs).Error
	return logs, err
}

// DistinctModelsAll returns all distinct models present in conversation logs.
func (r *ConversationLogRepository) DistinctModelsAll() ([]string, error) {
	var models []string
	err := r.db.Model(&model.ConversationLog{}).
		Distinct().
		Order("model ASC").
		Pluck("model", &models).Error
	return models, err
}

// ---------------------------------------------------------------------------
// Feedback
// ---------------------------------------------------------------------------

type FeedbackRepository struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

func (r *FeedbackRepository) Create(f *model.Feedback) error {
	return r.db.Create(f).Error
}

func (r *FeedbackRepository) ListByUser(userID uint, page, size int) ([]model.Feedback, int64, error) {
	q := r.db.Model(&model.Feedback{}).Where("user_id = ?", userID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Feedback
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

func (r *FeedbackRepository) FindByUserAndID(userID, id uint) (*model.Feedback, error) {
	var f model.Feedback
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

func (r *FeedbackRepository) ListAll(page, size int, status string) ([]model.Feedback, int64, error) {
	q := r.db.Model(&model.Feedback{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Feedback
	err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

// SetNote updates the admin note shown to the user.
func (r *FeedbackRepository) SetNote(id uint, note string) error {
	return r.db.Model(&model.Feedback{}).Where("id = ?", id).Update("admin_note", note).Error
}

// SetStatus updates feedback status and records resolution time.
func (r *FeedbackRepository) SetStatus(id uint, status string) error {
	updates := map[string]interface{}{"status": status}
	if status == "resolved" {
		updates["resolved_at"] = gorm.Expr("CURRENT_TIMESTAMP")
	} else {
		updates["resolved_at"] = nil
	}
	return r.db.Model(&model.Feedback{}).Where("id = ?", id).Updates(updates).Error
}
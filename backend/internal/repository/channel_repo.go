package repository

import (
	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

type ChannelRepository struct {
	db *gorm.DB
}

func NewChannelRepository(db *gorm.DB) *ChannelRepository {
	return &ChannelRepository{db: db}
}

// List returns all channels ordered by priority (desc) then id.
func (r *ChannelRepository) List() ([]model.LLMChannel, error) {
	var list []model.LLMChannel
	err := r.db.Order("priority DESC, id ASC").Find(&list).Error
	return list, err
}

// ListEnabled returns all enabled channels ordered by priority (desc).
func (r *ChannelRepository) ListEnabled() ([]model.LLMChannel, error) {
	var list []model.LLMChannel
	err := r.db.Where("enabled = ?", true).Order("priority DESC, id ASC").Find(&list).Error
	return list, err
}

func (r *ChannelRepository) FindByID(id uint) (*model.LLMChannel, error) {
	var ch model.LLMChannel
	err := r.db.First(&ch, id).Error
	return &ch, err
}

func (r *ChannelRepository) Create(ch *model.LLMChannel) error {
	return r.db.Create(ch).Error
}

func (r *ChannelRepository) Update(ch *model.LLMChannel) error {
	return r.db.Save(ch).Error
}

func (r *ChannelRepository) Delete(id uint) error {
	return r.db.Delete(&model.LLMChannel{}, id).Error
}
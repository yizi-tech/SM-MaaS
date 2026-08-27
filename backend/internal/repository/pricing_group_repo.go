package repository

import (
	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

type PricingGroupRepository struct {
	db *gorm.DB
}

func NewPricingGroupRepository(db *gorm.DB) *PricingGroupRepository {
	return &PricingGroupRepository{db: db}
}

// List returns all pricing groups ordered by id.
func (r *PricingGroupRepository) List() ([]model.PricingGroup, error) {
	var list []model.PricingGroup
	err := r.db.Order("id ASC").Find(&list).Error
	return list, err
}

func (r *PricingGroupRepository) FindByID(id uint) (*model.PricingGroup, error) {
	var g model.PricingGroup
	err := r.db.First(&g, id).Error
	return &g, err
}

func (r *PricingGroupRepository) Create(g *model.PricingGroup) error {
	return r.db.Create(g).Error
}

func (r *PricingGroupRepository) Update(g *model.PricingGroup) error {
	return r.db.Save(g).Error
}

func (r *PricingGroupRepository) Delete(id uint) error {
	return r.db.Delete(&model.PricingGroup{}, id).Error
}
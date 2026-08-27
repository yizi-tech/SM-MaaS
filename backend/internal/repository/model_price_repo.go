package repository

import (
	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

type ModelPriceRepository struct {
	db *gorm.DB
}

func NewModelPriceRepository(db *gorm.DB) *ModelPriceRepository {
	return &ModelPriceRepository{db: db}
}

// List returns all model prices ordered by id.
func (r *ModelPriceRepository) List() ([]model.ModelPrice, error) {
	var list []model.ModelPrice
	err := r.db.Order("id ASC").Find(&list).Error
	return list, err
}

// FindByID returns a model price entry by its primary key.
func (r *ModelPriceRepository) FindByID(id uint) (*model.ModelPrice, error) {
	var p model.ModelPrice
	err := r.db.First(&p, id).Error
	return &p, err
}

// FindByModel returns the price entry for an exact model name.
func (r *ModelPriceRepository) FindByModel(modelName string) (*model.ModelPrice, error) {
	var p model.ModelPrice
	err := r.db.Where("model = ?", modelName).First(&p).Error
	return &p, err
}

// Create inserts a new model price entry.
func (r *ModelPriceRepository) Create(p *model.ModelPrice) error {
	return r.db.Create(p).Error
}

// Update saves changes to an existing model price entry.
func (r *ModelPriceRepository) Update(p *model.ModelPrice) error {
	return r.db.Save(p).Error
}

// Delete removes a model price entry by id.
func (r *ModelPriceRepository) Delete(id uint) error {
	return r.db.Delete(&model.ModelPrice{}, id).Error
}
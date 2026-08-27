package repository

import (
	"github.com/mass-platform/backend/internal/model"
	"gorm.io/gorm"
)

type TokenPackageRepository struct {
	db *gorm.DB
}

func NewTokenPackageRepository(db *gorm.DB) *TokenPackageRepository {
	return &TokenPackageRepository{db: db}
}

// ListActive returns all active token packages ordered by sort_order.
func (r *TokenPackageRepository) ListActive() ([]model.TokenPackage, error) {
	var list []model.TokenPackage
	err := r.db.Where("status = ?", "active").
		Order("sort_order ASC, id ASC").
		Find(&list).Error
	return list, err
}

// FindByID retrieves a token package by its ID.
func (r *TokenPackageRepository) FindByID(id uint) (*model.TokenPackage, error) {
	var pkg model.TokenPackage
	err := r.db.First(&pkg, id).Error
	return &pkg, err
}

// ListAll returns all token packages (admin view), including inactive ones.
func (r *TokenPackageRepository) ListAll() ([]model.TokenPackage, error) {
	var list []model.TokenPackage
	err := r.db.Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

// Create inserts a new token package.
func (r *TokenPackageRepository) Create(pkg *model.TokenPackage) error {
	return r.db.Create(pkg).Error
}

// Update persists changes to an existing token package.
func (r *TokenPackageRepository) Update(pkg *model.TokenPackage) error {
	return r.db.Save(pkg).Error
}

// Delete removes a token package by ID.
func (r *TokenPackageRepository) Delete(id uint) error {
	return r.db.Delete(&model.TokenPackage{}, id).Error
}

// SetStatus toggles a token package's status (active / inactive).
func (r *TokenPackageRepository) SetStatus(id uint, status string) error {
	return r.db.Model(&model.TokenPackage{}).
		Where("id = ?", id).
		Update("status", status).Error
}
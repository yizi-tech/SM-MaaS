package repository

import (
	"time"

	"github.com/mass-platform/backend/internal/model"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

// Create inserts a new invoice application.
func (r *InvoiceRepository) Create(inv *model.Invoice) error {
	return r.db.Create(inv).Error
}

// FindByID retrieves an invoice by ID.
func (r *InvoiceRepository) FindByID(id uint) (*model.Invoice, error) {
	var inv model.Invoice
	err := r.db.First(&inv, id).Error
	return &inv, err
}

// ListByUserID returns the invoices of a user, newest first.
func (r *InvoiceRepository) ListByUserID(userID uint, page, size int) ([]model.Invoice, int64, error) {
	var items []model.Invoice
	var total int64
	q := r.db.Model(&model.Invoice{}).Where("user_id = ?", userID)
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

// ListPaginated returns all invoices (admin view) with optional status filter.
func (r *InvoiceRepository) ListPaginated(page, size int, status string) ([]model.Invoice, int64, error) {
	var items []model.Invoice
	var total int64
	q := r.db.Model(&model.Invoice{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q = r.db.Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

// Update persists changes to an invoice.
func (r *InvoiceRepository) Update(inv *model.Invoice) error {
	return r.db.Save(inv).Error
}

// IssuedAmount returns the total amount already occupied by pending and
// issued invoices of a user (used to compute the remaining invoice quota).
func (r *InvoiceRepository) IssuedAmount(userID uint) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.db.Model(&model.Invoice{}).
		Where("user_id = ? AND status IN ?", userID, []string{"pending", "issued"}).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}

// RechargeTotal returns the total amount successfully recharged by a user.
func (r *InvoiceRepository) RechargeTotal(userID uint) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.db.Model(&model.Transaction{}).
		Where("user_id = ? AND type = ? AND status = ?", userID, model.TransactionRecharge, model.TransactionSuccess).
		Select("COALESCE(SUM(amount),0)").Scan(&sum).Error
	return sum, err
}

// Issue marks an invoice as issued with the given invoice number.
func (r *InvoiceRepository) Issue(id uint, invoiceNo string, issuedAt time.Time) error {
	return r.db.Model(&model.Invoice{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{
			"status":     "issued",
			"invoice_no": invoiceNo,
			"issued_at":  issuedAt,
		}).Error
}

// Reject marks an invoice as rejected with a reason.
func (r *InvoiceRepository) Reject(id uint, reason string) error {
	return r.db.Model(&model.Invoice{}).
		Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]interface{}{
			"status":        "rejected",
			"reject_reason": reason,
		}).Error
}
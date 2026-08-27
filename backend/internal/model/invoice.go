package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Invoice 发票申请：用户从已充值金额中申请开票，
// 管理端审核后开具（填写发票号码）或驳回。
type Invoice struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	UserID      uint            `gorm:"index;not null" json:"user_id"`
	InvoiceNo   string          `gorm:"size:100" json:"invoice_no"` // 管理端开具时填写
	Amount      decimal.Decimal `gorm:"type:decimal(20,6);not null" json:"amount"`
	TitleType   string          `gorm:"size:20;default:company" json:"title_type"` // company | personal
	InvoiceType string          `gorm:"size:20;default:normal" json:"invoice_type"` // normal 普票 | vat 专票
	Title       string          `gorm:"size:200;not null" json:"title"`             // 发票抬头
	TaxNo       string          `gorm:"size:50" json:"tax_no"`                      // 税号（企业必填）
	BankName    string          `gorm:"size:100" json:"bank_name"`                  // 开户行（专票）
	BankAccount string          `gorm:"size:50" json:"bank_account"`                // 银行账号（专票）
	Address     string          `gorm:"size:200" json:"address"`                    // 注册地址（专票）
	Phone       string          `gorm:"size:30" json:"phone"`                       // 注册电话（专票）
	Email       string          `gorm:"size:100" json:"email"`                      // 发票接收邮箱
	Status      string          `gorm:"size:20;default:pending;index" json:"status"` // pending | issued | rejected
	RejectReason string         `gorm:"size:500" json:"reject_reason"`
	Remark      string          `gorm:"size:500" json:"remark"` // 用户备注
	IssuedAt    *time.Time      `json:"issued_at"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (Invoice) TableName() string { return "invoices" }
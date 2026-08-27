package model

import "time"

// Notification 站内通知：管理端可对指定用户或全员发送，
// 用户端可查看、标记已读。
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Title     string    `gorm:"size:128;not null" json:"title"`
	Content   string    `gorm:"size:2000;not null" json:"content"`
	Type      string    `gorm:"size:32;default:system" json:"type"`
	IsRead    bool      `gorm:"default:false" json:"is_read"`
	IssuedBy  uint      `json:"issued_by"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }
package model

import "time"

type SystemLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Level     string    `gorm:"size:20;index" json:"level"`
	Module    string    `gorm:"size:50;index" json:"module"`
	Action    string    `gorm:"size:100" json:"action"`
	UserID    *uint     `gorm:"index" json:"user_id"`
	IP        string    `gorm:"size:45" json:"ip"`
	RequestID string    `gorm:"size:100;index" json:"request_id"`
	Message   string    `gorm:"size:1000" json:"message"`
	Details   string    `gorm:"type:text" json:"details"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

type SystemAlert struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"size:50;index" json:"type"`
	Level     string    `gorm:"size:20" json:"level"`
	Title     string    `gorm:"size:200" json:"title"`
	Message   string    `gorm:"size:1000" json:"message"`
	Status    string    `gorm:"size:20;default:open" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

type SystemMetrics struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Timestamp       time.Time `gorm:"index" json:"timestamp"`
	TotalRequests   int64     `json:"total_requests"`
	SuccessRequests int64     `json:"success_requests"`
	FailedRequests  int64     `json:"failed_requests"`
	TotalTokens     int64     `json:"total_tokens"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	ActiveUsers     int       `json:"active_users"`
	ActiveKeys      int       `json:"active_keys"`
	Revenue         float64   `json:"revenue"`
	CreatedAt       time.Time `json:"created_at"`
}

func (SystemLog) TableName() string {
	return "system_logs"
}

func (SystemAlert) TableName() string {
	return "system_alerts"
}

func (SystemMetrics) TableName() string {
	return "system_metrics"
}
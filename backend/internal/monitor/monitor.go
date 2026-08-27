package monitor

import (
	"context"
	"time"

	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	HTTPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mass_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mass_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	LLMRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mass_llm_requests_total",
			Help: "Total number of LLM requests",
		},
		[]string{"model", "provider", "status"},
	)

	LLMTokenTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mass_llm_tokens_total",
			Help: "Total number of LLM tokens processed",
		},
		[]string{"model", "type"},
	)

	ActiveUsersGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mass_active_users",
			Help: "Number of active users",
		},
	)

	ActiveAPIKeysGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mass_active_api_keys",
			Help: "Number of active API keys",
		},
	)

	RevenueTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mass_revenue_total",
			Help: "Total revenue",
		},
		[]string{"type"},
	)

	SystemHealthGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mass_system_health",
			Help: "System health status (1=healthy, 0=unhealthy)",
		},
		[]string{"component"},
	)
)

type MonitorService struct {
	metricsRepo MetricsRepository
	logRepo     LogRepository
	db          *gorm.DB
	redis       *redis.Client
	startedAt   time.Time
}

type MetricsRepository interface {
	Create(m *model.SystemMetrics) error
	GetLatest() (*model.SystemMetrics, error)
	GetRange(start, end time.Time) ([]model.SystemMetrics, error)
}

type LogRepository interface {
	Create(log *model.SystemLog) error
	List(page, size int, filters map[string]interface{}) ([]model.SystemLog, int64, error)
}

func NewMonitorService(metricsRepo MetricsRepository, logRepo LogRepository, db *gorm.DB, redis *redis.Client) *MonitorService {
	return &MonitorService{
		metricsRepo: metricsRepo,
		logRepo:     logRepo,
		db:          db,
		redis:       redis,
		startedAt:   time.Now(),
	}
}

// collectSnapshot aggregates real usage data from the database.
func (s *MonitorService) collectSnapshot() *model.SystemMetrics {
	m := &model.SystemMetrics{Timestamp: time.Now()}
	if s.db == nil {
		return m
	}
	var req int64
	var tokens int64
	var revenue float64
	_ = s.db.Model(&model.BillingRecord{}).Count(&req).Error
	_ = s.db.Model(&model.BillingRecord{}).Select("COALESCE(SUM(total_tokens),0)").Scan(&tokens).Error
	_ = s.db.Model(&model.Transaction{}).
		Where("status = ?", "success").
		Select("COALESCE(SUM(amount),0)").Scan(&revenue).Error
	m.TotalRequests = req
	m.SuccessRequests = req
	m.TotalTokens = tokens
	m.Revenue = revenue
	var activeUsers, activeKeys int64
	_ = s.db.Model(&model.User{}).Where("status = ?", "active").Count(&activeUsers).Error
	_ = s.db.Model(&model.ApiKey{}).Where("status = ?", "active").Count(&activeKeys).Error
	m.ActiveUsers = int(activeUsers)
	m.ActiveKeys = int(activeKeys)
	return m
}

func (s *MonitorService) RecordMetrics() {
	metrics := s.collectSnapshot()
	if err := s.metricsRepo.Create(metrics); err != nil {
		logging.Error("monitor", "record_metrics", "failed to record metrics", err, nil)
	}
}

// GetSystemHealth performs real health checks on each component.
func (s *MonitorService) GetSystemHealth() map[string]bool {
	health := map[string]bool{
		"database": false,
		"redis":    false,
		"api":      true,
	}
	if s.db != nil {
		sqlDB, err := s.db.DB()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			health["database"] = sqlDB.PingContext(ctx) == nil
			cancel()
		}
	}
	if s.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		health["redis"] = s.redis.Ping(ctx).Err() == nil
		cancel()
	}
	return health
}

func (s *MonitorService) CollectAndStoreMetrics(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RecordMetrics()
		}
	}
}

func (s *MonitorService) LogOperation(level, module, action, message string, userID *uint, ip, requestID, details string) {
	log := &model.SystemLog{
		Level:     level,
		Module:    module,
		Action:    action,
		UserID:    userID,
		IP:        ip,
		RequestID: requestID,
		Message:   message,
		Details:   details,
	}
	if err := s.logRepo.Create(log); err != nil {
		logging.Error("monitor", "log_operation", "failed to save system log", err, nil)
	}
}

func (s *MonitorService) GetLogs(page, size int, filters map[string]interface{}) ([]model.SystemLog, int64, error) {
	return s.logRepo.List(page, size, filters)
}

func (s *MonitorService) GetMetrics(start, end time.Time) ([]model.SystemMetrics, error) {
	return s.metricsRepo.GetRange(start, end)
}

func (s *MonitorService) GetLatestMetrics() (*model.SystemMetrics, error) {
	return s.metricsRepo.GetLatest()
}
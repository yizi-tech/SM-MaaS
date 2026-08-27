package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mass-platform/backend/internal/api/handler"
	"github.com/mass-platform/backend/internal/api/router"
	"github.com/mass-platform/backend/internal/auth"
	"github.com/mass-platform/backend/internal/billing"
	"github.com/mass-platform/backend/internal/config"
	"github.com/mass-platform/backend/internal/llm/provider"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/internal/monitor"
	"github.com/mass-platform/backend/internal/rate"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/internal/service"
	"github.com/mass-platform/backend/pkg/database"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Hard-fail on insecure default credentials unless explicitly opted out
	// (MASS_ALLOW_INSECURE_DEFAULTS=true). Production must set strong JWT_SECRET
	// and DB_PASSWORD.
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}

	// Initialize logger
	logging.Init(cfg.Log.Level, cfg.Log.Output)
	logging.Logger.Info().Msg("Starting MASS Platform...")

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize database
	db := database.Init(&cfg.Database)
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get sql.DB handle: %v", err)
	}

	// Auto migrate database
	model.AutoMigrate(db)

	// Seed initial data
	model.SeedData(db)

	// Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logging.Logger.Warn().Err(err).Msg("Redis connection failed, rate limiting will be limited")
	} else {
		logging.Logger.Info().Msg("Redis connected successfully")
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	apiKeyRepo := repository.NewApiKeyRepository(db)
	billingRepo := repository.NewBillingRecordRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	planRepo := repository.NewPlanRepository(db)
	subRepo := repository.NewSubscriptionRepository(db)
	identityRepo := repository.NewIdentityVerificationRepository(db)
	configRepo := repository.NewSystemConfigRepository(db)
	metricsRepo := repository.NewSystemMetricsRepository(db)
	logRepo := repository.NewSystemLogRepository(db)
	tokenPkgRepo := repository.NewTokenPackageRepository(db)
	resetCouponRepo := repository.NewResetCouponRepository(db)
	notifRepo := repository.NewNotificationRepository(db)
	invoiceRepo := repository.NewInvoiceRepository(db)
	creditRepo := repository.NewCreditRepository(db)
	channelRepo := repository.NewChannelRepository(db)
	pricingGroupRepo := repository.NewPricingGroupRepository(db)
	modelPriceRepo := repository.NewModelPriceRepository(db)

	// Ensure upload directory exists
	if err := os.MkdirAll(cfg.Upload.Dir, 0o755); err != nil {
		logging.Logger.Warn().Err(err).Str("dir", cfg.Upload.Dir).Msg("failed to create upload directory")
	}

	// Initialize billing-scoped repositories (billing package defines its own types)
	billingUserRepo := billing.NewUserRepository(db)
	billingRecordRepo := billing.NewBillingRecordRepository(db)
	billingTxRepo := billing.NewTransactionRepository(db)
	billingPlanRepo := billing.NewPlanRepository(db)
	billingSubRepo := billing.NewSubscriptionRepository(db)

	// Initialize services
	authService := auth.NewAuthService(&cfg.JWT, userRepo)
	billingService := billing.NewBillingService(db, billingUserRepo, billingRecordRepo, billingTxRepo, billingPlanRepo, billingSubRepo, modelPriceRepo)
	rateLimiter := rate.NewRateLimiter(rdb)
	monitorService := monitor.NewMonitorService(metricsRepo, logRepo, db, rdb)
	verifyCodeService := service.NewVerifyCodeService(rdb, cfg.SMTP, cfg.SMS, "LLM Maas")
	billingService.SetAlertSender(verifyCodeService)
	openidService := service.NewOpenIDService(rdb, configRepo)

	// Initialize LLM providers
	providerFactory := provider.NewProviderFactory()

	openAIProvider := provider.NewOpenAIProvider(cfg.LLM.OpenAIBaseURL, cfg.LLM.OpenAIAPIKey)
	anthropicProvider := provider.NewAnthropicProvider(cfg.LLM.AnthropicBaseURL, cfg.LLM.AnthropicAPIKey)

	providerFactory.Register("openai", openAIProvider)
	providerFactory.Register("anthropic", anthropicProvider)

	logging.Logger.Info().
		Str("openai_base_url", cfg.LLM.OpenAIBaseURL).
		Str("anthropic_base_url", cfg.LLM.AnthropicBaseURL).
		Msg("LLM providers registered")

	// Initialize handlers
	userHandler := handler.NewUserHandler(
		authService, userRepo, apiKeyRepo, billingService,
		billingRepo, txRepo, planRepo, subRepo, identityRepo,
		tokenPkgRepo, resetCouponRepo, notifRepo, configRepo, invoiceRepo, creditRepo, channelRepo, verifyCodeService, openidService, cfg.Upload.Dir, cfg.Upload.MaxSizeMB,
	)

	adminHandler := handler.NewAdminHandler(
		userRepo, apiKeyRepo, billingService, billingRepo, txRepo,
		planRepo, subRepo, identityRepo, monitorService,
		configRepo, metricsRepo, logRepo, channelRepo, pricingGroupRepo, modelPriceRepo, resetCouponRepo, notifRepo, tokenPkgRepo, invoiceRepo, creditRepo, authService,
	)

	convoRepo := repository.NewConversationLogRepository(db)
	fbRepo := repository.NewFeedbackRepository(db)
	convoHandler := handler.NewConversationHandler(convoRepo, fbRepo)

	llmHandler := handler.NewLLMHandler(
		providerFactory, channelRepo, rateLimiter, billingService, apiKeyRepo, userRepo, convoRepo,
	)

	// Setup router
	r := router.Setup(userHandler, adminHandler, llmHandler, convoHandler, authService, apiKeyRepo)

	// Root context that cancels background workers on shutdown.
	rootCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()

	// Start metrics collection
	go monitorService.CollectAndStoreMetrics(rootCtx, 5*time.Minute)

	// Start auto-renewal processor
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-ticker.C:
				billingService.AutoRenewSubscription(rootCtx)
				if _, err := billingService.ExpireSubscriptions(rootCtx); err != nil {
					logging.Logger.Warn().Err(err).Msg("subscription expiry run failed")
				}
			}
		}
	}()

	// Auto-cancel gateway orders (epay / wechat / alipay) left unpaid for
	// longer than 30 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-ticker.C:
				if _, err := billingService.ExpirePendingOrders(30 * time.Minute); err != nil {
					logging.Logger.Warn().Err(err).Msg("pending order expiry run failed")
				}
			}
		}
	}()

	// Periodically reconcile pending gateway orders (epay / wechat / alipay) by
	// querying the gateway directly. This catches payments whose async callback
	// was missed (downtime / network). lookback stays inside the 30-minute
	// expiry window so orders about to be cancelled still get a chance to settle.
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-ticker.C:
				if n, err := billingService.ReconcilePendingOrders(29 * time.Minute); err != nil {
					logging.Logger.Warn().Err(err).Msg("pending order reconciliation run failed")
				} else if n > 0 {
					logging.Logger.Info().Int64("settled", n).Msg("reconciled pending orders")
				}
			}
		}
	}()

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logging.Logger.Info().Int("port", cfg.Server.Port).Msg("Server is starting...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Logger.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop background workers, then drain in-flight HTTP requests.
	stopWorkers()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Release external resources held by the process.
	if err := rdb.Close(); err != nil {
		logging.Logger.Warn().Err(err).Msg("redis client close failed")
	}
	if err := sqlDB.Close(); err != nil {
		logging.Logger.Warn().Err(err).Msg("database close failed")
	}

	logging.Logger.Info().Msg("Server exited gracefully")
}

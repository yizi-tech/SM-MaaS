package router

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mass-platform/backend/internal/api/handler"
	"github.com/mass-platform/backend/internal/api/middleware"
	"github.com/mass-platform/backend/internal/auth"
	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/pkg/response"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Setup(
	userHandler *handler.UserHandler,
	adminHandler *handler.AdminHandler,
	llmHandler *handler.LLMHandler,
	convoHandler *handler.ConversationHandler,
	authService *auth.AuthService,
	apiKeyRepo *repository.ApiKeyRepository,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(middleware.RequestIDMiddleware())
	r.Use(middleware.CORSMiddleware())
	// Hard 20 MiB cap on any request body (protects against memory-exhaustion
	// DoS on handlers that bind full JSON bodies).
	r.Use(middleware.BodyLimitMiddleware(20 << 20))
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", llmHandler.HealthCheck)

	// 亦 OpenID 回调别名：IdP（account.yiziyun.com）注册的回调地址是
	// https://maas.yiziyun.com/oauth/yiziauth-login（域名在本机另一代理下），
	// 此处以公开路由承载同一回调处理逻辑。
	r.GET("/oauth/yiziauth-login", userHandler.OpenIDCallback)

	// 易支付异步通知（公开，无需鉴权；服务端验签）。
	// 网关以 GET 形式回调，兼容 POST。
	r.GET("/pay/epay/notify", userHandler.EpayNotify)
	r.POST("/pay/epay/notify", userHandler.EpayNotify)
	r.POST("/pay/wechat/notify", userHandler.WechatNotify)
	r.POST("/pay/alipay/notify", userHandler.AlipayNotify)

	// Prometheus metrics (admin JWT required: metric labels include model
	// routing and upstream channel names, which must not be public).
	r.GET("/metrics", middleware.AuthMiddleware(authService), middleware.AdminMiddleware(), gin.WrapH(promhttp.Handler()))

	// Serve built frontend static files.
	// MASS_FRONTEND_DIR points to the directory containing user/ and admin/ subfolders.
	frontendDir := os.Getenv("MASS_FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "../frontend"
	}
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		r.Static("/frontend", frontendDir)
		// Include an alias so a browser resolving /assets/... (old relative
		// ../assets paths from /user or /admin) also finds the shared assets.
		if assetsDir := frontendDir + "/assets"; func() bool { _, e := os.Stat(assetsDir); return e == nil }() {
			r.Static("/assets", assetsDir)
		}
		// Landing page images (logo / starmoon) referenced as /landing-assets/*
		if landingDir := frontendDir + "/landing-assets"; func() bool { _, e := os.Stat(landingDir); return e == nil }() {
			r.Static("/landing-assets", landingDir)
		}
		r.StaticFile("/", frontendDir+"/index.html")
		r.StaticFile("/index.html", frontendDir+"/index.html")
		// Favicon（浏览器默认请求 /favicon.ico）
		if _, err := os.Stat(filepath.Join(frontendDir, "favicon.ico")); err == nil {
			r.StaticFile("/favicon.ico", filepath.Join(frontendDir, "favicon.ico"))
		}
		// Serve the user/admin portals as real multi-page apps: any .html
		// under these prefixes is served from disk (API routes live under
		// /api/v1 and are matched first by the router).
		r.Static("/user", frontendDir+"/user")
		r.Static("/admin", frontendDir+"/admin")
	}

	// Serve uploaded files (identity card images etc.).
	// Only the owner or an admin may download them; directory listing is
	// disabled; responses force attachment download + nosniff.
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}
	if info, err := os.Stat(uploadDir); err == nil && info.IsDir() {
		uploads := r.Group("/uploads")
		uploads.Use(middleware.AuthMiddleware(authService))
		uploads.GET("/identity/:uid/*filepath", func(c *gin.Context) {
			uid, err := strconv.ParseUint(c.Param("uid"), 10, 64)
			if err != nil {
				response.BadRequest(c, "invalid user id")
				return
			}
			cur, _ := c.Get("user_id")
			role, _ := c.Get("role")
			roleStr := ""
			if rs, ok := role.(fmt.Stringer); ok {
				roleStr = rs.String()
			} else if rs, ok := role.(string); ok {
				roleStr = rs
			}
			isOwner := cur == uint(uid)
			if !isOwner && roleStr != "admin" {
				response.Forbidden(c, "access denied")
				return
			}
			rel := c.Param("filepath")
			if rel == "" || strings.Contains(rel, "..") {
				response.NotFound(c, "file not found")
				return
			}
			fp := filepath.Join(uploadDir, "identity", c.Param("uid"), filepath.FromSlash(rel))
			if _, err := os.Stat(fp); err != nil {
				response.NotFound(c, "file not found")
				return
			}
			c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(fp)+"\"")
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Cache-Control", "private, no-store")
			c.File(fp)
		})
	}

	// API v1
	v1 := r.Group("/api/v1")
	{
		// 易支付异步通知（公开，无需鉴权；服务端验签）。
		// 网关以 GET 形式回调，兼容 POST。
		v1.GET("/pay/epay/notify", userHandler.EpayNotify)
		v1.POST("/pay/epay/notify", userHandler.EpayNotify)

		// Public routes
		authRoutes := v1.Group("/auth")
		{
			authRoutes.POST("/register", userHandler.Register)
			authRoutes.POST("/login", userHandler.Login)
			authRoutes.POST("/send-code", userHandler.SendVerifyCode)
			authRoutes.GET("/openid/authorize", userHandler.OpenIDAuthorize)
			authRoutes.GET("/openid/callback", userHandler.OpenIDCallback)
			authRoutes.GET("/openid/config", userHandler.OpenIDConfig)
		}

		// LLM Gateway routes (API Key auth)
		llmRoutes := v1.Group("/llm")
		llmRoutes.Use(middleware.APIKeyAuthMiddleware(apiKeyRepo))
		{
			llmRoutes.POST("/chat/completions", llmHandler.ChatCompletions)
			llmRoutes.POST("/completions", llmHandler.Completions)
			llmRoutes.GET("/models", llmHandler.ListModels)
			llmRoutes.POST("/messages", llmHandler.AnthropicMessages)
		}

		// OpenAI SDK / Anthropic SDK compatible endpoints on the standard /v1
		// paths so third-party clients can point their base_url at the
		// gateway directly.
		compatRoutes := r.Group("/v1")
		compatRoutes.Use(middleware.APIKeyAuthMiddleware(apiKeyRepo))
		{
			compatRoutes.POST("/chat/completions", llmHandler.ChatCompletions)
			compatRoutes.POST("/completions", llmHandler.Completions)
			compatRoutes.GET("/models", llmHandler.ListModels)
			compatRoutes.POST("/messages", llmHandler.AnthropicMessages)
		}

		// Plans & model catalog (public)
		v1.GET("/plans", userHandler.GetPlans)
		v1.GET("/models", userHandler.GetModelCatalog)

		// Public site branding (site_name/logo/description/footer etc.) used to
		// render the console brand without hardcoded values.
		v1.GET("/site-config", adminHandler.GetPublicSiteConfig)

		// User routes (JWT auth required)
		userRoutes := v1.Group("/user")
		userRoutes.Use(middleware.AuthMiddleware(authService))
		{
			userRoutes.GET("/profile", userHandler.GetProfile)
			userRoutes.GET("/unlimited-firepower", userHandler.GetUnlimitedModels)
			userRoutes.PUT("/profile", userHandler.UpdateProfile)
			userRoutes.PUT("/token-alert", userHandler.UpdateTokenAlert)
			userRoutes.PUT("/password", userHandler.ChangePassword)
			userRoutes.POST("/password/send-code", userHandler.SendPasswordVerifyCode)

			// API Keys
			userRoutes.GET("/api-keys", userHandler.GetApiKeys)
			userRoutes.POST("/api-keys", userHandler.CreateApiKey)
			userRoutes.DELETE("/api-keys/:id", userHandler.DeleteApiKey)

			// Billing & Usage
			userRoutes.GET("/usage", userHandler.GetUsage)
			userRoutes.POST("/chat/completions", llmHandler.UserChatCompletions)
			userRoutes.GET("/billing-records", userHandler.GetBillingRecords)
			userRoutes.GET("/billing-records/:id", userHandler.GetBillingRecord)
			userRoutes.GET("/transactions", userHandler.GetTransactions)

			// Recharge
			userRoutes.GET("/payment-config", userHandler.GetPaymentConfig)
			userRoutes.POST("/recharge/epay", userHandler.CreateEpayOrder)
			userRoutes.GET("/recharge/epay/status", userHandler.GetEpayOrderStatus)
			userRoutes.POST("/recharge/epay/query", userHandler.QueryEpayOrder)
			userRoutes.POST("/recharge/wechat", userHandler.CreateWechatOrder)
			userRoutes.POST("/recharge/alipay", userHandler.CreateAlipayOrder)
			userRoutes.GET("/recharge/status", userHandler.GetOrderStatus)

			// Token packages (加油包)
			userRoutes.GET("/token-packages", userHandler.ListTokenPackages)
			userRoutes.POST("/token-packages/:id/purchase", userHandler.PurchaseTokenPackage)

			// File upload (e.g. identity card photos)
			userRoutes.POST("/upload", userHandler.UploadFile)

			// Subscription
			userRoutes.POST("/subscribe", userHandler.Subscribe)
			userRoutes.GET("/subscriptions", userHandler.GetSubscriptions)
			userRoutes.POST("/subscriptions/:id/cancel", userHandler.CancelSubscription)
			userRoutes.POST("/subscriptions/:id/auto-renew", userHandler.SetAutoRenew)

			// Reset coupons (重置券)
			userRoutes.GET("/reset-coupons", userHandler.ListMyResetCoupons)
			userRoutes.POST("/reset-coupons/:id/redeem", userHandler.RedeemResetCoupon)

			// Notifications (站内通知)
			userRoutes.GET("/notifications", userHandler.ListNotifications)
			userRoutes.GET("/notifications/unread-count", userHandler.GetNotificationUnread)
			userRoutes.PUT("/notifications/:id/read", userHandler.MarkNotificationRead)
			userRoutes.PUT("/notifications/read-all", userHandler.MarkAllNotificationsRead)

			// Invoices (发票)
			userRoutes.GET("/invoice-quota", userHandler.GetInvoiceQuota)
			userRoutes.POST("/invoices", userHandler.CreateInvoice)
			userRoutes.GET("/invoices", userHandler.ListMyInvoices)

			// Credit (Token 授信)
			userRoutes.GET("/credit/status", userHandler.GetCreditStatus)
			userRoutes.POST("/credit/apply", userHandler.ApplyCredit)
			userRoutes.POST("/credit/repay", userHandler.RepayCredit)

			// Identity Verification
			userRoutes.POST("/identity-verification", userHandler.SubmitIdentityVerification)
			userRoutes.GET("/identity-verification", userHandler.GetIdentityVerificationStatus)

			// Conversation retention (对话数据留存 / JSONL 导出)
			userRoutes.GET("/conversations", convoHandler.ListConversations)
			userRoutes.GET("/conversations/export.jsonl", convoHandler.ExportConversations)
			userRoutes.GET("/conversations/:id", convoHandler.GetConversation)

			// Program issue feedback (反馈)
			userRoutes.POST("/feedback", convoHandler.CreateFeedback)
			userRoutes.GET("/feedback", convoHandler.ListMyFeedback)
			userRoutes.GET("/feedback/:id", convoHandler.GetFeedback)

			// 亦 OpenID 绑定管理
			userRoutes.GET("/openid/status", userHandler.OpenIDStatus)
			userRoutes.POST("/openid/bind", userHandler.OpenIDBind)
			userRoutes.POST("/openid/unbind", userHandler.OpenIDUnbind)
		}

		// 桌面 GUI 开放接口（登录联动 / 毫秒级额度同步 / 模型选择）
		guiRoutes := v1.Group("/gui")
		{
			guiRoutes.POST("/login", userHandler.GUILogin)
			guiRoutes.GET("/session", middleware.AuthMiddleware(authService), userHandler.GUISession)
			guiRoutes.GET("/sync", middleware.AuthMiddleware(authService), userHandler.GUISync)
			guiRoutes.GET("/models", middleware.AuthMiddleware(authService), userHandler.GUIModels)
		}
	}

	// Admin routes (JWT auth + admin role required)
	adminRoutes := v1.Group("/admin")
	adminRoutes.Use(middleware.AuthMiddleware(authService))
	adminRoutes.Use(middleware.AdminMiddleware())
	{
		// User management
		adminRoutes.GET("/users", adminHandler.ListUsers)
		adminRoutes.GET("/users/:id", adminHandler.GetUserDetail)
		adminRoutes.PUT("/users/:id", adminHandler.UpdateUserInfo)
		adminRoutes.PUT("/users/:id/status", adminHandler.UpdateUserStatus)

		// Identity verification
		adminRoutes.GET("/identity-verifications", adminHandler.ListIdentityVerifications)
		adminRoutes.POST("/identity-verifications/:id/review", adminHandler.ReviewIdentityVerification)

		// Plan management
		adminRoutes.GET("/plans", adminHandler.ListPlans)
		adminRoutes.POST("/plans", adminHandler.CreatePlan)
		adminRoutes.PUT("/plans/:id", adminHandler.UpdatePlan)
		adminRoutes.DELETE("/plans/:id", adminHandler.DeletePlan)

		// Feedback management (反馈处理)
		adminRoutes.GET("/feedback", convoHandler.AdminListFeedback)
		adminRoutes.PUT("/feedback/:id/status", convoHandler.AdminUpdateFeedbackStatus)

		// Conversation / call details (调用详情：跨用户查看)
		adminRoutes.GET("/conversations", convoHandler.AdminListConversations)
		adminRoutes.GET("/conversations/export.jsonl", convoHandler.AdminExportConversations)
		adminRoutes.GET("/conversations/:id", convoHandler.AdminGetConversation)

		// LLM channel management (模型渠道)
		adminRoutes.GET("/channels", adminHandler.ListLLMChannels)
		adminRoutes.POST("/channels", adminHandler.AddLLMChannel)
		adminRoutes.POST("/channels/test", adminHandler.TestLLMChannel)
		adminRoutes.POST("/channels/:id/fetch-models", adminHandler.FetchChannelModels)
		adminRoutes.PUT("/channels/:id", adminHandler.UpdateLLMChannel)
		adminRoutes.DELETE("/channels/:id", adminHandler.DeleteLLMChannel)

		// Pricing group management (模型定价 / 分组倍率)
		adminRoutes.GET("/pricing-groups", adminHandler.ListPricingGroups)
		adminRoutes.POST("/pricing-groups", adminHandler.AddPricingGroup)
		adminRoutes.PUT("/pricing-groups/:id", adminHandler.UpdatePricingGroup)
		adminRoutes.DELETE("/pricing-groups/:id", adminHandler.DeletePricingGroup)

		// Model price table (模型价格表：唯一价格源，未配置模型不可调用)
		adminRoutes.GET("/model-prices", adminHandler.ListModelPrices)
		adminRoutes.POST("/model-prices", adminHandler.AddModelPrice)
		adminRoutes.POST("/model-prices/sync", adminHandler.SyncModelPrices)
		adminRoutes.PUT("/model-prices/:id", adminHandler.UpdateModelPrice)
		adminRoutes.DELETE("/model-prices/:id", adminHandler.DeleteModelPrice)
		adminRoutes.POST("/model-prices/:id/unlimited", adminHandler.ToggleModelUnlimited)

		// Reset coupons (重置券发放)
		adminRoutes.GET("/reset-coupons", adminHandler.ListResetCoupons)
		adminRoutes.POST("/reset-coupons", adminHandler.IssueResetCoupons)

		// Notifications (站内通知)
		adminRoutes.GET("/notifications", adminHandler.ListNotifications)
		adminRoutes.POST("/notifications", adminHandler.SendNotification)

		// Token packages (加油包配置)
		adminRoutes.GET("/token-packages", adminHandler.ListTokenPackages)
		adminRoutes.POST("/token-packages", adminHandler.CreateTokenPackage)
		adminRoutes.PUT("/token-packages/:id", adminHandler.UpdateTokenPackage)
		adminRoutes.DELETE("/token-packages/:id", adminHandler.DeleteTokenPackage)
		adminRoutes.PUT("/token-packages/:id/status", adminHandler.UpdateTokenPackageStatus)

		// Invoices (发票审核)
		adminRoutes.GET("/invoices", adminHandler.ListInvoices)
		adminRoutes.POST("/invoices/:id/issue", adminHandler.IssueInvoice)
		adminRoutes.POST("/invoices/:id/reject", adminHandler.RejectInvoice)

		// Credit (Token 授信审核)
		adminRoutes.GET("/credit-applications", adminHandler.ListCreditApplications)
		adminRoutes.POST("/credit-applications/:id/approve", adminHandler.ApproveCreditApplication)
		adminRoutes.POST("/credit-applications/:id/reject", adminHandler.RejectCreditApplication)

		// Credit collections (催账)
		adminRoutes.GET("/credit-collections", adminHandler.ListCreditCollections)
		adminRoutes.POST("/credit-collect", adminHandler.CollectCredit)

		// Orders
		adminRoutes.GET("/orders", adminHandler.ListOrders)

		// Analytics
		adminRoutes.GET("/analytics/overview", adminHandler.GetAnalyticsOverview)
		adminRoutes.GET("/analytics/revenue", adminHandler.GetRevenueAnalytics)
		adminRoutes.GET("/analytics/daily", adminHandler.GetDailyAnalytics)

		// System
		adminRoutes.GET("/config", adminHandler.GetSystemConfig)
		adminRoutes.PUT("/config/batch", adminHandler.UpdateSystemConfigs)
		adminRoutes.PUT("/config", adminHandler.UpdateSystemConfig)
		adminRoutes.GET("/logs", adminHandler.GetSystemLogs)
		adminRoutes.GET("/metrics", adminHandler.GetSystemMetrics)
		adminRoutes.GET("/health", adminHandler.GetSystemHealth)
	}

	// SPA fallback: client-side routes under /user/* and /admin/* (e.g.
	// /user/login, /user/plans) must serve the app's index.html instead of
	// 404. NoRoute only fires when no static file route matched, so real
	// hashed assets keep being served from disk.
	frontendRoot := frontendDir
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case path == "/user" || strings.HasPrefix(path, "/user/"):
			c.File(filepath.Join(frontendRoot, "user", "index.html"))
		case path == "/admin" || strings.HasPrefix(path, "/admin/"):
			c.File(filepath.Join(frontendRoot, "admin", "index.html"))
		default:
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		}
	})

	return r
}

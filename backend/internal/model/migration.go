package model

import (
	"fmt"
	"log"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) {
	err := db.AutoMigrate(
		&User{},
		&ApiKey{},
		&RateLimit{},
		&IdentityVerification{},
		&Plan{},
		&Subscription{},
		&BillingRecord{},
		&Transaction{},
		&TokenPackage{},
		&ResetCoupon{},
		&Notification{},
		&Invoice{},
	&CreditApplication{},
	&CreditCollection{},
		&LLMChannel{},
		&PricingGroup{},
		&ModelPrice{},
		&SystemConfig{},
		&SystemLog{},
		&SystemAlert{},
		&SystemMetrics{},
		&ConversationLog{},
		&Feedback{},
	)
	if err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}
	ensureDecimalPrecision(db)
	log.Println("Database migration completed successfully")
}

// ensureDecimalPrecision widens monetary columns so token-level precision is
// not lost. GORM's AutoMigrate does not always alter the numeric scale of
// existing columns, and token-level billing costs (e.g. 0.0015 CNY per token)
// are lost at scale 2. Money columns use scale 6; per-token model prices need
// scale 9 (a price of ¥0.001 per 1M tokens is 1e-9 per token).
func ensureDecimalPrecision(db *gorm.DB) {
	fix := func(table, column string, targetScale int) {
		var scale int
		if err := db.Raw(
			"SELECT numeric_scale FROM information_schema.columns WHERE table_name = ? AND column_name = ?",
			table, column,
		).Scan(&scale).Error; err != nil {
			log.Printf("Failed to check %s.%s precision: %v", table, column, err)
			return
		}
		if scale >= targetScale {
			return
		}
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s DECIMAL(20,%d)", table, column, targetScale)).Error; err != nil {
			log.Printf("Failed to alter %s.%s to decimal(20,%d): %v", table, column, targetScale, err)
		}
	}
	fix("users", "balance", 6)
	fix("transactions", "amount", 6)
	fix("transactions", "balance_before", 6)
	fix("transactions", "balance_after", 6)
	fix("model_prices", "input_price", 9)
	fix("model_prices", "output_price", 9)
}

func SeedData(db *gorm.DB) {
	// Create default plans
	plans := []Plan{
		{
			Name:           "Starter",
			Description:    "适合个人开发者和小型项目",
			Price:          MustParseDecimal("29.00"),
			Currency:       "CNY",
			DurationDays:   30,
			RPM:            30,
			TPM:            50000,
			IncludedTokens: 1000000,
			ConcurrentLimit: 5,
			ModelAccess:    []string{"gpt-3.5-turbo", "gpt-4o-mini"},
			Status:         "active",
			SortOrder:      1,
		},
		{
			Name:           "Professional",
			Description:    "适合中小团队和商业项目",
			Price:          MustParseDecimal("99.00"),
			Currency:       "CNY",
			DurationDays:   30,
			RPM:            120,
			TPM:            500000,
			IncludedTokens: 10000000,
			ConcurrentLimit: 20,
			ModelAccess:    []string{"gpt-3.5-turbo", "gpt-4", "gpt-4o", "gpt-4o-mini", "claude-3-sonnet", "claude-3-haiku"},
			Status:         "active",
			SortOrder:      2,
		},
		{
			Name:           "Enterprise",
			Description:    "适合大规模部署和团队协作",
			Price:          MustParseDecimal("299.00"),
			Currency:       "CNY",
			DurationDays:   30,
			RPM:            500,
			TPM:            2000000,
			IncludedTokens: 50000000,
			ConcurrentLimit: 100,
			ModelAccess:    []string{"gpt-3.5-turbo", "gpt-4", "gpt-4o", "gpt-4o-mini", "claude-3-opus", "claude-3-sonnet", "claude-3-haiku", "claude-3-5-sonnet"},
			Status:         "active",
			SortOrder:      3,
		},
	}

	for _, plan := range plans {
		var existing Plan
		if db.Where("name = ?", plan.Name).First(&existing).RowsAffected == 0 {
			db.Create(&plan)
		}
	}

	// Create default token packages (加油包)
	tokenPackages := []TokenPackage{
		{
			Name:        "体验包",
			Description: "适合快速试用，1 元即可上手体验",
			Tokens:      100000,
			BonusTokens: 0,
			Price:       MustParseDecimal("1.00"),
			Status:      "active",
			SortOrder:   1,
		},
		{
			Name:        "标准包",
			Description: "适合日常开发，性价比之选",
			Tokens:      1000000,
			BonusTokens: 100000,
			Price:       MustParseDecimal("9.90"),
			Status:      "active",
			SortOrder:   2,
		},
		{
			Name:        "进阶包",
			Description: "适合高频调用与测试场景",
			Tokens:      5000000,
			BonusTokens: 1000000,
			Price:       MustParseDecimal("39.90"),
			Status:      "active",
			SortOrder:   3,
		},
		{
			Name:        "专业包",
			Description: "适合生产环境大规模调用",
			Tokens:      20000000,
			BonusTokens: 5000000,
			Price:       MustParseDecimal("129.00"),
			Status:      "active",
			SortOrder:   4,
		},
	}

	for _, pkg := range tokenPackages {
		var existing TokenPackage
		if db.Where("name = ?", pkg.Name).First(&existing).RowsAffected == 0 {
			db.Create(&pkg)
		}
	}

	// Create default LLM channels (disabled until an API key is configured).
	// The gateway falls back to the env-configured providers when no enabled
	// channel matches, so seeding disabled channels is safe.
	defaultChannels := []LLMChannel{
		{
			Name:     "默认 OpenAI 渠道",
			Type:     "openai",
			BaseURL:  "https://api.openai.com",
			APIKey:   "",
			Models:   StringSlice{"gpt-*", "text-*", "davinci-*", "o1-*"},
			Priority: 10,
			Enabled:  false,
			Remark:   "用于 OpenAI 系模型，配置 API Key 后启用",
		},
		{
			Name:     "默认 Anthropic 渠道",
			Type:     "anthropic",
			BaseURL:  "https://api.anthropic.com",
			APIKey:   "",
			Models:   StringSlice{"claude-*"},
			Priority: 10,
			Enabled:  false,
			Remark:   "用于 Claude 系模型，配置 API Key 后启用",
		},
	}
	for _, ch := range defaultChannels {
		var existing LLMChannel
		if db.Where("name = ?", ch.Name).First(&existing).RowsAffected == 0 {
			db.Create(&ch)
		}
	}

	// Create default admin user
	var adminCount int64
	db.Model(&User{}).Where("role = ?", RoleAdmin).Count(&adminCount)
	if adminCount == 0 {
		admin := User{
			Email:        "admin@mass-platform.com",
			PasswordHash: "$2a$10$placeholder", // Will be set by service
			Nickname:     "Admin",
			Role:         RoleAdmin,
			Status:       UserStatusActive,
		}
		db.Create(&admin)
	}

	log.Println("Seed data created successfully")
}

func MustParseDecimal(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}
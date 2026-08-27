// Command seed-demo-users creates or resets the local MASS demo accounts.
// It must only be used in a development or test database.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mass-platform/backend/internal/config"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/pkg/database"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	adminEmail = "admin@mass-platform.com"
	userEmail  = "demo@mass-platform.com"
)

type demoUser struct {
	Email    string
	Password string
	Nickname string
	Role     model.UserRole
	Balance  decimal.Decimal
}

func main() {
	if os.Getenv("MASS_ALLOW_DEMO_SEED") != "true" {
		log.Fatal("refusing to seed demo users: set MASS_ALLOW_DEMO_SEED=true to confirm this is a non-production database")
	}

	// Credentials come from the environment, never hardcoded in source.
	adminPassword := os.Getenv("MASS_ADMIN_PASSWORD")
	userPassword := os.Getenv("MASS_DEMO_PASSWORD")
	if adminPassword == "" || userPassword == "" {
		log.Fatal("refusing to seed demo users: MASS_ADMIN_PASSWORD and MASS_DEMO_PASSWORD must be set")
	}

	cfg := config.Load()
	db := database.Init(&cfg.Database)

	// Ensure a newly provisioned development database has the users table.
	if err := db.AutoMigrate(&model.User{}); err != nil {
		log.Fatalf("failed to migrate users table: %v", err)
	}

	users := []demoUser{
		{
			Email:    adminEmail,
			Password: adminPassword,
			Nickname: "MASS 管理员",
			Role:     model.RoleAdmin,
			Balance:  decimal.NewFromInt(10000),
		},
		{
			Email:    userEmail,
			Password: userPassword,
			Nickname: "演示用户",
			Role:     model.RoleUser,
			Balance:  decimal.NewFromInt(100),
		},
	}

	for _, item := range users {
		if err := upsertDemoUser(db, item); err != nil {
			log.Fatalf("failed to seed %s: %v", item.Email, err)
		}
	}

	fmt.Println("Demo accounts initialized successfully (passwords come from environment, not printed)")
}

func upsertDemoUser(db *gorm.DB, item demoUser) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	var user model.User
	err = db.Where("email = ?", item.Email).First(&user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("find user: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		user = model.User{
			Email:          item.Email,
			PasswordHash:   string(hash),
			Nickname:       item.Nickname,
			Role:           item.Role,
			Status:         model.UserStatusActive,
			Balance:        item.Balance,
			RealNameStatus: "unverified",
		}
		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		fmt.Printf("Created %s\n", item.Email)
		return nil
	}

	updates := map[string]interface{}{
		"password_hash": string(hash),
		"nickname":      item.Nickname,
		"role":          item.Role,
		"status":        model.UserStatusActive,
		"balance":       item.Balance,
	}
	if err := db.Model(&user).Updates(updates).Error; err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	fmt.Printf("Updated %s\n", item.Email)
	return nil
}

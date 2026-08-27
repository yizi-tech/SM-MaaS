package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mass-platform/backend/internal/config"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/pkg/logging"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserDisabled       = errors.New("user account is disabled")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

type Claims struct {
	UserID uint           `json:"user_id"`
	Email  string         `json:"email"`
	Role   model.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	cfg      *config.JWTConfig
	userRepo UserRepository
}

type UserRepository interface {
	FindByEmail(email string) (*model.User, error)
	FindByID(id uint) (*model.User, error)
	Update(user *model.User) error
}

func NewAuthService(cfg *config.JWTConfig, userRepo UserRepository) *AuthService {
	return &AuthService{cfg: cfg, userRepo: userRepo}
}

func (s *AuthService) Register(email, password, nickname string) (*model.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Nickname:     nickname,
		Role:         model.RoleUser,
		Status:       model.UserStatusActive,
	}

	return user, nil
}

func (s *AuthService) Login(email, password string) (*model.User, string, error) {
	user, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}

	if user.Status != model.UserStatusActive {
		return nil, "", ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

func (s *AuthService) GenerateToken(user *model.User) (string, error) {
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.ExpireTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mass-platform",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Secret))
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.cfg.Secret), nil
	})

	if err != nil {
		logging.Logger.Error().Err(err).Msg("Token validation failed")
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Issuer != "mass-platform" {
		return nil, ErrInvalidToken
	}

	// Re-check the account is still active: a disabled/deleted user's token
	// must not keep working until it expires.
	user, err := s.userRepo.FindByID(claims.UserID)
	if err != nil || user == nil {
		return nil, ErrInvalidToken
	}
	if user.Status != model.UserStatusActive {
		return nil, ErrInvalidToken
	}

	// Re-check the role from the DB so demoted admins lose admin access
	// immediately instead of waiting for token expiry.
	if user.Role != claims.Role {
		claims.Role = user.Role
	}

	return claims, nil
}

func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	return s.userRepo.Update(user)
}

// VerifyOldPassword checks the given password against the user's stored hash
// without modifying anything. Used as a pre-check before consuming the
// one-time code in the change-password flow.
func (s *AuthService) VerifyOldPassword(userID uint, oldPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// API Key authentication
func ValidateAPIKey(keyHash string, apiKeyRepo APIKeyRepository) (*model.ApiKey, error) {
	apiKey, err := apiKeyRepo.FindByKeyHash(keyHash)
	if err != nil {
		return nil, errors.New("invalid API key")
	}

	if apiKey.Status != "active" {
		return nil, errors.New("API key is not active")
	}

	// Reject API keys whose owning account has been disabled/suspended.
	if apiKey.User.ID != 0 && apiKey.User.Status != model.UserStatusActive {
		return nil, errors.New("API key owner account is not active")
	}

	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, errors.New("API key has expired")
	}

	return apiKey, nil
}

type APIKeyRepository interface {
	FindByKeyHash(keyHash string) (*model.ApiKey, error)
	Update(key *model.ApiKey) error
}

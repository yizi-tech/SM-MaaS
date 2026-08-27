package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Known insecure default values that must never be used in production.
var (
	errSecretTooWeak  = errors.New("JWT_SECRET must be explicitly set to a random value of at least 32 bytes")
	errDBWeakPassword = errors.New("DB_PASSWORD must be explicitly set and not use the default value mass123")
)

// Validate rejects insecure default credentials unless MASS_ALLOW_INSECURE_DEFAULTS=true.
// Production deployments must not rely on the escape hatch.
func (c *Config) Validate() error {
	if os.Getenv("MASS_ALLOW_INSECURE_DEFAULTS") == "true" {
		return nil
	}

	if len(c.JWT.Secret) < 32 || c.JWT.Secret == "mass-platform-secret-key-change-in-production" {
		return errSecretTooWeak
	}
	if c.Database.Password == "" || c.Database.Password == "mass123" {
		return errDBWeakPassword
	}
	return nil
}

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Log      LogConfig
	Payment  PaymentConfig
	SMTP     SMTPConfig
	SMS      SMSConfig
	LLM      LLMConfig
	Upload   UploadConfig
}

type UploadConfig struct {
	Dir       string // directory that stores uploaded files
	MaxSizeMB int64  // max upload size in MB
}

type ServerConfig struct {
	Port         int
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxIdle  int
	MaxOpen  int
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	ExpireTime time.Duration
}

type LogConfig struct {
	Level  string
	Output string
}

type PaymentConfig struct {
	StripeSecretKey string
	StripeWebhookSecret string
	AlipayAppID     string
	AlipayPrivateKey string
	AlipayPublicKey  string
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

// SMSConfig describes an SMS provider integration (e.g. Aliyun SMS).
// When Provider is empty the SMS channel is disabled and registration
// falls back to email verification only.
type SMSConfig struct {
	Provider     string // e.g. "aliyun" (empty = disabled)
	AccessKey    string
	AccessSecret string
	SignName     string
	TemplateCode string
}

type LLMConfig struct {
	OpenAIBaseURL    string
	OpenAIAPIKey     string
	AnthropicBaseURL string
	AnthropicAPIKey  string
	DefaultTimeout   time.Duration
	MaxRetries       int
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnvInt("SERVER_PORT", 8080),
			Mode:         getEnv("SERVER_MODE", "release"),
			ReadTimeout:  time.Second * 30,
			// WriteTimeout MUST stay 0: it is a TOTAL deadline for writing the
			// whole response, not an idle timeout. An SSE stream that runs
			// longer than WriteTimeout (e.g. long reasoning + long answer)
			// would be hard-cut mid-stream even while data is flowing.
			// Disconnections are handled via the request context (client
			// disconnect cancels the upstream call and billing finalizes).
			WriteTimeout: 0,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "mass"),
			Password: getEnv("DB_PASSWORD", "mass123"),
			DBName:   getEnv("DB_NAME", "mass"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxIdle:  getEnvInt("DB_MAX_IDLE", 10),
			MaxOpen:  getEnvInt("DB_MAX_OPEN", 100),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "mass-platform-secret-key-change-in-production"),
			ExpireTime: time.Hour * 24 * 7,
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Output: getEnv("LOG_OUTPUT", "stdout"),
		},
		Payment: PaymentConfig{
			StripeSecretKey:    getEnv("STRIPE_SECRET_KEY", ""),
			StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "smtp.example.com"),
			Port:     getEnvInt("SMTP_PORT", 587),
			User:     getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", "noreply@mass-platform.com"),
		},
		SMS: SMSConfig{
			Provider:     getEnv("SMS_PROVIDER", ""),
			AccessKey:    getEnv("SMS_ACCESS_KEY", ""),
			AccessSecret: getEnv("SMS_ACCESS_SECRET", ""),
			SignName:     getEnv("SMS_SIGN_NAME", ""),
			TemplateCode: getEnv("SMS_TEMPLATE_CODE", ""),
		},
		LLM: LLMConfig{
			OpenAIBaseURL:    getEnv("OPENAI_BASE_URL", "https://api.openai.com"),
			OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
			AnthropicBaseURL: getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
			AnthropicAPIKey:  getEnv("ANTHROPIC_API_KEY", ""),
			DefaultTimeout:   time.Second * 120,
			MaxRetries:       getEnvInt("LLM_MAX_RETRIES", 3),
		},
		Upload: UploadConfig{
			Dir:       getEnv("UPLOAD_DIR", "uploads"),
			MaxSizeMB: int64(getEnvInt("UPLOAD_MAX_SIZE_MB", 5)),
		},
	}
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.DBName)
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}
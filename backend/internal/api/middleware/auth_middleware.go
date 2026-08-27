package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mass-platform/backend/internal/auth"
	"github.com/mass-platform/backend/internal/model"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/mass-platform/backend/pkg/response"
)

func AuthMiddleware(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "missing authorization header")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "invalid authorization header format")
			c.Abort()
			return
		}

		claims, err := authService.ValidateToken(parts[1])
		if err != nil {
			response.Unauthorized(c, err.Error())
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists {
			response.Forbidden(c, "access denied")
			c.Abort()
			return
		}

		// role may be a string or a custom string type (e.g. model.UserRole).
		role := ""
		switch v := roleVal.(type) {
		case string:
			role = v
		case fmt.Stringer:
			role = v.String()
		default:
			role = fmt.Sprintf("%v", v)
		}

		if role != "admin" {
			response.Forbidden(c, "admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}

func OptionalAuthMiddleware(authService *auth.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(parts[1])
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)
		}

		c.Next()
	}
}

func APIKeyAuthMiddleware(apiKeyRepo auth.APIKeyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only accept the key via headers (X-API-Key or Authorization: Bearer).
		// The api_key query parameter was removed: passing secrets in the URL
		// leaks them into proxy / access logs.
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// OpenAI SDK sends the key as "Authorization: Bearer sk-…".
			if authHeader := c.GetHeader("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}
		if apiKey == "" {
			response.Unauthorized(c, "missing API key")
			c.Abort()
			return
		}

		// Hash the API key for lookup
		keyHash := sha256Hash(apiKey)
		key, err := auth.ValidateAPIKey(keyHash, apiKeyRepo)
		if err != nil {
			response.Unauthorized(c, err.Error())
			c.Abort()
			return
		}

		c.Set("api_key_id", key.ID)
		c.Set("user_id", key.UserID)
		c.Set("api_key", key)
		touchAPIKeyUsage(key, apiKeyRepo)
		c.Next()
	}
}

// touchAPIKeyUsage persists the API key's last_used_at timestamp so the portal
// shows truthful usage. Throttled to at most one DB write per minute per key to
// avoid a write on every request under load. A failure here must never fail the
// actual API request — log and continue.
func touchAPIKeyUsage(key *model.ApiKey, apiKeyRepo auth.APIKeyRepository) {
	now := time.Now()
	if key.LastUsedAt != nil && now.Sub(*key.LastUsedAt) < time.Minute {
		return
	}
	key.LastUsedAt = &now
	if err := apiKeyRepo.Update(key); err != nil {
		logging.Error("auth", "api_key_usage", "failed to update last_used_at", err, map[string]interface{}{
			"key_id": key.ID,
		})
	}
}

func sha256Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// corsAllowOrigins holds the configured CORS allow-list (lowercased). A value
// of "*" (or an empty list) keeps the legacy open behaviour; otherwise only the
// listed origins are reflected, denying cross-origin reads from any other site.
var corsAllowOrigins = func() map[string]bool {
	raw := os.Getenv("MASS_CORS_ALLOW_ORIGINS")
	m := map[string]bool{}
	for _, o := range strings.Split(raw, ",") {
		o = strings.ToLower(strings.TrimSpace(o))
		if o != "" {
			m[o] = true
		}
	}
	return m
}()

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowOrigin := ""
		if origin != "" {
			lo := strings.ToLower(origin)
			if corsAllowOrigins["*"] || corsAllowOrigins[lo] {
				allowOrigin = origin
			}
		}
		// Only emit CORS headers when the request origin is explicitly allowed.
		// Same-origin (no Origin header) requests are unaffected.
		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-API-Key")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			// Same-origin (no Origin header) or an allowed origin: reply 204.
			// A cross-origin preflight from a non-allowed origin is rejected.
			if allowOrigin != "" || origin == "" {
				c.AbortWithStatus(204)
			} else {
				c.AbortWithStatus(403)
			}
			return
		}

		c.Next()
	}
}

func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Rate limiting is handled by the rate limiter middleware
		// This is a placeholder for any additional rate limiting logic
		c.Next()
	}
}

// BodyLimitMiddleware caps the request body size to prevent memory-exhaustion
// DoS via oversized JSON bodies. Multipart uploads are bounded by their own
// handler limits but benefit from the same hard cap.
func BodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = c.GetHeader("X-Trace-ID")
		}
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func generateRequestID() string {
	return "req-" + randomString(16)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use a time-based suffix so IDs are still unique per call.
		return fmt.Sprintf("%s%d", hex.EncodeToString([]byte(time.Now().String())), time.Now().UnixNano())
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

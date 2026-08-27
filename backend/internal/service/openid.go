package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mass-platform/backend/internal/repository"
	"github.com/mass-platform/backend/pkg/logging"
	"github.com/redis/go-redis/v9"
)

// OpenIDConfig is the 亦 OpenID OAuth client configuration resolved from the
// system config store (admin-editable, group "oauth").
type OpenIDConfig struct {
	Enabled      bool
	Server       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// OpenIDUser is the user profile returned by the 亦 OpenID userinfo endpoint.
type OpenIDUser struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

// OpenIDService implements the OAuth2 authorization-code client for the
// 亦 OpenID account service (https://account.yiziyun.com).
type OpenIDService struct {
	rdb        *redis.Client
	configRepo *repository.SystemConfigRepository
	httpClient *http.Client
}

func NewOpenIDService(rdb *redis.Client, configRepo *repository.SystemConfigRepository) *OpenIDService {
	return &OpenIDService{
		rdb:        rdb,
		configRepo: configRepo,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

const openIDStateTTL = 10 * time.Minute

func stateKey(state string) string {
	return "openid:state:" + state
}

// LoadConfig reads the oauth_* keys from the system config store.
func (s *OpenIDService) LoadConfig(ctx context.Context) (*OpenIDConfig, error) {
	cfgs, err := s.configRepo.GetAll()
	if err != nil {
		return nil, err
	}
	vals := map[string]string{}
	for _, c := range cfgs {
		vals[c.Key] = c.Value
	}
	cfg := &OpenIDConfig{
		Enabled:      strings.EqualFold(strings.TrimSpace(vals["oauth_enabled"]), "true"),
		Server:       strings.TrimRight(strings.TrimSpace(vals["oauth_server"]), "/"),
		ClientID:     strings.TrimSpace(vals["oauth_client_id"]),
		ClientSecret: strings.TrimSpace(vals["oauth_client_secret"]),
		RedirectURI:  strings.TrimSpace(vals["oauth_redirect_uri"]),
	}
	if cfg.Server == "" {
		cfg.Server = "https://account.yiziyun.com"
	}
	return cfg, nil
}

// AuthorizeURL builds the authorization-code URL the browser is redirected to.
func (s *OpenIDService) AuthorizeURL(cfg *OpenIDConfig, state string) (string, error) {
	if cfg.Server == "" || cfg.ClientID == "" {
		return "", fmt.Errorf("openid oauth not configured")
	}
	u, err := url.Parse(cfg.Server + "/oauth/authorize.php")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("state", state)
	q.Set("scope", "basic")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ExchangeCode trades an authorization code for an access token.
func (s *OpenIDService) ExchangeCode(ctx context.Context, cfg *OpenIDConfig, code string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("redirect_uri", cfg.RedirectURI)

	resp, err := s.httpClient.PostForm(cfg.Server+"/oauth/token.php", form)
	if err != nil {
		return "", fmt.Errorf("openid token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("openid token response parse: %w", err)
	}
	if tokenResp.AccessToken == "" {
		logging.Warn("openid_service", "exchange_code", "openid token error",
			map[string]interface{}{"error": tokenResp.Error, "body": string(body)})
		return "", fmt.Errorf("openid token error: %s", tokenResp.Error)
	}
	return tokenResp.AccessToken, nil
}

// GetUserInfo fetches the user profile for an access token.
func (s *OpenIDService) GetUserInfo(ctx context.Context, cfg *OpenIDConfig, accessToken string) (*OpenIDUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Server+"/oauth/userinfo.php", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openid userinfo request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openid userinfo status %d: %s", resp.StatusCode, string(body))
	}

	var user OpenIDUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("openid userinfo parse: %w", err)
	}
	if user.UID == "" {
		return nil, fmt.Errorf("openid userinfo missing uid")
	}
	return &user, nil
}

// StoreState persists an OAuth state token together with its intent
// ("login" or "bind") and, for binding, the target user id.
func (s *OpenIDService) StoreState(ctx context.Context, state, intent string, userID uint) error {
	if s.rdb == nil {
		return nil
	}
	data := map[string]interface{}{"intent": intent}
	if userID > 0 {
		data["user_id"] = userID
	}
	b, _ := json.Marshal(data)
	return s.rdb.Set(ctx, stateKey(state), b, openIDStateTTL).Err()
}

// TakeState consumes and returns the intent/user recorded for a state token.
func (s *OpenIDService) TakeState(ctx context.Context, state string) (intent string, userID uint, ok bool) {
	if s.rdb == nil || state == "" {
		return "", 0, false
	}
	b, err := s.rdb.GetDel(ctx, stateKey(state)).Result()
	if err != nil {
		return "", 0, false
	}
	var data struct {
		Intent string `json:"intent"`
		UserID uint   `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(b), &data); err != nil {
		return "", 0, false
	}
	return data.Intent, data.UserID, true
}
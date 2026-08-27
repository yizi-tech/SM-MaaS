package payment

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WechatPayConfig holds the configuration for native WeChat Pay (v3 API).
type WechatPayConfig struct {
	Enabled    bool
	AppID      string // 公众号 / 小程序 / 应用 AppID
	MchID      string // 商户号
	APIKey     string // APIv3 密钥（32 字节）
	SerialNo   string // 商户证书序列号
	PrivateKey string // 商户 API 私钥（PEM / PKCS8 或 PKCS1）
	NotifyURL  string // 支付结果异步回调地址
}

// WechatPayClient talks to the WeChat Pay v3 API.
type WechatPayClient struct {
	cfg WechatPayConfig
}

func NewWechatPayClient(cfg WechatPayConfig) *WechatPayClient {
	return &WechatPayClient{cfg: cfg}
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	pemStr = strings.TrimSpace(pemStr)
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		// Maybe it is a bare base64 / single-line key; try wrapping.
		pemStr = "-----BEGIN PRIVATE KEY-----\n" + pemStr + "\n-----END PRIVATE KEY-----"
		block, _ = pem.Decode([]byte(pemStr))
		if block == nil {
			return nil, errors.New("invalid private key")
		}
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k2, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaK, ok := k2.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}
	return rsaK, nil
}

func parseRSAPublicKey(pemOrB64 string) (*rsa.PublicKey, error) {
	pemOrB64 = strings.TrimSpace(pemOrB64)
	if strings.HasPrefix(pemOrB64, "-----") {
		block, _ := pem.Decode([]byte(pemOrB64))
		if block == nil {
			return nil, errors.New("invalid public key pem")
		}
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return rsaPub, nil
	}
	der, err := base64.StdEncoding.DecodeString(pemOrB64)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPub, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// aeadKey resolves the 32-byte AES key used for resource decryption.
func aeadKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if len([]byte(s)) == 32 {
		return []byte(s), nil
	}
	return nil, errors.New("invalid APIv3 key")
}

// buildAuthorization builds the WECHATPAY2-SHA256-RSA2048 Authorization header.
func (c *WechatPayClient) buildAuthorization(method, path string, body []byte, nonce string, timestamp int64) (string, error) {
	key, err := parseRSAPrivateKey(c.cfg.PrivateKey)
	if err != nil {
		return "", err
	}
	message := fmt.Sprintf("%s\n%s\n%d\n%s\n%s\n", method, path, timestamp, nonce, string(body))
	h := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	sign := base64.StdEncoding.EncodeToString(signature)
	return fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%d",serial_no="%s"`,
		c.cfg.MchID, nonce, sign, timestamp, c.cfg.SerialNo), nil
}

// CreateNativeOrder places a Native payment order and returns the code_url
// (a string the user scans with WeChat to pay). amountCents is in 分.
func (c *WechatPayClient) CreateNativeOrder(outTradeNo, description, notifyURL string, amountCents int64) (string, error) {
	body := map[string]interface{}{
		"mchid":        c.cfg.MchID,
		"appid":        c.cfg.AppID,
		"description":  description,
		"out_trade_no": outTradeNo,
		"notify_url":   notifyURL,
		"amount":       map[string]interface{}{"total": amountCents, "currency": "CNY"},
	}
	raw, _ := json.Marshal(body)
	path := "/v3/pay/transactions/native"
	nonce := randomHex(16)
	timestamp := time.Now().Unix()
	auth, err := c.buildAuthorization("POST", path, raw, nonce, timestamp)
	if err != nil {
		return "", err
	}
	respBody, err := httpPostJSON("https://api.mch.weixin.qq.com"+path,
		map[string]string{"Authorization": auth, "Accept": "application/json"}, raw)
	if err != nil {
		return "", fmt.Errorf("wechat order request failed: %w", err)
	}
	var result struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		CodeURL string `json:"code_url"`
	}
	if err := jsonUnmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("invalid wechat order response: %w", err)
	}
	if result.CodeURL == "" {
		return "", fmt.Errorf("wechat order failed: %s %s", result.Code, result.Message)
	}
	return result.CodeURL, nil
}

// DecryptResource decrypts a WeChat Pay callback resource (AES-256-GCM) using
// the APIv3 key. On success it returns the parsed plaintext JSON.
func (c *WechatPayClient) DecryptResource(resource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	Nonce          string `json:"nonce"`
	AssociatedData string `json:"associated_data"`
}) (map[string]interface{}, error) {
	if resource.Algorithm != "AEAD_AES_256_GCM" {
		return nil, errors.New("unsupported wechat algorithm")
	}
	key, err := aeadKey(c.cfg.APIKey)
	if err != nil {
		return nil, err
	}
	nonce, _ := base64.StdEncoding.DecodeString(resource.Nonce)
	ciphertext, _ := base64.StdEncoding.DecodeString(resource.Ciphertext)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(resource.AssociatedData))
	if err != nil {
		return nil, fmt.Errorf("wechat resource decrypt failed: %w", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// QueryOrder queries WeChat Pay for an order's status. It returns whether the
// order is successfully paid and the paid amount in 分 (0 when unknown).
func (c *WechatPayClient) QueryOrder(outTradeNo string) (bool, int64, error) {
	path := fmt.Sprintf("/v3/pay/transactions/out-trade-no/%s", outTradeNo)
	nonce := randomHex(16)
	timestamp := time.Now().Unix()
	auth, err := c.buildAuthorization("GET", path, []byte{}, nonce, timestamp)
	if err != nil {
		return false, 0, err
	}
	respBody, err := httpGetWithHeaders("https://api.mch.weixin.qq.com"+path,
		map[string]string{"Authorization": auth, "Accept": "application/json"})
	if err != nil {
		return false, 0, err
	}
	var result struct {
		TradeState string `json:"trade_state"`
		Amount     struct {
			Total int64 `json:"total"`
		} `json:"amount"`
	}
	if err := jsonUnmarshal(respBody, &result); err != nil {
		return false, 0, fmt.Errorf("invalid wechat query response")
	}
	return result.TradeState == "SUCCESS", result.Amount.Total, nil
}

// httpGetWithHeaders performs a GET with custom headers.
func httpGetWithHeaders(rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

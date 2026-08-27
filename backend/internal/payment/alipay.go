package payment

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AlipayConfig holds the configuration for native Alipay integration.
type AlipayConfig struct {
	Enabled    bool
	AppID      string // 支付宝应用 AppId
	PrivateKey string // 商户应用私钥（PEM / PKCS8 或 PKCS1）
	PublicKey  string // 支付宝公钥（PEM 或单行 base64 DER）
	NotifyURL  string
	ReturnURL  string
	Gateway    string // 默认 https://openapi.alipay.com/gateway.do
}

// AlipayClient talks to the Alipay open platform gateway.
type AlipayClient struct {
	cfg AlipayConfig
}

func NewAlipayClient(cfg AlipayConfig) *AlipayClient {
	return &AlipayClient{cfg: cfg}
}

func (c *AlipayConfig) gateway() string {
	if strings.TrimSpace(c.Gateway) == "" {
		return "https://openapi.alipay.com/gateway.do"
	}
	return strings.TrimSpace(c.Gateway)
}

// sign builds the RSA2 (SHA256WithRSA) signature over the sorted params.
func (c *AlipayClient) sign(params map[string]string) (string, error) {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(params[k])
	}
	key, err := parseRSAPrivateKey(c.cfg.PrivateKey)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(buf.String()))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// CreatePrecreateOrder places an Alipay precreate (QR) order and returns the
// qr_code string the user scans with Alipay. amountYuan is in 元.
func (c *AlipayClient) CreatePrecreateOrder(outTradeNo, subject, amountYuan string) (string, error) {
	biz := map[string]string{
		"out_trade_no": outTradeNo,
		"total_amount": amountYuan,
		"subject":      subject,
	}
	bizRaw, _ := json.Marshal(biz)
	params := map[string]string{
		"app_id":      c.cfg.AppID,
		"method":      "alipay.trade.precreate",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   timeNowString(),
		"version":     "1.0",
		"notify_url":  c.cfg.NotifyURL,
		"biz_content": string(bizRaw),
	}
	sign, err := c.sign(params)
	if err != nil {
		return "", err
	}
	params["sign"] = sign

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	respBody, err := httpPostForm(c.cfg.gateway(), form)
	if err != nil {
		return "", fmt.Errorf("alipay order request failed: %w", err)
	}
	var result struct {
		AlipayTradePrecreateResponse struct {
			Code    string `json:"code"`
			Msg     string `json:"msg"`
			SubMsg  string `json:"sub_msg"`
			QrCode string `json:"qr_code"`
		} `json:"alipay_trade_precreate_response"`
	}
	if err := jsonUnmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("invalid alipay order response")
	}
	if result.AlipayTradePrecreateResponse.Code != "10000" || result.AlipayTradePrecreateResponse.QrCode == "" {
		return "", fmt.Errorf("alipay order failed: %s %s", result.AlipayTradePrecreateResponse.Code, result.AlipayTradePrecreateResponse.SubMsg)
	}
	return result.AlipayTradePrecreateResponse.QrCode, nil
}

// VerifyNotify verifies an Alipay async notification signature and returns the
// parsed out_trade_no and trade_status when valid.
func (c *AlipayClient) VerifyNotify(params map[string]string) (outTradeNo, tradeStatus, totalAmount string, ok bool) {
	signB64 := params["sign"]
	if signB64 == "" {
		return "", "", "", false
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(params[k])
	}
	pub, err := parseRSAPublicKey(c.cfg.PublicKey)
	if err != nil {
		return "", "", "", false
	}
	sig, err := base64.StdEncoding.DecodeString(signB64)
	if err != nil {
		return "", "", "", false
	}
	h := sha256.Sum256([]byte(buf.String()))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig); err != nil {
		return "", "", "", false
	}
	return params["out_trade_no"], params["trade_status"], params["total_amount"], true
}

// QueryOrder queries Alipay for an order's status via alipay.trade.query.
// It returns whether the order is paid and the paid amount (元, e.g. "100.00";
// empty when unknown).
func (c *AlipayClient) QueryOrder(outTradeNo string) (bool, string, error) {
	biz := map[string]string{"out_trade_no": outTradeNo}
	bizRaw, _ := json.Marshal(biz)
	params := map[string]string{
		"app_id":      c.cfg.AppID,
		"method":      "alipay.trade.query",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   timeNowString(),
		"version":     "1.0",
		"biz_content": string(bizRaw),
	}
	sign, err := c.sign(params)
	if err != nil {
		return false, "", err
	}
	params["sign"] = sign
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	respBody, err := httpPostForm(c.cfg.gateway(), form)
	if err != nil {
		return false, "", fmt.Errorf("alipay query request failed: %w", err)
	}
	var result struct {
		AlipayTradeQueryResponse struct {
			Code         string `json:"code"`
			SubMsg       string `json:"sub_msg"`
			TradeStatus  string `json:"trade_status"`
			TotalAmount  string `json:"total_amount"`
		} `json:"alipay_trade_query_response"`
	}
	if err := jsonUnmarshal(respBody, &result); err != nil {
		return false, "", fmt.Errorf("invalid alipay query response")
	}
	if result.AlipayTradeQueryResponse.Code != "10000" {
		return false, "", fmt.Errorf("alipay query failed: %s", result.AlipayTradeQueryResponse.SubMsg)
	}
	paid := result.AlipayTradeQueryResponse.TradeStatus == "TRADE_SUCCESS" ||
		result.AlipayTradeQueryResponse.TradeStatus == "TRADE_FINISHED"
	return paid, result.AlipayTradeQueryResponse.TotalAmount, nil
}

// payment package helpers (time + form post)

func timeNowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func httpPostForm(rawURL string, form url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), nil
}

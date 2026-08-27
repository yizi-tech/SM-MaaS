package payment

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/mass-platform/backend/pkg/logging"
)

// EpayConfig holds the configuration for the 易支付 (epay) payment gateway.
type EpayConfig struct {
	Gateway   string // e.g. https://epay.example.com/
	PID       string // merchant id
	Key       string // merchant secret key
	SignUpper bool   // whether the gateway expects uppercase MD5 signatures
	Enabled   bool
}

// EpayClient talks to a 易支付 (彩虹易支付 compatible) gateway.
type EpayClient struct {
	cfg EpayConfig
}

func NewEpayClient(cfg EpayConfig) *EpayClient {
	return &EpayClient{cfg: cfg}
}

// Sign computes the MD5 signature for a set of params.
// The canonical signing string is k1=v1&k2=v2...&key=KEY (keys sorted,
// empty values excluded). Per the standard 易支付 spec, "sign" and
// "sign_type" never participate in the signature.
func (c *EpayClient) Sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" || params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
	}
	// Per the 码支付 spec the KEY is appended directly to the query string
	// (no "key=" prefix): sign = md5(a=b&c=d... + KEY), lowercase hex.
	// (This gateway's mapi.php interface rejects uppercase signatures.)
	sb.WriteString(c.cfg.Key)
	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// VerifyNotify verifies the signature of an epay async notification.
// It accepts both uppercase and lowercase signatures regardless of config.
func (c *EpayClient) VerifyNotify(params map[string]string) bool {
	sign, ok := params["sign"]
	if !ok || sign == "" {
		return false
	}
	sign = strings.ToLower(sign)
	// "sign" and "sign_type" must not participate in the signature itself
	params = cloneMap(params)
	delete(params, "sign")
	delete(params, "sign_type")
	keys := make([]string, 0, len(params))
	for k := range params {
		if params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
	}
	sb.WriteString(c.cfg.Key)
	sum := md5.Sum([]byte(sb.String()))
	return strings.ToLower(hex.EncodeToString(sum[:])) == sign
}

// BuildOrder creates a payment order on the gateway's mapi.php interface
// and returns the user-facing payment page URL (payurl from the gateway
// JSON response). The gateway signature uses all submitted params sorted
// by key (sign/sign_type excluded) + KEY, lowercase MD5.
func (c *EpayClient) BuildOrder(amount, outTradeNo, name, notifyURL, returnURL, payType, clientIP string) (string, error) {
	params := map[string]string{
		"pid":          c.cfg.PID,
		"type":         payType,
		"out_trade_no": outTradeNo,
		"notify_url":   notifyURL,
		"return_url":   returnURL,
		"name":         name,
		"money":        amount,
		"clientip":     clientIP,
	}
	params["sign"] = c.Sign(params)
	params["sign_type"] = "MD5"
	base := strings.TrimRight(c.cfg.Gateway, "/") + "/mapi.php"
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	resp, err := httpGet(base + "?" + vals.Encode())
	if err != nil {
		return "", fmt.Errorf("epay order request failed: %w", err)
	}
	var result struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayURL  string `json:"payurl"`
	}
	if err := jsonUnmarshal(resp, &result); err != nil {
		logging.Error("payment", "epay_order", "bad order response", err, map[string]interface{}{"out_trade_no": outTradeNo, "body": string(resp)})
		return "", fmt.Errorf("invalid epay order response")
	}
	if result.Code != 1 {
		logging.Warn("payment", "epay_order", "gateway rejected order",
			map[string]interface{}{"out_trade_no": outTradeNo, "code": result.Code, "msg": result.Msg})
		return "", fmt.Errorf("epay order failed: %s", result.Msg)
	}
	if result.PayURL == "" {
		return "", fmt.Errorf("epay order returned no pay url")
	}
	return result.PayURL, nil
}

// QueryOrder actively queries the gateway for an order's status.
// Returns (paid bool, tradeNo string, err error).
// Note: legacy gateways require the merchant key to be sent as "key",
// newer ones require a signed query; we pass both pid/key and a signed query.
func (c *EpayClient) QueryOrder(outTradeNo string) (bool, string, error) {
	params := map[string]string{
		"act":          "order",
		"pid":          c.cfg.PID,
		"key":          c.cfg.Key,
		"out_trade_no": outTradeNo,
	}
	base := strings.TrimRight(c.cfg.Gateway, "/") + "/api.php"
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	resp, err := httpGet(base + "?" + vals.Encode())
	if err != nil {
		return false, "", err
	}
	var result struct {
		Code       int    `json:"code"`
		TradeNo    string `json:"trade_no"`
		OutTradeNo string `json:"out_trade_no"`
		Status     string `json:"status"`
		Msg        string `json:"msg"`
	}
	if err := jsonUnmarshal(resp, &result); err != nil {
		logging.Error("payment", "epay_query", "bad query response", err, map[string]interface{}{"out_trade_no": outTradeNo, "body": string(resp)})
		return false, "", fmt.Errorf("invalid epay query response")
	}
	if result.Code != 1 {
		return false, "", fmt.Errorf("epay query failed: %s", result.Msg)
	}
	// 彩虹易支付: status=1 表示已支付（TRADE_SUCCESS）
	return result.Status == "1" || result.Status == "TRADE_SUCCESS", result.TradeNo, nil
}

func cloneMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

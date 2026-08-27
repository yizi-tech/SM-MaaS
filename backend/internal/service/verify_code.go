package service

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"net/smtp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mass-platform/backend/internal/config"
)

// Verify code channels
const (
	ChannelEmail = "email"
	ChannelSMS   = "sms"

	codeTTL     = 5 * time.Minute
	resendWait  = 60 * time.Second
	maxAttempts = 10
)

var (
	ErrInvalidCode     = errors.New("invalid verification code")
	ErrCodeExpired     = errors.New("verification code expired")
	ErrTooManyAttempts = errors.New("too many attempts, please request a new code")
	ErrResendTooSoon   = errors.New("please wait 60s before resending")
	ErrChannelDisabled = errors.New("this verification channel is not configured")
	ErrInvalidTarget   = errors.New("invalid email or phone")
)

// VerifyCodeService sends and verifies one-time codes (email via SMTP,
// phone via an SMS provider). Codes are stored in Redis with a short TTL.
type VerifyCodeService struct {
	rdb     *redis.Client
	smtpCfg config.SMTPConfig
	smsCfg  config.SMSConfig
	appName string
	brand   string
}

func NewVerifyCodeService(rdb *redis.Client, smtpCfg config.SMTPConfig, smsCfg config.SMSConfig, appName string) *VerifyCodeService {
	return &VerifyCodeService{rdb: rdb, smtpCfg: smtpCfg, smsCfg: smsCfg, appName: appName}
}

func codeKey(channel, target string) string {
	return "verify:code:" + channel + ":" + target
}

func waitKey(channel, target string) string {
	return "verify:wait:" + channel + ":" + target
}

func tryKey(channel, target string) string {
	return "verify:try:" + channel + ":" + target
}

func generateCode() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

// Send sends a verification code to the given email address or phone number.
func (s *VerifyCodeService) Send(ctx context.Context, channel, target string) error {
	return s.SendWithBrand(ctx, channel, target, "")
}

// SendWithBrand sends a verification code; brand overrides the default app
// name used in the email template (empty falls back to appName).
func (s *VerifyCodeService) SendWithBrand(ctx context.Context, channel, target, brand string) error {
	return s.SendForPurpose(ctx, channel, target, "注册", brand)
}

// SendForPurpose behaves like SendWithBrand but allows the email template /
// subject to state the purpose of the code (e.g. "注册", "修改密码").
func (s *VerifyCodeService) SendForPurpose(ctx context.Context, channel, target, purpose, brand string) error {
	if strings.TrimSpace(purpose) == "" {
		purpose = "注册"
	}
	if channel == "" {
		channel = ChannelEmail
	}
	switch channel {
	case ChannelEmail:
		if !strings.Contains(target, "@") {
			return ErrInvalidTarget
		}
		if s.smtpCfg.Host == "" || s.smtpCfg.Host == "smtp.example.com" || s.smtpCfg.From == "" || s.smtpCfg.User == "" {
			return ErrChannelDisabled
		}
	case ChannelSMS:
		if s.smsCfg.Provider == "" || s.smsCfg.SignName == "" || s.smsCfg.TemplateCode == "" {
			return ErrChannelDisabled
		}
	default:
		return ErrInvalidTarget
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if s.rdb != nil {
		ok, _ := s.rdb.SetNX(ctx, waitKey(channel, target), "1", resendWait).Result()
		if !ok {
			return ErrResendTooSoon
		}
	}

	code, err := generateCode()
	if err != nil {
		return err
	}

	if strings.TrimSpace(brand) != "" {
		s.brand = strings.TrimSpace(brand)
	}

	if err := s.deliver(ctx, channel, target, code, purpose); err != nil {
		if s.rdb != nil {
			s.rdb.Del(ctx, waitKey(channel, target))
		}
		return err
	}

	if s.rdb != nil {
		if err := s.rdb.Set(ctx, codeKey(channel, target), code, codeTTL).Err(); err != nil {
			s.rdb.Del(ctx, waitKey(channel, target))
			return err
		}
		s.rdb.Del(ctx, tryKey(channel, target))
	}
	return nil
}

func (s *VerifyCodeService) deliver(ctx context.Context, channel, target, code, purpose string) error {
	switch channel {
	case ChannelEmail:
		return s.sendEmail(ctx, target, code, purpose)
	case ChannelSMS:
		return s.sendSMS(ctx, target, code, purpose)
	}
	return ErrInvalidTarget
}

// emailTemplate renders the verification-code email as a responsive HTML
// message (inline styles, widely compatible with mailbox clients). The plain
// text counterpart is kept for clients that strip HTML.
func (s *VerifyCodeService) emailTemplate(appName, purpose, code string) (htmlBody, textBody string) {
	brand := html.EscapeString(appName)
	escCode := html.EscapeString(code)

	// One cell per code character, bank-style.
	var cells strings.Builder
	for _, ch := range escCode {
		cells.WriteString(`<td style="width:52px;height:60px;background:#f0f4ff;border:1px solid #dbe3ff;border-radius:10px;text-align:center;font-size:32px;font-weight:700;color:#3a5bd9;font-family:Consolas,Menlo,monospace;letter-spacing:0;">` + string(ch) + `</td>`)
	}

	htmlBody = fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<body style="margin:0;padding:0;background-color:#eef1f8;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#eef1f8;padding:36px 12px;">
<tr><td align="center">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:18px;overflow:hidden;border:1px solid #e4e8f2;box-shadow:0 10px 40px rgba(43,58,103,.06);">

<!-- Brand header -->
<tr><td style="background-color:#2b3a67;background:linear-gradient(135deg,#2b3a67 0%%,#3a5bd9 55%%,#7b4fd8 100%%);padding:34px 40px 30px;">
<div style="color:#ffffff;font-size:24px;font-weight:800;letter-spacing:2px;">%s</div>
<div style="color:rgba(255,255,255,.65);font-size:11px;letter-spacing:.28em;margin-top:6px;">ENTERPRISE AI INFRASTRUCTURE</div>
</td></tr>

<!-- Greeting -->
<tr><td style="padding:30px 40px 6px;">
<div style="font-size:15px;color:#3c465c;line-height:1.8;">你好，</div>
<div style="font-size:15px;color:#3c465c;line-height:1.8;margin-top:4px;">你在 <b style="color:#2b3a67;">%s</b> 的%s验证码为：</div>
</td></tr>

<!-- Code cells -->
<tr><td align="center" style="padding:24px 40px 10px;">
<table role="presentation" cellpadding="0" cellspacing="0" style="border-collapse:separate;border-spacing:8px 0;">%s</table>
</td></tr>

<!-- Validity -->
<tr><td align="center" style="padding:6px 40px 18px;">
<div style="display:inline-block;background:#fff7e8;border:1px solid #ffd58a;border-radius:8px;padding:8px 18px;font-size:13px;color:#b07d17;">该验证码 <b>5 分钟内有效</b>，请尽快完成验证</div>
</td></tr>

<!-- Notice -->
<tr><td style="padding:6px 40px 8px;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f8faff;border:1px solid #eef1f8;border-radius:12px;">
<tr><td style="padding:18px 20px;">
<div style="font-size:13px;color:#6b7488;line-height:2;">
<div>· 验证码仅用于本次%s，请勿转发或泄露给任何人</div>
<div>· %s 官方不会以任何形式向你索要验证码</div>
<div>· 如非本人操作，请忽略本邮件；若遇异常请联系 business@yiziyun.com</div>
</div>
</td></tr>
</table>
</td></tr>

<!-- Footer -->
<tr><td style="padding:24px 40px 28px;border-top:1px solid #eef1f8;background:#fbfcff;">
<div style="font-size:12px;color:#9aa3b8;text-align:center;line-height:1.9;">
© 2026 亦梓科技 · %s<br>
Enterprise AI Infrastructure · Powered by StarMoon
</div>
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`, brand, brand, purpose, cells.String(), purpose, brand, brand)

	textBody = fmt.Sprintf(
		"你好，\n\n你在 %s 的%s验证码为：%s\n验证码 5 分钟内有效，请勿泄露给他人。\n\n若非本人操作，请忽略本邮件。",
		appName, purpose, code)
	return htmlBody, textBody
}

// brandName returns the effective brand for emails: the system site name
// when set, otherwise the default app name.
func (s *VerifyCodeService) brandName() string {
	if strings.TrimSpace(s.brand) != "" {
		return strings.TrimSpace(s.brand)
	}
	return s.appName
}

func (s *VerifyCodeService) sendEmail(ctx context.Context, to, code, purpose string) error {
	subject := fmt.Sprintf("验证码 %s（%s%s）", code, s.brandName(), purpose)
	htmlBody, _ := s.emailTemplate(s.brandName(), purpose, code)
	return s.sendSMTP(ctx, to, subject, htmlBody)
}

// sendSMTP opens a connection to the configured SMTP server and delivers a
// single HTML message. Used for both verification codes and notifications.
func (s *VerifyCodeService) sendSMTP(ctx context.Context, to, subject, htmlBody string) error {
	if s.smtpCfg.Host == "" || s.smtpCfg.Host == "smtp.example.com" || s.smtpCfg.From == "" || s.smtpCfg.User == "" {
		return ErrChannelDisabled
	}
	addr := fmt.Sprintf("%s:%d", s.smtpCfg.Host, s.smtpCfg.Port)
	from := s.smtpCfg.From
	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		htmlBody

	var auth smtp.Auth
	if s.smtpCfg.User != "" {
		auth = smtp.PlainAuth("", s.smtpCfg.User, s.smtpCfg.Password, s.smtpCfg.Host)
	}
	// Port 465 uses implicit TLS (SMTPS): wrap the connection in TLS before
	// speaking SMTP. Other ports start plaintext and upgrade via STARTTLS.
	var conn *smtp.Client
	if s.smtpCfg.Port == 465 {
		tconn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: s.smtpCfg.Host})
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		conn, err = smtp.NewClient(tconn, s.smtpCfg.Host)
		if err != nil {
			tconn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
	} else {
		var err error
		conn, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
	}
	defer conn.Close()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if s.smtpCfg.Port != 465 {
		if ok, _ := conn.Extension("STARTTLS"); ok {
			if err := conn.StartTLS(nil); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	if auth != nil {
		if err := conn.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := conn.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := conn.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return conn.Quit()
}

// SendTokenLowAlert sends a low-token-balance warning email to the user.
func (s *VerifyCodeService) SendTokenLowAlert(ctx context.Context, to, nickname string, remaining, threshold int64) error {
	brand := s.brandName()
	displayName := nickname
	if strings.TrimSpace(displayName) == "" {
		displayName = to
	}
	subject := fmt.Sprintf("【%s】您的 Token 余额即将不足", brand)
	htmlBody := s.lowTokenAlertTemplate(brand, displayName, remaining, threshold)
	return s.sendSMTP(ctx, to, subject, htmlBody)
}

// lowTokenAlertTemplate renders the low-token-balance warning email.
func (s *VerifyCodeService) lowTokenAlertTemplate(brand, name string, remaining, threshold int64) string {
	brandEsc := html.EscapeString(brand)
	nameEsc := html.EscapeString(name)
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN"><body style="margin:0;padding:0;background-color:#eef1f8;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#eef1f8;padding:36px 12px;">
<tr><td align="center">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background:#ffffff;border-radius:18px;overflow:hidden;border:1px solid #e4e8f2;box-shadow:0 10px 40px rgba(43,58,103,.06);">
<tr><td style="background:linear-gradient(135deg,#2b3a67 0%%,#3a5bd9 55%%,#7b4fd8 100%%);padding:34px 40px 30px;">
<div style="color:#ffffff;font-size:24px;font-weight:800;letter-spacing:2px;">%s</div>
</td></tr>
<tr><td style="padding:30px 40px 6px;">
<div style="font-size:15px;color:#3c465c;line-height:1.8;">你好 %s，</div>
<div style="font-size:15px;color:#3c465c;line-height:1.8;margin-top:8px;">您设置的 Token 余额预警已触发：当前可用 Token 余额 <b style="color:#d97706;">%d</b>，已低于您设定的阈值 <b>%d</b>。</div>
<div style="font-size:15px;color:#3c465c;line-height:1.8;margin-top:8px;">为避免影响您的业务，请尽快前往 <b>充值中心</b> 充值余额或购买加油包 / 订阅套餐。</div>
</td></tr>
<tr><td style="padding:24px 40px 28px;border-top:1px solid #eef1f8;background:#fbfcff;">
<div style="font-size:12px;color:#9aa3b8;text-align:center;line-height:1.9;">© 2026 亦梓科技 · %s</div>
</td></tr>
</table></td></tr></table></body></html>`, brandEsc, nameEsc, remaining, threshold, brandEsc)
}

func (s *VerifyCodeService) sendSMS(ctx context.Context, to, code, purpose string) error {
	// Provider integrations (Aliyun SMS etc.) plug in here.
	// The channel is enabled only when SMS_* env vars are configured.
	return ErrChannelDisabled
}

// Verify checks the code for the target and consumes it on success.
func (s *VerifyCodeService) Verify(ctx context.Context, channel, target, code string) error {
	if code == "" || (channel == ChannelSMS && s.smsCfg.Provider == "") {
		return ErrInvalidCode
	}
	if s.rdb == nil {
		return ErrInvalidCode
	}
	key := codeKey(channel, target)
	stored, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return ErrInvalidCode
	}
	if err != nil {
		return err
	}
	if stored != code {
		tries, _ := s.rdb.Incr(ctx, tryKey(channel, target)).Result()
		if tries >= maxAttempts {
			s.rdb.Del(ctx, key)
		}
		return ErrInvalidCode
	}
	s.rdb.Del(ctx, key, waitKey(channel, target), tryKey(channel, target))
	return nil
}

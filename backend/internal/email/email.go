package email

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidEmail        = errors.New("email: invalid email address")
	ErrTokenExpired        = errors.New("email: verification token expired")
	ErrTokenNotFound       = errors.New("email: verification token not found")
	ErrTokenMismatch       = errors.New("email: verification token mismatch")
	ErrRateLimitExceeded   = errors.New("email: sending rate limit exceeded, please retry later")
	ErrSMTPDeliveryFailed  = errors.New("email: SMTP delivery failed")
)

// EmailMessage models an outbound transactional email.
type EmailMessage struct {
	ID        string    `json:"id"`
	To        string    `json:"to"`
	Subject   string    `json:"subject"`
	TextBody  string    `json:"text_body"`
	HTMLBody  string    `json:"html_body"`
	Category  string    `json:"category"` // "registration", "password_reset", "security_alert"
	CreatedAt time.Time `json:"created_at"`
	SentAt    time.Time `json:"sent_at,omitempty"`
	Status    string    `json:"status"` // "queued", "sent", "failed"
}

// VerificationToken records a time-limited PIN or link token for email ownership confirmation.
type VerificationToken struct {
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	Code      string    `json:"code"` // 6-digit numeric PIN
	ExpiresAt time.Time `json:"expires_at"`
	Consumed  bool      `json:"consumed"`
}

// PasswordResetToken records a time-limited token for password resets.
type PasswordResetToken struct {
	Email     string    `json:"email"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Consumed  bool      `json:"consumed"`
}

// Sender delivers email messages.
type Sender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// MemorySender stores outbound emails in memory for local testing and inspection.
type MemorySender struct {
	mu       sync.RWMutex
	messages []EmailMessage
}

func NewMemorySender() *MemorySender {
	return &MemorySender{
		messages: make([]EmailMessage, 0),
	}
}

func (m *MemorySender) Send(ctx context.Context, msg EmailMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	msg.SentAt = time.Now()
	msg.Status = "sent"
	m.messages = append(m.messages, msg)
	return nil
}

func (m *MemorySender) GetMessagesFor(email string) []EmailMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var matched []EmailMessage
	for _, msg := range m.messages {
		if strings.EqualFold(msg.To, email) {
			matched = append(matched, msg)
		}
	}
	return matched
}

func (m *MemorySender) LatestMessageFor(email string) *EmailMessage {
	msgs := m.GetMessagesFor(email)
	if len(msgs) == 0 {
		return nil
	}
	return &msgs[len(msgs)-1]
}

// SMTPSender implements production delivery over SMTP with TLS.
type SMTPSender struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func NewSMTPSender(host, port, username, password, from string) *SMTPSender {
	return &SMTPSender{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
	}
}

func (s *SMTPSender) Send(ctx context.Context, msg EmailMessage) error {
	addr := s.Host + ":" + s.Port
	var auth smtp.Auth
	if s.Username != "" && s.Password != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	headers := make(map[string]string)
	headers["From"] = s.From
	headers["To"] = msg.To
	headers["Subject"] = msg.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	var body strings.Builder
	for k, v := range headers {
		body.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	body.WriteString("\r\n")
	body.WriteString(msg.HTMLBody)

	err := smtp.SendMail(addr, auth, s.From, []string{msg.To}, []byte(body.String()))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSMTPDeliveryFailed, err)
	}
	return nil
}

// Service manages transactional email flows: registration, password resets, and verification tokens.
type Service struct {
	mu             sync.RWMutex
	sender         Sender
	fromAddress    string
	verifyTokens   map[string]VerificationToken   // keyed by email
	resetTokens    map[string]PasswordResetToken  // keyed by email
	rateLimits     map[string][]time.Time         // keyed by email
	tokenLifetime  time.Duration
}

// NewService creates an email service instance.
func NewService(sender Sender, fromAddress string) *Service {
	if sender == nil {
		sender = NewMemorySender()
	}
	if fromAddress == "" {
		fromAddress = "no-reply@openscanner.com"
	}
	return &Service{
		sender:        sender,
		fromAddress:   fromAddress,
		verifyTokens:  make(map[string]VerificationToken),
		resetTokens:   make(map[string]PasswordResetToken),
		rateLimits:    make(map[string][]time.Time),
		tokenLifetime: 15 * time.Minute,
	}
}

func generateSecureToken(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateNumericPIN() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	num := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
	return fmt.Sprintf("%06d", num), nil
}

func (s *Service) checkRateLimit(email string) bool {
	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)
	var valid []time.Time
	for _, t := range s.rateLimits[email] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= 5 { // max 5 emails per 10 minutes per recipient
		return false
	}
	valid = append(valid, now)
	s.rateLimits[email] = valid
	return true
}

// SendRegistrationVerification generates a PIN and sends the verification email.
func (s *Service) SendRegistrationVerification(ctx context.Context, email, displayName string) (*VerificationToken, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(cleanEmail, "@") {
		return nil, ErrInvalidEmail
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.checkRateLimit(cleanEmail) {
		return nil, ErrRateLimitExceeded
	}

	tokenStr, err := generateSecureToken(24)
	if err != nil {
		return nil, err
	}
	pinCode, err := generateNumericPIN()
	if err != nil {
		return nil, err
	}

	token := VerificationToken{
		Email:     cleanEmail,
		Token:     tokenStr,
		Code:      pinCode,
		ExpiresAt: time.Now().Add(s.tokenLifetime),
		Consumed:  false,
	}
	s.verifyTokens[cleanEmail] = token

	subject := "[OpenScanner] 이메일 인증을 완료해 주세요: " + pinCode
	textBody := fmt.Sprintf(
		"안녕하세요, %s님.\n\nOpenScanner 회원가입을 완료하려면 아래 6자리 인증 코드를 입력하세요:\n\n인증 코드: %s\n\n본 코드는 15분간 유효합니다.\n\n© OpenScanner. All rights reserved.",
		displayName, pinCode,
	)
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:'Pretendard Variable',Pretendard,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#0F172A;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F8FAFC;padding:32px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;background-color:#FFFFFF;border:1px solid #E2E8F0;border-radius:12px;overflow:hidden;">
          <tr>
            <td style="padding:24px 32px;border-bottom:1px solid #E2E8F0;">
              <span style="font-size:20px;font-weight:600;color:#0F172A;letter-spacing:-0.03em;">Open</span><span style="font-size:20px;font-weight:800;color:#1D4ED8;letter-spacing:-0.03em;">Scanner</span>
            </td>
          </tr>
          <tr>
            <td style="padding:32px;">
              <h3 style="margin:0 0 12px 0;font-size:18px;color:#0F172A;">이메일 인증을 완료해 주세요</h3>
              <p style="font-size:15px;color:#475569;line-height:1.6;margin:0 0 20px 0;">
                안녕하세요, <strong>%s</strong>님. OpenScanner 플랫폼에 오신 것을 환영합니다.<br>
                가입을 완료하려면 아래 6자리 인증 코드를 화면에 입력해 주세요.
              </p>
              <div style="background-color:#EFF6FF;border:1px solid #BFDBFE;border-radius:10px;padding:18px;text-align:center;margin-bottom:24px;">
                <span style="font-size:30px;font-weight:800;letter-spacing:6px;color:#1D4ED8;font-family:monospace;">%s</span>
              </div>
              <p style="font-size:13px;color:#94A3B8;margin:0;">
                본 코드는 15분 후 만료됩니다. 본인이 가입을 요청하지 않았다면 이 메일을 무시해 주세요.
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 32px;background-color:#F8FAFC;border-top:1px solid #E2E8F0;font-size:12px;color:#475569;">
              <strong>© OpenScanner. All rights reserved.</strong>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, displayName, pinCode)

	msg := EmailMessage{
		ID:        "msg-reg-" + tokenStr[:8],
		To:        cleanEmail,
		Subject:   subject,
		TextBody:  textBody,
		HTMLBody:  htmlBody,
		Category:  "registration",
		CreatedAt: time.Now(),
	}

	if err := s.sender.Send(ctx, msg); err != nil {
		return nil, err
	}

	return &token, nil
}

// VerifyRegistrationToken checks whether the provided PIN code or token matches and is not expired.
func (s *Service) VerifyRegistrationToken(email, codeOrToken string) error {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	cleanCode := strings.TrimSpace(codeOrToken)

	s.mu.Lock()
	defer s.mu.Unlock()

	tok, exists := s.verifyTokens[cleanEmail]
	if !exists || tok.Consumed {
		return ErrTokenNotFound
	}
	if time.Now().After(tok.ExpiresAt) {
		return ErrTokenExpired
	}

	matchCode := subtle.ConstantTimeCompare([]byte(tok.Code), []byte(cleanCode)) == 1
	matchToken := subtle.ConstantTimeCompare([]byte(tok.Token), []byte(cleanCode)) == 1

	if !matchCode && !matchToken {
		return ErrTokenMismatch
	}

	tok.Consumed = true
	s.verifyTokens[cleanEmail] = tok
	return nil
}

// SendPasswordReset sends a password reset link and token.
func (s *Service) SendPasswordReset(ctx context.Context, email string) (*PasswordResetToken, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(cleanEmail, "@") {
		return nil, ErrInvalidEmail
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.checkRateLimit(cleanEmail) {
		return nil, ErrRateLimitExceeded
	}

	tokenStr, err := generateSecureToken(32)
	if err != nil {
		return nil, err
	}

	reset := PasswordResetToken{
		Email:     cleanEmail,
		Token:     tokenStr,
		ExpiresAt: time.Now().Add(s.tokenLifetime),
		Consumed:  false,
	}
	s.resetTokens[cleanEmail] = reset

	subject := "[OpenScanner] 비밀번호 재설정 안내"
	textBody := fmt.Sprintf(
		"안녕하세요.\n\nOpenScanner 계정의 비밀번호 재설정 요청이 접수되었습니다.\n\n아래 보안 토큰을 입력하여 비밀번호를 재설정하세요:\n\n토큰: %s\n\n본 링크 및 토큰은 15분간 유효합니다.\n\n© OpenScanner. All rights reserved.",
		tokenStr,
	)
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<body style="margin:0;padding:0;background-color:#F8FAFC;font-family:'Pretendard Variable',Pretendard,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#0F172A;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F8FAFC;padding:32px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;background-color:#FFFFFF;border:1px solid #E2E8F0;border-radius:12px;overflow:hidden;">
          <tr>
            <td style="padding:24px 32px;border-bottom:1px solid #E2E8F0;">
              <span style="font-size:20px;font-weight:600;color:#0F172A;letter-spacing:-0.03em;">Open</span><span style="font-size:20px;font-weight:800;color:#1D4ED8;letter-spacing:-0.03em;">Scanner</span>
            </td>
          </tr>
          <tr>
            <td style="padding:32px;">
              <h2 style="margin:0 0 16px 0;font-size:18px;font-weight:700;color:#0F172A;">비밀번호 재설정 안내</h2>
              <p style="font-size:15px;color:#475569;line-height:1.6;margin:0 0 20px 0;">
                회원님의 계정 비밀번호 재설정 요청이 접수되었습니다.<br>
                아래 일회성 보안 인증 토큰을 입력하여 새 비밀번호를 설정해 주세요.
              </p>
              <div style="background-color:#EFF6FF;border:1px solid #BFDBFE;border-radius:8px;padding:14px;font-family:monospace;font-size:16px;color:#1D4ED8;word-break:break-all;text-align:center;margin-bottom:20px;">
                %s
              </div>
              <p style="font-size:13px;color:#94A3B8;margin:0;">
                본 토큰은 15분 후 만료됩니다. 본인이 요청하지 않았다면 이 메일을 무시해 주세요.
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 32px;background-color:#F8FAFC;border-top:1px solid #E2E8F0;font-size:12px;color:#475569;">
              <strong>© OpenScanner. All rights reserved.</strong>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, tokenStr)

	msg := EmailMessage{
		ID:        "msg-reset-" + tokenStr[:8],
		To:        cleanEmail,
		Subject:   subject,
		TextBody:  textBody,
		HTMLBody:  htmlBody,
		Category:  "password_reset",
		CreatedAt: time.Now(),
	}

	if err := s.sender.Send(ctx, msg); err != nil {
		return nil, err
	}

	return &reset, nil
}

// VerifyPasswordResetToken checks whether the provided reset token is valid and unconsumed.
func (s *Service) VerifyPasswordResetToken(email, token string) error {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	cleanToken := strings.TrimSpace(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	res, exists := s.resetTokens[cleanEmail]
	if !exists || res.Consumed {
		return ErrTokenNotFound
	}
	if time.Now().After(res.ExpiresAt) {
		return ErrTokenExpired
	}
	if subtle.ConstantTimeCompare([]byte(res.Token), []byte(cleanToken)) != 1 {
		return ErrTokenMismatch
	}

	res.Consumed = true
	s.resetTokens[cleanEmail] = res
	return nil
}


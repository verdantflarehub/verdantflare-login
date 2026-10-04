package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

const resendEmailsURL = "https://api.resend.com/emails"

// ResendMailer sends transactional Login messages. Provider responses and
// message contents are never included in returned errors or application logs.
type ResendMailer struct {
	endpoint string
	apiKey   string
	from     string
	client   *http.Client
}

func NewResendMailer(apiKey, from string) (*ResendMailer, error) {
	apiKey = strings.TrimSpace(apiKey)
	from = strings.TrimSpace(from)
	address, err := mail.ParseAddress(from)
	if apiKey == "" || err != nil || address.Address == "" {
		return nil, errors.New("Resend key and valid sender address are required")
	}
	return &ResendMailer{
		endpoint: resendEmailsURL,
		apiKey:   apiKey,
		from:     from,
		client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}},
	}, nil
}

func (m *ResendMailer) SendVerificationCode(ctx context.Context, recipient, code string, ttl time.Duration) error {
	minutes := int(ttl.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	return m.send(ctx, recipient, "VerdantFlare 邮箱验证码", fmt.Sprintf(
		"您的 VerdantFlare 邮箱验证码是：%s\n\n验证码在 %d 分钟内有效。如果不是您本人操作，请忽略此邮件。", code, minutes))
}

func (m *ResendMailer) SendPasswordReset(ctx context.Context, recipient, resetURL string, ttl time.Duration) error {
	minutes := int(ttl.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	return m.send(ctx, recipient, "重置 VerdantFlare 密码", fmt.Sprintf(
		"请使用以下链接重置 VerdantFlare 密码：\n%s\n\n链接在 %d 分钟内有效。如果不是您本人操作，请忽略此邮件。", resetURL, minutes))
}

func (m *ResendMailer) send(ctx context.Context, recipient, subject, message string) error {
	body, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}{From: m.from, To: []string{recipient}, Subject: subject, Text: message})
	if err != nil {
		return errors.New("encode mail request failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("create mail request failed")
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := m.client.Do(request)
	if err != nil {
		return errors.New("mail delivery request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("mail provider rejected request (HTTP %d)", response.StatusCode)
	}
	var accepted struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&accepted); err != nil || strings.TrimSpace(accepted.ID) == "" {
		return errors.New("mail provider returned an invalid acceptance response")
	}
	return nil
}

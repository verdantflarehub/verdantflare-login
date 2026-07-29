package mailer

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// LogMailer records delivery metadata only. It deliberately never writes a
// verification code, reset token, or complete email address to application logs.
type LogMailer struct {
	Logger *slog.Logger
}

func (m LogMailer) SendVerificationCode(_ context.Context, email, _ string, ttl time.Duration) error {
	m.Logger.Info("verification email accepted", "recipient", maskEmail(email), "ttl", ttl.String())
	return nil
}

func (m LogMailer) SendPasswordReset(_ context.Context, email, _ string, ttl time.Duration) error {
	m.Logger.Info("password reset email accepted", "recipient", maskEmail(email), "ttl", ttl.String())
	return nil
}

func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "***"
	}
	return string(parts[0][0]) + "***@" + parts[1]
}

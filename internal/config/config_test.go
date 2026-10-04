package config

import "testing"

func TestProductionRequiresRealMailDelivery(t *testing.T) {
	t.Setenv("VF_ENV", "production")
	t.Setenv("VF_TOKEN_PEPPER", "test-token-pepper-with-at-least-thirty-two-characters")
	t.Setenv("VF_DATABASE_URL", "postgres://localhost/login")
	t.Setenv("VF_HUB_URL", "https://hub.example.com")
	t.Setenv("VF_MAIL_PROVIDER", "")
	t.Setenv("VF_RESEND_API_KEY", "")
	t.Setenv("VF_MAIL_FROM", "")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted missing mail delivery configuration")
	}
	t.Setenv("VF_RESEND_API_KEY", "re_test")
	t.Setenv("VF_MAIL_FROM", "VerdantFlare <no-reply@example.com>")
	configuration, err := Load()
	if err != nil || configuration.MailProvider != "resend" {
		t.Fatalf("valid Resend configuration rejected: %v", err)
	}
	t.Setenv("VF_MAIL_PROVIDER", "log")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted log-only mail delivery")
	}
	t.Setenv("VF_MAIL_PROVIDER", "resend")
	t.Setenv("VF_EXPOSE_DEBUG_CODES", "true")
	if _, err := Load(); err == nil {
		t.Fatal("production accepted exposed verification codes")
	}
}

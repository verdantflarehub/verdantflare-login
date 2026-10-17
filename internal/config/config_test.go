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

func TestAlternateRootDomainRequiresMatchingSecureConfiguration(t *testing.T) {
	t.Setenv("VF_ENV", "development")
	t.Setenv("VF_MAIL_PROVIDER", "log")
	t.Setenv("VF_ALT_LOGIN_HOST", "login.verdantflare.ai")
	if _, err := Load(); err == nil {
		t.Fatal("accepted an incomplete alternate domain")
	}
	t.Setenv("VF_ALT_HUB_URL", "https://hub.verdantflare.ai")
	t.Setenv("VF_ALT_PUBLIC_LOGIN_URL", "https://login.verdantflare.ai")
	t.Setenv("VF_ALT_SESSION_COOKIE_DOMAIN", ".verdantflare.ai")
	if _, err := Load(); err != nil {
		t.Fatalf("valid alternate domain rejected: %v", err)
	}
	t.Setenv("VF_ALT_HUB_URL", "http://hub.verdantflare.ai")
	if _, err := Load(); err == nil {
		t.Fatal("accepted insecure alternate Hub URL")
	}
	t.Setenv("VF_ALT_HUB_URL", "https://hub.other.example")
	if _, err := Load(); err == nil {
		t.Fatal("accepted an alternate Hub outside the cookie root")
	}
}

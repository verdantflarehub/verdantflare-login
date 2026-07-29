package auth_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
	"github.com/verdantflarehub/verdantflare-login/internal/store/memory"
)

type captureMailer struct {
	verificationCode string
	resetURL         string
}

func (m *captureMailer) SendVerificationCode(_ context.Context, _ string, code string, _ time.Duration) error {
	m.verificationCode = code
	return nil
}

func (m *captureMailer) SendPasswordReset(_ context.Context, _ string, resetURL string, _ time.Duration) error {
	m.resetURL = resetURL
	return nil
}

func TestAuthenticationLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mailer := &captureMailer{}
	service, err := auth.NewService(memory.New(), mailer, auth.ServiceConfig{
		HubURL:           "https://hub.verdantflarehub.com",
		PublicLoginURL:   "https://login.verdantflarehub.com",
		SessionTTL:       12 * time.Hour,
		VerificationTTL:  10 * time.Minute,
		PasswordResetTTL: 30 * time.Minute,
		TokenPepper:      []byte("test-pepper-with-at-least-thirty-two-characters"),
		ExposeDebugCodes: true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	verification, err := service.SendVerificationCode(ctx, "Owner@VerdantFlare.com")
	if err != nil {
		t.Fatalf("send verification: %v", err)
	}
	if verification.DebugCode == "" || verification.DebugCode != mailer.verificationCode {
		t.Fatalf("debug verification code does not match delivered code")
	}

	user, err := service.SignUp(ctx, auth.SignUpInput{
		Email: "owner@verdantflare.com", Code: verification.DebugCode,
		Password: "FirstPassword88", Accepted: true,
	})
	if err != nil {
		t.Fatalf("sign up: %v", err)
	}
	if user.Email != "owner@verdantflare.com" || user.EmailVerifiedAt == nil {
		t.Fatalf("unexpected registered user: %+v", user)
	}

	signIn, err := service.SignIn(ctx, auth.SignInInput{
		Email: "owner@verdantflare.com", Password: "FirstPassword88",
		ReturnTo: "/api/models?from=login", IPAddress: "127.0.0.1", UserAgent: "test",
	})
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if signIn.RedirectTo != "https://hub.verdantflarehub.com/api/models?from=login" {
		t.Fatalf("unexpected redirect: %s", signIn.RedirectTo)
	}
	if _, err := service.Session(ctx, signIn.SessionToken); err != nil {
		t.Fatalf("read session: %v", err)
	}

	if err := service.ForgotPassword(ctx, "owner@verdantflare.com"); err != nil {
		t.Fatalf("forgot password: %v", err)
	}
	resetURL, err := url.Parse(mailer.resetURL)
	if err != nil {
		t.Fatalf("parse reset URL: %v", err)
	}
	resetToken := resetURL.Query().Get("token")
	if len(resetToken) < 32 {
		t.Fatalf("reset token missing from delivered URL")
	}
	if err := service.ResetPassword(ctx, resetToken, "SecondPassword99"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if _, err := service.Session(ctx, signIn.SessionToken); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("old session should be revoked, got %v", err)
	}
	if _, err := service.SignIn(ctx, auth.SignInInput{Email: user.Email, Password: "FirstPassword88"}); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("old password should fail, got %v", err)
	}
	if _, err := service.SignIn(ctx, auth.SignInInput{Email: user.Email, Password: "SecondPassword99"}); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestHubRedirectRejectsExternalTargets(t *testing.T) {
	t.Parallel()
	service, err := auth.NewService(memory.New(), &captureMailer{}, auth.ServiceConfig{
		HubURL:           "https://hub.verdantflarehub.com",
		PublicLoginURL:   "https://login.verdantflarehub.com",
		SessionTTL:       time.Hour,
		VerificationTTL:  time.Minute,
		PasswordResetTTL: time.Minute,
		TokenPepper:      []byte("test-pepper-with-at-least-thirty-two-characters"),
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	tests := map[string]string{
		"absolute URL":      "https://evil.example/steal",
		"protocol relative": "//evil.example/steal",
		"backslash":         "/\\evil.example/steal",
		"empty":             "",
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			if redirect := service.HubRedirect(target); redirect != "https://hub.verdantflarehub.com/" {
				t.Fatalf("unsafe target %q produced %q", target, redirect)
			}
		})
	}
}

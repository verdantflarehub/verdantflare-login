package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
	"github.com/verdantflarehub/verdantflare-login/internal/httpapi"
	"github.com/verdantflarehub/verdantflare-login/internal/store/memory"
)

type testMailer struct {
	code            string
	verificationErr error
}

func (m *testMailer) SendVerificationCode(_ context.Context, _ string, code string, _ time.Duration) error {
	m.code = code
	return m.verificationErr
}

func (m *testMailer) SendPasswordReset(_ context.Context, _, _ string, _ time.Duration) error {
	return nil
}

func TestHTTPRegistrationLoginSessionAndLogout(t *testing.T) {
	t.Parallel()
	mailer := &testMailer{}
	handler := newTestHandler(t, mailer)

	verification := performJSON(t, handler, http.MethodPost, "/api/auth/verification-code", map[string]any{
		"email": "user@verdantflare.com",
	}, nil)
	if verification.Code != http.StatusAccepted {
		t.Fatalf("verification status = %d, body = %s", verification.Code, verification.Body.String())
	}
	var verificationBody struct {
		DebugCode string `json:"debugCode"`
	}
	decodeResponse(t, verification, &verificationBody)
	if verificationBody.DebugCode == "" || verificationBody.DebugCode != mailer.code {
		t.Fatalf("verification code missing")
	}

	signUp := performJSON(t, handler, http.MethodPost, "/api/auth/sign-up", map[string]any{
		"email": "user@verdantflare.com", "code": verificationBody.DebugCode,
		"password": "Password88", "accepted": true,
	}, nil)
	if signUp.Code != http.StatusCreated {
		t.Fatalf("signup status = %d, body = %s", signUp.Code, signUp.Body.String())
	}

	signIn := performJSON(t, handler, http.MethodPost, "/api/auth/sign-in", map[string]any{
		"email": "user@verdantflare.com", "password": "Password88", "returnTo": "/experience",
	}, nil)
	if signIn.Code != http.StatusOK {
		t.Fatalf("signin status = %d, body = %s", signIn.Code, signIn.Body.String())
	}
	cookies := signIn.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one session cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "vf_session" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("unsafe session cookie: %+v", cookie)
	}

	session := performJSON(t, handler, http.MethodGet, "/api/auth/session", nil, cookie)
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d, body = %s", session.Code, session.Body.String())
	}

	logout := performJSON(t, handler, http.MethodPost, "/api/auth/logout", map[string]any{}, cookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", logout.Code)
	}
	sessionAfterLogout := performJSON(t, handler, http.MethodGet, "/api/auth/session", nil, cookie)
	if sessionAfterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("session should be unauthorized after logout, got %d", sessionAfterLogout.Code)
	}
}

func TestCrossOriginWriteRequestIsRejected(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, &testMailer{})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/sign-in", bytes.NewBufferString(`{"email":"user@example.com","password":"Password88"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request status = %d, want 403", response.Code)
	}
}

func TestVerificationDoesNotReportSuccessWhenMailerFails(t *testing.T) {
	t.Parallel()
	handler := newTestHandler(t, &testMailer{verificationErr: errors.New("provider unavailable")})
	response := performJSON(t, handler, http.MethodPost, "/api/auth/verification-code", map[string]any{"email": "user@example.com"}, nil)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("verification failure returned HTTP %d instead of an error", response.Code)
	}
	if strings.Contains(response.Body.String(), "provider unavailable") {
		t.Fatal("provider error leaked to browser")
	}
}

func newTestHandler(t *testing.T, mailer auth.Mailer) http.Handler {
	t.Helper()
	service, err := auth.NewService(memory.New(), mailer, auth.ServiceConfig{
		HubURL: "https://hub.verdantflarehub.com", PublicLoginURL: "https://login.verdantflarehub.com",
		SessionTTL: time.Hour, VerificationTTL: 10 * time.Minute, PasswordResetTTL: 30 * time.Minute,
		TokenPepper: []byte("test-pepper-with-at-least-thirty-two-characters"), ExposeDebugCodes: true,
	})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return httpapi.New(service, httpapi.Config{
		CookieName: "vf_session", SessionTTL: time.Hour,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func performJSON(t *testing.T, handler http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

package httpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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

func TestAgentHandoffLoginFactsAreCurrentAndInternal(t *testing.T) {
	store := memory.New()
	now := time.Now().UTC()
	const subject = "00000000-0000-4000-8000-000000000099"
	const ref = "00000000-0000-4000-8000-000000000088"
	const cookieValue = "test-only-login-cookie"
	const bearer = "test-only-handoff-token-with-32-characters"
	user := auth.User{ID: subject, Email: "person@example.test", NormalizedEmail: "person@example.test", Status: auth.UserStatusActive, EmailVerifiedAt: &now, CreatedAt: now}
	if err := store.CreateUser(context.Background(), user, auth.PasswordCredential{UserID: subject, PasswordHash: "not-used"}); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(cookieValue))
	if err := store.SaveSession(context.Background(), auth.Session{ID: ref, UserID: subject, TokenHash: base64.RawURLEncoding.EncodeToString(hash[:]), CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	service, err := auth.NewService(store, &testMailer{}, auth.ServiceConfig{TokenPepper: []byte(strings.Repeat("p", 32))})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service, httpapi.Config{CookieName: "vf_session", HandoffToken: bearer}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := func(path, body, token string, cookie bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if cookie {
			r.AddCookie(&http.Cookie{Name: "vf_session", Value: cookieValue})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	resolvePath := "/api/internal/agent-handoff/v1/session/resolve"
	checkPath := "/api/internal/agent-handoff/v1/session/check"
	if got := request(resolvePath, "", "", true); got.Code != 401 {
		t.Fatalf("missing service identity: %d", got.Code)
	}
	if got := request(resolvePath, "", bearer, false); got.Code != 401 {
		t.Fatalf("missing Login cookie: %d", got.Code)
	}
	if got := request(resolvePath, `{}`, bearer, true); got.Code != 400 {
		t.Fatalf("resolve body accepted: %d", got.Code)
	}
	resolved := request(resolvePath, "", bearer, true)
	if resolved.Code != 200 || resolved.Header().Get("Cache-Control") != "no-store" || strings.Contains(resolved.Body.String(), cookieValue) || strings.Contains(resolved.Body.String(), "person@example.test") {
		t.Fatalf("resolved fact unsafe: %d %s", resolved.Code, resolved.Body.String())
	}
	var fact struct {
		SubjectRef        string `json:"subject_ref"`
		AuthenticationRef string `json:"authentication_ref"`
	}
	if err := json.Unmarshal(resolved.Body.Bytes(), &fact); err != nil || fact.SubjectRef != subject || fact.AuthenticationRef != ref {
		t.Fatalf("wrong fact: %+v %v", fact, err)
	}
	valid := `{"subject_ref":"` + subject + `","authentication_ref":"` + ref + `"}`
	if got := request(checkPath, valid, bearer, false); got.Code != 200 {
		t.Fatalf("current status: %d %s", got.Code, got.Body.String())
	}
	for _, malformed := range []string{
		`{"subject_ref":"` + subject + `","subject_ref":"` + subject + `","authentication_ref":"` + ref + `"}`,
		`{"subject_ref":"` + subject + `","authentication_ref":"` + ref + `","other":1}`,
		valid + `{}`, `{"subject_ref":3,"authentication_ref":"` + ref + `"}`,
		strings.Repeat("x", 16<<10),
	} {
		if got := request(checkPath, malformed, bearer, false); got.Code != 400 {
			t.Fatalf("malformed accepted: %d", got.Code)
		}
	}
	if got := request(checkPath, `{"subject_ref":"00000000-0000-4000-8000-000000000077","authentication_ref":"`+ref+`"}`, bearer, false); got.Code != 403 {
		t.Fatalf("subject mismatch: %d", got.Code)
	}
	if err := service.Logout(context.Background(), cookieValue); err != nil {
		t.Fatal(err)
	}
	if got := request(checkPath, valid, bearer, false); got.Code != 403 || !strings.Contains(got.Body.String(), "HANDOFF_REVOKED") {
		t.Fatalf("logout status: %d %s", got.Code, got.Body.String())
	}
}

func TestAgentHandoffSubjectCheckDoesNotRequireSessionOrRevealProfile(t *testing.T) {
	store := memory.New()
	now := time.Now().UTC()
	const active = "00000000-0000-4000-8000-000000000099"
	const disabled = "00000000-0000-4000-8000-000000000088"
	const bearer = "test-only-handoff-token-with-32-characters"
	for _, user := range []auth.User{
		{ID: active, Email: "active@example.test", NormalizedEmail: "active@example.test", Status: auth.UserStatusActive, EmailVerifiedAt: &now},
		{ID: disabled, Email: "disabled@example.test", NormalizedEmail: "disabled@example.test", Status: auth.UserStatusDisabled, EmailVerifiedAt: &now},
	} {
		if err := store.CreateUser(context.Background(), user, auth.PasswordCredential{UserID: user.ID, PasswordHash: "not-used"}); err != nil {
			t.Fatal(err)
		}
	}
	service, err := auth.NewService(store, &testMailer{}, auth.ServiceConfig{TokenPepper: []byte(strings.Repeat("p", 32))})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service, httpapi.Config{HandoffToken: bearer}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := func(body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/internal/agent-handoff/v1/subjects/check", strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	valid := `{"subject_ref":"` + active + `"}`
	if got := request(valid, ""); got.Code != 401 {
		t.Fatalf("missing service token: %d", got.Code)
	}
	if got := request(valid, bearer); got.Code != 200 || got.Header().Get("Cache-Control") != "no-store" || strings.Contains(got.Body.String(), "@example.test") || strings.Contains(got.Body.String(), "authentication_ref") {
		t.Fatalf("active subject fact: %d %s", got.Code, got.Body.String())
	}
	for _, subject := range []string{disabled, "00000000-0000-4000-8000-000000000077"} {
		if got := request(`{"subject_ref":"`+subject+`"}`, bearer); got.Code != 403 || strings.Contains(got.Body.String(), subject) {
			t.Fatalf("inactive subject disclosure: %d %s", got.Code, got.Body.String())
		}
	}
	for _, malformed := range []string{`{}`, valid + `{}`, `{"subject_ref":"` + active + `","subject_ref":"` + active + `"}`, `{"subject_ref":"` + active + `","email":"active@example.test"}`, `{"subject_ref":1}`, strings.Repeat("x", 16<<10)} {
		if got := request(malformed, bearer); got.Code != 400 {
			t.Fatalf("malformed subject accepted: %d", got.Code)
		}
	}
}

//go:build integration

package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
	"github.com/verdantflarehub/verdantflare-login/internal/httpapi"
	"github.com/verdantflarehub/verdantflare-login/internal/store/postgres"
)

// This test runs only in an isolated remote database. Mail delivery is a fixture;
// registration, password hashing, sessions, PostgreSQL and HTTPS are real components.
func TestAssistantNodeLoginSessionPostgres(t *testing.T) {
	connection := os.Getenv("ASSISTANT_TEST_DATABASE_URL")
	u, err := url.Parse(connection)
	if err != nil || !strings.HasPrefix(u.Path, "/pa_test_") {
		t.Fatal("isolated pa_test_ database required")
	}
	node, probe := os.Getenv("ASSISTANT_TEST_NODE"), os.Getenv("ASSISTANT_TEST_PROBE")
	if node == "" || probe == "" {
		t.Fatal("compiled Node Login probe required")
	}
	db, err := sql.Open("pgx", connection)
	if err != nil {
		t.Fatal("open isolated database failed")
	}
	defer db.Close()
	migration, err := os.ReadFile("migrations/postgres/001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatal("Login migration failed")
	}
	repo, err := postgres.Open(connection, 4, 1)
	if err != nil {
		t.Fatal("Login repository initialization failed")
	}
	defer repo.Close()
	mailer := &testMailer{}
	service, err := auth.NewService(repo, mailer, auth.ServiceConfig{
		HubURL: "https://hub.example.invalid", AssistantReturnURL: "https://assistant.example.invalid/connect", PublicLoginURL: "https://login.example.invalid",
		SessionTTL: time.Hour, VerificationTTL: 10 * time.Minute, PasswordResetTTL: 10 * time.Minute,
		TokenPepper: []byte("isolated-test-pepper-not-a-real-credential"), ExposeDebugCodes: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(service, httpapi.Config{CookieName: "vf_session", CookieSecure: true, SessionTTL: time.Hour,
		PublicLoginURL: "https://login.example.invalid"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	server := httptest.NewTLSServer(handler)
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	post := func(path string, body any, status int) *http.Response {
		t.Helper()
		encoded, e := json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
		response, e := client.Post(server.URL+path, "application/json", bytes.NewReader(encoded))
		if e != nil {
			t.Fatal("Login HTTPS request failed")
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("%s status %d; expected %d", path, response.StatusCode, status)
		}
		return response
	}
	const email = "assistant-test@example.invalid"
	post("/api/auth/verification-code", map[string]any{"email": email}, 202)
	post("/api/auth/sign-up", map[string]any{"email": email, "code": mailer.code, "password": "Test-only-password-88", "accepted": true}, 201)
	user, _, err := repo.UserByEmail(context.Background(), email)
	if err != nil {
		t.Fatal("registered fixture user not found")
	}
	signIn := func() string {
		response := post("/api/auth/sign-in", map[string]any{"email": email, "password": "Test-only-password-88"}, 200)
		cookies := response.Cookies()
		if len(cookies) != 1 || cookies[0].Name != "vf_session" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatal("Login returned unsafe session cookie")
		}
		return cookies[0].Name + "=" + cookies[0].Value
	}
	dir := t.TempDir()
	caPath := filepath.Join(dir, "login-ca.pem")
	if err = os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(t *testing.T, phase, cookie string) {
		timeout := 20 * time.Second
		if os.Getenv("ASSISTANT_APP_LIFECYCLE") == "1" {
			timeout = 60 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, node, probe)
		cmd.Env = append(os.Environ(), "ASSISTANT_LOGIN_ORIGIN="+server.URL, "ASSISTANT_LOGIN_CA="+caPath, "ASSISTANT_LOGIN_PHASE="+phase)
		input, e := json.Marshal(map[string]string{"cookie": cookie, "subject": user.ID})
		if e != nil {
			t.Fatal(e)
		}
		cmd.Stdin = bytes.NewReader(input)
		output, e := cmd.CombinedOutput()
		// A probe assertion must never leak a session token through its diagnostic.
		if strings.Contains(string(output), strings.TrimPrefix(cookie, "vf_session=")) {
			t.Fatal("probe output contained session credential; output suppressed")
		}
		t.Log(string(output))
		if e != nil {
			t.Fatalf("Node Login probe failed: %v", e)
		}
	}
	t.Run("fixed-app-return-for-signin-and-existing-session", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": email, "password": "Test-only-password-88", "returnTo": "/__assistant_connect"})
		response, e := client.Post(server.URL+"/api/auth/sign-in", "application/json", bytes.NewReader(body))
		if e != nil {
			t.Fatal("sign-in request failed")
		}
		var signed map[string]any
		e = json.NewDecoder(response.Body).Decode(&signed)
		response.Body.Close()
		if e != nil || response.StatusCode != 200 || signed["redirectTo"] != "https://assistant.example.invalid/connect" {
			t.Fatal("new login did not use fixed App return")
		}
		cookies := response.Cookies()
		if len(cookies) != 1 {
			t.Fatal("missing login cookie")
		}
		for _, fixture := range []struct {
			query, want   string
			authenticated bool
		}{
			{"/__assistant_connect", "https://assistant.example.invalid/connect", true},
			{"https://evil.example/steal", "https://hub.example.invalid/", true},
			{"/market", "https://hub.example.invalid/market", true},
			{"/__assistant_connect", "", false},
		} {
			req, _ := http.NewRequest("GET", server.URL+"/api/auth/session?return_to="+url.QueryEscape(fixture.query), nil)
			if fixture.authenticated {
				req.AddCookie(cookies[0])
			}
			response, e := client.Do(req)
			if e != nil {
				t.Fatal("session request failed")
			}
			var result map[string]any
			e = json.NewDecoder(response.Body).Decode(&result)
			response.Body.Close()
			if e != nil {
				t.Fatal("invalid session response")
			}
			if !fixture.authenticated {
				if response.StatusCode != 401 || result["redirectTo"] != nil {
					t.Fatal("unauthenticated redirect resolution allowed")
				}
			} else if response.StatusCode != 200 || result["redirectTo"] != fixture.want {
				t.Fatal("session redirect did not match fixed destination")
			}
		}
	})
	cookie := signIn()
	if os.Getenv("ASSISTANT_LOGIN_COMPOSITION") == "1" {
		t.Run("real-login-control-binding-composition", func(t *testing.T) { run(t, "composition", cookie) })
		return
	}
	if !t.Run("valid-session-and-real-logout", func(t *testing.T) { run(t, "active", cookie) }) {
		return
	}
	cookie = signIn()
	if _, err = db.Exec("UPDATE auth_users SET status='disabled' WHERE id=$1", user.ID); err != nil {
		t.Fatal("disable fixture failed")
	}
	if !t.Run("disabled-account-denied", func(t *testing.T) { run(t, "denied", cookie) }) {
		return
	}
	if _, err = db.Exec("UPDATE auth_users SET status='active' WHERE id=$1", user.ID); err != nil {
		t.Fatal("restore fixture failed")
	}
	cookie = signIn()
	if _, err = db.Exec("UPDATE auth_sessions SET expires_at=now()-interval '1 second' WHERE user_id=$1", user.ID); err != nil {
		t.Fatal("expire fixture failed")
	}
	t.Run("expired-session-denied", func(t *testing.T) { run(t, "denied", cookie) })
}

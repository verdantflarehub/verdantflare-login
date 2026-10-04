package httpapi_test

import (
	"context"
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

func TestDirectoryRequiresTokenAndPaginatesWithoutCredentials(t *testing.T) {
	store := memory.New()
	now := time.Now().UTC()
	for _, entry := range []struct{ id, email string }{{"001", "a@example.test"}, {"002", "b@example.test"}} {
		user := auth.User{ID: entry.id, Email: entry.email, NormalizedEmail: entry.email, Status: auth.UserStatusActive, EmailVerifiedAt: &now, CreatedAt: now}
		if err := store.CreateUser(context.Background(), user, auth.PasswordCredential{UserID: entry.id, PasswordHash: "never-disclose-this"}); err != nil {
			t.Fatal(err)
		}
	}
	service, err := auth.NewService(store, &testMailer{}, auth.ServiceConfig{TokenPepper: []byte("test-pepper-with-at-least-thirty-two-characters")})
	if err != nil {
		t.Fatal(err)
	}
	const token = "test-directory-token-with-at-least-thirty-two-characters"
	handler := httpapi.New(service, httpapi.Config{DirectoryToken: token}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	closed := httpapi.New(service, httpapi.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	closedRequest := httptest.NewRequest(http.MethodGet, "/api/internal/users", nil)
	closedRequest.Header.Set("Authorization", "Bearer "+token)
	closedResponse := httptest.NewRecorder()
	closed.ServeHTTP(closedResponse, closedRequest)
	if closedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unconfigured directory status: %d", closedResponse.Code)
	}
	request := func(path, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if result := request("/api/internal/users", ""); result.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: %d", result.Code)
	}
	if result := request("/api/internal/users", "wrong"); result.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", result.Code)
	}
	first := request("/api/internal/users?limit=1", token)
	if first.Code != http.StatusOK || strings.Contains(first.Body.String(), "never-disclose-this") {
		t.Fatalf("first page: %d %s", first.Code, first.Body.String())
	}
	var page struct {
		Users []struct {
			ID string `json:"id"`
		}
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Users) != 1 || page.Users[0].ID != "001" || page.NextCursor != "001" {
		t.Fatalf("first page: %+v", page)
	}
	second := request("/api/internal/users?cursor=001&limit=1&q=b%40example", token)
	if err := json.Unmarshal(second.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Users) != 1 || page.Users[0].ID != "002" || page.NextCursor != "" {
		t.Fatalf("second page: %+v", page)
	}
}

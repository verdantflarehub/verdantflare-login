package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResendMailerSendsVerificationAndReset(t *testing.T) {
	var messages []struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/emails" || request.Header.Get("Authorization") != "Bearer re_test" {
			t.Errorf("unexpected provider request")
		}
		var message struct {
			From    string   `json:"from"`
			To      []string `json:"to"`
			Subject string   `json:"subject"`
			Text    string   `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&message); err != nil {
			t.Errorf("decode provider request: %v", err)
		}
		messages = append(messages, message)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"email_test_1"}`))
	}))
	defer provider.Close()
	mailer, err := NewResendMailer("re_test", "VerdantFlare <no-reply@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	mailer.endpoint = provider.URL + "/emails"
	if err := mailer.SendVerificationCode(context.Background(), "user@example.com", "123456", 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := mailer.SendPasswordReset(context.Background(), "user@example.com", "https://login.example.com/reset-password?token=test", 30*time.Minute); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].From != "VerdantFlare <no-reply@example.com>" || len(messages[0].To) != 1 || messages[0].To[0] != "user@example.com" ||
		!strings.Contains(messages[0].Text, "123456") || !strings.Contains(messages[0].Text, "10 分钟") ||
		!strings.Contains(messages[1].Text, "https://login.example.com/reset-password?token=test") || !strings.Contains(messages[1].Text, "30 分钟") {
		t.Fatalf("mail payload mismatch")
	}
}

func TestResendMailerDoesNotReportAcceptanceWhenProviderFails(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{name: "rejected", body: `{"message":"secret-code-123456"}`, status: http.StatusForbidden},
		{name: "missing id", body: `{}`, status: http.StatusOK},
		{name: "invalid json", body: `{`, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer provider.Close()
			mailer, err := NewResendMailer("re_test", "no-reply@example.com")
			if err != nil {
				t.Fatal(err)
			}
			mailer.endpoint = provider.URL
			err = mailer.SendVerificationCode(context.Background(), "user@example.com", "123456", 10*time.Minute)
			if err == nil || strings.Contains(err.Error(), "123456") || strings.Contains(err.Error(), "re_test") {
				t.Fatalf("unsafe or missing provider error")
			}
		})
	}
}

func TestResendMailerRequiresSenderAndKey(t *testing.T) {
	for _, input := range [][2]string{{"", "no-reply@example.com"}, {"re_test", ""}, {"re_test", "invalid"}} {
		if _, err := NewResendMailer(input[0], input[1]); err == nil {
			t.Fatal("invalid Resend configuration accepted")
		}
	}
}

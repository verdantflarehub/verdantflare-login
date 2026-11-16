package auth

import "testing"

func TestAssistantReturnConfigurationRejectsUnsafeDestinations(t *testing.T) {
	for _, target := range []string{
		"http://assistant.example/connect", "https://user@assistant.example/connect", "https://assistant.example:8443/connect",
		"https://assistant.example:/connect", "https://assistant.example/other", "https://assistant.example/%63onnect",
		"https://assistant.example/connect?", "https://assistant.example/connect?next=https://evil.example",
		"https://assistant.example/connect#", "https://assistant.example/connect#token", "//assistant.example/connect",
	} {
		if _, err := NewService(nil, nil, ServiceConfig{AssistantReturnURL: target}); err == nil {
			t.Errorf("accepted invalid App return target %q", target)
		}
	}
	for _, target := range []string{"", "https://assistant.example/connect"} {
		if err := validateAssistantReturnURL(target); err != nil {
			t.Errorf("valid configuration rejected: %v", err)
		}
	}
}

func TestAssistantReturnRequiresExactSymbolicTarget(t *testing.T) {
	service := &Service{config: ServiceConfig{HubURL: "https://hub.example", AssistantReturnURL: "https://assistant.example/connect"}}
	if got := service.LoginRedirectAt("/__assistant_connect", "https://alternate-hub.example"); got != "https://assistant.example/connect" {
		t.Fatalf("wrong App return: %q", got)
	}
	for _, target := range []string{
		"https://evil.example/connect", "//evil.example/connect", "/__assistant_connect?next=https://evil.example",
		"/__assistant_connect#challenge=abc", "/__assistant_connect/", "/%5f%5fassistant_connect", "/\\evil.example",
	} {
		got := service.LoginRedirectAt(target, "")
		if got == "https://assistant.example/connect" || len(got) < len("https://hub.example/") || got[:len("https://hub.example/")] != "https://hub.example/" {
			t.Errorf("unexpected redirect for %q: %q", target, got)
		}
	}
	if got := service.LoginRedirectAt("/market?entry=www", "https://alternate-hub.example"); got != "https://alternate-hub.example/market?entry=www" {
		t.Fatalf("Hub routing regressed: %q", got)
	}
	disabled := &Service{config: ServiceConfig{HubURL: "https://hub.example"}}
	if got := disabled.LoginRedirectAt("/__assistant_connect", ""); got != "https://hub.example/__assistant_connect" {
		t.Fatalf("disabled App routing changed: %q", got)
	}
}

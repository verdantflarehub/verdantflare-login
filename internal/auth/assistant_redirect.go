package auth

import (
	"errors"
	"net/url"
	"strings"
)

func validateAssistantReturnURL(value string) error {
	if value == "" {
		return nil
	}
	target, err := url.Parse(value)
	if err != nil || target.Scheme != "https" || target.Hostname() == "" || target.Host != target.Hostname() || target.User != nil ||
		target.Path != "/connect" || target.RawPath != "" || target.RawQuery != "" || target.ForceQuery ||
		target.Fragment != "" || target.Opaque != "" || strings.ContainsAny(value, "\\\r\n\t ") || strings.Contains(value, "#") {
		return errors.New("VF_ASSISTANT_RETURN_URL must be a fixed HTTPS /connect URL")
	}
	return nil
}

// A symbolic target selects one administrator-configured destination. No URL
// from browser input is ever used as an external redirect destination.
func (s *Service) LoginRedirectAt(returnTo, hubURL string) string {
	if returnTo == "/__assistant_connect" && s.config.AssistantReturnURL != "" {
		return s.config.AssistantReturnURL
	}
	if hubURL == "" {
		hubURL = s.config.HubURL
	}
	return s.HubRedirectAt(returnTo, hubURL)
}

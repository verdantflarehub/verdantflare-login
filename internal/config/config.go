package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address          string
	Environment      string
	DatabaseURL      string
	DatabaseMaxOpen  int
	DatabaseMaxIdle  int
	HubURL           string
	PublicLoginURL   string
	CookieName       string
	CookieDomain     string
	CookieSecure     bool
	SessionTTL       time.Duration
	VerificationTTL  time.Duration
	PasswordResetTTL time.Duration
	TokenPepper      []byte
	ExposeDebugCodes bool
}

func Load() (Config, error) {
	environment := env("VF_ENV", "development")
	production := environment == "production"

	cfg := Config{
		Address:          env("VF_LISTEN_ADDR", ":8088"),
		Environment:      environment,
		DatabaseURL:      strings.TrimSpace(os.Getenv("VF_DATABASE_URL")),
		DatabaseMaxOpen:  envInt("VF_DATABASE_MAX_OPEN", 10),
		DatabaseMaxIdle:  envInt("VF_DATABASE_MAX_IDLE", 5),
		HubURL:           env("VF_HUB_URL", "https://hub.verdantflarehub.com"),
		PublicLoginURL:   env("VF_PUBLIC_LOGIN_URL", "http://localhost:4174"),
		CookieName:       env("VF_SESSION_COOKIE_NAME", "vf_session"),
		CookieDomain:     strings.TrimSpace(os.Getenv("VF_SESSION_COOKIE_DOMAIN")),
		CookieSecure:     envBool("VF_SESSION_COOKIE_SECURE", production),
		SessionTTL:       envDuration("VF_SESSION_TTL", 12*time.Hour),
		VerificationTTL:  envDuration("VF_VERIFICATION_TTL", 10*time.Minute),
		PasswordResetTTL: envDuration("VF_PASSWORD_RESET_TTL", 30*time.Minute),
		ExposeDebugCodes: envBool("VF_EXPOSE_DEBUG_CODES", false),
	}

	pepper := strings.TrimSpace(os.Getenv("VF_TOKEN_PEPPER"))
	if pepper == "" && production {
		return Config{}, errors.New("VF_TOKEN_PEPPER is required in production")
	}
	if pepper == "" {
		generated := make([]byte, 32)
		if _, err := rand.Read(generated); err != nil {
			return Config{}, fmt.Errorf("generate development token pepper: %w", err)
		}
		pepper = base64.RawURLEncoding.EncodeToString(generated)
	}
	if production && len(pepper) < 32 {
		return Config{}, errors.New("VF_TOKEN_PEPPER must contain at least 32 characters in production")
	}
	cfg.TokenPepper = []byte(pepper)

	if !strings.HasPrefix(cfg.HubURL, "https://") && production {
		return Config{}, errors.New("VF_HUB_URL must use https in production")
	}
	if production && !cfg.CookieSecure {
		return Config{}, errors.New("VF_SESSION_COOKIE_SECURE cannot be disabled in production")
	}
	if production && cfg.DatabaseURL == "" {
		return Config{}, errors.New("VF_DATABASE_URL is required in production")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrExpired            = errors.New("expired")
	ErrInvalidChallenge   = errors.New("invalid challenge")
	ErrTooManyAttempts    = errors.New("too many attempts")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidInput       = errors.New("invalid input")
	ErrAccountDisabled    = errors.New("account disabled")
)

const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
	PurposeSignUp      = "sign_up"
)

type User struct {
	ID              string
	Email           string
	NormalizedEmail string
	Status          string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PasswordCredential struct {
	UserID            string
	PasswordHash      string
	PasswordChangedAt time.Time
}

type EmailChallenge struct {
	ID          string
	Email       string
	Purpose     string
	SecretHash  string
	Attempts    int
	MaxAttempts int
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
	CreatedAt   time.Time
}

type PasswordReset struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

type Session struct {
	ID         string
	UserID     string
	TokenHash  string
	IPAddress  string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

type Store interface {
	SaveEmailChallenge(context.Context, EmailChallenge) error
	ConsumeEmailChallenge(context.Context, string, string, string, time.Time) error
	CreateUser(context.Context, User, PasswordCredential) error
	CreateUserWithChallenge(context.Context, string, string, string, time.Time, User, PasswordCredential) error
	UserByEmail(context.Context, string) (User, PasswordCredential, error)
	UserByID(context.Context, string) (User, error)
	ListUsers(context.Context, string, string, int) ([]User, string, error)
	UpdatePassword(context.Context, string, PasswordCredential) error
	SavePasswordReset(context.Context, PasswordReset) error
	ConsumePasswordReset(context.Context, string, time.Time) (string, error)
	ResetPassword(context.Context, string, PasswordCredential, time.Time) error
	SaveSession(context.Context, Session) error
	SessionByTokenHash(context.Context, string, time.Time) (Session, User, error)
	DeleteSessionByTokenHash(context.Context, string) error
	DeleteSessionsByUserID(context.Context, string) error
}

type Mailer interface {
	SendVerificationCode(context.Context, string, string, time.Duration) error
	SendPasswordReset(context.Context, string, string, time.Duration) error
}

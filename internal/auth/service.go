package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
)

type ServiceConfig struct {
	HubURL           string
	PublicLoginURL   string
	SessionTTL       time.Duration
	VerificationTTL  time.Duration
	PasswordResetTTL time.Duration
	TokenPepper      []byte
	ExposeDebugCodes bool
}

type Service struct {
	store      Store
	mailer     Mailer
	config     ServiceConfig
	hasher     PasswordHasher
	dummyHash  string
	registerMu sync.Mutex
}

func (s *Service) DirectoryUsers(ctx context.Context, cursor, query string, limit int) ([]User, string, error) {
	return s.store.ListUsers(ctx, cursor, query, limit)
}

func (s *Service) DirectoryUser(ctx context.Context, id string) (User, error) {
	return s.store.UserByID(ctx, id)
}

type VerificationResult struct {
	ExpiresInSeconds int
	DebugCode        string
}

type SignUpInput struct {
	Email    string
	Code     string
	Password string
	Accepted bool
}

type SignInInput struct {
	Email     string
	Password  string
	ReturnTo  string
	IPAddress string
	UserAgent string
}

type SignInResult struct {
	SessionToken string
	RedirectTo   string
	ExpiresAt    time.Time
}

type SessionResult struct {
	UserID    string
	Email     string
	ExpiresAt time.Time
}

func NewService(store Store, mailer Mailer, config ServiceConfig) (*Service, error) {
	hasher := DefaultPasswordHasher()
	dummyHash, err := hasher.Hash("VerdantFlare timing equalization password 4E2A9F")
	if err != nil {
		return nil, err
	}
	return &Service{
		store:     store,
		mailer:    mailer,
		config:    config,
		hasher:    hasher,
		dummyHash: dummyHash,
	}, nil
}

func (s *Service) SendVerificationCode(ctx context.Context, rawEmail string) (VerificationResult, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return VerificationResult{}, ErrInvalidInput
	}
	if _, _, err := s.store.UserByEmail(ctx, email); err == nil {
		return VerificationResult{}, ErrConflict
	} else if err != ErrNotFound {
		return VerificationResult{}, err
	}

	code, err := randomNumericCode()
	if err != nil {
		return VerificationResult{}, err
	}
	challengeID, err := randomID()
	if err != nil {
		return VerificationResult{}, err
	}
	now := time.Now().UTC()
	challenge := EmailChallenge{
		ID:          challengeID,
		Email:       email,
		Purpose:     PurposeSignUp,
		SecretHash:  s.secretHash("verification", email+"\x00"+code),
		MaxAttempts: 5,
		ExpiresAt:   now.Add(s.config.VerificationTTL),
		CreatedAt:   now,
	}
	if err := s.store.SaveEmailChallenge(ctx, challenge); err != nil {
		return VerificationResult{}, err
	}
	if err := s.mailer.SendVerificationCode(ctx, email, code, s.config.VerificationTTL); err != nil {
		return VerificationResult{}, err
	}

	result := VerificationResult{ExpiresInSeconds: int(s.config.VerificationTTL.Seconds())}
	if s.config.ExposeDebugCodes {
		result.DebugCode = code
	}
	return result, nil
}

func (s *Service) SignUp(ctx context.Context, input SignUpInput) (User, error) {
	email, err := normalizeEmail(input.Email)
	if err != nil || !validVerificationCode(input.Code) || !input.Accepted {
		return User{}, ErrInvalidInput
	}
	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}

	s.registerMu.Lock()
	defer s.registerMu.Unlock()
	if _, _, err := s.store.UserByEmail(ctx, email); err == nil {
		return User{}, ErrConflict
	} else if err != ErrNotFound {
		return User{}, err
	}

	now := time.Now().UTC()
	codeHash := s.secretHash("verification", email+"\x00"+input.Code)
	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return User{}, err
	}
	userID, err := randomID()
	if err != nil {
		return User{}, err
	}
	user := User{
		ID:              userID,
		Email:           email,
		NormalizedEmail: email,
		Status:          UserStatusActive,
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	credential := PasswordCredential{
		UserID:            user.ID,
		PasswordHash:      passwordHash,
		PasswordChangedAt: now,
	}
	if err := s.store.CreateUserWithChallenge(ctx, email, PurposeSignUp, codeHash, now, user, credential); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) SignIn(ctx context.Context, input SignInInput) (SignInResult, error) {
	email, normalizeErr := normalizeEmail(input.Email)
	user, credential, lookupErr := s.store.UserByEmail(ctx, email)
	passwordHash := s.dummyHash
	if lookupErr == nil {
		passwordHash = credential.PasswordHash
	}
	passwordOK, verifyErr := s.hasher.Verify(passwordHash, input.Password)
	if normalizeErr != nil || lookupErr != nil || verifyErr != nil || !passwordOK {
		return SignInResult{}, ErrInvalidCredentials
	}
	if user.Status != UserStatusActive {
		return SignInResult{}, ErrAccountDisabled
	}

	token, err := randomToken(32)
	if err != nil {
		return SignInResult{}, err
	}
	sessionID, err := randomID()
	if err != nil {
		return SignInResult{}, err
	}
	now := time.Now().UTC()
	session := Session{
		ID:         sessionID,
		UserID:     user.ID,
		TokenHash:  sessionTokenHash(token),
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.config.SessionTTL),
	}
	if err := s.store.SaveSession(ctx, session); err != nil {
		return SignInResult{}, err
	}
	return SignInResult{
		SessionToken: token,
		RedirectTo:   s.HubRedirect(input.ReturnTo),
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

func (s *Service) Session(ctx context.Context, token string) (SessionResult, error) {
	if token == "" {
		return SessionResult{}, ErrNotFound
	}
	session, user, err := s.store.SessionByTokenHash(ctx, sessionTokenHash(token), time.Now().UTC())
	if err != nil {
		return SessionResult{}, err
	}
	if user.Status != UserStatusActive {
		return SessionResult{}, ErrAccountDisabled
	}
	return SessionResult{UserID: user.ID, Email: user.Email, ExpiresAt: session.ExpiresAt}, nil
}

func (s *Service) ForgotPassword(ctx context.Context, rawEmail string) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return nil
	}
	user, _, err := s.store.UserByEmail(ctx, email)
	if err == ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	token, err := randomToken(32)
	if err != nil {
		return err
	}
	resetID, err := randomID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	reset := PasswordReset{
		ID:        resetID,
		UserID:    user.ID,
		TokenHash: s.secretHash("password-reset", token),
		ExpiresAt: now.Add(s.config.PasswordResetTTL),
		CreatedAt: now,
	}
	if err := s.store.SavePasswordReset(ctx, reset); err != nil {
		return err
	}
	resetURL, err := url.Parse(s.config.PublicLoginURL)
	if err != nil {
		return err
	}
	resetURL.Path = "/reset-password"
	query := resetURL.Query()
	query.Set("token", token)
	resetURL.RawQuery = query.Encode()
	return s.mailer.SendPasswordReset(ctx, email, resetURL.String(), s.config.PasswordResetTTL)
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if len(token) < 32 {
		return ErrInvalidInput
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	credential := PasswordCredential{PasswordHash: passwordHash, PasswordChangedAt: now}
	return s.store.ResetPassword(ctx, s.secretHash("password-reset", token), credential, now)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSessionByTokenHash(ctx, sessionTokenHash(token))
}

func (s *Service) HubRedirect(returnTo string) string {
	base, err := url.Parse(s.config.HubURL)
	if err != nil {
		return s.config.HubURL
	}
	safePath := safeReturnTo(returnTo)
	target, err := url.Parse(safePath)
	if err != nil {
		return base.String()
	}
	base.Path = target.Path
	base.RawQuery = target.RawQuery
	base.Fragment = target.Fragment
	return base.String()
}

func safeReturnTo(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, "\\") {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return parsed.String()
}

func normalizeEmail(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 254 || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalidInput
	}
	address, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(address.Address, value) || !strings.Contains(address.Address, "@") {
		return "", ErrInvalidInput
	}
	return strings.ToLower(address.Address), nil
}

func validatePassword(password string) error {
	if len(password) < 8 || len(password) > 128 {
		return ErrInvalidInput
	}
	var hasLetter, hasDigit bool
	for _, character := range password {
		hasLetter = hasLetter || unicode.IsLetter(character)
		hasDigit = hasDigit || unicode.IsDigit(character)
	}
	if !hasLetter || !hasDigit {
		return ErrInvalidInput
	}
	return nil
}

func validVerificationCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, character := range code {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func randomNumericCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", buffer[0:4], buffer[4:6], buffer[6:8], buffer[8:10], buffer[10:16]), nil
}

func sessionTokenHash(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (s *Service) secretHash(purpose, value string) string {
	mac := hmac.New(sha256.New, s.config.TokenPepper)
	mac.Write([]byte(purpose))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

package memory

import (
	"context"
	"crypto/subtle"
	"sync"
	"time"

	"github.com/verdantflarehub/verdantflare-login/internal/auth"
)

type Store struct {
	mu             sync.RWMutex
	users          map[string]auth.User
	userIDsByEmail map[string]string
	credentials    map[string]auth.PasswordCredential
	challenges     map[string]auth.EmailChallenge
	passwordResets map[string]auth.PasswordReset
	sessions       map[string]auth.Session
}

func New() *Store {
	return &Store{
		users:          make(map[string]auth.User),
		userIDsByEmail: make(map[string]string),
		credentials:    make(map[string]auth.PasswordCredential),
		challenges:     make(map[string]auth.EmailChallenge),
		passwordResets: make(map[string]auth.PasswordReset),
		sessions:       make(map[string]auth.Session),
	}
}

func (s *Store) SaveEmailChallenge(_ context.Context, challenge auth.EmailChallenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenges[challengeKey(challenge.Email, challenge.Purpose)] = challenge
	return nil
}

func (s *Store) ConsumeEmailChallenge(_ context.Context, email, purpose, secretHash string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := challengeKey(email, purpose)
	challenge, ok := s.challenges[key]
	if !ok || challenge.ConsumedAt != nil {
		return auth.ErrInvalidChallenge
	}
	if !now.Before(challenge.ExpiresAt) {
		return auth.ErrExpired
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		return auth.ErrTooManyAttempts
	}
	challenge.Attempts++
	if subtle.ConstantTimeCompare([]byte(challenge.SecretHash), []byte(secretHash)) != 1 {
		s.challenges[key] = challenge
		return auth.ErrInvalidChallenge
	}
	challenge.ConsumedAt = &now
	s.challenges[key] = challenge
	return nil
}

func (s *Store) CreateUser(_ context.Context, user auth.User, credential auth.PasswordCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.userIDsByEmail[user.NormalizedEmail]; exists {
		return auth.ErrConflict
	}
	s.users[user.ID] = user
	s.userIDsByEmail[user.NormalizedEmail] = user.ID
	s.credentials[user.ID] = credential
	return nil
}

func (s *Store) CreateUserWithChallenge(_ context.Context, email, purpose, secretHash string, now time.Time, user auth.User, credential auth.PasswordCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[challengeKey(email, purpose)]
	if !ok || challenge.ConsumedAt != nil {
		return auth.ErrInvalidChallenge
	}
	if !now.Before(challenge.ExpiresAt) {
		return auth.ErrExpired
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		return auth.ErrTooManyAttempts
	}
	challenge.Attempts++
	if subtle.ConstantTimeCompare([]byte(challenge.SecretHash), []byte(secretHash)) != 1 {
		s.challenges[challengeKey(email, purpose)] = challenge
		return auth.ErrInvalidChallenge
	}
	if _, exists := s.userIDsByEmail[user.NormalizedEmail]; exists {
		return auth.ErrConflict
	}
	challenge.ConsumedAt = &now
	s.challenges[challengeKey(email, purpose)] = challenge
	s.users[user.ID] = user
	s.userIDsByEmail[user.NormalizedEmail] = user.ID
	s.credentials[user.ID] = credential
	return nil
}

func (s *Store) UserByEmail(_ context.Context, normalizedEmail string) (auth.User, auth.PasswordCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID, ok := s.userIDsByEmail[normalizedEmail]
	if !ok {
		return auth.User{}, auth.PasswordCredential{}, auth.ErrNotFound
	}
	return s.users[userID], s.credentials[userID], nil
}

func (s *Store) UserByID(_ context.Context, userID string) (auth.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userID]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return user, nil
}

func (s *Store) UpdatePassword(_ context.Context, userID string, credential auth.PasswordCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userID]; !ok {
		return auth.ErrNotFound
	}
	s.credentials[userID] = credential
	return nil
}

func (s *Store) SavePasswordReset(_ context.Context, reset auth.PasswordReset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passwordResets[reset.TokenHash] = reset
	return nil
}

func (s *Store) ConsumePasswordReset(_ context.Context, tokenHash string, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reset, ok := s.passwordResets[tokenHash]
	if !ok || reset.ConsumedAt != nil {
		return "", auth.ErrNotFound
	}
	if !now.Before(reset.ExpiresAt) {
		return "", auth.ErrExpired
	}
	reset.ConsumedAt = &now
	s.passwordResets[tokenHash] = reset
	return reset.UserID, nil
}

func (s *Store) ResetPassword(_ context.Context, tokenHash string, credential auth.PasswordCredential, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	reset, ok := s.passwordResets[tokenHash]
	if !ok || reset.ConsumedAt != nil {
		return auth.ErrNotFound
	}
	if !now.Before(reset.ExpiresAt) {
		return auth.ErrExpired
	}
	reset.ConsumedAt = &now
	s.passwordResets[tokenHash] = reset
	credential.UserID = reset.UserID
	s.credentials[reset.UserID] = credential
	for hash, session := range s.sessions {
		if session.UserID == reset.UserID {
			delete(s.sessions, hash)
		}
	}
	return nil
}

func (s *Store) SaveSession(_ context.Context, session auth.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.TokenHash] = session
	return nil
}

func (s *Store) SessionByTokenHash(_ context.Context, tokenHash string, now time.Time) (auth.Session, auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[tokenHash]
	if !ok {
		return auth.Session{}, auth.User{}, auth.ErrNotFound
	}
	if !now.Before(session.ExpiresAt) {
		delete(s.sessions, tokenHash)
		return auth.Session{}, auth.User{}, auth.ErrExpired
	}
	user, ok := s.users[session.UserID]
	if !ok {
		delete(s.sessions, tokenHash)
		return auth.Session{}, auth.User{}, auth.ErrNotFound
	}
	session.LastSeenAt = now
	s.sessions[tokenHash] = session
	return session, user, nil
}

func (s *Store) DeleteSessionByTokenHash(_ context.Context, tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tokenHash)
	return nil
}

func (s *Store) DeleteSessionsByUserID(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tokenHash, session := range s.sessions {
		if session.UserID == userID {
			delete(s.sessions, tokenHash)
		}
	}
	return nil
}

func challengeKey(email, purpose string) string {
	return email + "\x00" + purpose
}

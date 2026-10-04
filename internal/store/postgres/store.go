package postgres

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/verdantflarehub/verdantflare-login/internal/auth"
)

type Store struct {
	db *sql.DB
}

func Open(databaseURL string, maxOpen, maxIdle int) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) SaveEmailChallenge(ctx context.Context, challenge auth.EmailChallenge) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_email_challenges
			(id, normalized_email, purpose, secret_hash, attempts, max_attempts, expires_at, consumed_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (normalized_email, purpose) DO UPDATE SET
			id=EXCLUDED.id, secret_hash=EXCLUDED.secret_hash, attempts=0,
			max_attempts=EXCLUDED.max_attempts, expires_at=EXCLUDED.expires_at,
			consumed_at=NULL, created_at=EXCLUDED.created_at`,
		challenge.ID, challenge.Email, challenge.Purpose, challenge.SecretHash, challenge.Attempts,
		challenge.MaxAttempts, challenge.ExpiresAt, challenge.ConsumedAt, challenge.CreatedAt)
	return wrap(err)
}

func (s *Store) ConsumeEmailChallenge(ctx context.Context, email, purpose, secretHash string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrap(err)
	}
	defer tx.Rollback()
	var storedHash string
	var attempts, maxAttempts int
	var expiresAt time.Time
	var consumedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT secret_hash, attempts, max_attempts, expires_at, consumed_at
		FROM auth_email_challenges
		WHERE normalized_email=$1 AND purpose=$2
		FOR UPDATE`, email, purpose).Scan(&storedHash, &attempts, &maxAttempts, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) || consumedAt.Valid {
		return auth.ErrInvalidChallenge
	}
	if err != nil {
		return wrap(err)
	}
	if !now.Before(expiresAt) {
		return auth.ErrExpired
	}
	if attempts >= maxAttempts {
		return auth.ErrTooManyAttempts
	}
	attempts++
	if storedHash != secretHash {
		if _, err := tx.ExecContext(ctx, `UPDATE auth_email_challenges SET attempts=$1 WHERE normalized_email=$2 AND purpose=$3`, attempts, email, purpose); err != nil {
			return wrap(err)
		}
		if err := tx.Commit(); err != nil {
			return wrap(err)
		}
		return auth.ErrInvalidChallenge
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_email_challenges SET attempts=$1, consumed_at=$2 WHERE normalized_email=$3 AND purpose=$4`, attempts, now, email, purpose); err != nil {
		return wrap(err)
	}
	return wrap(tx.Commit())
}

func (s *Store) CreateUser(ctx context.Context, user auth.User, credential auth.PasswordCredential) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrap(err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth_users (id,email,normalized_email,status,email_verified_at,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		user.ID, user.Email, user.NormalizedEmail, user.Status, user.EmailVerifiedAt, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return auth.ErrConflict
		}
		return wrap(err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth_password_credentials (user_id,password_hash,password_changed_at,updated_at)
		VALUES ($1,$2,$3,$3)`, credential.UserID, credential.PasswordHash, credential.PasswordChangedAt)
	if err != nil {
		return wrap(err)
	}
	return wrap(tx.Commit())
}

func (s *Store) CreateUserWithChallenge(ctx context.Context, email, purpose, secretHash string, now time.Time, user auth.User, credential auth.PasswordCredential) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrap(err)
	}
	defer tx.Rollback()
	var storedHash string
	var attempts, maxAttempts int
	var expiresAt time.Time
	var consumedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT secret_hash,attempts,max_attempts,expires_at,consumed_at
		FROM auth_email_challenges
		WHERE normalized_email=$1 AND purpose=$2
		FOR UPDATE`, email, purpose).Scan(&storedHash, &attempts, &maxAttempts, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) || consumedAt.Valid {
		return auth.ErrInvalidChallenge
	}
	if err != nil {
		return wrap(err)
	}
	if !now.Before(expiresAt) {
		return auth.ErrExpired
	}
	if attempts >= maxAttempts {
		return auth.ErrTooManyAttempts
	}
	attempts++
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(secretHash)) != 1 {
		if _, err := tx.ExecContext(ctx, `UPDATE auth_email_challenges SET attempts=$1 WHERE normalized_email=$2 AND purpose=$3`, attempts, email, purpose); err != nil {
			return wrap(err)
		}
		if err := tx.Commit(); err != nil {
			return wrap(err)
		}
		return auth.ErrInvalidChallenge
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_users (id,email,normalized_email,status,email_verified_at,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		user.ID, user.Email, user.NormalizedEmail, user.Status, user.EmailVerifiedAt, user.CreatedAt, user.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return auth.ErrConflict
		}
		return wrap(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_password_credentials (user_id,password_hash,password_changed_at,updated_at)
		VALUES ($1,$2,$3,$3)`, credential.UserID, credential.PasswordHash, credential.PasswordChangedAt); err != nil {
		return wrap(err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_challenges SET attempts=$1,consumed_at=$2
		WHERE normalized_email=$3 AND purpose=$4`, attempts, now, email, purpose); err != nil {
		return wrap(err)
	}
	return wrap(tx.Commit())
}

func (s *Store) UserByEmail(ctx context.Context, normalizedEmail string) (auth.User, auth.PasswordCredential, error) {
	return s.userWithCredential(ctx, `u.normalized_email=$1`, normalizedEmail)
}

func (s *Store) userWithCredential(ctx context.Context, predicate string, value any) (auth.User, auth.PasswordCredential, error) {
	var user auth.User
	var credential auth.PasswordCredential
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id,u.email,u.normalized_email,u.status,u.email_verified_at,u.created_at,u.updated_at,
		       c.user_id,c.password_hash,c.password_changed_at
		FROM auth_users u JOIN auth_password_credentials c ON c.user_id=u.id
		WHERE `+predicate+` AND u.deleted_at IS NULL`, value).Scan(
		&user.ID, &user.Email, &user.NormalizedEmail, &user.Status, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt,
		&credential.UserID, &credential.PasswordHash, &credential.PasswordChangedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.PasswordCredential{}, auth.ErrNotFound
	}
	return user, credential, wrap(err)
}

func (s *Store) UserByID(ctx context.Context, userID string) (auth.User, error) {
	var user auth.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id,email,normalized_email,status,email_verified_at,created_at,updated_at
		FROM auth_users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(
		&user.ID, &user.Email, &user.NormalizedEmail, &user.Status, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	return user, wrap(err)
}

func (s *Store) ListUsers(ctx context.Context, cursor, query string, limit int) ([]auth.User, string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,email,normalized_email,status,email_verified_at,created_at,updated_at
		FROM auth_users
		WHERE deleted_at IS NULL AND ($1 = '' OR id::text > $1)
		  AND POSITION($2 IN normalized_email) > 0
		ORDER BY id LIMIT $3`, cursor, strings.ToLower(query), limit+1)
	if err != nil {
		return nil, "", wrap(err)
	}
	defer rows.Close()
	users := make([]auth.User, 0, limit+1)
	for rows.Next() {
		var user auth.User
		if err := rows.Scan(&user.ID, &user.Email, &user.NormalizedEmail, &user.Status, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, "", wrap(err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, "", wrap(err)
	}
	if len(users) <= limit {
		return users, "", nil
	}
	return users[:limit], users[limit-1].ID, nil
}

func (s *Store) UpdatePassword(ctx context.Context, userID string, credential auth.PasswordCredential) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE auth_password_credentials
		SET password_hash=$1,password_changed_at=$2,updated_at=$2,failed_attempts=0,locked_until=NULL
		WHERE user_id=$3`, credential.PasswordHash, credential.PasswordChangedAt, userID)
	return rowsAffected(result, err)
}

func (s *Store) SavePasswordReset(ctx context.Context, reset auth.PasswordReset) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_password_reset_tokens (id,user_id,token_hash,expires_at,consumed_at,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		reset.ID, reset.UserID, reset.TokenHash, reset.ExpiresAt, reset.ConsumedAt, reset.CreatedAt)
	return wrap(err)
}

func (s *Store) ConsumePasswordReset(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var userID string
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx, `
		UPDATE auth_password_reset_tokens SET consumed_at=$2
		WHERE token_hash=$1 AND consumed_at IS NULL
		RETURNING user_id,expires_at`, tokenHash, now).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", auth.ErrNotFound
	}
	if err != nil {
		return "", wrap(err)
	}
	if !now.Before(expiresAt) {
		return "", auth.ErrExpired
	}
	return userID, nil
}

func (s *Store) ResetPassword(ctx context.Context, tokenHash string, credential auth.PasswordCredential, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrap(err)
	}
	defer tx.Rollback()
	var userID string
	var expiresAt time.Time
	var consumedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT user_id,expires_at,consumed_at
		FROM auth_password_reset_tokens
		WHERE token_hash=$1
		FOR UPDATE`, tokenHash).Scan(&userID, &expiresAt, &consumedAt)
	if errors.Is(err, sql.ErrNoRows) || consumedAt.Valid {
		return auth.ErrNotFound
	}
	if err != nil {
		return wrap(err)
	}
	if !now.Before(expiresAt) {
		return auth.ErrExpired
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE auth_password_credentials
		SET password_hash=$1,password_changed_at=$2,updated_at=$2,failed_attempts=0,locked_until=NULL
		WHERE user_id=$3`, credential.PasswordHash, credential.PasswordChangedAt, userID)
	if err := rowsAffected(result, err); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_password_reset_tokens SET consumed_at=$1 WHERE token_hash=$2`, now, tokenHash); err != nil {
		return wrap(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=$1 WHERE user_id=$2 AND revoked_at IS NULL`, now, userID); err != nil {
		return wrap(err)
	}
	return wrap(tx.Commit())
}

func (s *Store) SaveSession(ctx context.Context, session auth.Session) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (id,user_id,token_hash,ip_address,user_agent,created_at,last_seen_at,expires_at)
		VALUES ($1,$2,$3,NULLIF($4,'')::inet,$5,$6,$7,$8)`,
		session.ID, session.UserID, session.TokenHash, session.IPAddress, session.UserAgent,
		session.CreatedAt, session.LastSeenAt, session.ExpiresAt)
	return wrap(err)
}

func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (auth.Session, auth.User, error) {
	var session auth.Session
	var user auth.User
	var ipAddress sql.NullString
	err := s.db.QueryRowContext(ctx, `
		UPDATE auth_sessions s SET last_seen_at=$2
		FROM auth_users u
		WHERE s.token_hash=$1 AND s.user_id=u.id AND s.revoked_at IS NULL AND s.expires_at>$2 AND u.deleted_at IS NULL
		RETURNING s.id,s.user_id,s.token_hash,s.ip_address::text,s.user_agent,s.created_at,s.last_seen_at,s.expires_at,
		          u.id,u.email,u.normalized_email,u.status,u.email_verified_at,u.created_at,u.updated_at`,
		tokenHash, now).Scan(
		&session.ID, &session.UserID, &session.TokenHash, &ipAddress, &session.UserAgent,
		&session.CreatedAt, &session.LastSeenAt, &session.ExpiresAt,
		&user.ID, &user.Email, &user.NormalizedEmail, &user.Status, &user.EmailVerifiedAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Session{}, auth.User{}, auth.ErrNotFound
	}
	session.IPAddress = ipAddress.String
	return session, user, wrap(err)
}

func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=NOW() WHERE token_hash=$1 AND revoked_at IS NULL`, tokenHash)
	return wrap(err)
}

func (s *Store) DeleteSessionsByUserID(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE auth_sessions SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return wrap(err)
}

func rowsAffected(result sql.Result, err error) error {
	if err != nil {
		return wrap(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return wrap(err)
	}
	if count == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres store: %w", err)
}

func isUniqueViolation(err error) bool {
	type sqlState interface{ SQLState() string }
	var state sqlState
	return errors.As(err, &state) && state.SQLState() == "23505"
}

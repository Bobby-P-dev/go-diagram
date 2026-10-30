package repository

import (
	"context"
	"database/sql"
	"time"
)

type PasswordAccount struct {
	UserID       string
	Email        string
	PasswordHash string
}

type BrowserRepository interface {
	AccountByEmail(context.Context, string) (*PasswordAccount, error)
	AccountByID(context.Context, string) (*PasswordAccount, error)
	ClaimLogin(context.Context, string, string) (bool, error)
	CreateSession(context.Context, string, string, string, time.Time) error
	SessionUser(context.Context, string) (string, error)
	DeleteSession(context.Context, string) error
}

type browserRepository struct{ db *sql.DB }

func NewBrowserRepository(db *sql.DB) BrowserRepository { return &browserRepository{db: db} }

func (repo *browserRepository) AccountByEmail(ctx context.Context, email string) (*PasswordAccount, error) {
	var account PasswordAccount
	err := repo.db.QueryRowContext(ctx, `SELECT user_id, email, password_hash FROM password_accounts WHERE email = $1`, email).Scan(&account.UserID, &account.Email, &account.PasswordHash)
	return &account, err
}

func (repo *browserRepository) AccountByID(ctx context.Context, userID string) (*PasswordAccount, error) {
	var account PasswordAccount
	err := repo.db.QueryRowContext(ctx, `SELECT user_id, email, password_hash FROM password_accounts WHERE user_id = $1`, userID).Scan(&account.UserID, &account.Email, &account.PasswordHash)
	return &account, err
}

func (repo *browserRepository) ClaimLogin(ctx context.Context, emailBucket, ipBucket string) (bool, error) {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM password_login_attempts WHERE window_started < NOW() - INTERVAL '1 day'`); err != nil {
		return false, err
	}
	allowed := true
	for index, bucket := range []string{emailBucket, ipBucket} {
		var attempts int
		err = tx.QueryRowContext(ctx, `INSERT INTO password_login_attempts(bucket, attempts) VALUES ($1, 1)
		ON CONFLICT(bucket) DO UPDATE SET
		attempts = CASE WHEN password_login_attempts.window_started < NOW() - INTERVAL '15 minutes' THEN 1 ELSE LEAST(password_login_attempts.attempts + 1, 1000) END,
		window_started = CASE WHEN password_login_attempts.window_started < NOW() - INTERVAL '15 minutes' THEN NOW() ELSE password_login_attempts.window_started END
		RETURNING attempts`, bucket).Scan(&attempts)
		if err != nil {
			return false, err
		}
		limit := 10
		if index == 1 {
			limit = 100
		}
		if attempts > limit {
			allowed = false
		}
	}
	return allowed, tx.Commit()
}

func (repo *browserRepository) CreateSession(ctx context.Context, hash, userID, previousHash string, expires time.Time) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM browser_sessions WHERE expires_at <= NOW() OR token_hash = $1`, previousHash); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO browser_sessions(token_hash, user_id, expires_at) VALUES ($1,$2,$3)`, hash, userID, expires); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE access_credentials SET last_login_at = NOW() WHERE id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *browserRepository) SessionUser(ctx context.Context, hash string) (string, error) {
	var userID string
	err := repo.db.QueryRowContext(ctx, `SELECT user_id FROM browser_sessions WHERE token_hash = $1 AND expires_at > NOW()`, hash).Scan(&userID)
	return userID, err
}

func (repo *browserRepository) DeleteSession(ctx context.Context, hash string) error {
	_, err := repo.db.ExecContext(ctx, `DELETE FROM browser_sessions WHERE token_hash = $1`, hash)
	return err
}

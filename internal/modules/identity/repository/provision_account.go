package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type ProvisionAccount struct {
	Email           string
	Name            string
	PasswordHash    string
	DisabledKeyHash string
	ExistingUserID  string
}

func ProvisionPasswordAccount(ctx context.Context, db *sql.DB, input ProvisionAccount) (string, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	userID := input.ExistingUserID
	if userID == "" {
		err = tx.QueryRowContext(ctx, `INSERT INTO access_credentials(credential_key,credential_key_prefix,name,role) VALUES($1,'disabled',$2,'user') RETURNING id`, input.DisabledKeyHash, input.Name).Scan(&userID)
	} else {
		var active bool
		err = tx.QueryRowContext(ctx, `SELECT is_active FROM access_credentials WHERE id=$1 FOR UPDATE`, userID).Scan(&active)
		if err == nil && !active {
			return "", fmt.Errorf("cannot provision an inactive identity")
		}
	}
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO password_accounts(user_id,email,password_hash) VALUES($1,$2,$3)`, userID, input.Email, input.PasswordHash); err != nil {
		return "", fmt.Errorf("create password account (existing accounts are never overwritten): %w", err)
	}
	return userID, tx.Commit()
}

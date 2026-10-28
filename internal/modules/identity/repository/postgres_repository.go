package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
)

type postgresRepository struct {
	db *sql.DB
}

type rowScanner interface {
	Scan(dest ...any) error
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) MigrateCredentialKeys(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin credential key migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE access_credentials IN EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock credential key migration: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, credential_key FROM access_credentials WHERE credential_key_prefix = ''`)
	if err != nil {
		return fmt.Errorf("load legacy credentials: %w", err)
	}
	type legacyCredential struct {
		id  string
		key string
	}
	var legacy []legacyCredential
	for rows.Next() {
		var credential legacyCredential
		if err := rows.Scan(&credential.id, &credential.key); err != nil {
			rows.Close()
			return fmt.Errorf("scan legacy credential: %w", err)
		}
		legacy = append(legacy, credential)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read legacy credentials: %w", err)
	}
	rows.Close()

	for _, credential := range legacy {
		raw := strings.TrimSpace(credential.key)
		if raw == "" {
			return fmt.Errorf("credential %s has an empty key", credential.id)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE access_credentials
			SET credential_key = $1, credential_key_prefix = $2
			WHERE id = $3 AND credential_key_prefix = ''
		`, identity.HashCredentialKey(raw), identity.HiddenCredentialPrefix, credential.id)
		if err != nil {
			return fmt.Errorf("hash credential %s: %w", credential.id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check migrated credential: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf("credential %s changed during migration", credential.id)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit credential key migration: %w", err)
	}
	return nil
}

func (r *postgresRepository) EnsureMasterAdmin(ctx context.Context, bootstrapKey string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin master admin bootstrap: %w", err)
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM access_credentials WHERE id = $1)`, identity.MasterAdminID).Scan(&exists); err != nil {
		return fmt.Errorf("check master admin: %w", err)
	}
	if !exists {
		bootstrapKey = strings.TrimSpace(bootstrapKey)
		if len(bootstrapKey) < 32 || len(bootstrapKey) > 64 {
			return fmt.Errorf("MASTER_ADMIN_KEY must contain 32 to 64 bytes for first-time bootstrap")
		}
		query := `
			INSERT INTO access_credentials (id, credential_key, credential_key_prefix, name, role, is_active, expires_at, can_generate_diagram, can_generate_ui)
			VALUES ($1, $2, $3, 'Administrator Utama', 'admin', true, NULL, true, true)
			ON CONFLICT (id) DO NOTHING
		`
		if _, err := tx.ExecContext(ctx, query, identity.MasterAdminID, identity.HashCredentialKey(bootstrapKey), identity.CredentialKeyPrefix(bootstrapKey)); err != nil {
			return fmt.Errorf("create master admin: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE projects SET user_id = $1 WHERE user_id IS NULL`, identity.MasterAdminID); err != nil {
		return fmt.Errorf("associate legacy projects: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit master admin bootstrap: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByKeyHash(ctx context.Context, keyHash string) (*Credential, error) {
	query := `
		SELECT id, credential_key_prefix, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
		FROM access_credentials
		WHERE credential_key = $1 AND credential_key_prefix <> ''
	`
	credential, err := scanCredential(r.db.QueryRowContext(ctx, query, keyHash))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("kredensial tidak ditemukan")
		}
		return nil, fmt.Errorf("gagal memeriksa kredensial: %w", err)
	}
	return credential, nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id string) (*Credential, error) {
	query := `
		SELECT id, credential_key_prefix, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
		FROM access_credentials
		WHERE id = $1
	`
	credential, err := scanCredential(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("kredensial tidak ditemukan: %s", id)
		}
		return nil, fmt.Errorf("gagal mengambil kredensial: %w", err)
	}
	return credential, nil
}

func (r *postgresRepository) Create(ctx context.Context, credential NewCredential) (*Credential, error) {
	query := `
		INSERT INTO access_credentials (credential_key, credential_key_prefix, name, role, is_active, expires_at, can_generate_diagram, can_generate_ui, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $6, $7, NOW(), NOW())
		RETURNING id, credential_key_prefix, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
	`
	created, err := scanCredential(r.db.QueryRowContext(ctx, query,
		credential.KeyHash,
		credential.KeyPrefix,
		credential.Name,
		credential.Role,
		credential.ExpiresAt,
		credential.CanGenerateDiagram,
		credential.CanGenerateUI,
	))
	if err != nil {
		return nil, fmt.Errorf("gagal membuat kredensial: %w", err)
	}
	return created, nil
}

func (r *postgresRepository) Update(ctx context.Context, credential UpdateCredential) (*Credential, error) {
	query := `
		UPDATE access_credentials
		SET name = $1, role = $2, is_active = $3, expires_at = $4, can_generate_diagram = $5, can_generate_ui = $6, updated_at = NOW()
		WHERE id = $7
		RETURNING id, credential_key_prefix, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
	`
	updated, err := scanCredential(r.db.QueryRowContext(ctx, query,
		credential.Name,
		credential.Role,
		credential.IsActive,
		credential.ExpiresAt,
		credential.CanGenerateDiagram,
		credential.CanGenerateUI,
		credential.ID,
	))
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui kredensial: %w", err)
	}
	return updated, nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM access_credentials WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus kredensial: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("kredensial tidak ditemukan: %s", id)
	}
	return nil
}

func (r *postgresRepository) UpdateLastLogin(ctx context.Context, id string) error {
	query := `UPDATE access_credentials SET last_login_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *postgresRepository) GetAllWithStats(ctx context.Context) ([]CredentialStats, error) {
	query := `
		SELECT
			c.id,
			c.credential_key_prefix,
			c.name,
			c.role,
			c.is_active,
			c.can_generate_diagram,
			c.can_generate_ui,
			c.expires_at,
			c.last_login_at,
			c.created_at,
			c.updated_at,
			COALESCE(p.proj_count, 0) as project_count
		FROM access_credentials c
		LEFT JOIN (
			SELECT user_id, COUNT(*) as proj_count
			FROM projects
			WHERE user_id IS NOT NULL
			GROUP BY user_id
		) p ON p.user_id = c.id
		ORDER BY
			CASE WHEN c.role = 'admin' THEN 0 ELSE 1 END,
			c.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar kredensial: %w", err)
	}
	defer rows.Close()

	var result []CredentialStats
	for rows.Next() {
		var credential Credential
		var expiresAt, lastLoginAt sql.NullTime
		var projectCount int

		if err := rows.Scan(
			&credential.ID,
			&credential.KeyPrefix,
			&credential.Name,
			&credential.Role,
			&credential.IsActive,
			&credential.CanGenerateDiagram,
			&credential.CanGenerateUI,
			&expiresAt,
			&lastLoginAt,
			&credential.CreatedAt,
			&credential.UpdatedAt,
			&projectCount,
		); err != nil {
			return nil, fmt.Errorf("gagal membaca data kredensial: %w", err)
		}
		if expiresAt.Valid {
			credential.ExpiresAt = &expiresAt.Time
		}
		if lastLoginAt.Valid {
			credential.LastLoginAt = &lastLoginAt.Time
		}

		result = append(result, CredentialStats{
			Credential:   credential,
			ProjectCount: projectCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("gagal membaca daftar kredensial: %w", err)
	}
	return result, nil
}

func scanCredential(scanner rowScanner) (*Credential, error) {
	var credential Credential
	var expiresAt, lastLoginAt sql.NullTime
	if err := scanner.Scan(
		&credential.ID,
		&credential.KeyPrefix,
		&credential.Name,
		&credential.Role,
		&credential.IsActive,
		&credential.CanGenerateDiagram,
		&credential.CanGenerateUI,
		&expiresAt,
		&lastLoginAt,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		credential.ExpiresAt = &expiresAt.Time
	}
	if lastLoginAt.Valid {
		credential.LastLoginAt = &lastLoginAt.Time
	}
	return &credential, nil
}

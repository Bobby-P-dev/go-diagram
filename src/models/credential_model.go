package models

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

const MasterAdminID = "a0000000-0000-0000-0000-000000000001"
const DefaultMasterAdminKey = "@bbaystr772"

type CredentialModelInterface interface {
	FindByKey(key string) (*entities.AccessCredential, error)
	FindByID(id string) (*entities.AccessCredential, error)
	Create(name, role string, expiresAt *time.Time, canDiagram, canUI bool) (*entities.AccessCredential, error)
	Update(id string, name string, isActive *bool, expiresAt *time.Time) (*entities.AccessCredential, error)
	UpdateFull(id string, name string, role string, isActive *bool, expiresAt *time.Time, updateExpiresAt bool, canDiagram *bool, canUI *bool) (*entities.AccessCredential, error)
	Delete(id string) error
	GetAllWithStats() ([]dtos.CredentialSummaryDTO, error)
	UpdateLastLogin(id string) error
	EnsureMasterAdmin(defaultKey string) error
}

type credentialModel struct {
	db *sql.DB
}

func NewCredentialModel(db *sql.DB) CredentialModelInterface {
	return &credentialModel{db: db}
}

// GenerateRandomKey creates a clean, readable token format like RL-9K4M-X2PQ-7W8T
func GenerateRandomKey() string {
	const charset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("RL-%d", time.Now().UnixNano())
	}
	var sb strings.Builder
	sb.WriteString("RL-")
	for i, b := range bytes {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(charset[int(b)%len(charset)])
	}
	return sb.String()
}

func FormatExpiresInHuman(expiresAt *time.Time) (string, bool) {
	if expiresAt == nil {
		return "Permanen (Tanpa Batas)", false
	}
	now := time.Now()
	if now.After(*expiresAt) {
		return "Kedaluwarsa", true
	}
	diff := expiresAt.Sub(now)
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins <= 1 {
			return "Sisa < 1 menit", false
		}
		return fmt.Sprintf("Sisa %d menit", mins), false
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("Sisa %d jam", int(diff.Hours())), false
	}
	days := int(diff.Hours() / 24)
	return fmt.Sprintf("Sisa %d hari", days), false
}

func (m *credentialModel) EnsureMasterAdmin(defaultKey string) error {
	if strings.TrimSpace(defaultKey) == "" {
		defaultKey = DefaultMasterAdminKey
	}
	query := `
		INSERT INTO access_credentials (id, credential_key, name, role, is_active, expires_at, can_generate_diagram, can_generate_ui)
		VALUES ($1, $2, 'Administrator Utama', 'admin', true, NULL, true, true)
		ON CONFLICT (id) DO UPDATE SET credential_key = $2, role = 'admin', is_active = true, can_generate_diagram = true, can_generate_ui = true
	`
	_, err := m.db.Exec(query, MasterAdminID, defaultKey)
	return err
}

func (m *credentialModel) FindByKey(key string) (*entities.AccessCredential, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("credential key is required")
	}

	query := `
		SELECT id, credential_key, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
		FROM access_credentials
		WHERE credential_key = $1
	`

	var c entities.AccessCredential
	var expiresAt, lastLoginAt sql.NullTime

	err := m.db.QueryRow(query, key).Scan(
		&c.ID,
		&c.CredentialKey,
		&c.Name,
		&c.Role,
		&c.IsActive,
		&c.CanGenerateDiagram,
		&c.CanGenerateUI,
		&expiresAt,
		&lastLoginAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("kredensial tidak ditemukan: %s", key)
		}
		return nil, fmt.Errorf("gagal memeriksa kredensial: %w", err)
	}

	if expiresAt.Valid {
		c.ExpiresAt = &expiresAt.Time
	}
	if lastLoginAt.Valid {
		c.LastLoginAt = &lastLoginAt.Time
	}

	return &c, nil
}

func (m *credentialModel) FindByID(id string) (*entities.AccessCredential, error) {
	query := `
		SELECT id, credential_key, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
		FROM access_credentials
		WHERE id = $1
	`

	var c entities.AccessCredential
	var expiresAt, lastLoginAt sql.NullTime

	err := m.db.QueryRow(query, id).Scan(
		&c.ID,
		&c.CredentialKey,
		&c.Name,
		&c.Role,
		&c.IsActive,
		&c.CanGenerateDiagram,
		&c.CanGenerateUI,
		&expiresAt,
		&lastLoginAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("kredensial tidak ditemukan: %s", id)
		}
		return nil, fmt.Errorf("gagal mengambil kredensial: %w", err)
	}

	if expiresAt.Valid {
		c.ExpiresAt = &expiresAt.Time
	}
	if lastLoginAt.Valid {
		c.LastLoginAt = &lastLoginAt.Time
	}

	return &c, nil
}

func (m *credentialModel) Create(name, role string, expiresAt *time.Time, canDiagram, canUI bool) (*entities.AccessCredential, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Pengguna Baru"
	}
	if role != "admin" {
		role = "user"
	}

	key := GenerateRandomKey()

	query := `
		INSERT INTO access_credentials (credential_key, name, role, is_active, expires_at, can_generate_diagram, can_generate_ui, created_at, updated_at)
		VALUES ($1, $2, $3, true, $4, $5, $6, NOW(), NOW())
		RETURNING id, credential_key, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
	`

	var c entities.AccessCredential
	var dbExpiresAt, lastLoginAt sql.NullTime

	err := m.db.QueryRow(query, key, name, role, expiresAt, canDiagram, canUI).Scan(
		&c.ID,
		&c.CredentialKey,
		&c.Name,
		&c.Role,
		&c.IsActive,
		&c.CanGenerateDiagram,
		&c.CanGenerateUI,
		&dbExpiresAt,
		&lastLoginAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat kredensial: %w", err)
	}

	if dbExpiresAt.Valid {
		c.ExpiresAt = &dbExpiresAt.Time
	}

	return &c, nil
}

func (m *credentialModel) Update(id string, name string, isActive *bool, expiresAt *time.Time) (*entities.AccessCredential, error) {
	return m.UpdateFull(id, name, "", isActive, expiresAt, expiresAt != nil, nil, nil)
}

func (m *credentialModel) UpdateFull(id string, name string, role string, isActive *bool, expiresAt *time.Time, updateExpiresAt bool, canDiagram *bool, canUI *bool) (*entities.AccessCredential, error) {
	// 1. Fetch current
	current, err := m.FindByID(id)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(name) != "" {
		current.Name = strings.TrimSpace(name)
	}
	if role == "admin" || role == "user" {
		if id != MasterAdminID || role == "admin" {
			current.Role = role
		}
	}
	if isActive != nil {
		if id != MasterAdminID || *isActive {
			current.IsActive = *isActive
		}
	}
	if updateExpiresAt {
		if id != MasterAdminID {
			current.ExpiresAt = expiresAt
		}
	}
	if id != MasterAdminID {
		if canDiagram != nil {
			current.CanGenerateDiagram = *canDiagram
		}
		if canUI != nil {
			current.CanGenerateUI = *canUI
		}
	} else {
		// Master Admin always has full access
		current.CanGenerateDiagram = true
		current.CanGenerateUI = true
	}

	query := `
		UPDATE access_credentials
		SET name = $1, role = $2, is_active = $3, expires_at = $4, can_generate_diagram = $5, can_generate_ui = $6, updated_at = NOW()
		WHERE id = $7
		RETURNING id, credential_key, name, role, is_active, can_generate_diagram, can_generate_ui, expires_at, last_login_at, created_at, updated_at
	`

	var c entities.AccessCredential
	var dbExpiresAt, lastLoginAt sql.NullTime

	err = m.db.QueryRow(query, current.Name, current.Role, current.IsActive, current.ExpiresAt, current.CanGenerateDiagram, current.CanGenerateUI, id).Scan(
		&c.ID,
		&c.CredentialKey,
		&c.Name,
		&c.Role,
		&c.IsActive,
		&c.CanGenerateDiagram,
		&c.CanGenerateUI,
		&dbExpiresAt,
		&lastLoginAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal memperbarui kredensial: %w", err)
	}

	if dbExpiresAt.Valid {
		c.ExpiresAt = &dbExpiresAt.Time
	}
	if lastLoginAt.Valid {
		c.LastLoginAt = &lastLoginAt.Time
	}

	return &c, nil
}

func (m *credentialModel) Delete(id string) error {
	if id == MasterAdminID {
		return fmt.Errorf("kredensial Master Admin tidak dapat dihapus")
	}

	query := `DELETE FROM access_credentials WHERE id = $1`
	res, err := m.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("gagal menghapus kredensial: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("kredensial tidak ditemukan: %s", id)
	}
	return nil
}

func (m *credentialModel) UpdateLastLogin(id string) error {
	query := `UPDATE access_credentials SET last_login_at = NOW() WHERE id = $1`
	_, err := m.db.Exec(query, id)
	return err
}

func (m *credentialModel) GetAllWithStats() ([]dtos.CredentialSummaryDTO, error) {
	query := `
		SELECT 
			c.id, 
			c.credential_key, 
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

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar kredensial: %w", err)
	}
	defer rows.Close()

	var result []dtos.CredentialSummaryDTO
	for rows.Next() {
		var id, key, name, role string
		var isActive, canDiagram, canUI bool
		var expiresAt, lastLoginAt sql.NullTime
		var createdAt, updatedAt time.Time
		var projectCount int

		err := rows.Scan(
			&id,
			&key,
			&name,
			&role,
			&isActive,
			&canDiagram,
			&canUI,
			&expiresAt,
			&lastLoginAt,
			&createdAt,
			&updatedAt,
			&projectCount,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca data kredensial: %w", err)
		}

		var expiresAtStr *string
		var lastLoginStr *string
		humanText, isExpired := "Permanen (Tanpa Batas)", false

		if expiresAt.Valid {
			s := expiresAt.Time.Format(time.RFC3339)
			expiresAtStr = &s
			humanText, isExpired = FormatExpiresInHuman(&expiresAt.Time)
		}
		if lastLoginAt.Valid {
			s := lastLoginAt.Time.Format(time.RFC3339)
			lastLoginStr = &s
		}

		result = append(result, dtos.CredentialSummaryDTO{
			ID:                 id,
			CredentialKey:      key,
			Name:               name,
			Role:               role,
			IsActive:           isActive,
			CanGenerateDiagram: canDiagram,
			CanGenerateUI:      canUI,
			ExpiresAt:          expiresAtStr,
			IsExpired:          isExpired,
			ExpiresInHuman:     humanText,
			ProjectCount:       projectCount,
			LastLoginAt:        lastLoginStr,
			CreatedAt:          createdAt.Format(time.RFC3339),
			UpdatedAt:          updatedAt.Format(time.RFC3339),
		})
	}

	return result, nil
}

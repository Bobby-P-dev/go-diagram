package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type ShareModelInterface interface {
	CreateOrGetShare(projectID, title string) (*entities.ProjectShare, error)
	GetShareByProjectID(projectID string) (*entities.ProjectShare, error)
	GetShareByToken(token string) (*entities.ProjectShare, *entities.Project, []entities.ChatMessage, error)
	RevokeShare(projectID string) error
	ForkSharedProject(token, newTitle string) (*entities.Project, error)
	ForkSharedProjectWithUser(token, newTitle, userID string) (*entities.Project, error)
}

type shareModel struct {
	db *sql.DB
}

func NewShareModel(db *sql.DB) ShareModelInterface {
	return &shareModel{db: db}
}

func generateShareToken() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("share_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (m *shareModel) CreateOrGetShare(projectID, title string) (*entities.ProjectShare, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Shared Project"
	}

	// 1. Check if a share record already exists
	var s entities.ProjectShare
	checkQuery := `
		SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at
		FROM project_shares
		WHERE project_id = $1
	`
	err := m.db.QueryRow(checkQuery, projectID).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)

	if err == nil {
		// Existing share found, ensure it's active and update title
		updateQuery := `
			UPDATE project_shares
			SET is_active = true, title = $1, updated_at = NOW()
			WHERE id = $2
			RETURNING id, share_token, project_id, title, is_active, view_count, created_at, updated_at
		`
		err = m.db.QueryRow(updateQuery, title, s.ID).Scan(
			&s.ID,
			&s.ShareToken,
			&s.ProjectID,
			&s.Title,
			&s.IsActive,
			&s.ViewCount,
			&s.CreatedAt,
			&s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to reactivate existing share: %w", err)
		}
		return &s, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing share: %w", err)
	}

	// 2. Create new share with unique token
	token := generateShareToken()
	insertQuery := `
		INSERT INTO project_shares (share_token, project_id, title, is_active, view_count, created_at, updated_at)
		VALUES ($1, $2, $3, true, 0, NOW(), NOW())
		RETURNING id, share_token, project_id, title, is_active, view_count, created_at, updated_at
	`
	err = m.db.QueryRow(insertQuery, token, projectID, title).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project share: %w", err)
	}

	return &s, nil
}

func (m *shareModel) GetShareByProjectID(projectID string) (*entities.ProjectShare, error) {
	query := `
		SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at
		FROM project_shares
		WHERE project_id = $1
	`
	var s entities.ProjectShare
	err := m.db.QueryRow(query, projectID).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query project share: %w", err)
	}
	return &s, nil
}

func (m *shareModel) GetShareByToken(token string) (*entities.ProjectShare, *entities.Project, []entities.ChatMessage, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, nil, fmt.Errorf("share token is required")
	}

	// 1. Query share record
	queryShare := `
		SELECT id, share_token, project_id, title, is_active, view_count, created_at, updated_at
		FROM project_shares
		WHERE share_token = $1 AND is_active = true
	`
	var s entities.ProjectShare
	err := m.db.QueryRow(queryShare, token).Scan(
		&s.ID,
		&s.ShareToken,
		&s.ProjectID,
		&s.Title,
		&s.IsActive,
		&s.ViewCount,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil, fmt.Errorf("shared project not found or sharing has been disabled")
		}
		return nil, nil, nil, fmt.Errorf("failed to query share: %w", err)
	}

	// Increment view count asynchronously
	go func(shareID string) {
		_, _ = m.db.Exec("UPDATE project_shares SET view_count = view_count + 1 WHERE id = $1", shareID)
	}(s.ID)

	// 2. Query Project
	queryProject := `
		SELECT id, title, diagram_type, COALESCE(project_mode, 'diagram'), COALESCE(metadata, '{}'::jsonb), is_pinned, current_nodes, current_edges, created_at, updated_at
		FROM projects
		WHERE id = $1
	`
	var p entities.Project
	err = m.db.QueryRow(queryProject, s.ProjectID).Scan(
		&p.ID,
		&p.Title,
		&p.DiagramType,
		&p.ProjectMode,
		&p.Metadata,
		&p.IsPinned,
		&p.CurrentNodes,
		&p.CurrentEdges,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("associated project not found: %w", err)
	}

	// 3. Query Chat Messages
	queryMessages := `
		SELECT id, project_id, role, content, target_node_ids, created_at
		FROM chat_messages
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := m.db.Query(queryMessages, s.ProjectID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to query chat messages: %w", err)
	}
	defer rows.Close()

	var messages []entities.ChatMessage
	for rows.Next() {
		var msg entities.ChatMessage
		var rawTarget []byte
		err := rows.Scan(
			&msg.ID,
			&msg.ProjectID,
			&msg.Role,
			&msg.Content,
			&rawTarget,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to scan chat message: %w", err)
		}
		if len(rawTarget) > 0 {
			msg.TargetNodeIDs = json.RawMessage(rawTarget)
		}
		messages = append(messages, msg)
	}

	return &s, &p, messages, nil
}

func (m *shareModel) RevokeShare(projectID string) error {
	query := `
		UPDATE project_shares
		SET is_active = false, updated_at = NOW()
		WHERE project_id = $1
	`
	res, err := m.db.Exec(query, projectID)
	if err != nil {
		return fmt.Errorf("failed to revoke share: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no share record found for project %s", projectID)
	}
	return nil
}

func (m *shareModel) ForkSharedProject(token, newTitle string) (*entities.Project, error) {
	return m.ForkSharedProjectWithUser(token, newTitle, "")
}

func (m *shareModel) ForkSharedProjectWithUser(token, newTitle, userID string) (*entities.Project, error) {
	_, originalProj, messages, err := m.GetShareByToken(token)
	if err != nil {
		return nil, err
	}

	forkTitle := strings.TrimSpace(newTitle)
	if forkTitle == "" {
		forkTitle = fmt.Sprintf("%s (Shared Copy)", originalProj.Title)
	}

	if userID == "" {
		userID = MasterAdminID
	}

	// Begin Transaction for complete duplication
	tx, err := m.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert new cloned project
	insertProjectQuery := `
		INSERT INTO projects (user_id, title, diagram_type, project_mode, metadata, current_nodes, current_edges, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, false, NOW(), NOW())
		RETURNING id, COALESCE(user_id, 'a0000000-0000-0000-0000-000000000001'::uuid), title, diagram_type, project_mode, metadata, is_pinned, current_nodes, current_edges, created_at, updated_at
	`
	var newProj entities.Project
	err = tx.QueryRow(
		insertProjectQuery,
		userID,
		forkTitle,
		originalProj.DiagramType,
		originalProj.ProjectMode,
		originalProj.Metadata,
		originalProj.CurrentNodes,
		originalProj.CurrentEdges,
	).Scan(
		&newProj.ID,
		&newProj.UserID,
		&newProj.Title,
		&newProj.DiagramType,
		&newProj.ProjectMode,
		&newProj.Metadata,
		&newProj.IsPinned,
		&newProj.CurrentNodes,
		&newProj.CurrentEdges,
		&newProj.CreatedAt,
		&newProj.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fork project: %w", err)
	}

	// 2. Clone chat messages
	if len(messages) > 0 {
		insertMsgQuery := `
			INSERT INTO chat_messages (project_id, role, content, target_node_ids, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`
		for _, msg := range messages {
			_, err = tx.Exec(insertMsgQuery, newProj.ID, msg.Role, msg.Content, msg.TargetNodeIDs, msg.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("failed to clone chat message: %w", err)
			}
		}
	}

	// 3. Insert initial version 1 snapshot
	insertVerQuery := `
		INSERT INTO diagram_versions (project_id, version_number, change_summary, nodes, edges, created_at)
		VALUES ($1, 1, $2, $3, $4, NOW())
	`
	summary := fmt.Sprintf("Cloned from shared project: %s", originalProj.Title)
	_, _ = tx.Exec(insertVerQuery, newProj.ID, summary, newProj.CurrentNodes, newProj.CurrentEdges)

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit fork transaction: %w", err)
	}

	return &newProj, nil
}

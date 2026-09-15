package models

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
)

type SettingsModelInterface interface {
	Get() (*entities.WorkspaceSetting, error)
	Update(defaultMode, theme, preferredModel string, canvasPreferences json.RawMessage) (*entities.WorkspaceSetting, error)
}

type settingsModel struct {
	db *sql.DB
}

func NewSettingsModel(db *sql.DB) SettingsModelInterface {
	return &settingsModel{db: db}
}

func (m *settingsModel) Get() (*entities.WorkspaceSetting, error) {
	query := `
		SELECT id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at
		FROM workspace_settings
		WHERE id = 'default'
		LIMIT 1
	`
	var s entities.WorkspaceSetting
	var rawCanvas []byte
	err := m.db.QueryRow(query).Scan(
		&s.ID,
		&s.DefaultMode,
		&s.Theme,
		&s.PreferredAIModel,
		&rawCanvas,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// fallback default
			return &entities.WorkspaceSetting{
				ID:                "default",
				DefaultMode:       "ui_design",
				Theme:             "dark",
				PreferredAIModel:  "ag/gemini-3.8-flash-high",
				CanvasPreferences: json.RawMessage(`{"snap_to_grid": true, "grid_size": 20}`),
			}, nil
		}
		return nil, fmt.Errorf("failed to query workspace settings: %w", err)
	}
	s.CanvasPreferences = rawCanvas
	return &s, nil
}

func (m *settingsModel) Update(defaultMode, theme, preferredModel string, canvasPreferences json.RawMessage) (*entities.WorkspaceSetting, error) {
	if len(canvasPreferences) == 0 {
		canvasPreferences = json.RawMessage(`{"snap_to_grid": true, "grid_size": 20}`)
	}

	query := `
		INSERT INTO workspace_settings (id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at)
		VALUES ('default', $1, $2, $3, $4, NOW())
		ON CONFLICT (id) DO UPDATE
		SET default_mode = EXCLUDED.default_mode,
		    theme = EXCLUDED.theme,
		    preferred_ai_model = EXCLUDED.preferred_ai_model,
		    canvas_preferences = EXCLUDED.canvas_preferences,
		    updated_at = NOW()
		RETURNING id, default_mode, theme, preferred_ai_model, canvas_preferences, updated_at
	`

	var s entities.WorkspaceSetting
	var rawCanvas []byte
	err := m.db.QueryRow(query, defaultMode, theme, preferredModel, canvasPreferences).Scan(
		&s.ID,
		&s.DefaultMode,
		&s.Theme,
		&s.PreferredAIModel,
		&rawCanvas,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update workspace settings: %w", err)
	}
	s.CanvasPreferences = rawCanvas
	return &s, nil
}

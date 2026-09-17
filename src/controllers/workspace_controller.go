package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/src/models"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type WorkspaceController struct {
	commentModel    models.CommentModelInterface
	foundationModel models.FoundationModelInterface
	settingsModel   models.SettingsModelInterface
	exportModel     models.ExportModelInterface
}

func NewWorkspaceController(
	commentModel models.CommentModelInterface,
	foundationModel models.FoundationModelInterface,
	settingsModel models.SettingsModelInterface,
	exportModel models.ExportModelInterface,
) *WorkspaceController {
	return &WorkspaceController{
		commentModel:    commentModel,
		foundationModel: foundationModel,
		settingsModel:   settingsModel,
		exportModel:     exportModel,
	}
}

// ----------------- DESIGN COMMENTS -----------------

func (c *WorkspaceController) GetComments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	comments, err := c.commentModel.GetByProjectID(projectID)
	if err != nil {
		log.Printf("[WorkspaceController.GetComments] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to get comments", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Comments retrieved successfully", comments)
}

type CreateCommentRequest struct {
	NodeID   string          `json:"node_id"`
	Author   string          `json:"author"`
	Content  string          `json:"content"`
	PosX     float64         `json:"position_x"`
	PosY     float64         `json:"position_y"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

func (c *WorkspaceController) CreateComment(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if req.Content == "" {
		utils.RespondError(w, http.StatusBadRequest, "Content is required", "content cannot be empty")
		return
	}

	if req.Author == "" {
		req.Author = "Designer"
	}

	comment, err := c.commentModel.Create(projectID, req.NodeID, req.Author, req.Content, req.PosX, req.PosY, req.Metadata)
	if err != nil {
		log.Printf("[WorkspaceController.CreateComment] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to create comment", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, "Comment created successfully", comment)
}

type UpdateCommentStatusRequest struct {
	Status string `json:"status"`
}

func (c *WorkspaceController) UpdateCommentStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing comment ID", "path parameter 'id' is required")
		return
	}

	var req UpdateCommentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if req.Status == "" {
		req.Status = "resolved"
	}

	if err := c.commentModel.UpdateStatus(id, req.Status); err != nil {
		log.Printf("[WorkspaceController.UpdateCommentStatus] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to update comment", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Comment status updated", map[string]string{"id": id, "status": req.Status})
}

func (c *WorkspaceController) DeleteComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing comment ID", "path parameter 'id' is required")
		return
	}

	if err := c.commentModel.Delete(id); err != nil {
		log.Printf("[WorkspaceController.DeleteComment] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to delete comment", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Comment deleted", map[string]string{"id": id})
}

// ----------------- DESIGN FOUNDATIONS -----------------

func (c *WorkspaceController) GetFoundations(w http.ResponseWriter, r *http.Request) {
	foundations, err := c.foundationModel.GetAll()
	if err != nil {
		log.Printf("[WorkspaceController.GetFoundations] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to get foundations", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Foundations retrieved successfully", foundations)
}

// ----------------- WORKSPACE SETTINGS -----------------

func (c *WorkspaceController) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := c.settingsModel.Get()
	if err != nil {
		log.Printf("[WorkspaceController.GetSettings] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to get settings", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Settings retrieved successfully", settings)
}

type UpdateSettingsRequest struct {
	DefaultMode       string          `json:"default_mode"`
	Theme             string          `json:"theme"`
	PreferredAIModel  string          `json:"preferred_ai_model"`
	CanvasPreferences json.RawMessage `json:"canvas_preferences"`
}

func (c *WorkspaceController) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	updated, err := c.settingsModel.Update(req.DefaultMode, req.Theme, req.PreferredAIModel, req.CanvasPreferences)
	if err != nil {
		log.Printf("[WorkspaceController.UpdateSettings] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to update settings", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Settings updated successfully", updated)
}

// ----------------- PROJECT EXPORTS -----------------

func (c *WorkspaceController) GetExports(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	exports, err := c.exportModel.GetByProjectID(projectID)
	if err != nil {
		log.Printf("[WorkspaceController.GetExports] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to get exports", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Exports retrieved successfully", exports)
}

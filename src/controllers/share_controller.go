package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
	"github.com/Bobby-P-dev/go-diagram.git/src/services"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type ShareController struct {
	shareModel     models.ShareModelInterface
	projectModel   models.ProjectModelInterface
	projectService *services.ProjectService
}

func NewShareController(
	shareModel models.ShareModelInterface,
	projectModel models.ProjectModelInterface,
	projectService *services.ProjectService,
) *ShareController {
	return &ShareController{
		shareModel:     shareModel,
		projectModel:   projectModel,
		projectService: projectService,
	}
}

func (c *ShareController) CreateShare(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	project, _, err := c.projectModel.GetByIDScoped(projectID, userID, "")
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Project not found", err.Error())
		return
	}

	share, err := c.shareModel.CreateOrGetShare(projectID, project.Title)
	if err != nil {
		log.Printf("[ShareController.CreateShare] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to create share link", err.Error())
		return
	}

	shareURL := fmt.Sprintf("/share/%s", share.ShareToken)

	resp := dtos.ProjectShareDTO{
		ID:         share.ID,
		ShareToken: share.ShareToken,
		ProjectID:  share.ProjectID,
		Title:      share.Title,
		IsActive:   share.IsActive,
		ViewCount:  share.ViewCount,
		ShareURL:   shareURL,
		CreatedAt:  share.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  share.UpdatedAt.Format(time.RFC3339),
	}

	utils.RespondJSON(w, http.StatusOK, "Share link created successfully", resp)
}

func (c *ShareController) GetShareStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	if userID != "" {
		if _, _, err := c.projectModel.GetByIDScoped(projectID, userID, ""); err != nil {
			utils.RespondError(w, http.StatusNotFound, "Project not found or unauthorized", err.Error())
			return
		}
	}

	share, err := c.shareModel.GetShareByProjectID(projectID)
	if err != nil {
		log.Printf("[ShareController.GetShareStatus] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to get share status", err.Error())
		return
	}

	if share == nil || !share.IsActive {
		utils.RespondJSON(w, http.StatusOK, "Project is not shared", map[string]interface{}{
			"is_active": false,
		})
		return
	}

	resp := dtos.ProjectShareDTO{
		ID:         share.ID,
		ShareToken: share.ShareToken,
		ProjectID:  share.ProjectID,
		Title:      share.Title,
		IsActive:   share.IsActive,
		ViewCount:  share.ViewCount,
		ShareURL:   fmt.Sprintf("/share/%s", share.ShareToken),
		CreatedAt:  share.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  share.UpdatedAt.Format(time.RFC3339),
	}

	utils.RespondJSON(w, http.StatusOK, "Share status retrieved", resp)
}

func (c *ShareController) RevokeShare(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	if userID != "" {
		if _, _, err := c.projectModel.GetByIDScoped(projectID, userID, ""); err != nil {
			utils.RespondError(w, http.StatusNotFound, "Project not found or unauthorized", err.Error())
			return
		}
	}

	err := c.shareModel.RevokeShare(projectID)
	if err != nil {
		log.Printf("[ShareController.RevokeShare] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to revoke share", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Share link revoked successfully", map[string]interface{}{
		"is_active": false,
	})
}

func (c *ShareController) GetSharedProject(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing share token", "path parameter 'token' is required")
		return
	}

	share, project, messages, err := c.shareModel.GetShareByToken(token)
	if err != nil {
		utils.RespondError(w, http.StatusNotFound, "Shared project not found", err.Error())
		return
	}

	dtoMessages := make([]dtos.ChatMessageDTO, 0, len(messages))
	for _, m := range messages {
		dtoMessages = append(dtoMessages, dtos.ChatMessageDTO{
			ID:            m.ID,
			ProjectID:     m.ProjectID,
			Role:          m.Role,
			Content:       m.Content,
			TargetNodeIDs: m.TargetNodeIDs,
			CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		})
	}

	detail := dtos.SharedProjectDetailResponse{
		Share: dtos.ProjectShareDTO{
			ID:         share.ID,
			ShareToken: share.ShareToken,
			ProjectID:  share.ProjectID,
			Title:      share.Title,
			IsActive:   share.IsActive,
			ViewCount:  share.ViewCount,
			ShareURL:   fmt.Sprintf("/share/%s", share.ShareToken),
			CreatedAt:  share.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  share.UpdatedAt.Format(time.RFC3339),
		},
		Project: dtos.ProjectResponse{
			ID:           project.ID,
			Title:        project.Title,
			DiagramType:  project.DiagramType,
			CurrentNodes: project.CurrentNodes,
			CurrentEdges: project.CurrentEdges,
			Nodes:        project.CurrentNodes,
			Edges:        project.CurrentEdges,
			Messages:     dtoMessages,
			CreatedAt:    project.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    project.UpdatedAt.Format(time.RFC3339),
		},
		Messages: dtoMessages,
	}

	utils.RespondJSON(w, http.StatusOK, "Shared project retrieved successfully", detail)
}

func (c *ShareController) ForkSharedProject(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing share token", "path parameter 'token' is required")
		return
	}

	var req dtos.ForkSharedProjectRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	clonedProject, err := c.shareModel.ForkSharedProjectWithUser(token, strings.TrimSpace(req.Title), userID)
	if err != nil {
		log.Printf("[ShareController.ForkSharedProject] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to fork shared project", err.Error())
		return
	}

	fullProject, err := c.projectService.GetProjectByID(clonedProject.ID)
	if err != nil {
		log.Printf("[ShareController.ForkSharedProject] GetByID Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to load forked project", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, "Shared project cloned to workspace successfully", fullProject)
}

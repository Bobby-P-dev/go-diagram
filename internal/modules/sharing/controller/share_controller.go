package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type ProjectAccessReader interface {
	VerifyProjectAccess(ctx context.Context, projectID, userID string) (string, error)
}

type ProjectAccessReaderFunc func(ctx context.Context, projectID, userID string) (string, error)

func (f ProjectAccessReaderFunc) VerifyProjectAccess(ctx context.Context, projectID, userID string) (string, error) {
	return f(ctx, projectID, userID)
}

type ProjectDetailLoader interface {
	LoadProjectDetail(ctx context.Context, id string) (interface{}, error)
}

type ProjectDetailLoaderFunc func(ctx context.Context, id string) (interface{}, error)

func (f ProjectDetailLoaderFunc) LoadProjectDetail(ctx context.Context, id string) (interface{}, error) {
	return f(ctx, id)
}

type ShareController struct {
	service       service.ShareService
	projectReader ProjectAccessReader
	projectLoader ProjectDetailLoader
}

func NewShareController(
	service service.ShareService,
	projectReader ProjectAccessReader,
	projectLoader ProjectDetailLoader,
) *ShareController {
	return &ShareController{
		service:       service,
		projectReader: projectReader,
		projectLoader: projectLoader,
	}
}

func (c *ShareController) CreateShare(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	userID := ""
	if user := authcontext.UserFromContext(r.Context()); user != nil {
		userID = user.ID
	}

	title, err := c.projectReader.VerifyProjectAccess(r.Context(), projectID, userID)
	if err != nil {
		httpresponse.RespondError(w, http.StatusNotFound, "Project not found", err.Error())
		return
	}

	share, err := c.service.CreateOrGetShare(r.Context(), projectID, title)
	if err != nil {
		log.Printf("[ShareController.CreateShare] Error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to create share link", err.Error())
		return
	}

	resp := dto.ProjectShareDTO{
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

	httpresponse.RespondJSON(w, http.StatusOK, "Share link created successfully", resp)
}

func (c *ShareController) GetShareStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	userID := ""
	if user := authcontext.UserFromContext(r.Context()); user != nil {
		userID = user.ID
	}

	if userID != "" {
		if _, err := c.projectReader.VerifyProjectAccess(r.Context(), projectID, userID); err != nil {
			httpresponse.RespondError(w, http.StatusNotFound, "Project not found or unauthorized", err.Error())
			return
		}
	}

	share, err := c.service.GetShareByProjectID(r.Context(), projectID)
	if err != nil {
		log.Printf("[ShareController.GetShareStatus] Error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to get share status", err.Error())
		return
	}

	if share == nil || !share.IsActive {
		httpresponse.RespondJSON(w, http.StatusOK, "Project is not shared", map[string]interface{}{
			"is_active": false,
		})
		return
	}

	resp := dto.ProjectShareDTO{
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

	httpresponse.RespondJSON(w, http.StatusOK, "Share status retrieved", resp)
}

func (c *ShareController) RevokeShare(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	userID := ""
	if user := authcontext.UserFromContext(r.Context()); user != nil {
		userID = user.ID
	}

	if userID != "" {
		if _, err := c.projectReader.VerifyProjectAccess(r.Context(), projectID, userID); err != nil {
			httpresponse.RespondError(w, http.StatusNotFound, "Project not found or unauthorized", err.Error())
			return
		}
	}

	err := c.service.RevokeShare(r.Context(), projectID)
	if err != nil {
		log.Printf("[ShareController.RevokeShare] Error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to revoke share", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Share link revoked successfully", map[string]interface{}{
		"is_active": false,
	})
}

func (c *ShareController) GetSharedProject(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing share token", "path parameter 'token' is required")
		return
	}

	share, project, messages, err := c.service.GetShareByToken(r.Context(), token)
	if err != nil {
		httpresponse.RespondError(w, http.StatusNotFound, "Shared project not found", err.Error())
		return
	}

	dtoMessages := make([]dto.ChatMessageDTO, 0, len(messages))
	for _, m := range messages {
		dtoMessages = append(dtoMessages, dto.ChatMessageDTO{
			ID:            m.ID,
			ProjectID:     m.ProjectID,
			Role:          m.Role,
			Content:       m.Content,
			TargetNodeIDs: m.TargetNodeIDs,
			CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		})
	}

	detail := dto.SharedProjectDetailResponse{
		Share: dto.ProjectShareDTO{
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
		Project: dto.ProjectResponse{
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

	httpresponse.RespondJSON(w, http.StatusOK, "Shared project retrieved successfully", detail)
}

func (c *ShareController) ForkSharedProject(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing share token", "path parameter 'token' is required")
		return
	}

	var req dto.ForkSharedProjectRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	userID := ""
	if user := authcontext.UserFromContext(r.Context()); user != nil {
		userID = user.ID
	}

	cloned, err := c.service.ForkSharedProjectWithUser(r.Context(), token, strings.TrimSpace(req.Title), userID)
	if err != nil {
		log.Printf("[ShareController.ForkSharedProject] Error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to fork shared project", err.Error())
		return
	}

	fullProject, err := c.projectLoader.LoadProjectDetail(r.Context(), cloned.ID)
	if err != nil {
		log.Printf("[ShareController.ForkSharedProject] GetByID Error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to load forked project", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusCreated, "Shared project cloned to workspace successfully", fullProject)
}

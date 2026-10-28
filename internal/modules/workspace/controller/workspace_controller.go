package controller

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type UserReader func(context.Context) *workspace.Actor

type WorkspaceController struct {
	service    WorkspaceService
	userReader UserReader
}

func NewWorkspaceController(svc WorkspaceService) *WorkspaceController {
	return NewWorkspaceControllerWithUserReader(svc, func(ctx context.Context) *workspace.Actor {
		user := authcontext.UserFromContext(ctx)
		if user == nil {
			return nil
		}
		return &workspace.Actor{ID: user.ID, Role: user.Role}
	})
}

func NewWorkspaceControllerWithUserReader(svc WorkspaceService, reader UserReader) *WorkspaceController {
	return &WorkspaceController{service: svc, userReader: reader}
}

func (c *WorkspaceController) actor(r *http.Request) *workspace.Actor {
	if c.userReader == nil {
		return nil
	}
	return c.userReader(r.Context())
}

func respondFailure(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, service.ErrUnauthenticated):
		httpresponse.RespondError(w, http.StatusUnauthorized, "Authentication required", "A valid access credential is required.")
	case errors.Is(err, service.ErrAccessUnavailable):
		httpresponse.RespondError(w, http.StatusInternalServerError, "Workspace authorization unavailable", "project model is not configured")
	case errors.Is(err, service.ErrProjectNotFound):
		httpresponse.RespondError(w, http.StatusNotFound, "Project not found", "Project does not exist or is not owned by the caller.")
	case errors.Is(err, service.ErrCommentNotFound):
		httpresponse.RespondError(w, http.StatusNotFound, "Comment not found", "Comment does not exist.")
	case errors.Is(err, service.ErrContentRequired):
		httpresponse.RespondError(w, http.StatusBadRequest, "Content is required", "content cannot be empty")
	default:
		log.Printf("[WorkspaceController] %s: %v", message, err)
		httpresponse.RespondError(w, http.StatusInternalServerError, message, err.Error())
	}
}

func (c *WorkspaceController) GetComments(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	comments, err := c.service.GetComments(r.Context(), c.actor(r), projectID)
	if err != nil {
		respondFailure(w, err, "Failed to get comments")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Comments retrieved successfully", comments)
}

func (c *WorkspaceController) CreateComment(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	var req dto.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	comment, err := c.service.CreateComment(r.Context(), c.actor(r), projectID, req)
	if err != nil {
		respondFailure(w, err, "Failed to create comment")
		return
	}

	httpresponse.RespondJSON(w, http.StatusCreated, "Comment created successfully", comment)
}

func (c *WorkspaceController) UpdateCommentStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing comment ID", "path parameter 'id' is required")
		return
	}

	var req dto.UpdateCommentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	status, err := c.service.UpdateCommentStatus(r.Context(), c.actor(r), id, req)
	if err != nil {
		respondFailure(w, err, "Failed to update comment")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Comment status updated", map[string]string{"id": id, "status": status})
}

func (c *WorkspaceController) DeleteComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing comment ID", "path parameter 'id' is required")
		return
	}

	if err := c.service.DeleteComment(r.Context(), c.actor(r), id); err != nil {
		respondFailure(w, err, "Failed to delete comment")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Comment deleted", map[string]string{"id": id})
}

func (c *WorkspaceController) GetFoundations(w http.ResponseWriter, r *http.Request) {
	foundations, err := c.service.GetFoundations()
	if err != nil {
		respondFailure(w, err, "Failed to get foundations")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Foundations retrieved successfully", foundations)
}

func (c *WorkspaceController) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := c.service.GetSettings()
	if err != nil {
		respondFailure(w, err, "Failed to get settings")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Settings retrieved successfully", settings)
}

func (c *WorkspaceController) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req dto.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	updated, err := c.service.UpdateSettings(req)
	if err != nil {
		respondFailure(w, err, "Failed to update settings")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Settings updated successfully", updated)
}

func (c *WorkspaceController) GetExports(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	exports, err := c.service.GetExports(r.Context(), c.actor(r), projectID)
	if err != nil {
		respondFailure(w, err, "Failed to get exports")
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Exports retrieved successfully", exports)
}

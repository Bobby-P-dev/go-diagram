package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/middlewares"
	"github.com/Bobby-P-dev/go-diagram.git/src/services"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type ProjectController struct {
	service *services.ProjectService
}

func NewProjectController(service *services.ProjectService) *ProjectController {
	return &ProjectController{service: service}
}

func (c *ProjectController) Create(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.Create] Decode error: %v", err)
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	user := middlewares.GetUserFromContext(r.Context())
	if user != nil && !user.CanGenerateDiagram {
		utils.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk membuat atau mengedit Diagram Alur.")
		return
	}
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := c.service.CreateNewProjectForUser(userID, req)
	if err != nil {
		log.Printf("[ProjectController.Create] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to create project", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, "Project created successfully", result)
}

func (c *ProjectController) Chat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	if user != nil && !user.CanGenerateDiagram {
		utils.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk fitur Diagram Alur.")
		return
	}
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var req dtos.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.Chat] Decode error: %v", err)
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	result, err := c.service.SendChatMessageScoped(id, req, userID)
	if err != nil {
		log.Printf("[ProjectController.Chat] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to process chat message", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Diagram updated successfully", result)
}

func (c *ProjectController) GetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID, userRole := "", ""
	if user != nil {
		userID = user.ID
		userRole = user.Role
	}

	result, err := c.service.GetProjectByIDScoped(id, userID, userRole)
	if err != nil {
		log.Printf("[ProjectController.GetByID] Service error: %v", err)
		utils.RespondError(w, http.StatusNotFound, "Project not found", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Project retrieved successfully", result)
}

func (c *ProjectController) UpdateGraph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var req struct {
		Nodes json.RawMessage `json:"nodes"`
		Edges json.RawMessage `json:"edges"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.UpdateGraph] Decode error: %v", err)
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if err := c.service.UpdateProjectGraphScoped(id, userID, req.Nodes, req.Edges); err != nil {
		log.Printf("[ProjectController.UpdateGraph] Service error: %v", err)
		utils.RespondError(w, http.StatusNotFound, "Failed to update project graph", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Graph positions updated successfully", map[string]bool{"success": true})
}

func (c *ProjectController) GetAll(w http.ResponseWriter, r *http.Request) {
	limit := 15
	offset := 0

	query := r.URL.Query()
	if l := query.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := query.Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := c.service.ListProjectsForUser(userID, limit, offset)
	if err != nil {
		log.Printf("[ProjectController.GetAll] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve projects", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Projects retrieved successfully", result)
}

func (c *ProjectController) TogglePin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := c.service.ToggleProjectPinScoped(id, userID)
	if err != nil {
		log.Printf("[ProjectController.TogglePin] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to toggle pin", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Project pin status updated", result)
}

func (c *ProjectController) GetVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	versions, err := c.service.GetProjectVersionsScoped(id, userID)
	if err != nil {
		log.Printf("[ProjectController.GetVersions] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve project versions", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Versions retrieved successfully", versions)
}

func (c *ProjectController) Rollback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versionID := r.PathValue("versionId")
	if id == "" || versionID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing parameters", "path parameters 'id' and 'versionId' are required")
		return
	}

	user := middlewares.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := c.service.RollbackToVersionScoped(id, versionID, userID)
	if err != nil {
		log.Printf("[ProjectController.Rollback] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to rollback project version", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Project rolled back successfully", result)
}

func (c *ProjectController) GetTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := c.service.ListTemplates()
	if err != nil {
		log.Printf("[ProjectController.GetTemplates] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve templates", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Templates retrieved successfully", templates)
}

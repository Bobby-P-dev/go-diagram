package controller

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type ProjectController struct {
	service    DiagramService
	userReader UserReader
}

func NewProjectController(service DiagramService) *ProjectController {
	return NewProjectControllerWithUserReader(service, DefaultUserReader)
}

func NewProjectControllerWithUserReader(service DiagramService, userReader UserReader) *ProjectController {
	if userReader == nil {
		userReader = DefaultUserReader
	}
	return &ProjectController{
		service:    service,
		userReader: userReader,
	}
}

func (controller *ProjectController) Create(response http.ResponseWriter, request *http.Request) {
	var req dto.CreateProjectRequest
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.Create] Decode error: %v", err)
		httpresponse.RespondError(response, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer request.Body.Close()

	user := readUser(controller.userReader, request.Context())
	if user != nil && !user.CanGenerateDiagram {
		httpresponse.RespondError(response, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk membuat atau mengedit Diagram Alur.")
		return
	}
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := controller.service.CreateNewProjectForUser(userID, req)
	if err != nil {
		log.Printf("[ProjectController.Create] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to create project", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Project created successfully", result)
}

func (controller *ProjectController) Chat(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if id == "" {
		httpresponse.RespondError(response, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := readUser(controller.userReader, request.Context())
	if user != nil && !user.CanGenerateDiagram {
		httpresponse.RespondError(response, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk fitur Diagram Alur.")
		return
	}
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var req dto.ChatRequest
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.Chat] Decode error: %v", err)
		httpresponse.RespondError(response, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer request.Body.Close()

	result, err := controller.service.SendChatMessageScoped(id, req, userID)
	if err != nil {
		log.Printf("[ProjectController.Chat] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to process chat message", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Diagram updated successfully", result)
}

func (controller *ProjectController) GetByID(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if id == "" {
		httpresponse.RespondError(response, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := readUser(controller.userReader, request.Context())
	userID, userRole := "", ""
	if user != nil {
		userID = user.ID
		userRole = user.Role
	}

	result, err := controller.service.GetProjectByIDScoped(id, userID, userRole)
	if err != nil {
		log.Printf("[ProjectController.GetByID] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusNotFound, "Project not found", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Project retrieved successfully", result)
}

func (controller *ProjectController) UpdateGraph(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if id == "" {
		httpresponse.RespondError(response, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := readUser(controller.userReader, request.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var req struct {
		Nodes json.RawMessage `json:"nodes"`
		Edges json.RawMessage `json:"edges"`
	}
	if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.UpdateGraph] Decode error: %v", err)
		httpresponse.RespondError(response, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer request.Body.Close()

	if err := controller.service.UpdateProjectGraphScoped(id, userID, req.Nodes, req.Edges); err != nil {
		log.Printf("[ProjectController.UpdateGraph] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusNotFound, "Failed to update project graph", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Graph positions updated successfully", map[string]bool{"success": true})
}

func (controller *ProjectController) GetAll(response http.ResponseWriter, request *http.Request) {
	limit := 15
	offset := 0

	query := request.URL.Query()
	if limitQuery := query.Get("limit"); limitQuery != "" {
		if value, err := strconv.Atoi(limitQuery); err == nil && value > 0 {
			limit = value
		}
	}
	if offsetQuery := query.Get("offset"); offsetQuery != "" {
		if value, err := strconv.Atoi(offsetQuery); err == nil && value >= 0 {
			offset = value
		}
	}

	user := readUser(controller.userReader, request.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := controller.service.ListProjectsForUser(userID, limit, offset)
	if err != nil {
		log.Printf("[ProjectController.GetAll] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to retrieve projects", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Projects retrieved successfully", result)
}

func (controller *ProjectController) TogglePin(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if id == "" {
		httpresponse.RespondError(response, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := readUser(controller.userReader, request.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := controller.service.ToggleProjectPinScoped(id, userID)
	if err != nil {
		log.Printf("[ProjectController.TogglePin] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to toggle pin", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Project pin status updated", result)
}

func (controller *ProjectController) GetVersions(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	if id == "" {
		httpresponse.RespondError(response, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := readUser(controller.userReader, request.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	versions, err := controller.service.GetProjectVersionsScoped(id, userID)
	if err != nil {
		log.Printf("[ProjectController.GetVersions] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to retrieve project versions", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Versions retrieved successfully", versions)
}

func (controller *ProjectController) Rollback(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	versionID := request.PathValue("versionId")
	if id == "" || versionID == "" {
		httpresponse.RespondError(response, http.StatusBadRequest, "Missing parameters", "path parameters 'id' and 'versionId' are required")
		return
	}

	user := readUser(controller.userReader, request.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	result, err := controller.service.RollbackToVersionScoped(id, versionID, userID)
	if err != nil {
		log.Printf("[ProjectController.Rollback] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to rollback project version", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Project rolled back successfully", result)
}

func (controller *ProjectController) GetTemplates(response http.ResponseWriter, request *http.Request) {
	templates, err := controller.service.ListTemplates()
	if err != nil {
		log.Printf("[ProjectController.GetTemplates] Service error: %v", err)
		httpresponse.RespondError(response, http.StatusInternalServerError, "Failed to retrieve templates", err.Error())
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Templates retrieved successfully", templates)
}

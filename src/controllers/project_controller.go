package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
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

	result, err := c.service.CreateNewProject(req)
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

	var req dtos.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ProjectController.Chat] Decode error: %v", err)
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	result, err := c.service.SendChatMessage(id, req)
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

	result, err := c.service.GetProjectByID(id)
	if err != nil {
		log.Printf("[ProjectController.GetByID] Service error: %v", err)
		utils.RespondError(w, http.StatusNotFound, "Project not found", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Project retrieved successfully", result)
}

func (c *ProjectController) GetAll(w http.ResponseWriter, r *http.Request) {
	result, err := c.service.ListProjects()
	if err != nil {
		log.Printf("[ProjectController.GetAll] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to retrieve projects", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Projects retrieved successfully", result)
}

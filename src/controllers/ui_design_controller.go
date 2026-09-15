package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/services"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type UIDesignController struct {
	service services.UIDesignServiceInterface
}

func NewUIDesignController(service services.UIDesignServiceInterface) *UIDesignController {
	return &UIDesignController{service: service}
}

func (c *UIDesignController) GetTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := c.service.GetTemplates(r.Context())
	if err != nil {
		log.Printf("[UIDesignController.GetTemplates] Error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to fetch UI templates", err.Error())
		return
	}
	utils.RespondJSON(w, http.StatusOK, "UI templates retrieved successfully", templates)
}

func (c *UIDesignController) Create(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateUIDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[UIDesignController.Create] Decode error: %v", err)
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	result, err := c.service.GenerateUIDesign(r.Context(), req)
	if err != nil {
		log.Printf("[UIDesignController.Create] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to generate UI design", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusCreated, "UI design created successfully", result)
}

func (c *UIDesignController) Chat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		utils.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	var req dtos.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[UIDesignController.Chat] Decode error: %v", err)
		utils.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	result, err := c.service.IterateUIDesignWithChat(r.Context(), id, req.Prompt, req.TargetedNodeIDs)
	if err != nil {
		log.Printf("[UIDesignController.Chat] Service error: %v", err)
		utils.RespondError(w, http.StatusInternalServerError, "Failed to update UI design", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "UI design updated successfully", result)
}

package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type UIDesignController struct {
	service    UIDesignService
	userReader UserReader
}

func NewUIDesignController(service UIDesignService) *UIDesignController {
	return NewUIDesignControllerWithUserReader(service, DefaultUserReader)
}

func NewUIDesignControllerWithUserReader(service UIDesignService, userReader UserReader) *UIDesignController {
	if userReader == nil {
		userReader = DefaultUserReader
	}
	return &UIDesignController{
		service:    service,
		userReader: userReader,
	}
}

func (c *UIDesignController) GetTemplates(w http.ResponseWriter, r *http.Request) {
	if c.service == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to fetch UI templates", "service unavailable")
		return
	}
	templates, err := c.service.GetTemplates(r.Context())
	if err != nil {
		log.Printf("[UIDesignController.GetTemplates] Error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to fetch UI templates", err.Error())
		return
	}
	httpresponse.RespondJSON(w, http.StatusOK, "UI templates retrieved successfully", templates)
}

func (c *UIDesignController) Create(w http.ResponseWriter, r *http.Request) {
	user := readUser(c.userReader, r.Context())
	if user != nil && !user.CanGenerateUI {
		httpresponse.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk fitur Desain Antarmuka (UI).")
		return
	}

	var req dto.CreateUIDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[UIDesignController.Create] Decode error: %v", err)
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if c.service == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to generate UI design", "service unavailable")
		return
	}

	result, err := c.service.GenerateUIDesign(r.Context(), req)
	if err != nil {
		log.Printf("[UIDesignController.Create] Service error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to generate UI design", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusCreated, "UI design created successfully", result)
}

func (c *UIDesignController) CreateStream(w http.ResponseWriter, r *http.Request) {
	user := readUser(c.userReader, r.Context())
	if user != nil && !user.CanGenerateUI {
		httpresponse.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk fitur Desain Antarmuka (UI).")
		return
	}

	var req dto.CreateUIDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[UIDesignController.CreateStream] Decode error: %v", err)
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if c.service == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to generate UI design", "service unavailable")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		result, err := c.service.GenerateUIDesign(r.Context(), req)
		if err != nil {
			httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to generate UI design", err.Error())
			return
		}
		httpresponse.RespondJSON(w, http.StatusCreated, "UI design created successfully", result)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sendSSE := func(eventType string, data any) {
		dataBytes, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(dataBytes))
		flusher.Flush()
	}

	_, err := c.service.GenerateUIDesignStream(r.Context(), req, sendSSE)
	if err != nil {
		log.Printf("[UIDesignController.CreateStream] Service error: %v", err)
		sendSSE("error", map[string]string{
			"message": err.Error(),
		})
	}
}

func (c *UIDesignController) Chat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Missing project ID", "path parameter 'id' is required")
		return
	}

	user := readUser(c.userReader, r.Context())
	if user != nil && !user.CanGenerateUI {
		httpresponse.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk fitur Desain Antarmuka (UI).")
		return
	}

	var req dto.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[UIDesignController.Chat] Decode error: %v", err)
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	defer r.Body.Close()

	if req.Prompt == "" && req.Instruction != "" {
		req.Prompt = req.Instruction
	}

	if c.service == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to update UI design", "service unavailable")
		return
	}

	result, err := c.service.IterateUIDesignWithTargetedChat(r.Context(), id, &req)
	if err != nil {
		log.Printf("[UIDesignController.Chat] Service error: %v", err)
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to update UI design", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "UI design updated successfully", result)
}

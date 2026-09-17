package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/services"
	"github.com/Bobby-P-dev/go-diagram.git/src/utils"
)

type AsyncJobController struct {
	dispatcherService *services.JobDispatcherService
	projectService    *services.ProjectService
}

func NewAsyncJobController(
	dispatcher *services.JobDispatcherService,
	projectService *services.ProjectService,
) *AsyncJobController {
	return &AsyncJobController{
		dispatcherService: dispatcher,
		projectService:    projectService,
	}
}

func (c *AsyncJobController) CreateAsyncUIDesign(w http.ResponseWriter, r *http.Request) {
	var req dtos.CreateUIDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid JSON payload", err.Error())
		return
	}

	res, err := c.dispatcherService.DispatchUIDesignJob(r.Context(), req)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to dispatch async job", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusAccepted, res.Message, res)
}

func (c *AsyncJobController) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		utils.RespondError(w, http.StatusBadRequest, "Job ID is required", "Missing job id parameter")
		return
	}

	statusRes, err := c.dispatcherService.GetJobStatus(r.Context(), jobID)
	if err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to get job status", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Job status retrieved", statusRes)
}

func (c *AsyncJobController) HandleJobCallback(w http.ResponseWriter, r *http.Request) {
	// 1. Verify internal service authorization
	serviceKey := r.Header.Get("X-Internal-Service-Key")
	if serviceKey == "" || serviceKey != config.Env.InternalServiceKey {
		utils.RespondError(w, http.StatusUnauthorized, "Unauthorized", "Invalid or missing internal service key")
		return
	}

	// 2. Decode callback payload
	var callbackReq dtos.JobCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&callbackReq); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "Invalid callback JSON payload", err.Error())
		return
	}

	// 3. Process callback and persist canvas state
	if err := c.dispatcherService.ProcessJobCallback(r.Context(), callbackReq); err != nil {
		utils.RespondError(w, http.StatusInternalServerError, "Failed to process job callback", err.Error())
		return
	}

	utils.RespondJSON(w, http.StatusOK, "Job callback processed successfully", map[string]string{
		"job_id": callbackReq.JobID,
		"status": "persisted",
	})
}

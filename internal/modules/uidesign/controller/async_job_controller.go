package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type AsyncJobController struct {
	dispatcher   AsyncJobDispatcher
	userReader   UserReader
	keyValidator KeyValidator
}

func NewAsyncJobController(dispatcher AsyncJobDispatcher) *AsyncJobController {
	return NewAsyncJobControllerWithOptions(dispatcher, DefaultUserReader, nil)
}

func NewAsyncJobControllerWithOptions(dispatcher AsyncJobDispatcher, userReader UserReader, keyValidator KeyValidator) *AsyncJobController {
	if userReader == nil {
		userReader = DefaultUserReader
	}
	return &AsyncJobController{
		dispatcher:   dispatcher,
		userReader:   userReader,
		keyValidator: keyValidator,
	}
}

func (c *AsyncJobController) CreateAsyncUIDesign(w http.ResponseWriter, r *http.Request) {
	user := readUser(c.userReader, r.Context())
	if user == nil {
		httpresponse.RespondError(w, http.StatusUnauthorized, "Authentication required", "A valid access credential is required.")
		return
	}
	if !user.CanGenerateUI {
		httpresponse.RespondError(w, http.StatusForbidden, "Akses Ditolak", "Kredensial Anda tidak memiliki izin untuk fitur Desain Antarmuka (UI).")
		return
	}

	var req dto.CreateUIDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid JSON payload", err.Error())
		return
	}

	if err := req.ValidateGenerationMode(); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Unsupported UI generation mode", err.Error())
		return
	}

	if c.dispatcher == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to dispatch async job", "job dispatcher unavailable")
		return
	}

	res, err := c.dispatcher.DispatchUIDesignJob(r.Context(), req, user.ID)
	if err != nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to dispatch async job", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusAccepted, res.Message, res)
}

func (c *AsyncJobController) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	user := readUser(c.userReader, r.Context())
	if user == nil {
		httpresponse.RespondError(w, http.StatusUnauthorized, "Authentication required", "A valid access credential is required.")
		return
	}

	jobID := strings.TrimSpace(r.PathValue("id"))
	if jobID == "" {
		httpresponse.RespondError(w, http.StatusBadRequest, "Job ID is required", "Missing job id parameter")
		return
	}

	if c.dispatcher == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to get job status", "job dispatcher unavailable")
		return
	}

	statusRes, err := c.dispatcher.GetJobStatus(r.Context(), jobID, user.ID)
	if err != nil {
		if errors.Is(err, service.ErrJobNotFound) {
			httpresponse.RespondError(w, http.StatusNotFound, "Job not found", "Job does not exist or is not owned by the caller.")
			return
		}
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to get job status", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Job status retrieved", statusRes)
}

func (c *AsyncJobController) HandleJobCallback(w http.ResponseWriter, r *http.Request) {
	if c.keyValidator == nil || !c.keyValidator(r) {
		httpresponse.RespondError(w, http.StatusUnauthorized, "Unauthorized", "Invalid or missing internal service key")
		return
	}

	var callbackReq dto.JobCallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&callbackReq); err != nil {
		httpresponse.RespondError(w, http.StatusBadRequest, "Invalid callback JSON payload", err.Error())
		return
	}

	if c.dispatcher == nil {
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to process job callback", "job dispatcher unavailable")
		return
	}

	if err := c.dispatcher.ProcessJobCallback(r.Context(), callbackReq); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidJobCallback):
			httpresponse.RespondError(w, http.StatusBadRequest, "Invalid job callback", err.Error())
			return
		case errors.Is(err, service.ErrJobProjectMismatch):
			httpresponse.RespondError(w, http.StatusConflict, "Job/project mismatch", "Callback project does not match the dispatched job.")
			return
		case errors.Is(err, service.ErrJobNotFound):
			httpresponse.RespondError(w, http.StatusNotFound, "Project not found", "Callback project does not exist.")
			return
		}
		httpresponse.RespondError(w, http.StatusInternalServerError, "Failed to process job callback", err.Error())
		return
	}

	httpresponse.RespondJSON(w, http.StatusOK, "Job callback processed successfully", map[string]string{
		"job_id": callbackReq.JobID,
		"status": "persisted",
	})
}

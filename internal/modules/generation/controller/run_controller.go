package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type RunController struct {
	service generation.RunService
}

func NewRunController(service generation.RunService) *RunController {
	return &RunController{service: service}
}

func (controller *RunController) Start(response http.ResponseWriter, request *http.Request) {
	user := authcontext.UserFromContext(request.Context())
	if user == nil || user.ID == "" {
		httpresponse.RespondV2Error(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" {
		httpresponse.RespondV2Error(response, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required")
		return
	}

	request.Body = http.MaxBytesReader(response, request.Body, 64*1024)
	defer request.Body.Close()
	var input *generation.StartInput
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err == nil {
		var extra interface{}
		if trailingErr := decoder.Decode(&extra); trailingErr != io.EOF {
			err = trailingErr
			if err == nil {
				err = generation.ErrInvalidInput
			}
		}
	}
	if err != nil || input == nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			httpresponse.RespondV2Error(response, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request exceeds size limit")
		} else {
			httpresponse.RespondV2Error(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid generation request payload")
		}
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()

	scope := generation.Scope{
		UserID:    user.ID,
		ProjectID: request.PathValue("projectId"),
		ThreadID:  request.PathValue("threadId"),
	}

	run, err := controller.service.Start(ctx, scope, *input, key)
	if err != nil {
		status, code, message := http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to start generation run"
		switch {
		case errors.Is(err, generation.ErrUnauthenticated):
			status, code, message = http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required"
		case errors.Is(err, generation.ErrIdempotencyRequired):
			status, code, message = http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required"
		case errors.Is(err, generation.ErrIdempotencyConflict):
			status, code, message = http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency key reused with different request payload"
		case errors.Is(err, generation.ErrInvalidInput) || errors.Is(err, generation.ErrUnsupportedType):
			status, code, message = http.StatusBadRequest, "INVALID_REQUEST", "Invalid generation parameters or target revision"
		case errors.Is(err, generation.ErrNotFound):
			status, code, message = http.StatusNotFound, "RESOURCE_NOT_FOUND", "Project, thread, trigger message, or target artifact not found"
		case errors.Is(err, generation.ErrRevisionConflict):
			status, code, message = http.StatusConflict, "REVISION_CONFLICT", "Target artifact revision changed concurrently"
		case errors.Is(err, generation.ErrBudgetExceeded):
			status, code, message = http.StatusUnprocessableEntity, "CONTEXT_BUDGET_EXCEEDED", "Project context exceeds content budget"
		default:
			log.Printf("start generation failed: %v", err)
		}
		httpresponse.RespondV2Error(response, status, code, message)
		return
	}

	httpresponse.RespondJSON(response, http.StatusAccepted, "Generation run accepted", run)
}

func (controller *RunController) Get(response http.ResponseWriter, request *http.Request) {
	user := authcontext.UserFromContext(request.Context())
	if user == nil || user.ID == "" {
		httpresponse.RespondV2Error(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()

	scope := generation.Scope{
		UserID:    user.ID,
		ProjectID: request.PathValue("projectId"),
	}
	run, err := controller.service.Get(ctx, scope, request.PathValue("runId"))
	if err != nil {
		if errors.Is(err, generation.ErrNotFound) || errors.Is(err, generation.ErrInvalidInput) {
			httpresponse.RespondV2Error(response, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Generation run not found")
		} else {
			log.Printf("get generation failed: %v", err)
			httpresponse.RespondV2Error(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve generation run")
		}
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Generation run retrieved", run)
}

func (controller *RunController) Cancel(response http.ResponseWriter, request *http.Request) {
	user := authcontext.UserFromContext(request.Context())
	if user == nil || user.ID == "" {
		httpresponse.RespondV2Error(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()

	scope := generation.Scope{
		UserID:    user.ID,
		ProjectID: request.PathValue("projectId"),
	}
	run, err := controller.service.Cancel(ctx, scope, request.PathValue("runId"))
	if err != nil {
		switch {
		case errors.Is(err, generation.ErrNotFound) || errors.Is(err, generation.ErrInvalidInput):
			httpresponse.RespondV2Error(response, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Generation run not found")
		case errors.Is(err, generation.ErrRunTerminal):
			httpresponse.RespondV2Error(response, http.StatusConflict, "RUN_TERMINAL", "Generation run is already completed")
		default:
			log.Printf("cancel generation failed: %v", err)
			httpresponse.RespondV2Error(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to cancel generation run")
		}
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Generation run canceled", run)
}

func (controller *RunController) Events(response http.ResponseWriter, request *http.Request) {
	user := authcontext.UserFromContext(request.Context())
	if user == nil || user.ID == "" {
		httpresponse.RespondV2Error(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	flusher, ok := response.(http.Flusher)
	if !ok {
		httpresponse.RespondV2Error(response, http.StatusInternalServerError, "STREAMING_UNSUPPORTED", "Streaming is not supported")
		return
	}

	ctx := request.Context()
	scope := generation.Scope{
		UserID:    user.ID,
		ProjectID: request.PathValue("projectId"),
	}
	runID := request.PathValue("runId")

	getCtx, getCancel := context.WithTimeout(ctx, 5*time.Second)
	run, err := controller.service.Get(getCtx, scope, runID)
	getCancel()
	if err != nil {
		if errors.Is(err, generation.ErrNotFound) || errors.Is(err, generation.ErrInvalidInput) {
			httpresponse.RespondV2Error(response, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Generation run not found")
		} else {
			httpresponse.RespondV2Error(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to access generation run")
		}
		return
	}

	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	flusher.Flush()

	sendSSE := func(eventType string, data any) bool {
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(response, "event: %s\ndata: %s\n\n", eventType, string(dataBytes)); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	finish := func(current *generation.Run) bool {
		if !current.Terminal() {
			return false
		}
		if current.Status == "failed" {
			sendSSE("error", map[string]any{"code": current.ErrorCode, "message": current.ErrorMessage, "run": current})
		} else {
			sendSSE("complete", map[string]any{"run": current})
		}
		return true
	}

	lastStatus := run.Status
	if !sendSSE("status", map[string]string{"status": lastStatus}) || finish(run) {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	streamLimit := time.NewTimer(4 * time.Minute)
	defer streamLimit.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-streamLimit.C:
			return
		case <-heartbeat.C:
			if !sendSSE("progress", map[string]string{"status": lastStatus, "phase": "waiting_for_committed_result"}) {
				return
			}
		case <-ticker.C:
			pollCtx, pollCancel := context.WithTimeout(ctx, 3*time.Second)
			currentRun, err := controller.service.Get(pollCtx, scope, runID)
			pollCancel()
			if err != nil {
				sendSSE("error", map[string]string{"code": "STREAM_INTERRUPTED", "message": "Reconnect to retrieve the current run status"})
				return
			}
			if currentRun.Status != lastStatus {
				lastStatus = currentRun.Status
				if !sendSSE("status", map[string]string{"status": lastStatus}) {
					return
				}
			}
			if finish(currentRun) {
				return
			}
		}
	}
}

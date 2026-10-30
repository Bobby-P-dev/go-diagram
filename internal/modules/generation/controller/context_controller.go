package controller

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/generation"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type ContextController struct{ resolver generation.ContextResolver }

func NewContextController(resolver generation.ContextResolver) *ContextController {
	return &ContextController{resolver: resolver}
}

func (controller *ContextController) Resolve(response http.ResponseWriter, request *http.Request) {
	user := authcontext.UserFromContext(request.Context())
	if user == nil || user.ID == "" {
		httpresponse.RespondV2Error(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}
	var input *generation.ResolveInput
	request.Body = http.MaxBytesReader(response, request.Body, 32*1024)
	defer request.Body.Close()
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
			httpresponse.RespondV2Error(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid context request")
		}
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	result, err := controller.resolver.Resolve(ctx, generation.Scope{UserID: user.ID, ProjectID: request.PathValue("projectId"), ThreadID: request.PathValue("threadId")}, *input)
	if err != nil {
		status, code, message := http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to resolve context"
		switch {
		case errors.Is(err, generation.ErrUnauthenticated):
			status, code, message = http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required"
		case errors.Is(err, generation.ErrInvalidInput):
			status, code, message = http.StatusBadRequest, "INVALID_REQUEST", "Invalid context selection or budget"
		case errors.Is(err, generation.ErrNotFound):
			status, code, message = http.StatusNotFound, "RESOURCE_NOT_FOUND", "Context resource not found"
		case errors.Is(err, generation.ErrBudgetExceeded):
			status, code, message = http.StatusUnprocessableEntity, "CONTEXT_BUDGET_EXCEEDED", "Increase the budget for project context"
		default:
			log.Printf("context resolve failed: %v", err)
		}
		httpresponse.RespondV2Error(response, status, code, message)
		return
	}
	httpresponse.RespondJSON(response, http.StatusOK, "Context preview resolved", result)
}

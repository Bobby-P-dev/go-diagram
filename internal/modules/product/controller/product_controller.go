package controller

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type ProductController struct {
	service service.ProductService
}

func NewProductController(service service.ProductService) *ProductController {
	return &ProductController{service: service}
}

func getUserID(request *http.Request) string {
	user := authcontext.UserFromContext(request.Context())
	if user != nil {
		return user.ID
	}
	return ""
}

func (controller *ProductController) mapError(response http.ResponseWriter, err error, defaultMsg string) {
	status, code, message := http.StatusInternalServerError, "INTERNAL_ERROR", defaultMsg
	switch {
	case errors.Is(err, product.ErrNotFound):
		status, code, message = http.StatusNotFound, "RESOURCE_NOT_FOUND", "Resource not found"
	case errors.Is(err, product.ErrUnauthenticated):
		status, code, message = http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required"
	case errors.Is(err, product.ErrInvalidInput):
		status, code, message = http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body, identifier, pagination or Idempotency-Key"
	case errors.Is(err, product.ErrIdempotencyConflict):
		status, code, message = http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency key reused with different input"
	case errors.Is(err, product.ErrMessageConflict), errors.Is(err, product.ErrIdempotencyInFlight):
		status, code, message = http.StatusConflict, "RESOURCE_CONFLICT", "Request conflicts with an existing resource"
	default:
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			status, code, message = http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request exceeds 1 MiB"
		} else {
			log.Printf("product request failed: %v", err)
		}
	}
	httpresponse.RespondV2Error(response, status, code, message)
}

func parsePageQuery(request *http.Request) (product.PageQuery, error) {
	limit := 20
	values := request.URL.Query()
	for key, entries := range values {
		if (key != "limit" && key != "cursor") || len(entries) != 1 {
			return product.PageQuery{}, product.ErrInvalidInput
		}
	}
	if raw, exists := values["limit"]; exists {
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed < 1 || parsed > 100 {
			return product.PageQuery{}, product.ErrInvalidInput
		}
		limit = parsed
	}
	return product.PageQuery{Limit: limit, Cursor: values.Get("cursor")}, nil
}

func decodeBody[T any](response http.ResponseWriter, request *http.Request) (T, error) {
	var empty T
	request.Body = http.MaxBytesReader(response, request.Body, 1<<20)
	defer request.Body.Close()
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var target *T
	if err := decoder.Decode(&target); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			return empty, err
		}
		return empty, product.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			return empty, err
		}
		return empty, product.ErrInvalidInput
	}
	if target == nil {
		return empty, product.ErrInvalidInput
	}
	return *target, nil
}

func (controller *ProductController) CreateProject(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, product.ErrUnauthenticated, "Authentication required")
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Idempotency-Key header is required")
		return
	}

	req, err := decodeBody[dto.CreateProjectRequest](response, request)
	if err != nil {
		controller.mapError(response, err, "Invalid request body")
		return
	}

	result, err := controller.service.CreateProject(request.Context(), userID, req, idempotencyKey)
	if err != nil {
		controller.mapError(response, err, "Failed to create project")
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Project created successfully", result)
}

func (controller *ProductController) ListProjects(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, product.ErrUnauthenticated, "Authentication required")
		return
	}

	query, err := parsePageQuery(request)
	if err != nil {
		controller.mapError(response, err, "Invalid pagination")
		return
	}
	result, err := controller.service.ListProjects(request.Context(), userID, query)
	if err != nil {
		controller.mapError(response, err, "Failed to list projects")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Projects retrieved successfully", result)
}

func (controller *ProductController) CreateThread(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, product.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	if strings.TrimSpace(projectID) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Project ID is required")
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Idempotency-Key header is required")
		return
	}

	req, err := decodeBody[dto.CreateThreadRequest](response, request)
	if err != nil {
		controller.mapError(response, err, "Invalid request body")
		return
	}

	result, err := controller.service.CreateThread(request.Context(), userID, projectID, req, idempotencyKey)
	if err != nil {
		controller.mapError(response, err, "Failed to create thread")
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Thread created successfully", result)
}

func (controller *ProductController) ListThreads(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, product.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	if strings.TrimSpace(projectID) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Project ID is required")
		return
	}

	query, err := parsePageQuery(request)
	if err != nil {
		controller.mapError(response, err, "Invalid pagination")
		return
	}
	result, err := controller.service.ListThreads(request.Context(), userID, projectID, query)
	if err != nil {
		controller.mapError(response, err, "Failed to list threads")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Threads retrieved successfully", result)
}

func (controller *ProductController) PostMessage(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, product.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	threadID := request.PathValue("threadId")
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(threadID) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Project ID and Thread ID are required")
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Idempotency-Key header is required")
		return
	}

	req, err := decodeBody[dto.PostMessageRequest](response, request)
	if err != nil {
		controller.mapError(response, err, "Invalid request body")
		return
	}

	result, err := controller.service.PostUserMessage(request.Context(), userID, projectID, threadID, req, idempotencyKey)
	if err != nil {
		controller.mapError(response, err, "Failed to post message")
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Message posted successfully", result)
}

func (controller *ProductController) ListMessages(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, product.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	threadID := request.PathValue("threadId")
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(threadID) == "" {
		controller.mapError(response, product.ErrInvalidInput, "Project ID and Thread ID are required")
		return
	}

	query, err := parsePageQuery(request)
	if err != nil {
		controller.mapError(response, err, "Invalid pagination")
		return
	}
	result, err := controller.service.ListMessages(request.Context(), userID, projectID, threadID, query)
	if err != nil {
		controller.mapError(response, err, "Failed to list messages")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Messages retrieved successfully", result)
}

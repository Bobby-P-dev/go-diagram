package controller

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/httpresponse"
)

type ArtifactController struct {
	service service.ArtifactService
}

func NewArtifactController(service service.ArtifactService) *ArtifactController {
	return &ArtifactController{service: service}
}

func getUserID(request *http.Request) string {
	user := authcontext.UserFromContext(request.Context())
	if user != nil {
		return user.ID
	}
	return ""
}

func (controller *ArtifactController) mapError(response http.ResponseWriter, err error, defaultMsg string) {
	status, code, message := http.StatusInternalServerError, "INTERNAL_ERROR", defaultMsg
	switch {
	case errors.Is(err, artifact.ErrNotFound):
		status, code, message = http.StatusNotFound, "RESOURCE_NOT_FOUND", "Resource not found"
	case errors.Is(err, artifact.ErrUnauthenticated):
		status, code, message = http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required"
	case errors.Is(err, artifact.ErrInvalidInput):
		status, code, message = http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body, identifier, pagination or Idempotency-Key"
	case errors.Is(err, artifact.ErrContentInvalid):
		status, code, message = http.StatusUnprocessableEntity, "CONTENT_INVALID", "Content does not match the artifact format or schema version"
	case errors.Is(err, artifact.ErrRevisionConflict):
		status, code, message = http.StatusConflict, "REVISION_CONFLICT", "Artifact changed; reload before applying this revision"
	case errors.Is(err, artifact.ErrIdempotencyConflict):
		status, code, message = http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency key reused with different input"
	case errors.Is(err, artifact.ErrSlugConflict), errors.Is(err, artifact.ErrIdempotencyInFlight):
		status, code, message = http.StatusConflict, "RESOURCE_CONFLICT", "Request conflicts with an existing resource"
	default:
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			status, code, message = http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request exceeds 1 MiB"
		} else {
			log.Printf("artifact request failed: %v", err)
		}
	}
	httpresponse.RespondV2Error(response, status, code, message)
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
		return empty, artifact.ErrInvalidInput
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			return empty, err
		}
		return empty, artifact.ErrInvalidInput
	}
	if target == nil {
		return empty, artifact.ErrInvalidInput
	}
	return *target, nil
}

func parsePageQuery(request *http.Request) (artifact.PageQuery, error) {
	limit := 20
	values := request.URL.Query()
	for key, entries := range values {
		if (key != "limit" && key != "cursor" && key != "type") || len(entries) != 1 {
			return artifact.PageQuery{}, artifact.ErrInvalidInput
		}
	}
	if raw, exists := values["limit"]; exists {
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed < 1 || parsed > 100 {
			return artifact.PageQuery{}, artifact.ErrInvalidInput
		}
		limit = parsed
	}
	return artifact.PageQuery{
		Limit:      limit,
		Cursor:     values.Get("cursor"),
		TypeFilter: values.Get("type"),
	}, nil
}

func (controller *ArtifactController) CreateArtifact(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	if strings.TrimSpace(projectID) == "" {
		controller.mapError(response, artifact.ErrInvalidInput, "Project ID is required")
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		controller.mapError(response, artifact.ErrInvalidInput, "Idempotency-Key header is required")
		return
	}

	req, err := decodeBody[dto.CreateArtifactRequest](response, request)
	if err != nil {
		controller.mapError(response, err, "Invalid request body")
		return
	}

	result, err := controller.service.CreateArtifact(request.Context(), userID, projectID, req, idempotencyKey)
	if err != nil {
		controller.mapError(response, err, "Failed to create artifact")
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Artifact created successfully", result)
}

func (controller *ArtifactController) GetArtifact(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	artifactID := request.PathValue("artifactId")

	result, err := controller.service.GetArtifact(request.Context(), userID, projectID, artifactID)
	if err != nil {
		controller.mapError(response, err, "Failed to get artifact")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Artifact retrieved successfully", result)
}

func (controller *ArtifactController) ListArtifacts(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")

	query, err := parsePageQuery(request)
	if err != nil {
		controller.mapError(response, err, "Invalid pagination")
		return
	}

	result, err := controller.service.ListArtifacts(request.Context(), userID, projectID, query)
	if err != nil {
		controller.mapError(response, err, "Failed to list artifacts")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Artifacts retrieved successfully", result)
}

func (controller *ArtifactController) GetRevision(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	artifactID := request.PathValue("artifactId")
	revisionID := request.PathValue("revisionId")

	result, err := controller.service.GetRevision(request.Context(), userID, projectID, artifactID, revisionID)
	if err != nil {
		controller.mapError(response, err, "Failed to get revision")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Revision retrieved successfully", result)
}

func (controller *ArtifactController) ListRevisions(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	artifactID := request.PathValue("artifactId")

	query, err := parsePageQuery(request)
	if err != nil {
		controller.mapError(response, err, "Invalid pagination")
		return
	}

	result, err := controller.service.ListRevisions(request.Context(), userID, projectID, artifactID, query)
	if err != nil {
		controller.mapError(response, err, "Failed to list revisions")
		return
	}

	httpresponse.RespondJSON(response, http.StatusOK, "Revisions retrieved successfully", result)
}

func (controller *ArtifactController) AppendRevision(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	artifactID := request.PathValue("artifactId")
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		controller.mapError(response, artifact.ErrInvalidInput, "Idempotency-Key header is required")
		return
	}

	req, err := decodeBody[dto.AppendRevisionRequest](response, request)
	if err != nil {
		controller.mapError(response, err, "Invalid request body")
		return
	}

	result, err := controller.service.AppendRevision(request.Context(), userID, projectID, artifactID, req, idempotencyKey)
	if err != nil {
		controller.mapError(response, err, "Failed to append revision")
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Revision appended successfully", result)
}

func (controller *ArtifactController) RestoreRevision(response http.ResponseWriter, request *http.Request) {
	userID := getUserID(request)
	if userID == "" {
		controller.mapError(response, artifact.ErrUnauthenticated, "Authentication required")
		return
	}
	projectID := request.PathValue("projectId")
	artifactID := request.PathValue("artifactId")
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		controller.mapError(response, artifact.ErrInvalidInput, "Idempotency-Key header is required")
		return
	}

	req, err := decodeBody[dto.RestoreRevisionRequest](response, request)
	if err != nil {
		controller.mapError(response, err, "Invalid request body")
		return
	}

	result, err := controller.service.RestoreRevision(request.Context(), userID, projectID, artifactID, req, idempotencyKey)
	if err != nil {
		controller.mapError(response, err, "Failed to restore revision")
		return
	}

	httpresponse.RespondJSON(response, http.StatusCreated, "Revision restored successfully", result)
}

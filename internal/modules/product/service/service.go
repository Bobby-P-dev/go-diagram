package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/product/dto"
)

type Service struct {
	repo product.Repository
}

func NewService(repo product.Repository) *Service {
	return &Service{repo: repo}
}

func computeRequestHash(payload interface{}) string {
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateActor(userID string) error {
	if !product.ValidID(userID) {
		return product.ErrUnauthenticated
	}
	return nil
}

func validateID(value string) error {
	if !product.ValidID(value) {
		return fmt.Errorf("%w: malformed resource ID", product.ErrInvalidInput)
	}
	return nil
}

func validateText(value string, maximum int, required bool) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > maximum || (required && strings.TrimSpace(value) == "") {
		return fmt.Errorf("%w: text is empty, too long or invalid", product.ErrInvalidInput)
	}
	return nil
}

func validateKey(key string) error {
	if len(key) < 1 || len(key) > 128 {
		return fmt.Errorf("%w: Idempotency-Key must contain 1..128 printable ASCII characters", product.ErrInvalidInput)
	}
	for _, char := range key {
		if char < '!' || char > '~' {
			return fmt.Errorf("%w: invalid Idempotency-Key", product.ErrInvalidInput)
		}
	}
	return nil
}

func pageQuery(query product.PageQuery) (product.PageQuery, error) {
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Cursor) > 2048 {
		return query, product.ErrInvalidInput
	}
	return query, nil
}

func (service *Service) CreateProject(
	ctx context.Context,
	userID string,
	req dto.CreateProjectRequest,
	idempotencyKey string,
) (*product.Project, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if err := validateText(title, 255, true); err != nil {
		return nil, err
	}
	if err := validateText(req.Description, 10000, false); err != nil {
		return nil, err
	}
	if err := validateKey(idempotencyKey); err != nil {
		return nil, err
	}

	input := product.ProjectInput{
		Title:       title,
		Description: strings.TrimSpace(req.Description),
	}
	write := product.WriteRequest{
		Key:         strings.TrimSpace(idempotencyKey),
		RequestHash: computeRequestHash(input),
		Resource:    "/api/v2/projects",
	}

	return service.repo.CreateProject(ctx, userID, input, write)
}

func (service *Service) ListProjects(
	ctx context.Context,
	userID string,
	query product.PageQuery,
) (*product.PageResult[*product.Project], error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	query, err := pageQuery(query)
	if err != nil {
		return nil, err
	}
	return service.repo.ListProjects(ctx, userID, query)
}

func (service *Service) CreateThread(
	ctx context.Context,
	userID, projectID string,
	req dto.CreateThreadRequest,
	idempotencyKey string,
) (*product.ChatThread, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(projectID)
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if err := validateText(title, 255, true); err != nil {
		return nil, err
	}
	if err := validateKey(idempotencyKey); err != nil {
		return nil, err
	}

	modeHint := strings.TrimSpace(req.ModeHint)
	if modeHint == "" {
		modeHint = "general"
	}
	if err := validateText(modeHint, 50, true); err != nil {
		return nil, err
	}
	if req.ActiveArtifactID != nil {
		if err := validateID(*req.ActiveArtifactID); err != nil {
			return nil, err
		}
		artifactID := strings.ToLower(*req.ActiveArtifactID)
		req.ActiveArtifactID = &artifactID
	}

	input := product.ThreadInput{
		Title:            title,
		ModeHint:         modeHint,
		ActiveArtifactID: req.ActiveArtifactID,
	}
	write := product.WriteRequest{
		Key:         strings.TrimSpace(idempotencyKey),
		RequestHash: computeRequestHash(input),
		Resource:    fmt.Sprintf("/api/v2/projects/%s/threads", projectID),
	}

	return service.repo.CreateThread(ctx, userID, projectID, input, write)
}

func (service *Service) ListThreads(
	ctx context.Context,
	userID, projectID string,
	query product.PageQuery,
) (*product.PageResult[*product.ChatThread], error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID = strings.ToLower(projectID)
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	query, err := pageQuery(query)
	if err != nil {
		return nil, err
	}
	return service.repo.ListThreads(ctx, userID, projectID, query)
}

func (service *Service) PostUserMessage(
	ctx context.Context,
	userID, projectID, threadID string,
	req dto.PostMessageRequest,
	idempotencyKey string,
) (*product.ChatMessage, error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID, threadID = strings.ToLower(projectID), strings.ToLower(threadID)
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(threadID); err != nil {
		return nil, err
	}
	if err := validateText(req.Content, 64000, true); err != nil {
		return nil, err
	}
	if err := validateText(req.ClientMessageID, 128, true); err != nil {
		return nil, err
	}
	if err := validateKey(idempotencyKey); err != nil {
		return nil, err
	}

	input := product.MessageInput{
		Content:         req.Content,
		ClientMessageID: req.ClientMessageID,
	}
	write := product.WriteRequest{
		Key:         strings.TrimSpace(idempotencyKey),
		RequestHash: computeRequestHash(input),
		Resource:    fmt.Sprintf("/api/v2/projects/%s/threads/%s/messages", projectID, threadID),
	}

	return service.repo.PostUserMessage(ctx, userID, projectID, threadID, input, write)
}

func (service *Service) ListMessages(
	ctx context.Context,
	userID, projectID, threadID string,
	query product.PageQuery,
) (*product.PageResult[*product.ChatMessage], error) {
	if err := validateActor(userID); err != nil {
		return nil, err
	}
	projectID, threadID = strings.ToLower(projectID), strings.ToLower(threadID)
	if err := validateID(projectID); err != nil {
		return nil, err
	}
	if err := validateID(threadID); err != nil {
		return nil, err
	}
	query, err := pageQuery(query)
	if err != nil {
		return nil, err
	}
	return service.repo.ListMessages(ctx, userID, projectID, threadID, query)
}

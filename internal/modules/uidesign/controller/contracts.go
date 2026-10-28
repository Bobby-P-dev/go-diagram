package controller

import (
	"context"
	"net/http"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity/authcontext"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type UIDesignService interface {
	GetTemplates(ctx context.Context) ([]uidesign.UITemplate, error)
	GenerateUIDesign(ctx context.Context, req dto.CreateUIDesignRequest) (*dto.ProjectResponse, error)
	GenerateUIDesignStream(ctx context.Context, req dto.CreateUIDesignRequest, onEvent func(eventType string, data any)) (*dto.ProjectResponse, error)
	IterateUIDesignWithTargetedChat(ctx context.Context, projectID string, req *dto.ChatRequest) (*dto.ProjectResponse, error)
}

type AsyncJobDispatcher interface {
	DispatchUIDesignJob(ctx context.Context, req dto.CreateUIDesignRequest, userID string) (*dto.AsyncJobInitResponse, error)
	GetJobStatus(ctx context.Context, jobID, userID string) (*dto.JobStatusResponse, error)
	ProcessJobCallback(ctx context.Context, req dto.JobCallbackRequest) error
}

type AuthenticatedUser struct {
	ID            string
	CanGenerateUI bool
}

type UserReader func(context.Context) *AuthenticatedUser

type KeyValidator func(*http.Request) bool

func DefaultUserReader(ctx context.Context) *AuthenticatedUser {
	if ctx == nil {
		return nil
	}
	user := authcontext.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	return &AuthenticatedUser{
		ID:            user.ID,
		CanGenerateUI: user.CanGenerateUI,
	}
}

func readUser(reader UserReader, ctx context.Context) *AuthenticatedUser {
	if reader == nil {
		return DefaultUserReader(ctx)
	}
	return reader(ctx)
}

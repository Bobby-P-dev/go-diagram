package service

import (
	"context"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type UIDesignServiceInterface interface {
	GenerateUIDesign(ctx context.Context, req dtos.CreateUIDesignRequest) (*dtos.ProjectResponse, error)
	GenerateUIDesignStream(ctx context.Context, req dtos.CreateUIDesignRequest, onEvent func(eventType string, data any)) (*dtos.ProjectResponse, error)
	IterateUIDesignWithChat(ctx context.Context, projectID, prompt string, targetedNodeIDs []string) (*dtos.ProjectResponse, error)
	IterateUIDesignWithTargetedChat(ctx context.Context, projectID string, req *dtos.ChatRequest) (*dtos.ProjectResponse, error)
}

type uiDesignService struct {
	projects        uidesign.ProjectGateway
	aiService       LLMClient
	compiler        *UIDesignCompiler
	changeAnalyzer  *ChangeAnalyzer
	patchEngine     *PatchEngine
	targetedPatcher *TargetedPatcher
	userID          UserIDReader
}

func NewUIDesignService(projects uidesign.ProjectGateway, ai LLMClient, userID UserIDReader) UIDesignServiceInterface {
	if userID == nil {
		userID = func(context.Context) string { return "" }
	}
	return &uiDesignService{
		projects: projects, aiService: ai,
		compiler: NewUIDesignCompiler(ai), changeAnalyzer: NewChangeAnalyzer(ai),
		patchEngine: NewPatchEngine(), targetedPatcher: NewTargetedPatcher(ai), userID: userID,
	}
}

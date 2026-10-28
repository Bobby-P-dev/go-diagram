package projectgateway

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	diagramrepository "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/repository"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/identity"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
)

type ProjectStore interface {
	CreateWithUser(string, string, string, string, json.RawMessage, json.RawMessage, json.RawMessage) (*diagram.Project, error)
	GetByIDScoped(string, string, string) (*diagram.Project, []diagram.ChatMessage, error)
	UpdateGraph(string, json.RawMessage, json.RawMessage) error
}

type MessageStore interface {
	AppendMessage(string, string, string, json.RawMessage) (*diagram.ChatMessage, error)
}

type VersionStore interface {
	CreateVersion(string, int, string, json.RawMessage, json.RawMessage, *string) (*diagram.DiagramVersion, error)
	GetLatestVersionNumber(string) (int, error)
}

type UIDesignProjectGateway struct {
	projectModel ProjectStore
	messageModel MessageStore
	versionModel VersionStore
}

func NewUIDesignProjectGateway(
	projectModel ProjectStore,
	messageModel MessageStore,
	versionModel VersionStore,
) uidesign.ProjectGateway {
	return &UIDesignProjectGateway{
		projectModel: projectModel,
		messageModel: messageModel,
		versionModel: versionModel,
	}
}

func NewSQLUIDesignProjectGateway(db *sql.DB) uidesign.ProjectGateway {
	return NewUIDesignProjectGateway(
		diagramrepository.NewProjectRepository(db, identity.MasterAdminID),
		diagramrepository.NewMessageRepository(db),
		diagramrepository.NewVersionRepository(db),
	)
}

func NewUIDesignProjectAccessReader(model ProjectStore) uidesign.ProjectAccessReader {
	return NewUIDesignProjectGateway(model, nil, nil)
}

func NewSQLUIDesignProjectAccessReader(db *sql.DB) uidesign.ProjectAccessReader {
	return NewSQLUIDesignProjectGateway(db)
}

func (g *UIDesignProjectGateway) CheckProjectAccess(ctx context.Context, projectID, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if g.projectModel == nil {
		return sql.ErrNoRows
	}
	if model, ok := g.projectModel.(interface {
		GetByIDScopedContext(context.Context, string, string, string) (*diagram.Project, []diagram.ChatMessage, error)
	}); ok {
		_, _, err := model.GetByIDScopedContext(ctx, projectID, userID, "")
		return err
	}
	_, _, err := g.projectModel.GetByIDScoped(projectID, userID, "")
	return err
}

func (g *UIDesignProjectGateway) CreateProject(ctx context.Context, input uidesign.CreateProjectInput) (*uidesign.Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	create := func() (*diagram.Project, error) {
		return g.projectModel.CreateWithUser(
			input.UserID,
			input.Title,
			input.DiagramType,
			input.ProjectMode,
			input.Metadata,
			input.Nodes,
			input.Edges,
		)
	}
	if model, ok := g.projectModel.(interface {
		CreateWithUserContext(context.Context, string, string, string, string, json.RawMessage, json.RawMessage, json.RawMessage) (*diagram.Project, error)
	}); ok {
		create = func() (*diagram.Project, error) {
			return model.CreateWithUserContext(ctx, input.UserID, input.Title, input.DiagramType, input.ProjectMode, input.Metadata, input.Nodes, input.Edges)
		}
	}
	p, err := create()
	if err != nil {
		return nil, err
	}
	return mapProjectToUIDesign(p), nil
}

func (g *UIDesignProjectGateway) GetProjectScoped(ctx context.Context, id, userID, userRole string) (*uidesign.Project, []uidesign.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	var p *diagram.Project
	var msgs []diagram.ChatMessage
	var err error

	if model, ok := g.projectModel.(interface {
		GetByIDScopedContext(context.Context, string, string, string) (*diagram.Project, []diagram.ChatMessage, error)
	}); ok {
		p, msgs, err = model.GetByIDScopedContext(ctx, id, userID, userRole)
	} else {
		p, msgs, err = g.projectModel.GetByIDScoped(id, userID, userRole)
	}
	if err != nil {
		return nil, nil, err
	}

	var mappedMsgs []uidesign.ChatMessage
	for _, m := range msgs {
		mappedMsgs = append(mappedMsgs, uidesign.ChatMessage{
			ID:            m.ID,
			ProjectID:     m.ProjectID,
			Role:          m.Role,
			Content:       m.Content,
			TargetNodeIDs: m.TargetNodeIDs,
			CreatedAt:     m.CreatedAt,
		})
	}

	return mapProjectToUIDesign(p), mappedMsgs, nil
}

func (g *UIDesignProjectGateway) UpdateGraph(ctx context.Context, id string, nodes, edges json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if model, ok := g.projectModel.(interface {
		UpdateGraphContext(context.Context, string, json.RawMessage, json.RawMessage) error
	}); ok {
		return model.UpdateGraphContext(ctx, id, nodes, edges)
	}
	return g.projectModel.UpdateGraph(id, nodes, edges)
}

func (g *UIDesignProjectGateway) CreateVersion(ctx context.Context, input uidesign.CreateVersionInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if g.versionModel == nil {
		return nil
	}
	if model, ok := g.versionModel.(interface {
		CreateVersionContext(context.Context, string, int, string, json.RawMessage, json.RawMessage, *string) (*diagram.DiagramVersion, error)
	}); ok {
		_, err := model.CreateVersionContext(ctx, input.ProjectID, input.VersionNumber, input.ChangeSummary, input.Nodes, input.Edges, input.TriggerMessageID)
		return err
	}
	_, err := g.versionModel.CreateVersion(
		input.ProjectID,
		input.VersionNumber,
		input.ChangeSummary,
		input.Nodes,
		input.Edges,
		input.TriggerMessageID,
	)
	return err
}

func (g *UIDesignProjectGateway) GetLatestVersionNumber(ctx context.Context, projectID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if g.versionModel == nil {
		return 0, nil
	}
	if model, ok := g.versionModel.(interface {
		GetLatestVersionNumberContext(context.Context, string) (int, error)
	}); ok {
		return model.GetLatestVersionNumberContext(ctx, projectID)
	}
	return g.versionModel.GetLatestVersionNumber(projectID)
}

func (g *UIDesignProjectGateway) AppendMessage(ctx context.Context, projectID, role, content string, targetNodeIDs json.RawMessage) (*uidesign.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g.messageModel == nil {
		return nil, nil
	}
	appendMessage := func() (*diagram.ChatMessage, error) {
		return g.messageModel.AppendMessage(projectID, role, content, targetNodeIDs)
	}
	if model, ok := g.messageModel.(interface {
		AppendMessageContext(context.Context, string, string, string, json.RawMessage) (*diagram.ChatMessage, error)
	}); ok {
		appendMessage = func() (*diagram.ChatMessage, error) {
			return model.AppendMessageContext(ctx, projectID, role, content, targetNodeIDs)
		}
	}
	msg, err := appendMessage()
	if err != nil {
		return nil, err
	}
	return &uidesign.ChatMessage{
		ID:            msg.ID,
		ProjectID:     msg.ProjectID,
		Role:          msg.Role,
		Content:       msg.Content,
		TargetNodeIDs: msg.TargetNodeIDs,
		CreatedAt:     msg.CreatedAt,
	}, nil
}

func mapProjectToUIDesign(p *diagram.Project) *uidesign.Project {
	if p == nil {
		return nil
	}
	return &uidesign.Project{
		ID:           p.ID,
		UserID:       p.UserID,
		Title:        p.Title,
		DiagramType:  p.DiagramType,
		ProjectMode:  p.ProjectMode,
		Metadata:     p.Metadata,
		IsPinned:     p.IsPinned,
		CurrentNodes: p.CurrentNodes,
		CurrentEdges: p.CurrentEdges,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

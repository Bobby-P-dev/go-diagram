package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	uidesignservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
)

type inMemoryProjectGateway struct {
	projects map[string]*uidesign.Project
	messages map[string][]uidesign.ChatMessage
	versions map[string]int
}

func newInMemoryProjectGateway() *inMemoryProjectGateway {
	return &inMemoryProjectGateway{
		projects: make(map[string]*uidesign.Project),
		messages: make(map[string][]uidesign.ChatMessage),
		versions: make(map[string]int),
	}
}

func (g *inMemoryProjectGateway) CheckProjectAccess(ctx context.Context, projectID, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, ok := g.projects[projectID]
	if !ok {
		return errors.New("project not found")
	}
	if userID != "" && p.UserID != userID {
		return errors.New("unauthorized")
	}
	return nil
}

func (g *inMemoryProjectGateway) CreateProject(ctx context.Context, input uidesign.CreateProjectInput) (*uidesign.Project, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id := "proj-1"
	p := &uidesign.Project{
		ID:           id,
		UserID:       input.UserID,
		Title:        input.Title,
		DiagramType:  input.DiagramType,
		ProjectMode:  input.ProjectMode,
		Metadata:     input.Metadata,
		CurrentNodes: input.Nodes,
		CurrentEdges: input.Edges,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	g.projects[id] = p
	return p, nil
}

func (g *inMemoryProjectGateway) GetProjectScoped(ctx context.Context, id, userID, userRole string) (*uidesign.Project, []uidesign.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	p, ok := g.projects[id]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	if userID != "" && p.UserID != userID {
		return nil, nil, errors.New("unauthorized")
	}
	return p, g.messages[id], nil
}

func (g *inMemoryProjectGateway) UpdateGraph(ctx context.Context, id string, nodes, edges json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, ok := g.projects[id]
	if !ok {
		return errors.New("not found")
	}
	p.CurrentNodes = nodes
	p.CurrentEdges = edges
	return nil
}

func (g *inMemoryProjectGateway) CreateVersion(ctx context.Context, input uidesign.CreateVersionInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.versions[input.ProjectID] = input.VersionNumber
	return nil
}

func (g *inMemoryProjectGateway) GetLatestVersionNumber(ctx context.Context, projectID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return g.versions[projectID], nil
}

func (g *inMemoryProjectGateway) AppendMessage(ctx context.Context, projectID, role, content string, targetNodeIDs json.RawMessage) (*uidesign.ChatMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	msg := uidesign.ChatMessage{
		ID:            "msg-1",
		ProjectID:     projectID,
		Role:          role,
		Content:       content,
		TargetNodeIDs: targetNodeIDs,
		CreatedAt:     time.Now(),
	}
	g.messages[projectID] = append(g.messages[projectID], msg)
	return &msg, nil
}

type fakeTemplateRepo struct {
	templates []uidesign.UITemplate
}

func (r *fakeTemplateRepo) GetAllUITemplates(ctx context.Context) ([]uidesign.UITemplate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.templates, nil
}

func (r *fakeTemplateRepo) GetUITemplateByID(ctx context.Context, id string) (*uidesign.UITemplate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, tpl := range r.templates {
		if tpl.ID == id {
			return &tpl, nil
		}
	}
	return nil, errors.New("ui template not found")
}

type fakeAIClient struct {
	response string
}

func (c *fakeAIClient) CallLLM(systemPrompt, prompt string) (string, error) {
	return c.response, nil
}
func (c *fakeAIClient) CallLLMText(systemPrompt, prompt string) (string, error) {
	return c.response, nil
}
func (c *fakeAIClient) CallLLMStream(systemPrompt, prompt string, onChunk uidesignservice.LLMStreamCallback) (string, error) {
	return c.response, nil
}
func (c *fakeAIClient) CallLLMTextStream(systemPrompt, prompt string, onChunk uidesignservice.LLMStreamCallback) (string, error) {
	return c.response, nil
}
func (c *fakeAIClient) SanitizeJSON(raw string) string {
	return raw
}

func TestUIDesignService_GetTemplates(t *testing.T) {
	repo := &fakeTemplateRepo{
		templates: []uidesign.UITemplate{
			{ID: "tpl-1", Name: "Admin Dashboard"},
		},
	}
	gw := newInMemoryProjectGateway()
	svc := uidesignservice.NewUIDesignService(gw, repo, nil, nil)

	t.Run("Context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := svc.GetTemplates(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		tpls, err := svc.GetTemplates(context.Background())
		if err != nil || len(tpls) != 1 {
			t.Fatalf("expected 1 template, got %v, len %d", err, len(tpls))
		}
	})
}

func TestUIDesignService_CreateProjectFromTemplate(t *testing.T) {
	repo := &fakeTemplateRepo{
		templates: []uidesign.UITemplate{
			{
				ID:         "tpl-1",
				Name:       "Mobile App",
				Device:     "mobile",
				Theme:      json.RawMessage(`{"accent":"#10b981"}`),
				Sections:   json.RawMessage(`[]`),
				CodeExport: json.RawMessage(`{}`),
			},
		},
	}
	gw := newInMemoryProjectGateway()
	userReader := func(ctx context.Context) string { return "user-123" }
	svc := uidesignservice.NewUIDesignService(gw, repo, nil, userReader)

	resp, err := svc.CreateProjectFromTemplate(context.Background(), "tpl-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "proj-1" || resp.Title != "Mobile App" {
		t.Fatalf("unexpected project response: %+v", resp)
	}

	// Verify project recorded in gateway
	p := gw.projects["proj-1"]
	if p == nil || p.UserID != "user-123" {
		t.Fatalf("expected project with user-123, got %+v", p)
	}
	if gw.versions["proj-1"] != 1 {
		t.Fatalf("expected version 1, got %d", gw.versions["proj-1"])
	}
	if len(gw.messages["proj-1"]) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(gw.messages["proj-1"]))
	}
}

func TestUIDesignService_GenerateUIDesign_BlankCanvas(t *testing.T) {
	gw := newInMemoryProjectGateway()
	svc := uidesignservice.NewUIDesignService(gw, nil, nil, func(ctx context.Context) string { return "user-123" })

	events := make([]string, 0)
	resp, err := svc.GenerateUIDesignStream(context.Background(), dtos.CreateUIDesignRequest{
		Prompt: "", // Blank canvas
	}, func(eventType string, data any) {
		events = append(events, eventType)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Title != "Untitled UI Design" {
		t.Fatalf("expected Untitled UI Design, got %s", resp.Title)
	}
	if len(events) != 1 || events[0] != "complete" {
		t.Fatalf("expected complete event, got %v", events)
	}
}

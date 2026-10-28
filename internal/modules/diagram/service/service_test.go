package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram"
	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
)

type memoryProjectRepo struct {
	mu       sync.Mutex
	projects map[string]*diagram.Project
	messages map[string][]diagram.ChatMessage
	err      error
}

func newMemoryProjectRepo() *memoryProjectRepo {
	return &memoryProjectRepo{
		projects: make(map[string]*diagram.Project),
		messages: make(map[string][]diagram.ChatMessage),
	}
}

func (repository *memoryProjectRepo) CreateWithUser(userID, title, diagramType, projectMode string, metadata, nodes, edges json.RawMessage) (*diagram.Project, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.err != nil {
		return nil, repository.err
	}
	id := fmt.Sprintf("p-%d", len(repository.projects)+1)
	project := &diagram.Project{
		ID:           id,
		UserID:       userID,
		Title:        title,
		DiagramType:  diagramType,
		ProjectMode:  projectMode,
		Metadata:     metadata,
		CurrentNodes: nodes,
		CurrentEdges: edges,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repository.projects[id] = project
	return project, nil
}

func (repository *memoryProjectRepo) GetByIDScoped(id, userID, userRole string) (*diagram.Project, []diagram.ChatMessage, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	project, ok := repository.projects[id]
	if !ok {
		return nil, nil, fmt.Errorf("project not found: %s", id)
	}
	if userID != "" && project.UserID != userID && userRole != "admin" {
		return nil, nil, fmt.Errorf("unauthorized")
	}
	return project, repository.messages[id], nil
}

func (repository *memoryProjectRepo) UpdateGraph(id string, nodes, edges json.RawMessage) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	project, ok := repository.projects[id]
	if !ok {
		return fmt.Errorf("project not found: %s", id)
	}
	project.CurrentNodes = nodes
	project.CurrentEdges = edges
	project.UpdatedAt = time.Now()
	return nil
}

func (repository *memoryProjectRepo) GetPaginatedScoped(userID string, limit, offset int) ([]diagram.ProjectSummary, int, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.err != nil {
		return nil, 0, repository.err
	}
	var all []diagram.ProjectSummary
	for _, project := range repository.projects {
		if userID == "" || project.UserID == userID {
			all = append(all, diagram.ProjectSummary{
				ID:          project.ID,
				UserID:      project.UserID,
				Title:       project.Title,
				DiagramType: project.DiagramType,
				ProjectMode: project.ProjectMode,
				IsPinned:    project.IsPinned,
				CreatedAt:   project.CreatedAt,
				UpdatedAt:   project.UpdatedAt,
			})
		}
	}
	total := len(all)
	if offset >= total {
		return []diagram.ProjectSummary{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (repository *memoryProjectRepo) TogglePin(id string) (bool, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	project, ok := repository.projects[id]
	if !ok {
		return false, fmt.Errorf("project not found: %s", id)
	}
	project.IsPinned = !project.IsPinned
	return project.IsPinned, nil
}

type memoryMessageRepo struct {
	mu          sync.Mutex
	projectRepo *memoryProjectRepo
	appendErr   error
}

func (repository *memoryMessageRepo) AppendMessage(projectID, role, content string, targetNodeIDs json.RawMessage) (*diagram.ChatMessage, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.appendErr != nil {
		return nil, repository.appendErr
	}
	msg := diagram.ChatMessage{
		ID:            fmt.Sprintf("m-%d", time.Now().UnixNano()),
		ProjectID:     projectID,
		Role:          role,
		Content:       content,
		TargetNodeIDs: targetNodeIDs,
		CreatedAt:     time.Now(),
	}
	repository.projectRepo.mu.Lock()
	repository.projectRepo.messages[projectID] = append(repository.projectRepo.messages[projectID], msg)
	repository.projectRepo.mu.Unlock()
	return &msg, nil
}

type memoryVersionRepo struct {
	mu       sync.Mutex
	versions map[string][]diagram.DiagramVersion
	err      error
}

func newMemoryVersionRepo() *memoryVersionRepo {
	return &memoryVersionRepo{
		versions: make(map[string][]diagram.DiagramVersion),
	}
}

func (repository *memoryVersionRepo) CreateVersion(projectID string, versionNumber int, summary string, nodes, edges json.RawMessage, triggerMessageID *string) (*diagram.DiagramVersion, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.err != nil {
		return nil, repository.err
	}
	version := diagram.DiagramVersion{
		ID:               fmt.Sprintf("v-%s-%d", projectID, versionNumber),
		ProjectID:        projectID,
		VersionNumber:    versionNumber,
		ChangeSummary:    summary,
		Nodes:            nodes,
		Edges:            edges,
		TriggerMessageID: triggerMessageID,
		CreatedAt:        time.Now(),
	}
	repository.versions[projectID] = append(repository.versions[projectID], version)
	return &version, nil
}

func (repository *memoryVersionRepo) GetLatestVersionNumber(projectID string) (int, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	list := repository.versions[projectID]
	if len(list) == 0 {
		return 0, nil
	}
	return list[len(list)-1].VersionNumber, nil
}

func (repository *memoryVersionRepo) GetVersionByID(id string) (*diagram.DiagramVersion, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	for _, list := range repository.versions {
		for _, version := range list {
			if version.ID == id {
				return &version, nil
			}
		}
	}
	return nil, fmt.Errorf("version not found: %s", id)
}

func (repository *memoryVersionRepo) GetVersionsByProjectID(projectID string) ([]diagram.DiagramVersion, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.err != nil {
		return nil, repository.err
	}
	return repository.versions[projectID], nil
}

type memoryTemplateRepo struct {
	templates map[string]diagram.DiagramTemplate
	err       error
}

func newMemoryTemplateRepo() *memoryTemplateRepo {
	return &memoryTemplateRepo{
		templates: make(map[string]diagram.DiagramTemplate),
	}
}

func (repository *memoryTemplateRepo) GetTemplateByID(id string) (*diagram.DiagramTemplate, error) {
	if repository.err != nil {
		return nil, repository.err
	}
	test, ok := repository.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return &test, nil
}

func (repository *memoryTemplateRepo) GetAllTemplates() ([]diagram.DiagramTemplate, error) {
	if repository.err != nil {
		return nil, repository.err
	}
	var res []diagram.DiagramTemplate
	for _, test := range repository.templates {
		res = append(res, test)
	}
	return res, nil
}

type fakeAIClient struct {
	mu           sync.Mutex
	calls        int
	lastHistory  []diagram.ChatMessage
	lastGraph    string
	lastPrompt   string
	lastType     string
	lastTargeted []string
	response     *dto.GraphPayload
	reply        string
	err          error
}

func (client *fakeAIClient) GenerateDiagram(history []diagram.ChatMessage, currentGraph, prompt, diagramType string, targetedNodeIDs []string) (*dto.GraphPayload, string, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.calls++
	client.lastHistory = history
	client.lastGraph = currentGraph
	client.lastPrompt = prompt
	client.lastType = diagramType
	client.lastTargeted = targetedNodeIDs
	if client.err != nil {
		return nil, "", client.err
	}
	return client.response, client.reply, nil
}

func (client *fakeAIClient) Provider() string { return "fake-provider" }
func (client *fakeAIClient) Model() string    { return "fake-model-turbo" }

type fakeUsageRecorder struct {
	mu      sync.Mutex
	entries chan aiusage.Entry
}

func (client *fakeUsageRecorder) LogAIUsage(ctx context.Context, entry aiusage.Entry) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.entries <- entry
	return nil
}

func TestDiagramService_CreateNewProject(test *testing.T) {
	test.Run("Create blank project default", func(test *testing.T) {
		pRepo := newMemoryProjectRepo()
		mRepo := &memoryMessageRepo{projectRepo: pRepo}
		vRepo := newMemoryVersionRepo()

		svc := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, nil)
		res, err := svc.CreateNewProject(dto.CreateProjectRequest{
			Prompt:      "",
			DiagramType: "flowchart",
		})
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.Title != "Untitled Project" {
			test.Errorf("expected title 'Untitled Project', got %q", res.Title)
		}
		if res.DiagramType != "flowchart" {
			test.Errorf("expected diagram_type flowchart, got %q", res.DiagramType)
		}
		if len(res.Messages) != 1 {
			test.Fatalf("expected 1 assistant message, got %d", len(res.Messages))
		}
		if !strings.Contains(res.Messages[0].Content, "Kanvas kosong telah siap") {
			test.Errorf("expected blank welcome message, got %q", res.Messages[0].Content)
		}
		vers, _ := vRepo.GetVersionsByProjectID(res.ID)
		if len(vers) != 1 || vers[0].VersionNumber != 1 {
			test.Errorf("expected 1 initial version, got %v", vers)
		}
	})

	test.Run("Create blank project ui_design mode", func(test *testing.T) {
		pRepo := newMemoryProjectRepo()
		mRepo := &memoryMessageRepo{projectRepo: pRepo}
		vRepo := newMemoryVersionRepo()

		svc := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, nil)
		res, err := svc.CreateNewProject(dto.CreateProjectRequest{
			Prompt:      "",
			DiagramType: "ui_design",
		})
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.Title != "Untitled UI Canvas" {
			test.Errorf("expected 'Untitled UI Canvas', got %q", res.Title)
		}
	})

	test.Run("Create from template", func(test *testing.T) {
		pRepo := newMemoryProjectRepo()
		mRepo := &memoryMessageRepo{projectRepo: pRepo}
		vRepo := newMemoryVersionRepo()
		tRepo := newMemoryTemplateRepo()
		tRepo.templates["tpl-1"] = diagram.DiagramTemplate{
			ID:          "tpl-1",
			Name:        "Microservice Template",
			DiagramType: "architecture",
			Nodes:       json.RawMessage(`[{"id":"gw"}]`),
			Edges:       json.RawMessage(`[]`),
		}

		svc := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, tRepo)
		res, err := svc.CreateNewProject(dto.CreateProjectRequest{
			TemplateID: "tpl-1",
		})
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if res.Title != "Microservice Template" {
			test.Errorf("expected template title, got %q", res.Title)
		}
		if !strings.Contains(string(res.CurrentNodes), "gw") {
			test.Errorf("expected template nodes, got %s", string(res.CurrentNodes))
		}
		if len(res.Messages) != 2 {
			test.Fatalf("expected 2 messages (user + assistant), got %d", len(res.Messages))
		}
	})

	test.Run("Create with AI generation and telemetry", func(test *testing.T) {
		pRepo := newMemoryProjectRepo()
		mRepo := &memoryMessageRepo{projectRepo: pRepo}
		vRepo := newMemoryVersionRepo()
		recorder := &fakeUsageRecorder{entries: make(chan aiusage.Entry, 1)}
		ai := &fakeAIClient{
			response: &dto.GraphPayload{
				Nodes: []dto.GraphNode{{ID: "start", Type: "input"}},
				Edges: []dto.GraphEdge{},
			},
			reply: "Diagram alur pembayaran berhasil dibuat",
		}

		svc := NewDiagramService(ai, pRepo, mRepo, vRepo, recorder, nil)
		res, err := svc.CreateNewProjectForUser("user-alpha", dto.CreateProjectRequest{
			Prompt:      "Buat diagram use case pendaftaran mahasiswa baru di kampus",
			DiagramType: "",
		})
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}

		if ai.calls != 1 {
			test.Fatalf("expected 1 AI call, got %d", ai.calls)
		}
		if ai.lastType != "usecase" {
			test.Errorf("expected auto-detected type 'usecase', got %q", ai.lastType)
		}
		if !strings.Contains(res.Title, "Buat diagram use case pendaftaran") {
			test.Errorf("unexpected generated title: %q", res.Title)
		}
		if !strings.Contains(string(res.CurrentNodes), "start") {
			test.Errorf("expected node 'start' in current nodes, got %s", string(res.CurrentNodes))
		}
		if len(res.Messages) != 2 {
			test.Fatalf("expected 2 messages, got %d", len(res.Messages))
		}
		select {
		case entry := <-recorder.entries:
			if entry.Provider != "fake-provider" || entry.Model != "fake-model-turbo" || entry.ProjectID == nil || *entry.ProjectID != res.ID {
				test.Errorf("unexpected telemetry payload: %+v", entry)
			}
		case <-time.After(time.Second):
			test.Fatal("telemetry was not recorded")
		}
	})

	test.Run("AI failure returns error", func(test *testing.T) {
		pRepo := newMemoryProjectRepo()
		mRepo := &memoryMessageRepo{projectRepo: pRepo}
		vRepo := newMemoryVersionRepo()
		ai := &fakeAIClient{
			err: errors.New("rate limit exceeded"),
		}

		svc := NewDiagramService(ai, pRepo, mRepo, vRepo, nil, nil)
		_, err := svc.CreateNewProject(dto.CreateProjectRequest{
			Prompt: "bikin diagram",
		})
		if err == nil {
			test.Fatalf("expected error from failed AI generation, got nil")
		}
	})
}

func TestDiagramService_SendChatMessage(test *testing.T) {
	pRepo := newMemoryProjectRepo()
	mRepo := &memoryMessageRepo{projectRepo: pRepo}
	vRepo := newMemoryVersionRepo()
	recorder := &fakeUsageRecorder{entries: make(chan aiusage.Entry, 1)}
	ai := &fakeAIClient{
		response: &dto.GraphPayload{
			Nodes: []dto.GraphNode{{ID: "step-1"}, {ID: "step-2"}},
			Edges: []dto.GraphEdge{{ID: "e1", Source: "step-1", Target: "step-2"}},
		},
		reply: "Node pembayaran ditambahkan.",
	}

	svc := NewDiagramService(ai, pRepo, mRepo, vRepo, recorder, nil)
	proj, _ := pRepo.CreateWithUser("user-1", "Alur Checkout", "flowchart", "diagram", nil, json.RawMessage(`[{"id":"step-1"}]`), json.RawMessage("[]"))
	_, _ = mRepo.AppendMessage(proj.ID, "assistant", "Initial conversation", nil)
	_, _ = vRepo.CreateVersion(proj.ID, 1, "Initial", proj.CurrentNodes, proj.CurrentEdges, nil)

	test.Run("Empty prompt returns error", func(test *testing.T) {
		_, err := svc.SendChatMessage(proj.ID, dto.ChatRequest{Prompt: "   "})
		if err == nil {
			test.Errorf("expected error on empty prompt")
		}
	})

	test.Run("Empty projectID returns error", func(test *testing.T) {
		_, err := svc.SendChatMessage("", dto.ChatRequest{Prompt: "tambah node"})
		if err == nil {
			test.Errorf("expected error on empty project id")
		}
	})

	test.Run("Unauthorized user returns error", func(test *testing.T) {
		_, err := svc.SendChatMessageScoped(proj.ID, dto.ChatRequest{Prompt: "tambah node"}, "user-stranger")
		if err == nil {
			test.Errorf("expected authorization error")
		}
		if ai.calls != 0 || len(pRepo.messages[proj.ID]) != 1 {
			test.Fatal("unauthorized chat must not call AI or append messages")
		}
	})

	test.Run("Authorized user targeted chat increments version", func(test *testing.T) {
		res, err := svc.SendChatMessageScoped(proj.ID, dto.ChatRequest{
			Prompt:          "hubungkan ke node konfirmasi dengan use case flow",
			TargetedNodeIDs: []string{"step-1"},
		}, "user-1")
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}

		if ai.calls != 1 {
			test.Fatalf("expected 1 call to AI, got %d", ai.calls)
		}
		if len(ai.lastTargeted) != 1 || ai.lastTargeted[0] != "step-1" {
			test.Errorf("expected targeted node step-1, got %v", ai.lastTargeted)
		}
		if ai.lastType != "usecase" {
			test.Errorf("expected effective diagram type 'usecase', got %q", ai.lastType)
		}
		if len(ai.lastHistory) != 1 || ai.lastHistory[0].Content != "Initial conversation" {
			test.Fatalf("AI should receive previous history, got %+v", ai.lastHistory)
		}
		if !strings.Contains(ai.lastGraph, `"step-1"`) || strings.Contains(ai.lastGraph, `"step-2"`) {
			test.Fatalf("AI should receive graph before mutation, got %s", ai.lastGraph)
		}
		if string(pRepo.messages[proj.ID][1].TargetNodeIDs) != `["step-1"]` {
			test.Fatal("user message should preserve targeted node IDs")
		}
		if !strings.Contains(string(res.CurrentNodes), "step-2") {
			test.Errorf("expected node step-2 in result, got %s", string(res.CurrentNodes))
		}

		versions, _ := vRepo.GetVersionsByProjectID(proj.ID)
		if len(versions) != 2 {
			test.Fatalf("expected 2 versions, got %d", len(versions))
		}
		if versions[1].VersionNumber != 2 {
			test.Errorf("expected version number 2, got %d", versions[1].VersionNumber)
		}
		if versions[1].TriggerMessageID == nil || *versions[1].TriggerMessageID != pRepo.messages[proj.ID][1].ID {
			test.Fatal("snapshot should reference the triggering user message")
		}
	})
}

func TestDiagramService_RollbackToVersion(test *testing.T) {
	pRepo := newMemoryProjectRepo()
	mRepo := &memoryMessageRepo{projectRepo: pRepo}
	vRepo := newMemoryVersionRepo()

	svc := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, nil)

	nodesV1 := json.RawMessage(`[{"id":"v1"}]`)
	nodesV2 := json.RawMessage(`[{"id":"v2"}]`)

	project, _ := pRepo.CreateWithUser("user-1", "Project Versi", "flowchart", "diagram", nil, nodesV2, json.RawMessage("[]"))
	v1, _ := vRepo.CreateVersion(project.ID, 1, "V1 Snapshot", nodesV1, json.RawMessage("[]"), nil)
	_, _ = vRepo.CreateVersion(project.ID, 2, "V2 Snapshot", nodesV2, json.RawMessage("[]"), nil)

	test.Run("Invalid parameters", func(test *testing.T) {
		if _, err := svc.RollbackToVersion("", v1.ID); err == nil {
			test.Errorf("expected error for empty project id")
		}
		if _, err := svc.RollbackToVersion(project.ID, ""); err == nil {
			test.Errorf("expected error for empty version id")
		}
	})

	test.Run("Unauthorized user rollback fails", func(test *testing.T) {
		_, err := svc.RollbackToVersionScoped(project.ID, v1.ID, "user-other")
		if err == nil {
			test.Errorf("expected error for unauthorized rollback")
		}
	})

	test.Run("Version mismatch error", func(test *testing.T) {
		otherProj, _ := pRepo.CreateWithUser("user-1", "Other", "flowchart", "diagram", nil, nodesV1, json.RawMessage("[]"))
		otherVer, _ := vRepo.CreateVersion(otherProj.ID, 1, "Other V1", nodesV1, json.RawMessage("[]"), nil)

		_, err := svc.RollbackToVersionScoped(project.ID, otherVer.ID, "user-1")
		if err == nil || !strings.Contains(err.Error(), "does not belong") {
			test.Errorf("expected mismatch error, got %v", err)
		}
	})

	test.Run("Successful rollback creates new snapshot version 3", func(test *testing.T) {
		res, err := svc.RollbackToVersionScoped(project.ID, v1.ID, "user-1")
		if err != nil {
			test.Fatalf("unexpected rollback error: %v", err)
		}
		if string(res.CurrentNodes) != string(nodesV1) {
			test.Errorf("expected current nodes restored to v1, got %s", string(res.CurrentNodes))
		}

		vers, _ := vRepo.GetVersionsByProjectID(project.ID)
		if len(vers) != 3 {
			test.Fatalf("expected 3 total versions, got %d", len(vers))
		}
		if vers[2].VersionNumber != 3 || !strings.Contains(vers[2].ChangeSummary, "Rollback ke Versi 1") {
			test.Errorf("expected Version 3 rollback summary, got %+v", vers[2])
		}
	})
}

func TestDiagramService_VersionsAndTemplates(test *testing.T) {
	pRepo := newMemoryProjectRepo()
	mRepo := &memoryMessageRepo{projectRepo: pRepo}
	vRepo := newMemoryVersionRepo()
	tRepo := newMemoryTemplateRepo()

	svc := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, tRepo)

	proj, _ := pRepo.CreateWithUser("user-1", "Test", "flowchart", "diagram", nil, json.RawMessage("[]"), json.RawMessage("[]"))
	_, _ = vRepo.CreateVersion(proj.ID, 1, "Init", json.RawMessage("[]"), json.RawMessage("[]"), nil)

	test.Run("GetProjectVersionsScoped", func(test *testing.T) {
		if _, err := svc.GetProjectVersionsScoped(proj.ID, "user-2"); err == nil {
			test.Errorf("expected unauthorized error")
		}
		list, err := svc.GetProjectVersionsScoped(proj.ID, "user-1")
		if err != nil || len(list) != 1 {
			test.Fatalf("expected 1 version, got %v (err: %v)", list, err)
		}
		if list[0].VersionNumber != 1 {
			test.Errorf("expected version number 1, got %d", list[0].VersionNumber)
		}
	})

	test.Run("ListTemplates", func(test *testing.T) {
		tRepo.templates["t1"] = diagram.DiagramTemplate{
			ID:          "t1",
			Name:        "Flowchart Template",
			Category:    "Process",
			DiagramType: "flowchart",
		}

		templates, err := svc.ListTemplates()
		if err != nil || len(templates) != 1 {
			test.Fatalf("expected 1 template, got %v (err: %v)", templates, err)
		}
		if templates[0].Name != "Flowchart Template" {
			test.Errorf("expected template name, got %s", templates[0].Name)
		}
	})
}

func TestDiagramService_PaginationAndPinning(test *testing.T) {
	pRepo := newMemoryProjectRepo()
	svc := NewDiagramService(nil, pRepo, nil, nil, nil, nil)

	p1, _ := pRepo.CreateWithUser("user-1", "Proj 1", "flowchart", "diagram", nil, json.RawMessage("[]"), json.RawMessage("[]"))
	_, _ = pRepo.CreateWithUser("user-1", "Proj 2", "flowchart", "diagram", nil, json.RawMessage("[]"), json.RawMessage("[]"))
	_, _ = pRepo.CreateWithUser("user-2", "Proj 3", "flowchart", "diagram", nil, json.RawMessage("[]"), json.RawMessage("[]"))

	test.Run("ListProjectsForUser pagination defaults", func(test *testing.T) {
		page, err := svc.ListProjectsForUser("user-1", 0, -1)
		if err != nil {
			test.Fatalf("unexpected error: %v", err)
		}
		if page.TotalCount != 2 || len(page.Items) != 2 {
			test.Errorf("expected 2 items for user-1, got total %d, items %d", page.TotalCount, len(page.Items))
		}
		if page.Limit != 15 || page.Offset != 0 {
			test.Errorf("expected defaults limit 15, offset 0, got limit %d, offset %d", page.Limit, page.Offset)
		}
	})

	test.Run("ToggleProjectPinScoped", func(test *testing.T) {
		if _, err := svc.ToggleProjectPinScoped(p1.ID, "user-2"); err == nil {
			test.Errorf("expected error when user-2 toggles user-1 pin")
		}
		res, err := svc.ToggleProjectPinScoped(p1.ID, "user-1")
		if err != nil || !res.IsPinned {
			test.Fatalf("expected pinned=true, got %+v (err: %v)", res, err)
		}
		res2, err := svc.ToggleProjectPinScoped(p1.ID, "user-1")
		if err != nil || res2.IsPinned {
			test.Fatalf("expected pinned=false, got %+v (err: %v)", res2, err)
		}
	})
}

func TestDiagramService_WrappersAndEdgeCases(test *testing.T) {
	pRepo := newMemoryProjectRepo()
	mRepo := &memoryMessageRepo{projectRepo: pRepo}
	vRepo := newMemoryVersionRepo()

	svc := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, nil)

	project, _ := pRepo.CreateWithUser("user-1", "Flow", "flowchart", "diagram", nil, json.RawMessage("[]"), json.RawMessage("[]"))
	v1, _ := vRepo.CreateVersion(project.ID, 1, "V1", json.RawMessage("[]"), json.RawMessage("[]"), nil)

	test.Run("generateTitle empty", func(test *testing.T) {
		if generateTitle("   ") != "Untitled Diagram" {
			test.Errorf("expected Untitled Diagram")
		}
	})

	test.Run("ListProjects wrapper", func(test *testing.T) {
		res, err := svc.ListProjects(10, 0)
		if err != nil || res.TotalCount != 1 {
			test.Errorf("unexpected list projects result: %+v (err: %v)", res, err)
		}
	})

	test.Run("ToggleProjectPin wrapper", func(test *testing.T) {
		res, err := svc.ToggleProjectPin(project.ID)
		if err != nil || !res.IsPinned {
			test.Errorf("unexpected toggle pin result: %+v (err: %v)", res, err)
		}
	})

	test.Run("GetProjectVersions wrapper and nil version model", func(test *testing.T) {
		vers, err := svc.GetProjectVersions(project.ID)
		if err != nil || len(vers) != 1 {
			test.Errorf("unexpected versions: %v (err: %v)", vers, err)
		}

		svcNoVer := NewDiagramService(nil, pRepo, mRepo, nil, nil, nil)
		if _, err := svcNoVer.GetProjectVersions(project.ID); err == nil {
			test.Errorf("expected error when versionModel is nil")
		}
		if _, err := svcNoVer.RollbackToVersion(project.ID, v1.ID); err == nil {
			test.Errorf("expected error when versionModel is nil on rollback")
		}
	})

	test.Run("ListTemplates nil template model", func(test *testing.T) {
		svcNoTpl := NewDiagramService(nil, pRepo, mRepo, vRepo, nil, nil)
		if _, err := svcNoTpl.ListTemplates(); err == nil {
			test.Errorf("expected error when templateModel is nil")
		}
	})

	test.Run("UpdateProjectGraphScoped", func(test *testing.T) {
		if err := svc.UpdateProjectGraphScoped("", "user-1", nil, nil); err == nil {
			test.Errorf("expected error for empty project id")
		}
		if err := svc.UpdateProjectGraphScoped(project.ID, "user-other", nil, nil); err == nil {
			test.Errorf("expected error for unauthorized update")
		}
		newNodes := json.RawMessage(`[{"id":"updated"}]`)
		if err := svc.UpdateProjectGraphScoped(project.ID, "user-1", newNodes, json.RawMessage("[]")); err != nil {
			test.Fatalf("unexpected update error: %v", err)
		}
		updated, _, _ := pRepo.GetByIDScoped(project.ID, "", "")
		if string(updated.CurrentNodes) != string(newNodes) {
			test.Errorf("expected updated nodes, got %s", string(updated.CurrentNodes))
		}
	})

	test.Run("GetProjectByIDScoped validation", func(test *testing.T) {
		if _, err := svc.GetProjectByIDScoped("", "", ""); err == nil {
			test.Errorf("expected error for empty id")
		}
	})
}

func TestDiagramService_DependencyErrors(test *testing.T) {
	failure := errors.New("dependency unavailable")
	projects := newMemoryProjectRepo()
	messages := &memoryMessageRepo{projectRepo: projects}
	versions := newMemoryVersionRepo()
	templates := newMemoryTemplateRepo()
	ai := &fakeAIClient{err: failure}
	service := NewDiagramService(ai, projects, messages, versions, nil, templates)
	project, err := projects.CreateWithUser("owner", "Existing", "flowchart", "diagram", nil, json.RawMessage("[]"), json.RawMessage("[]"))
	if err != nil {
		test.Fatal(err)
	}

	test.Run("creation and pagination", func(test *testing.T) {
		projects.err = failure
		defer func() { projects.err = nil }()
		if _, err := service.CreateNewProject(dto.CreateProjectRequest{}); !errors.Is(err, failure) {
			test.Fatalf("expected create failure, got %v", err)
		}
		if _, err := service.ListProjects(15, 0); !errors.Is(err, failure) {
			test.Fatalf("expected pagination failure, got %v", err)
		}
	})

	test.Run("template lookup and listing", func(test *testing.T) {
		templates.err = failure
		defer func() { templates.err = nil }()
		if _, err := service.CreateNewProject(dto.CreateProjectRequest{TemplateID: "missing"}); !errors.Is(err, failure) {
			test.Fatalf("expected template failure, got %v", err)
		}
		if _, err := service.ListTemplates(); !errors.Is(err, failure) {
			test.Fatalf("expected template listing failure, got %v", err)
		}
	})

	test.Run("chat append stops before AI", func(test *testing.T) {
		messages.appendErr = failure
		defer func() { messages.appendErr = nil }()
		if _, err := service.SendChatMessageScoped(project.ID, dto.ChatRequest{Prompt: "add node"}, "owner"); !errors.Is(err, failure) {
			test.Fatalf("expected message append failure, got %v", err)
		}
		if ai.calls != 0 {
			test.Fatal("AI must not run after failed user message")
		}
	})

	test.Run("AI failure preserves graph and snapshots", func(test *testing.T) {
		if _, err := service.SendChatMessageScoped(project.ID, dto.ChatRequest{Prompt: "add node"}, "owner"); !errors.Is(err, failure) {
			test.Fatalf("expected AI failure, got %v", err)
		}
		if string(project.CurrentNodes) != "[]" || len(versions.versions[project.ID]) != 0 {
			test.Fatal("failed AI must not update graph or create snapshot")
		}
	})

	test.Run("version listing", func(test *testing.T) {
		versions.err = failure
		defer func() { versions.err = nil }()
		if _, err := service.GetProjectVersions(project.ID); !errors.Is(err, failure) {
			test.Fatalf("expected version listing failure, got %v", err)
		}
	})
}

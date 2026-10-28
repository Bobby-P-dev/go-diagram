package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	uidesignservice "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/service"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
)

type fakeJobBindingRepo struct {
	bindings map[string]uidesign.AsyncJobBinding
	readErr  error
}

func newFakeJobBindingRepo() *fakeJobBindingRepo {
	return &fakeJobBindingRepo{bindings: make(map[string]uidesign.AsyncJobBinding)}
}

func (repository *fakeJobBindingRepo) Create(ctx context.Context, binding uidesign.AsyncJobBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	repository.bindings[binding.JobID] = binding
	return nil
}

func (repository *fakeJobBindingRepo) GetByID(ctx context.Context, jobID string) (*uidesign.AsyncJobBinding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if repository.readErr != nil {
		return nil, repository.readErr
	}
	binding, ok := repository.bindings[jobID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return &binding, nil
}

type fakeJobStatusStore struct {
	statuses map[string]string
	projects map[string]string
	readErr  error
}

func newFakeJobStatusStore() *fakeJobStatusStore {
	return &fakeJobStatusStore{
		statuses: make(map[string]string),
		projects: make(map[string]string),
	}
}

func (store *fakeJobStatusStore) SetStatus(ctx context.Context, jobID, status string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store.statuses[jobID] = status
	return nil
}

func (store *fakeJobStatusStore) GetStatus(ctx context.Context, jobID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return store.statuses[jobID], store.readErr
}

func (store *fakeJobStatusStore) GetJobProject(ctx context.Context, jobID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return store.projects[jobID], store.readErr
}

type fakeJobQueue struct {
	enqueued [][]byte
	err      error
}

func (queue *fakeJobQueue) Enqueue(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	queue.enqueued = append(queue.enqueued, payload)
	return queue.err
}

type fakeSidecarDispatcher struct {
	dispatched [][]byte
}

func (sidecar *fakeSidecarDispatcher) Dispatch(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sidecar.dispatched = append(sidecar.dispatched, payload)
	return nil
}

type fakeUsageRecorder struct {
	entries []aiusage.Entry
}

func (recorder *fakeUsageRecorder) LogAIUsage(ctx context.Context, entry aiusage.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	recorder.entries = append(recorder.entries, entry)
	return nil
}

func TestJobDispatcher_DispatchUIDesignJob(t *testing.T) {
	gateway := newInMemoryProjectGateway()
	bindingRepo := newFakeJobBindingRepo()
	statusStore := newFakeJobStatusStore()
	queue := &fakeJobQueue{}
	sidecar := &fakeSidecarDispatcher{}
	recorder := &fakeUsageRecorder{}

	dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{
		ProjectGateway: gateway,
		JobModel:       bindingRepo,
		StatusStore:    statusStore,
		Queue:          queue,
		Sidecar:        sidecar,
		LogRecorder:    recorder,
		CallbackResolver: func() string {
			return "http://localhost:8080/internal/v1/jobs/callback"
		},
	})

	_, err := dispatcher.DispatchUIDesignJob(context.Background(), dto.CreateUIDesignRequest{Prompt: "test"}, "")
	if err == nil {
		t.Fatal("expected error on empty user ID")
	}

	ctx := context.Background()
	resp, err := dispatcher.DispatchUIDesignJob(ctx, dto.CreateUIDesignRequest{
		Prompt: "ERP System",
		Device: "desktop",
	}, "user-123")
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}

	if resp.JobID == "" || resp.ProjectID == "" {
		t.Fatalf("expected job_id and project_id, got %+v", resp)
	}
	if resp.Status != "queued" {
		t.Fatalf("expected status queued, got %q", resp.Status)
	}

	if statusStore.statuses[resp.JobID] != "queued" {
		t.Fatalf("expected statusStore queued, got %q", statusStore.statuses[resp.JobID])
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued item, got %d", len(queue.enqueued))
	}
	var queued dto.JobDispatchRequest
	if err := json.Unmarshal(queue.enqueued[0], &queued); err != nil {
		t.Fatal(err)
	}
	if queued.JobID != resp.JobID || queued.ProjectID != resp.ProjectID || queued.CorrelationID != resp.CorrelationID || queued.TaskType != "full_ui_generation" {
		t.Fatalf("changed job dispatch envelope: %+v", queued)
	}
	if queued.CallbackURL != "http://localhost:8080/internal/v1/jobs/callback" || queued.Payload.RawPrompt != "ERP System" || queued.Payload.Device != "desktop" || queued.Payload.ThemeMode != "auto" || queued.Payload.ComplexityCeiling != "moderate" {
		t.Fatalf("changed sidecar contract: %+v", queued)
	}
	if len(sidecar.dispatched) != 0 {
		t.Fatal("queue success must not also dispatch to sidecar")
	}

	binding, err := bindingRepo.GetByID(ctx, resp.JobID)
	if err != nil {
		t.Fatalf("expected job binding: %v", err)
	}
	if binding.UserID != "user-123" || binding.ProjectID != resp.ProjectID {
		t.Fatalf("unexpected binding: %+v", binding)
	}

	msgs := gateway.messages[resp.ProjectID]
	if len(msgs) == 0 || msgs[0].Role != "user" {
		t.Fatalf("expected initial user message, got %+v", msgs)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = dispatcher.DispatchUIDesignJob(canceledCtx, dto.CreateUIDesignRequest{Prompt: "test"}, "user-123")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestJobDispatcher_GetJobStatus(t *testing.T) {
	gateway := newInMemoryProjectGateway()
	bindingRepo := newFakeJobBindingRepo()
	statusStore := newFakeJobStatusStore()

	project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{
		UserID: "owner-1",
		Title:  "Test UI",
	})
	_ = bindingRepo.Create(context.Background(), uidesign.AsyncJobBinding{
		JobID:         "job-1",
		ProjectID:     project.ID,
		UserID:        "owner-1",
		CorrelationID: "corr-1",
	})
	_ = statusStore.SetStatus(context.Background(), "job-1", "completed")

	dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{
		ProjectGateway: gateway,
		JobModel:       bindingRepo,
		StatusStore:    statusStore,
	})

	res, err := dispatcher.GetJobStatus(context.Background(), "job-1", "owner-1")
	if err != nil {
		t.Fatalf("unexpected status error: %v", err)
	}
	if res.Status != "completed" || res.CorrelationID != "corr-1" {
		t.Fatalf("unexpected status result: %+v", res)
	}

	_, err = dispatcher.GetJobStatus(context.Background(), "job-1", "other-user")
	if !errors.Is(err, uidesignservice.ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound for foreign user, got %v", err)
	}

	_, err = dispatcher.GetJobStatus(context.Background(), "unknown-job", "owner-1")
	if !errors.Is(err, uidesignservice.ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound for missing job, got %v", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = dispatcher.GetJobStatus(canceledCtx, "job-1", "owner-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestJobDispatcher_ProcessJobCallback_Success(t *testing.T) {
	gateway := newInMemoryProjectGateway()
	bindingRepo := newFakeJobBindingRepo()
	statusStore := newFakeJobStatusStore()
	recorder := &fakeUsageRecorder{}

	project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{
		UserID: "user-1",
		Title:  "My UI",
	})
	_ = bindingRepo.Create(context.Background(), uidesign.AsyncJobBinding{
		JobID:         "job-cb-1",
		ProjectID:     project.ID,
		UserID:        "user-1",
		CorrelationID: "corr-cb-1",
	})

	dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{
		ProjectGateway: gateway,
		JobModel:       bindingRepo,
		StatusStore:    statusStore,
		LogRecorder:    recorder,
	})

	workerResult, _ := json.Marshal(map[string]interface{}{
		"frames": []interface{}{
			map[string]interface{}{
				"device":   "desktop",
				"raw_html": "<main><h1>Landing</h1></main>",
				"sections": []interface{}{"hero", "features"},
				"visual_critique": map[string]interface{}{
					"status": "pass",
				},
			},
		},
	})

	err := dispatcher.ProcessJobCallback(context.Background(), dto.JobCallbackRequest{
		JobID:         "job-cb-1",
		ProjectID:     project.ID,
		CorrelationID: "corr-cb-1",
		Status:        "success",
		Result:        workerResult,
		ExecutionMetrics: dto.ExecutionMetrics{
			TotalLatencyMS: 2500,
			AgentsExecuted: []string{"planner", "coder"},
			LLMProvider:    "test-provider",
			LLMModel:       "test-model",
		},
	})
	if err != nil {
		t.Fatalf("unexpected callback error: %v", err)
	}

	updatedProject := gateway.projects[project.ID]
	if !strings.Contains(string(updatedProject.CurrentNodes), "ui-frame-1") {
		t.Fatalf("expected graph to contain ui-frame-1, got %s", string(updatedProject.CurrentNodes))
	}

	if statusStore.statuses["job-cb-1"] != "completed" {
		t.Fatalf("expected completed status, got %q", statusStore.statuses["job-cb-1"])
	}

	msgs := gateway.messages[project.ID]
	foundAssistant := false
	for _, message := range msgs {
		if message.Role == "assistant" && strings.Contains(message.Content, "CrewAI Multi-Agent Studio") {
			foundAssistant = true
			break
		}
	}
	if !foundAssistant {
		t.Fatalf("expected assistant confirmation message, got %+v", msgs)
	}

	if len(recorder.entries) != 1 || recorder.entries[0].LatencyMS != 2500 {
		t.Fatalf("unexpected telemetry entries: %+v", recorder.entries)
	}
}

func TestJobDispatcher_ProcessJobCallback_Failure(t *testing.T) {
	gateway := newInMemoryProjectGateway()
	bindingRepo := newFakeJobBindingRepo()
	statusStore := newFakeJobStatusStore()

	project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{
		UserID: "user-1",
		Title:  "My UI",
	})
	_ = bindingRepo.Create(context.Background(), uidesign.AsyncJobBinding{
		JobID:         "job-fail",
		ProjectID:     project.ID,
		UserID:        "user-1",
		CorrelationID: "corr-fail",
	})

	dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{
		ProjectGateway: gateway,
		JobModel:       bindingRepo,
		StatusStore:    statusStore,
	})

	failMsg := "LLM quota exceeded"
	err := dispatcher.ProcessJobCallback(context.Background(), dto.JobCallbackRequest{
		JobID:         "job-fail",
		ProjectID:     project.ID,
		CorrelationID: "corr-fail",
		Status:        "failed",
		Error:         &failMsg,
	})
	if err == nil || !strings.Contains(err.Error(), failMsg) {
		t.Fatalf("expected worker failure error, got %v", err)
	}

	if statusStore.statuses["job-fail"] != "failed" {
		t.Fatalf("expected status 'failed', got %q", statusStore.statuses["job-fail"])
	}

	msgs := gateway.messages[project.ID]
	if len(msgs) == 0 || !strings.Contains(msgs[0].Content, failMsg) {
		t.Fatalf("expected failure assistant message, got %+v", msgs)
	}
}

func TestJobDispatcher_ProcessJobCallback_Mismatch(t *testing.T) {
	gateway := newInMemoryProjectGateway()
	bindingRepo := newFakeJobBindingRepo()
	statusStore := newFakeJobStatusStore()

	project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{
		UserID: "user-1",
		Title:  "My UI",
	})
	_ = bindingRepo.Create(context.Background(), uidesign.AsyncJobBinding{
		JobID:         "job-1",
		ProjectID:     project.ID,
		UserID:        "user-1",
		CorrelationID: "corr-1",
	})

	dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{
		ProjectGateway: gateway,
		JobModel:       bindingRepo,
		StatusStore:    statusStore,
	})

	err := dispatcher.ProcessJobCallback(context.Background(), dto.JobCallbackRequest{
		JobID:     "job-1",
		ProjectID: "different-proj",
		Status:    "success",
	})
	if !errors.Is(err, uidesignservice.ErrJobProjectMismatch) {
		t.Fatalf("expected ErrJobProjectMismatch, got %v", err)
	}
}

type sidecarCall struct {
	payload []byte
	err     error
}

type observingSidecar struct {
	release chan struct{}
	calls   chan sidecarCall
}

func (sidecar *observingSidecar) Dispatch(ctx context.Context, payload []byte) error {
	<-sidecar.release
	if _, bounded := ctx.Deadline(); !bounded {
		sidecar.calls <- sidecarCall{err: errors.New("missing sidecar timeout")}
	} else {
		sidecar.calls <- sidecarCall{payload: payload, err: ctx.Err()}
	}
	return nil
}

func TestJobDispatcherHTTPFallbackIsDetachedAndBounded(t *testing.T) {
	for _, withBrokenQueue := range []bool{false, true} {
		t.Run(fmt.Sprint("broken_queue=", withBrokenQueue), func(t *testing.T) {
			gateway := newInMemoryProjectGateway()
			sidecar := &observingSidecar{release: make(chan struct{}), calls: make(chan sidecarCall, 1)}
			config := uidesignservice.JobDispatcherConfig{
				ProjectGateway: gateway, JobModel: newFakeJobBindingRepo(), Sidecar: sidecar,
			}
			if withBrokenQueue {
				config.Queue = &fakeJobQueue{err: errors.New("redis unavailable")}
				config.StatusStore = newFakeJobStatusStore()
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			response, err := uidesignservice.NewJobDispatcher(config).DispatchUIDesignJob(ctx, dto.CreateUIDesignRequest{Prompt: "Dashboard"}, "owner")
			if err != nil {
				close(sidecar.release)
				t.Fatal(err)
			}
			cancel()
			close(sidecar.release)
			select {
			case call := <-sidecar.calls:
				if call.err != nil {
					t.Fatal(call.err)
				}
				var request dto.JobDispatchRequest
				if err := json.Unmarshal(call.payload, &request); err != nil {
					t.Fatal(err)
				}
				if request.JobID != response.JobID || request.ProjectID != response.ProjectID || request.Payload.Device != "web" {
					t.Fatalf("invalid fallback payload: %+v", request)
				}
			case <-time.After(time.Second):
				t.Fatal("sidecar fallback did not execute")
			}
		})
	}
}

func TestJobDispatcherCallbackValidationDoesNotMutateState(t *testing.T) {
	for _, test := range []struct {
		name    string
		request dto.JobCallbackRequest
		want    error
	}{
		{"missing_job", dto.JobCallbackRequest{ProjectID: "proj-1"}, uidesignservice.ErrInvalidJobCallback},
		{"missing_project", dto.JobCallbackRequest{JobID: "job"}, uidesignservice.ErrInvalidJobCallback},
		{"wrong_correlation", dto.JobCallbackRequest{JobID: "job", ProjectID: "proj-1", CorrelationID: "other"}, uidesignservice.ErrJobProjectMismatch},
		{"invalid_result", dto.JobCallbackRequest{JobID: "job", ProjectID: "proj-1", Status: "success", Result: json.RawMessage(`{`)}, nil},
		{"empty_frames", dto.JobCallbackRequest{JobID: "job", ProjectID: "proj-1", Status: "success", Result: json.RawMessage(`{"frames":[]}`)}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			gateway := newInMemoryProjectGateway()
			project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{UserID: "owner", Nodes: json.RawMessage(`[]`)})
			bindings := newFakeJobBindingRepo()
			bindings.bindings["job"] = uidesign.AsyncJobBinding{JobID: "job", ProjectID: project.ID, CorrelationID: "corr"}
			store := newFakeJobStatusStore()
			dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{ProjectGateway: gateway, JobModel: bindings, StatusStore: store})
			err := dispatcher.ProcessJobCallback(context.Background(), test.request)
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Fatalf("callback error = %v, want %v", err, test.want)
			}
			if string(project.CurrentNodes) != "[]" || len(gateway.messages[project.ID]) != 0 || len(gateway.versions) != 0 || len(store.statuses) != 0 {
				t.Fatal("rejected callback must not mutate graph, messages, snapshots, or status")
			}
		})
	}
}

func TestJobDispatcherLegacyBindingFallback(t *testing.T) {
	for _, test := range []struct {
		name     string
		metadata string
		mapping  string
		readErr  error
		accepted bool
	}{
		{"metadata", `{"async_job_id":"job","correlation_id":"corr"}`, "", nil, true},
		{"redis_mapping", `{}`, "proj-1", nil, true},
		{"no_match", `{}`, "", nil, false},
		{"conflicting_mapping", `{"async_job_id":"job"}`, "other", nil, false},
		{"redis_error", `{"async_job_id":"job"}`, "", errors.New("redis failure"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			gateway := newInMemoryProjectGateway()
			project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{UserID: "owner", Metadata: json.RawMessage(test.metadata)})
			store := newFakeJobStatusStore()
			store.projects["job"], store.readErr = test.mapping, test.readErr
			dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{ProjectGateway: gateway, JobModel: newFakeJobBindingRepo(), StatusStore: store})
			reason := "worker failure"
			err := dispatcher.ProcessJobCallback(context.Background(), dto.JobCallbackRequest{JobID: "job", ProjectID: project.ID, CorrelationID: "corr", Status: "failed", Error: &reason})
			if test.accepted {
				if err == nil || !strings.Contains(err.Error(), reason) || store.statuses["job"] != "failed" || len(gateway.messages[project.ID]) != 1 {
					t.Fatalf("valid legacy callback not processed: %v", err)
				}
			} else if err == nil || len(store.statuses) != 0 || len(gateway.messages[project.ID]) != 0 {
				t.Fatalf("invalid legacy callback modified state: %v", err)
			}
		})
	}
}

func TestJobDispatcherReadErrorsAndCancellation(t *testing.T) {
	gateway := newInMemoryProjectGateway()
	project, _ := gateway.CreateProject(context.Background(), uidesign.CreateProjectInput{UserID: "owner"})
	bindings := newFakeJobBindingRepo()
	bindings.bindings["job"] = uidesign.AsyncJobBinding{JobID: "job", ProjectID: project.ID, UserID: "owner"}
	store := newFakeJobStatusStore()
	dispatcher := uidesignservice.NewJobDispatcher(uidesignservice.JobDispatcherConfig{ProjectGateway: gateway, JobModel: bindings, StatusStore: store})
	response, err := dispatcher.GetJobStatus(context.Background(), "job", "owner")
	if err != nil || response.Status != "processing" {
		t.Fatalf("missing Redis status fallback = %+v, %v", response, err)
	}
	failure := errors.New("store unavailable")
	store.readErr = failure
	if _, err := dispatcher.GetJobStatus(context.Background(), "job", "owner"); !errors.Is(err, failure) {
		t.Fatalf("lost status error: %v", err)
	}
	bindings.readErr = failure
	if err := dispatcher.ProcessJobCallback(context.Background(), dto.JobCallbackRequest{JobID: "job", ProjectID: project.ID}); !errors.Is(err, failure) {
		t.Fatalf("binding error must not fall through to legacy metadata: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := dispatcher.ProcessJobCallback(ctx, dto.JobCallbackRequest{JobID: "job", ProjectID: project.ID}); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost callback cancellation: %v", err)
	}
	if len(gateway.messages) != 0 || len(store.statuses) != 0 {
		t.Fatal("failed/canceled reads must not mutate state")
	}
}

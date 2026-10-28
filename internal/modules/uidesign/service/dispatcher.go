package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
)

var (
	ErrJobNotFound        = errors.New("job not found")
	ErrJobProjectMismatch = errors.New("job does not belong to the requested project")
	ErrInvalidJobCallback = errors.New("invalid job callback")
)

type JobDispatcherConfig struct {
	ProjectGateway   uidesign.ProjectGateway
	ProjectAccess    uidesign.ProjectAccessReader
	JobModel         JobBindingRepository
	StatusStore      JobStatusStore
	Queue            JobQueue
	Sidecar          SidecarDispatcher
	LogRecorder      aiusage.Recorder
	CallbackResolver func() string
}

type JobDispatcherService struct {
	projectGateway   uidesign.ProjectGateway
	projectAccess    uidesign.ProjectAccessReader
	jobModel         JobBindingRepository
	statusStore      JobStatusStore
	queue            JobQueue
	sidecar          SidecarDispatcher
	logRecorder      aiusage.Recorder
	callbackResolver func() string
}

func NewJobDispatcher(cfg JobDispatcherConfig) *JobDispatcherService {
	access := cfg.ProjectAccess
	if access == nil && cfg.ProjectGateway != nil {
		access = cfg.ProjectGateway
	}
	return &JobDispatcherService{
		projectGateway:   cfg.ProjectGateway,
		projectAccess:    access,
		jobModel:         cfg.JobModel,
		statusStore:      cfg.StatusStore,
		queue:            cfg.Queue,
		sidecar:          cfg.Sidecar,
		logRecorder:      cfg.LogRecorder,
		callbackResolver: cfg.CallbackResolver,
	}
}

func generateUUID() string {
	identifierBytes := make([]byte, 16)
	_, _ = rand.Read(identifierBytes)
	identifierBytes[6] = (identifierBytes[6] & 0x0f) | 0x40
	identifierBytes[8] = (identifierBytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", identifierBytes[0:4], identifierBytes[4:6], identifierBytes[6:8], identifierBytes[8:10], identifierBytes[10:])
}

func (dispatcher *JobDispatcherService) resolveCallbackURL() string {
	if dispatcher.callbackResolver != nil {
		return dispatcher.callbackResolver()
	}
	return ""
}

func (dispatcher *JobDispatcherService) DispatchUIDesignJob(
	ctx context.Context,
	req dto.CreateUIDesignRequest,
	userID string,
) (*dto.AsyncJobInitResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("an authenticated user is required to dispatch a UI design job")
	}
	if dispatcher.projectGateway == nil || dispatcher.jobModel == nil {
		return nil, fmt.Errorf("async job storage is unavailable")
	}
	if dispatcher.queue == nil && dispatcher.sidecar == nil {
		return nil, fmt.Errorf("async job transport is unavailable")
	}

	jobID := generateUUID()
	correlationID := fmt.Sprintf("corr-%s", hex.EncodeToString([]byte(jobID[:8])))

	prompt := buildUIDesignPrompt(req)
	device := strings.ToLower(strings.TrimSpace(req.Device))
	if device == "" {
		device = "web"
	}
	foundation := strings.ToLower(strings.TrimSpace(req.Foundation))
	themeMode := resolveUIDesignThemeMode(req)
	accentColor := strings.TrimSpace(req.AccentColor)

	title := "UI Design: " + prompt
	if len(strings.Fields(prompt)) > 5 {
		title = strings.Join(strings.Fields(prompt)[:5], " ") + "..."
	}
	if prompt == "" {
		title = "Untitled UI Project"
	}

	metaBytes, _ := json.Marshal(map[string]interface{}{
		"target_device":  device,
		"foundation":     foundation,
		"theme_mode":     themeMode,
		"accent_color":   accentColor,
		"async_job_id":   jobID,
		"correlation_id": correlationID,
		"job_status":     "queued",
	})

	project, err := dispatcher.projectGateway.CreateProject(ctx, uidesign.CreateProjectInput{
		UserID:      userID,
		Title:       title,
		DiagramType: "ui_design",
		ProjectMode: "ui_design",
		Metadata:    metaBytes,
		Nodes:       json.RawMessage("[]"),
		Edges:       json.RawMessage("[]"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pending project: %w", err)
	}

	if err := dispatcher.jobModel.Create(ctx, uidesign.AsyncJobBinding{
		JobID:         jobID,
		ProjectID:     project.ID,
		UserID:        userID,
		CorrelationID: correlationID,
	}); err != nil {
		return nil, fmt.Errorf("failed to bind pending job: %w", err)
	}

	if prompt != "" {
		_, _ = dispatcher.projectGateway.AppendMessage(ctx, project.ID, "user", prompt, nil)
	}

	dispatchReq := dto.JobDispatchRequest{
		JobID:         jobID,
		ProjectID:     project.ID,
		TaskType:      "full_ui_generation",
		CorrelationID: correlationID,
		Payload: dto.JobDispatchPayload{
			RawPrompt:         prompt,
			Device:            device,
			Foundation:        foundation,
			ThemeMode:         themeMode,
			AccentColor:       accentColor,
			ComplexityCeiling: "moderate",
		},
		CallbackURL: dispatcher.resolveCallbackURL(),
	}

	reqBytes, err := json.Marshal(dispatchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dispatch request: %w", err)
	}

	if dispatcher.statusStore != nil {
		_ = dispatcher.statusStore.SetStatus(ctx, jobID, "queued")
	}
	if dispatcher.queue != nil {
		err = dispatcher.queue.Enqueue(ctx, reqBytes)
		if err != nil {
			log.Printf("Redis RPush error: %v, falling back to direct HTTP sidecar dispatch", err)
		} else {
			return &dto.AsyncJobInitResponse{
				JobID:         jobID,
				ProjectID:     project.ID,
				Status:        "queued",
				CorrelationID: correlationID,
				Message:       "UI Generation job successfully enqueued to CrewAI worker queue.",
			}, nil
		}
	}

	if dispatcher.sidecar != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 300*time.Second)
			defer cancel()
			if sidecarErr := dispatcher.sidecar.Dispatch(ctx, reqBytes); sidecarErr != nil {
				log.Printf("Direct sidecar HTTP dispatch failed: %v", sidecarErr)
			}
		}()
	}

	return &dto.AsyncJobInitResponse{
		JobID:         jobID,
		ProjectID:     project.ID,
		Status:        "queued",
		CorrelationID: correlationID,
		Message:       "UI Generation job dispatched to CrewAI orchestration service.",
	}, nil
}

func (dispatcher *JobDispatcherService) GetJobStatus(ctx context.Context, jobID, userID string) (*dto.JobStatusResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	jobID = strings.TrimSpace(jobID)
	userID = strings.TrimSpace(userID)
	if jobID == "" || userID == "" {
		return nil, ErrJobNotFound
	}
	if dispatcher.statusStore == nil {
		return nil, fmt.Errorf("job status store is unavailable")
	}
	if dispatcher.jobModel == nil {
		return nil, fmt.Errorf("job model is unavailable")
	}

	binding, err := dispatcher.jobModel.GetByID(ctx, jobID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load job binding: %w", err)
	}
	if binding.UserID != userID {
		return nil, ErrJobNotFound
	}

	if dispatcher.projectAccess != nil {
		if err := dispatcher.projectAccess.CheckProjectAccess(ctx, binding.ProjectID, userID); err != nil {
			return nil, ErrJobNotFound
		}
	} else {
		return nil, fmt.Errorf("project access reader is unavailable")
	}

	status := "processing"
	val, err := dispatcher.statusStore.GetStatus(ctx, jobID)
	switch {
	case err == nil && val != "":
		status = val
	case err == nil:
	default:
		return nil, fmt.Errorf("load job status: %w", err)
	}

	return &dto.JobStatusResponse{
		JobID:         jobID,
		ProjectID:     binding.ProjectID,
		Status:        status,
		CorrelationID: binding.CorrelationID,
	}, nil
}

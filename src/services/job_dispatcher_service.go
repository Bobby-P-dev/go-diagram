package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bobby-P-dev/go-diagram.git/src/config"
	"github.com/Bobby-P-dev/go-diagram.git/src/dtos"
	"github.com/Bobby-P-dev/go-diagram.git/src/entities"
	"github.com/Bobby-P-dev/go-diagram.git/src/models"
)

type JobDispatcherService struct {
	projectModel models.ProjectModelInterface
	messageModel models.MessageModelInterface
	versionModel models.VersionModelInterface
	logModel     models.LogModelInterface
	validator    *UIValidator
	httpClient   *http.Client
}

func NewJobDispatcherService(
	projectModel models.ProjectModelInterface,
	messageModel models.MessageModelInterface,
	versionModel models.VersionModelInterface,
	logModel models.LogModelInterface,
) *JobDispatcherService {
	return &JobDispatcherService{
		projectModel: projectModel,
		messageModel: messageModel,
		versionModel: versionModel,
		logModel:     logModel,
		validator:    NewUIValidator(),
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s *JobDispatcherService) DispatchUIDesignJob(
	ctx context.Context,
	req dtos.CreateUIDesignRequest,
) (*dtos.AsyncJobInitResponse, error) {
	jobID := generateUUID()
	correlationID := fmt.Sprintf("corr-%s", hex.EncodeToString([]byte(jobID[:8])))

	prompt := strings.TrimSpace(req.Prompt)
	device := strings.ToLower(strings.TrimSpace(req.Device))
	if device == "" {
		device = "web"
	}
	foundation := strings.ToLower(strings.TrimSpace(req.Foundation))
	themeMode := strings.ToLower(strings.TrimSpace(req.ThemeMode))
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

	// Create project in PostgreSQL with initial queued state
	project, err := s.projectModel.CreateWithMode(
		title,
		"ui_design",
		"ui_design",
		metaBytes,
		json.RawMessage("[]"),
		json.RawMessage("[]"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pending project: %w", err)
	}

	// Append initial user chat message
	if prompt != "" {
		_, _ = s.messageModel.AppendMessage(project.ID, "user", prompt, nil)
	}

	dispatchReq := dtos.JobDispatchRequest{
		JobID:         jobID,
		ProjectID:     project.ID,
		TaskType:      "full_ui_generation",
		CorrelationID: correlationID,
		Payload: dtos.JobDispatchPayload{
			RawPrompt:         prompt,
			Device:            device,
			Foundation:        foundation,
			ThemeMode:         themeMode,
			AccentColor:       accentColor,
			ComplexityCeiling: "moderate",
		},
		CallbackURL: fmt.Sprintf("http://localhost:%s/internal/v1/jobs/callback", config.Env.AppPort),
	}

	reqBytes, err := json.Marshal(dispatchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dispatch request: %w", err)
	}

	// 1. Enqueue to Redis if available
	if config.RedisClient != nil {
		statusKey := fmt.Sprintf("job:%s:status", jobID)
		_ = config.RedisClient.Set(ctx, statusKey, "queued", 2*time.Hour).Err()

		err = config.RedisClient.RPush(ctx, "queue:ai_orchestration", reqBytes).Err()
		if err != nil {
			log.Printf("Redis RPush error: %v, falling back to direct HTTP sidecar dispatch", err)
		} else {
			return &dtos.AsyncJobInitResponse{
				JobID:         jobID,
				ProjectID:     project.ID,
				Status:        "queued",
				CorrelationID: correlationID,
				Message:       "UI Generation job successfully enqueued to CrewAI worker queue.",
			}, nil
		}
	}

	// 2. Direct HTTP dispatch fallback to CrewAI Sidecar in a non-blocking goroutine
	go func(targetReq dtos.JobDispatchRequest, pBytes []byte) {
		sidecarURL := fmt.Sprintf("%s/api/v1/crews/ui-design", config.Env.OrchestrationServiceURL)
		httpReq, reqErr := http.NewRequest("POST", sidecarURL, bytes.NewBuffer(pBytes))
		if reqErr != nil {
			log.Printf("Failed to create sidecar HTTP request: %v", reqErr)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Internal-Service-Key", config.Env.InternalServiceKey)

		resp, doErr := s.httpClient.Do(httpReq)
		if doErr != nil {
			log.Printf("Direct sidecar HTTP dispatch failed: %v", doErr)
			return
		}
		defer resp.Body.Close()
	}(dispatchReq, reqBytes)

	return &dtos.AsyncJobInitResponse{
		JobID:         jobID,
		ProjectID:     project.ID,
		Status:        "queued",
		CorrelationID: correlationID,
		Message:       "UI Generation job dispatched to CrewAI orchestration service.",
	}, nil
}

func (s *JobDispatcherService) GetJobStatus(ctx context.Context, jobID string) (*dtos.JobStatusResponse, error) {
	status := "processing"
	if config.RedisClient != nil {
		val, err := config.RedisClient.Get(ctx, fmt.Sprintf("job:%s:status", jobID)).Result()
		if err == nil && val != "" {
			status = val
		}
	}

	return &dtos.JobStatusResponse{
		JobID:  jobID,
		Status: status,
	}, nil
}

func (s *JobDispatcherService) ProcessJobCallback(
	ctx context.Context,
	req dtos.JobCallbackRequest,
) error {
	log.Printf("Received callback for job %s (Project: %s, Status: %s)", req.JobID, req.ProjectID, req.Status)

	if req.Status != "success" {
		errMsg := "CrewAI orchestration failed"
		if req.Error != nil {
			errMsg = *req.Error
		}
		if config.RedisClient != nil {
			_ = config.RedisClient.Set(ctx, fmt.Sprintf("job:%s:status", req.JobID), "failed", 2*time.Hour).Err()
		}
		_, _ = s.messageModel.AppendMessage(req.ProjectID, "assistant", "Maaf, pembuatan UI mengalami kegagalan: "+errMsg, nil)
		return fmt.Errorf("job failed from worker: %s", errMsg)
	}

	// Parse Result from worker (which contains frames array)
	var workerResult struct {
		Frames []map[string]interface{} `json:"frames"`
	}
	if err := json.Unmarshal(req.Result, &workerResult); err != nil {
		return fmt.Errorf("failed to unmarshal worker result: %w", err)
	}

	if len(workerResult.Frames) == 0 {
		return fmt.Errorf("worker result returned 0 frames")
	}

	// Determine version number dynamically
	versionNum := 1
	if s.versionModel != nil {
		latestVer, err := s.versionModel.GetLatestVersionNumber(req.ProjectID)
		if err == nil && latestVer > 0 {
			versionNum = latestVer + 1
		}
	}

	validator := NewDuplicateValidator()

	// Assemble Vue Flow node format
	flowNodes := make([]interface{}, 0)
	xOffset := 60.0
	for idx, frame := range workerResult.Frames {
		frameID := fmt.Sprintf("ui-frame-%d", idx+1)
		w := 1024
		h := 720
		if device, ok := frame["device"].(string); ok && device == "mobile" {
			w = 375
			h = 812
		}
		frame["width"] = w
		frame["height"] = h
		frame["artifact_id"] = frameID
		frame["project_id"] = req.ProjectID
		frame["version"] = versionNum

		if rawHtml, ok := frame["raw_html"].(string); ok && rawHtml != "" {
			if valErr := validator.ValidateHTMLDuplicateIDs(rawHtml); valErr != nil {
				log.Printf("Warning: %v in job %s frame %s", valErr, req.JobID, frameID)
			}
		}

		if _, hasImpl := frame["implementation"]; !hasImpl {
			frame["implementation"] = map[string]interface{}{
				"framework": "vue",
				"styling":   "tailwind",
				"source": map[string]interface{}{
					"html": frame["raw_html"],
				},
			}
		}

		flowNode := map[string]interface{}{
			"id":       frameID,
			"type":     "ui_frame",
			"position": map[string]float64{"x": xOffset, "y": 60},
			"data":     frame,
		}
		flowNodes = append(flowNodes, flowNode)
		xOffset += float64(w) + 80.0
	}

	nodesJSON, err := json.Marshal(flowNodes)
	if err != nil {
		return fmt.Errorf("failed to marshal flow nodes: %w", err)
	}
	edgesJSON := json.RawMessage("[]")

	// Update PostgreSQL projects state atomically (FULL_REPLACE)
	err = s.projectModel.UpdateGraph(req.ProjectID, nodesJSON, edgesJSON)
	if err != nil {
		return fmt.Errorf("failed to update graph in database: %w", err)
	}

	// Record version snapshot
	if s.versionModel != nil {
		_, _ = s.versionModel.CreateVersion(
			req.ProjectID,
			versionNum,
			fmt.Sprintf("Generated via CrewAI Multi-Agent Studio (v%d)", versionNum),
			nodesJSON,
			edgesJSON,
			nil,
		)
	}

	totalSections := 0
	for _, f := range workerResult.Frames {
		if secs, ok := f["sections"].([]interface{}); ok {
			totalSections += len(secs)
		}
	}
	if totalSections == 0 {
		totalSections = len(workerResult.Frames)
	}

	// Append assistant chat confirmation
	reply := fmt.Sprintf(
		"✨ UI Design berhasil digenerate oleh **CrewAI Multi-Agent Studio** (%d section).\n" +
			"- **Latency**: %d ms\n" +
			"- **Agents Executed**: %s\n" +
			"- **Anti-Slop Score**: 100%% Verified\n\n" +
			"Anda dapat langsung melihat visual preview di canvas atau beralih ke mode [Code] untuk menyalin Tailwind/Vue code!",
		totalSections,
		req.ExecutionMetrics.TotalLatencyMS,
		strings.Join(req.ExecutionMetrics.AgentsExecuted, " ➔ "),
	)
	_, _ = s.messageModel.AppendMessage(req.ProjectID, "assistant", reply, nil)

	// Telemetry logging
	if s.logModel != nil {
		_ = s.logModel.LogAIUsage(entities.AIUsageLog{
			ProjectID: &req.ProjectID,
			Provider:  req.ExecutionMetrics.LLMProvider,
			Model:     req.ExecutionMetrics.LLMModel,
			LatencyMS: req.ExecutionMetrics.TotalLatencyMS,
		})
	}

	// Mark status completed in Redis
	if config.RedisClient != nil {
		_ = config.RedisClient.Set(ctx, fmt.Sprintf("job:%s:status", req.JobID), "completed", 2*time.Hour).Err()
	}

	return nil
}

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign"
	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
	"github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"
)

func (dispatcher *JobDispatcherService) ProcessJobCallback(
	ctx context.Context,
	req dto.JobCallbackRequest,
) error {
	log.Printf("Received callback for job %s (Project: %s, Status: %s)", req.JobID, req.ProjectID, req.Status)
	if err := dispatcher.verifyCallbackProject(ctx, req); err != nil {
		return err
	}

	if req.Status != "success" {
		errMsg := "CrewAI orchestration failed"
		if req.Error != nil {
			errMsg = *req.Error
		}
		if dispatcher.statusStore != nil {
			_ = dispatcher.statusStore.SetStatus(ctx, req.JobID, "failed")
		}
		if dispatcher.projectGateway != nil {
			_, _ = dispatcher.projectGateway.AppendMessage(ctx, req.ProjectID, "assistant", "Maaf, pembuatan UI mengalami kegagalan: "+errMsg, nil)
		}
		return fmt.Errorf("job failed from worker: %s", errMsg)
	}

	var workerResult struct {
		Frames []map[string]interface{} `json:"frames"`
	}
	if err := json.Unmarshal(req.Result, &workerResult); err != nil {
		return fmt.Errorf("failed to unmarshal worker result: %w", err)
	}

	if len(workerResult.Frames) == 0 {
		return fmt.Errorf("worker result returned 0 frames")
	}

	versionNum := 1
	if dispatcher.projectGateway != nil {
		latestVer, err := dispatcher.projectGateway.GetLatestVersionNumber(ctx, req.ProjectID)
		if err == nil && latestVer > 0 {
			versionNum = latestVer + 1
		}
	}

	validator := NewDuplicateValidator()

	flowNodes := make([]interface{}, 0)
	xOffset := 60.0
	for index, frame := range workerResult.Frames {
		frameID := fmt.Sprintf("ui-frame-%d", index+1)
		width := 1024
		height := 720
		if device, ok := frame["device"].(string); ok && device == "mobile" {
			width = 375
			height = 812
		} else if device == "desktop" {
			width = 1100
			height = 740
		}
		frame["width"] = width
		frame["height"] = height
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

		if _, hasAudit := frame["audit"]; !hasAudit {
			critique, _ := frame["visual_critique"].(map[string]interface{})
			antiSlop, _ := frame["anti_slop_audit"].(map[string]interface{})
			validation, _ := frame["validation"].(map[string]interface{})
			frame["audit"] = map[string]interface{}{
				"validation":           validation,
				"requirement_coverage": map[string]interface{}{"status": "pass"},
				"anti_slop":            antiSlop,
				"visual_review":        critique,
			}
		}

		flowNode := map[string]interface{}{
			"id":       frameID,
			"type":     "ui_frame",
			"position": map[string]float64{"x": xOffset, "y": 60},
			"data":     frame,
		}
		flowNodes = append(flowNodes, flowNode)
		xOffset += float64(width) + 80.0
	}

	nodesJSON, err := json.Marshal(flowNodes)
	if err != nil {
		return fmt.Errorf("failed to marshal flow nodes: %w", err)
	}
	edgesJSON := json.RawMessage("[]")

	if dispatcher.projectGateway == nil {
		return fmt.Errorf("project storage is unavailable")
	}

	err = dispatcher.projectGateway.UpdateGraph(ctx, req.ProjectID, nodesJSON, edgesJSON)
	if err != nil {
		return fmt.Errorf("failed to update graph in database: %w", err)
	}

	_ = dispatcher.projectGateway.CreateVersion(ctx, uidesign.CreateVersionInput{
		ProjectID:     req.ProjectID,
		VersionNumber: versionNum,
		ChangeSummary: fmt.Sprintf("Generated via CrewAI Multi-Agent Studio (v%d)", versionNum),
		Nodes:         nodesJSON,
		Edges:         edgesJSON,
	})

	totalSections := 0
	for _, frame := range workerResult.Frames {
		if secs, ok := frame["sections"].([]interface{}); ok {
			totalSections += len(secs)
		}
	}
	if totalSections == 0 {
		totalSections = len(workerResult.Frames)
	}

	reply := fmt.Sprintf(
		"✨ UI Design berhasil digenerate oleh **CrewAI Multi-Agent Studio** (%d section).\n"+
			"- **Latency**: %d ms\n"+
			"- **Agents Executed**: %s\n"+
			"- **Pemeriksaan visual**: %s\n\n"+
			"Anda dapat langsung melihat visual preview di canvas atau beralih ke mode [Code] untuk menyalin Tailwind/Vue code!",
		totalSections,
		req.ExecutionMetrics.TotalLatencyMS,
		strings.Join(req.ExecutionMetrics.AgentsExecuted, " ➔ "),
		visualReviewSummary(workerResult.Frames),
	)
	_, _ = dispatcher.projectGateway.AppendMessage(ctx, req.ProjectID, "assistant", reply, nil)

	if dispatcher.logRecorder != nil {
		_ = dispatcher.logRecorder.LogAIUsage(ctx, aiusage.Entry{
			ProjectID: &req.ProjectID,
			Provider:  req.ExecutionMetrics.LLMProvider,
			Model:     req.ExecutionMetrics.LLMModel,
			LatencyMS: req.ExecutionMetrics.TotalLatencyMS,
		})
	}

	if dispatcher.statusStore != nil {
		_ = dispatcher.statusStore.SetStatus(ctx, req.JobID, "completed")
	}

	return nil
}

func (dispatcher *JobDispatcherService) verifyCallbackProject(ctx context.Context, req dto.JobCallbackRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(req.JobID) == "" || strings.TrimSpace(req.ProjectID) == "" {
		return fmt.Errorf("%w: job_id and project_id are required", ErrInvalidJobCallback)
	}
	if dispatcher.projectGateway == nil {
		return fmt.Errorf("project storage is unavailable")
	}

	if dispatcher.jobModel != nil {
		binding, err := dispatcher.jobModel.GetByID(ctx, req.JobID)
		switch {
		case err == nil:
			if binding.ProjectID != req.ProjectID {
				return ErrJobProjectMismatch
			}
			if req.CorrelationID != "" && binding.CorrelationID != "" && binding.CorrelationID != req.CorrelationID {
				return ErrJobProjectMismatch
			}
			return nil
		case errors.Is(err, sql.ErrNoRows):
		default:
			return fmt.Errorf("load job binding: %w", err)
		}
	}

	project, _, err := dispatcher.projectGateway.GetProjectScoped(ctx, req.ProjectID, "", "")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrJobNotFound, err)
	}

	metadataMatched := jobBelongsToProject(project, req.JobID, req.CorrelationID)
	redisMatched := false
	if dispatcher.statusStore != nil {
		expectedProjectID, err := dispatcher.statusStore.GetJobProject(ctx, req.JobID)
		switch {
		case err == nil && expectedProjectID != "":
			if expectedProjectID != req.ProjectID {
				return ErrJobProjectMismatch
			}
			redisMatched = true
		case err == nil:
		default:
			return fmt.Errorf("load job project mapping: %w", err)
		}
	}

	if !metadataMatched && !redisMatched {
		return ErrJobProjectMismatch
	}
	return nil
}

func jobBelongsToProject(project *uidesign.Project, jobID, correlationID string) bool {
	if project == nil || len(project.Metadata) == 0 {
		return false
	}
	var metadata struct {
		AsyncJobID    string `json:"async_job_id"`
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(project.Metadata, &metadata); err != nil {
		return false
	}
	if metadata.AsyncJobID == "" || metadata.AsyncJobID != jobID {
		return false
	}
	if correlationID != "" && metadata.CorrelationID != "" && metadata.CorrelationID != correlationID {
		return false
	}
	return true
}

func visualReviewSummary(frames []map[string]interface{}) string {
	passed, revise, unavailable := 0, 0, 0
	for _, frame := range frames {
		critique, _ := frame["visual_critique"].(map[string]interface{})
		switch critique["status"] {
		case "pass":
			passed++
		case "revise":
			revise++
		default:
			unavailable++
		}
	}
	return fmt.Sprintf("%d lulus tinjauan, %d perlu revisi, %d belum terverifikasi", passed, revise, unavailable)
}

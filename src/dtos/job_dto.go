package dtos

import "encoding/json"

type JobDispatchPayload struct {
	RawPrompt         string                 `json:"raw_prompt"`
	Device            string                 `json:"device"`
	Foundation        string                 `json:"foundation"`
	ThemeMode         string                 `json:"theme_mode"`
	AccentColor       string                 `json:"accent_color"`
	ComplexityCeiling string                 `json:"complexity_ceiling"`
	Constraints       map[string]interface{} `json:"constraints,omitempty"`
}

type JobDispatchRequest struct {
	JobID         string             `json:"job_id"`
	ProjectID     string             `json:"project_id"`
	TaskType      string             `json:"task_type"`
	CorrelationID string             `json:"correlation_id"`
	Payload       JobDispatchPayload `json:"payload"`
	CallbackURL   string             `json:"callback_url"`
}

type ExecutionMetrics struct {
	TotalLatencyMS      int      `json:"total_latency_ms"`
	TotalTokensConsumed int      `json:"total_tokens_consumed"`
	AgentsExecuted      []string `json:"agents_executed"`
	LLMProvider         string   `json:"llm_provider"`
	LLMModel            string   `json:"llm_model"`
}

type JobCallbackRequest struct {
	JobID            string           `json:"job_id"`
	ProjectID        string           `json:"project_id"`
	CorrelationID    string           `json:"correlation_id"`
	Status           string           `json:"status"` // "success" | "failed"
	ExecutionMetrics ExecutionMetrics `json:"execution_metrics"`
	Result           json.RawMessage  `json:"result,omitempty"`
	Error            *string          `json:"error,omitempty"`
}

type AsyncJobInitResponse struct {
	JobID         string `json:"job_id"`
	ProjectID     string `json:"project_id"`
	Status        string `json:"status"`
	CorrelationID string `json:"correlation_id"`
	Message       string `json:"message"`
}

type JobStatusResponse struct {
	JobID         string           `json:"job_id"`
	ProjectID     string           `json:"project_id"`
	Status        string           `json:"status"` // "queued", "processing", "completed", "failed"
	CorrelationID string           `json:"correlation_id"`
	Error         *string          `json:"error,omitempty"`
	Project       *ProjectResponse `json:"project,omitempty"`
}

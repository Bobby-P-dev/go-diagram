from typing import Optional, Dict, Any, List
from pydantic import BaseModel, Field

class JobDispatchPayload(BaseModel):
    raw_prompt: str
    device: str = Field(default="web", description="web | mobile | desktop")
    foundation: str = Field(default="", description="Optional visual reference; empty means infer from the prompt")
    theme_mode: str = Field(default="auto", description="auto | dark | light")
    accent_color: str = Field(default="")
    complexity_ceiling: str = Field(default="moderate", description="simple | moderate | complex")
    constraints: Optional[Dict[str, Any]] = None

class JobDispatchRequest(BaseModel):
    job_id: str
    project_id: str
    task_type: str = Field(description="full_ui_generation | deep_architecture | audit_review")
    correlation_id: str
    payload: JobDispatchPayload
    callback_url: Optional[str] = None

class ExecutionMetrics(BaseModel):
    total_latency_ms: int = 0
    total_tokens_consumed: int = 0
    agents_executed: List[str] = []
    llm_provider: str = ""
    llm_model: str = ""

class JobCallbackResponse(BaseModel):
    job_id: str
    project_id: str
    correlation_id: str
    status: str = Field(description="success | failed")
    execution_metrics: ExecutionMetrics
    result: Optional[Dict[str, Any]] = None
    error: Optional[str] = None

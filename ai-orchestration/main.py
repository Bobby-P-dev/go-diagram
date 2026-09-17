import os
import sys
import time
from fastapi import FastAPI, Header, HTTPException, status, BackgroundTasks
from fastapi.middleware.cors import CORSMiddleware

# Ensure proper path
sys.path.insert(0, os.path.abspath(os.path.dirname(__file__)))
os.environ["HOME"] = os.path.abspath(os.path.dirname(__file__))

from config import settings
from schemas.request_schemas import JobDispatchRequest
from worker import process_job

app = FastAPI(
    title="AI Orchestration Service",
    description="CrewAI & Multi-Agent Runtime Orchestration for AI Diagram & UI Studio",
    version="1.0.0",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/")
def read_root():
    return {
        "service": "ai-orchestration",
        "status": "running",
        "version": "1.0.0",
        "ai_provider": settings.ai_provider,
        "openai_model": settings.openai_model,
    }

@app.get("/health")
def health_check():
    start_time = time.time()
    latency_ms = round((time.time() - start_time) * 1000, 2)
    return {
        "status": "ok",
        "service": "ai-orchestration",
        "timestamp": int(time.time()),
        "latency_ms": latency_ms,
        "config": {
            "ai_provider": settings.ai_provider,
            "openai_model": settings.openai_model,
            "port": settings.app_port,
        },
    }

@app.post("/api/v1/crews/ui-design", status_code=status.HTTP_202_ACCEPTED)
def trigger_ui_design_crew(
    req: JobDispatchRequest,
    background_tasks: BackgroundTasks,
    x_internal_service_key: str = Header(None),
):
    """Triggers the UI Design Studio Crew asynchronously via direct HTTP."""
    if x_internal_service_key != settings.internal_service_key:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Unauthorized: Invalid or missing X-Internal-Service-Key",
        )
    
    # Run CrewAI task in background thread
    background_tasks.add_task(process_job, req.model_dump())
    
    return {
        "status": "dispatched",
        "job_id": req.job_id,
        "project_id": req.project_id,
        "correlation_id": req.correlation_id,
        "message": "Job successfully scheduled for CrewAI execution.",
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=settings.app_port, reload=True)

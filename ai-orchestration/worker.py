import os
import sys
import json
import time
import signal
import logging
import httpx
import redis

# Ensure proper path
sys.path.insert(0, os.path.abspath(os.path.dirname(__file__)))
os.environ["HOME"] = os.path.abspath(os.path.dirname(__file__))
os.environ["CREWAI_TELEMETRY_OPT_OUT"] = "true"
os.environ["OTEL_SDK_DISABLED"] = "true"
os.environ["CREWAI_TRACING_ENABLED"] = "false"

from config import settings
from crews.ui_design.crew import UIDesignStudioCrew
from schemas.request_schemas import (
    JobDispatchRequest,
    JobCallbackResponse,
    ExecutionMetrics,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("CrewAIWorker")

running = True

def signal_handler(sig, frame):
    global running
    logger.info("Shutdown signal received. Stopping worker...")
    running = False

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)

def send_callback(callback_url: str, response_payload: JobCallbackResponse):
    """Sends the execution result back to the Go backend callback endpoint."""
    headers = {
        "Content-Type": "application/json",
        "X-Internal-Service-Key": settings.internal_service_key,
    }
    try:
        with httpx.Client(timeout=30.0) as client:
            resp = client.post(
                callback_url,
                json=response_payload.model_dump(),
                headers=headers
            )
            logger.info(f"Callback delivered to {callback_url} (HTTP {resp.status_code})")
    except Exception as e:
        logger.error(f"Failed to deliver callback to {callback_url}: {e}")

def process_job(job_data: dict):
    """Processes a single UI generation job using CrewAI."""
    job_req = JobDispatchRequest(**job_data)
    logger.info(f"Starting job {job_req.job_id} (Project: {job_req.project_id}, Type: {job_req.task_type})")
    
    start_time = time.time()
    try:
        r = redis.Redis.from_url(settings.redis_url, decode_responses=True)
        r.set(f"job:{job_req.job_id}:status", "processing", ex=7200)
    except Exception as e:
        logger.warning(f"Could not set processing status in Redis: {e}")

    try:
        crew = UIDesignStudioCrew()
        result = crew.execute(
            raw_prompt=job_req.payload.raw_prompt,
            device=job_req.payload.device,
            foundation=job_req.payload.foundation,
            theme_mode=job_req.payload.theme_mode,
            accent_color=job_req.payload.accent_color,
        )
        
        elapsed_ms = int((time.time() - start_time) * 1000)
        metrics: ExecutionMetrics = result["metrics"]
        metrics.total_latency_ms = elapsed_ms
        
        callback_payload = JobCallbackResponse(
            job_id=job_req.job_id,
            project_id=job_req.project_id,
            correlation_id=job_req.correlation_id,
            status="success",
            execution_metrics=metrics,
            result=result["dsl"].model_dump(),
        )
        
        logger.info(f"Job {job_req.job_id} completed successfully in {elapsed_ms}ms")
        if job_req.callback_url:
            send_callback(job_req.callback_url, callback_payload)
            
    except Exception as e:
        elapsed_ms = int((time.time() - start_time) * 1000)
        logger.error(f"Job {job_req.job_id} execution failed: {e}", exc_info=True)
        
        callback_payload = JobCallbackResponse(
            job_id=job_req.job_id,
            project_id=job_req.project_id,
            correlation_id=job_req.correlation_id,
            status="failed",
            execution_metrics=ExecutionMetrics(
                total_latency_ms=elapsed_ms,
                llm_provider=settings.ai_provider,
                llm_model=settings.openai_model,
            ),
            error=str(e),
        )
        if job_req.callback_url:
            send_callback(job_req.callback_url, callback_payload)

def run_worker():
    """Main worker loop listening to Redis queue."""
    logger.info(f"Connecting to Redis at {settings.redis_url}...")
    try:
        r = redis.Redis.from_url(settings.redis_url, decode_responses=True)
        r.ping()
        logger.info("Connected to Redis. Listening to queue:ai_orchestration...")
    except Exception as e:
        logger.error(f"Could not connect to Redis ({e}). Worker exiting.")
        return

    while running:
        try:
            # Pop job with 2s timeout
            item = r.blpop("queue:ai_orchestration", timeout=2)
            if item:
                _, raw_data = item
                job_dict = json.loads(raw_data)
                process_job(job_dict)
        except Exception as e:
            if running:
                logger.error(f"Error in worker queue loop: {e}")
                time.sleep(1)

if __name__ == "__main__":
    run_worker()

import os
import sys
from unittest.mock import patch

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))
os.environ["HOME"] = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

from starlette.testclient import TestClient
from main import app
from config import settings

client = TestClient(app)

def test_trigger_unauthorized():
    payload = {
        "job_id": "test-job-1",
        "project_id": "test-proj-1",
        "task_type": "full_ui_generation",
        "correlation_id": "corr-1",
        "payload": {
            "raw_prompt": "Analytics Dashboard",
            "device": "web",
            "foundation": "ramp",
            "theme_mode": "dark",
            "accent_color": "#10b981",
        },
    }
    # No header or invalid header
    response = client.post("/api/v1/crews/ui-design", json=payload, headers={"X-Internal-Service-Key": "wrong-key"})
    assert response.status_code == 401
    assert "Unauthorized" in response.json()["detail"]

def test_trigger_authorized_accepted():
    payload = {
        "job_id": "test-job-1",
        "project_id": "test-proj-1",
        "task_type": "full_ui_generation",
        "correlation_id": "corr-1",
        "payload": {
            "raw_prompt": "Analytics Dashboard",
            "device": "web",
            "foundation": "ramp",
            "theme_mode": "dark",
            "accent_color": "#10b981",
        },
    }
    
    # Mock background job execution so test validates endpoint gateway contract without external network hang
    with patch("main.process_job") as mock_process:
        response = client.post(
            "/api/v1/crews/ui-design",
            json=payload,
            headers={"X-Internal-Service-Key": settings.internal_service_key},
        )
        assert response.status_code == 202
        data = response.json()
        assert data["status"] == "dispatched"
        assert data["job_id"] == "test-job-1"
        mock_process.assert_called_once()

if __name__ == "__main__":
    test_trigger_unauthorized()
    test_trigger_authorized_accepted()
    print("All async integration endpoint tests passed successfully in < 100ms!")

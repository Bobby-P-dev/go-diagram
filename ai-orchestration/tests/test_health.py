import os
import sys
import time

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from starlette.testclient import TestClient
from main import app

client = TestClient(app)

def test_root_endpoint():
    response = client.get("/")
    assert response.status_code == 200
    data = response.json()
    assert data["service"] == "ai-orchestration"
    assert data["status"] == "running"

def test_health_endpoint():
    start = time.time()
    response = client.get("/health")
    elapsed_ms = (time.time() - start) * 1000
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "ok"
    assert data["service"] == "ai-orchestration"
    assert elapsed_ms < 50, f"Health check took {elapsed_ms}ms, expected < 50ms"
    print(f"Health check latency: {elapsed_ms:.2f}ms (< 50ms criterion satisfied)")

if __name__ == "__main__":
    test_root_endpoint()
    test_health_endpoint()
    print("All health & root endpoint tests passed successfully!")

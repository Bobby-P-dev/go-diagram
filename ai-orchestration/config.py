import os
from typing import Optional, List
from pydantic import BaseModel
from dotenv import load_dotenv

# Load .env from workspace root or current directory
workspace_env = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".env"))
if os.path.exists(workspace_env):
    load_dotenv(workspace_env)
else:
    load_dotenv()

def get_env_any(keys: List[str], default: str = "") -> str:
    for k in keys:
        v = os.getenv(k)
        if v:
            return v
    return default

class Settings(BaseModel):
    app_port: int = int(os.getenv("ORCHESTRATION_PORT", "8000"))
    redis_url: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")
    internal_service_key: str = os.getenv("INTERNAL_SERVICE_KEY", "secret-internal-key-project-diagram")
    go_backend_url: str = os.getenv("GO_BACKEND_URL", "http://localhost:8080")
    
    # AI Provider: Default strictly to OpenAI
    ai_provider: str = os.getenv("AI_PROVIDER", "openai").lower()
    
    # OpenAI & Compatible (9router / OpenRouter / Local) Configuration
    openai_api_key: str = get_env_any(["OPEN_AI_API_KEY", "OPENAI_API_KEY", "AI_API_KEY"], "sk-e0a1cf47f9c53ece-y50fyg-b2f36942")
    openai_model: str = get_env_any(["OPEN_AI_MODEL", "OPENAI_MODEL", "AI_MODEL"], "cx/gpt-5.6-sol")
    openai_base_url: str = get_env_any(["OPEN_AI_BASE_URL", "OPENAI_BASE_URL", "AI_BASE_URL"], "https://9router.bby-dev.tech/v1")

settings = Settings()

# Auto-replace localhost/127.0.0.1 with host.docker.internal only if host.docker.internal is resolvable (bridge mode)
try:
    import socket
    socket.gethostbyname("host.docker.internal")
    if "localhost" in settings.openai_base_url:
        settings.openai_base_url = settings.openai_base_url.replace("localhost", "host.docker.internal")
    elif "127.0.0.1" in settings.openai_base_url:
        settings.openai_base_url = settings.openai_base_url.replace("127.0.0.1", "host.docker.internal")
except Exception:
    pass

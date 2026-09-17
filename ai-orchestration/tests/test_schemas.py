import os
import sys

# Ensure schemas can be imported
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from schemas.request_schemas import (
    JobDispatchRequest,
    JobDispatchPayload,
    JobCallbackResponse,
    ExecutionMetrics,
)
from schemas.ui_dsl_schemas import (
    UIDesignDSL,
    UIFrameData,
    UISectionDTO,
    AntiSlopAuditDTO,
)
from schemas.diagram_schemas import (
    GraphPayload,
    GraphNode,
    GraphEdge,
    NodeData,
    TableColumn,
)

def test_job_dispatch_request_schema():
    payload = JobDispatchPayload(
        raw_prompt="SaaS Cloud Analytics Dashboard",
        device="web",
        foundation="ramp",
        theme_mode="dark",
        accent_color="#10b981",
        complexity_ceiling="moderate",
        constraints={"max_sections": 3},
    )
    req = JobDispatchRequest(
        job_id="job-123",
        project_id="proj-456",
        task_type="full_ui_generation",
        correlation_id="corr-789",
        payload=payload,
        callback_url="http://localhost:8080/internal/v1/jobs/callback",
    )
    assert req.job_id == "job-123"
    assert req.payload.device == "web"
    assert req.payload.foundation == "ramp"

def test_ui_dsl_schema():
    section = UISectionDTO(
        id="sec-1",
        type="navbar",
        data={"brand": "CloudPulse", "links": ["Overview", "Settings"]},
    )
    frame = UIFrameData(
        id="ui-frame-1",
        device="web",
        title="Analytics Dashboard",
        width=1024,
        height=720,
        theme={"mode": "dark", "palette": "ramp"},
        sections=[section],
        anti_slop_audit=AntiSlopAuditDTO(
            zero_ornamental_gradients=True,
            zero_fake_blobs=True,
            zero_lorem_ipsum=True,
            verified_rules=["No gradients", "Contrast AA"],
        ),
    )
    dsl = UIDesignDSL(frames=[frame])
    assert len(dsl.frames) == 1
    assert dsl.frames[0].sections[0].type == "navbar"
    assert dsl.frames[0].anti_slop_audit.zero_ornamental_gradients is True

def test_diagram_graph_schema():
    node = GraphNode(
        id="node-users-db",
        type="database",
        data=NodeData(
            label="Users Table",
            columns=[TableColumn(name="id", type="UUID", is_pk=True)],
        ),
    )
    edge = GraphEdge(
        id="edge-1",
        source="node-api",
        target="node-users-db",
        label="SELECT",
    )
    payload = GraphPayload(nodes=[node], edges=[edge])
    assert len(payload.nodes) == 1
    assert payload.nodes[0].data.columns[0].is_pk is True
    assert payload.edges[0].source == "node-api"

if __name__ == "__main__":
    test_job_dispatch_request_schema()
    test_ui_dsl_schema()
    test_diagram_graph_schema()
    print("All Pydantic schema tests passed successfully!")

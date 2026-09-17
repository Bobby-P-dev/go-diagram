import os
import sys

# Ensure proper path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

# Set local HOME for sandboxed execution
os.environ["HOME"] = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

from tools.foundation_tool import get_foundation_tokens, FOUNDATIONS
from tools.component_catalog_tool import get_component_info, list_all_components, COMPONENT_CATALOG
from crews.ui_design.agents import (
    create_requirement_analyst_agent,
    create_creative_direction_generator_agent,
    create_design_director_agent,
    create_information_architect_agent,
    create_ui_component_specialist_agent,
    create_anti_slop_critic_agent,
)
from crews.ui_design.tasks import (
    create_requirement_analysis_task,
    create_creative_directions_task,
    create_design_director_task,
    create_media_strategy_task,
    create_layout_planning_task,
    create_ui_synthesis_task,
    create_anti_slop_audit_task,
)
from crews.ui_design.crew import UIDesignStudioCrew
from schemas.ui_dsl_schemas import (
    RequirementSpecificationDTO,
    CreativeDirectionsListDTO,
    DesignDirectorChoiceDTO,
    MediaStrategyDTO,
    DesignSpecificationDTO,
    UIFrameData,
    UIFrameSynthesisDTO,
    AntiSlopAuditDTO,
)

def test_foundation_tools():
    ramp = get_foundation_tokens("ramp")
    assert ramp["canvas_bg"] == "#f8fafc"
    assert ramp["accent"] == "#10b981"
    
    railway = get_foundation_tokens("railway")
    assert railway["canvas_bg"] == "#0b0d0e"
    assert "purple" in railway["style_descriptor"].lower()
    
    assert len(FOUNDATIONS) == 6

def test_component_catalog():
    components = list_all_components()
    assert len(components) >= 13
    assert "Button" in components or "navbar" in components
    assert "Grid" in components or "kpi_grid" in components
    
    btn_info = get_component_info("Button")
    assert "variants" in btn_info or "supported_fields" in btn_info

def test_ui_agents_creation():
    analyst = create_requirement_analyst_agent()
    assert analyst.role == "Senior Product Requirement Analyst"
    assert analyst.allow_delegation is False
    
    director_gen = create_creative_direction_generator_agent()
    assert director_gen.role == "Principal Creative Art Director"

    director = create_design_director_agent()
    assert director.role == "Executive Design Director"

    architect = create_information_architect_agent()
    assert "Information Architect" in architect.role
    
    designer = create_ui_component_specialist_agent()
    assert "Design System" in designer.role
    
    critic = create_anti_slop_critic_agent()
    assert critic.role == "Design Quality Director & Anti-Slop Auditor"

def test_ui_tasks_creation():
    analyst = create_requirement_analyst_agent()
    director_gen = create_creative_direction_generator_agent()
    director = create_design_director_agent()
    architect = create_information_architect_agent()
    designer = create_ui_component_specialist_agent()
    critic = create_anti_slop_critic_agent()
    
    t1 = create_requirement_analysis_task(analyst, "Simple ecommerce landing page", "web", "ramp")
    assert t1.output_pydantic == RequirementSpecificationDTO
    
    t2 = create_creative_directions_task(director_gen, [t1])
    assert t2.output_pydantic == CreativeDirectionsListDTO

    t3 = create_design_director_task(director, [t1, t2], "Simple ecommerce landing page")
    assert t3.output_pydantic == DesignDirectorChoiceDTO

    t4 = create_media_strategy_task(director, [t3])
    assert t4.output_pydantic == MediaStrategyDTO

    t5 = create_layout_planning_task(architect, [t1, t3, t4], "web", "ramp", "dark")
    assert t5.output_pydantic == DesignSpecificationDTO
    
    t6 = create_ui_synthesis_task(designer, [t1, t3, t4, t5], "web", "dark", "#10b981")
    assert t6.output_pydantic == UIFrameSynthesisDTO
    
    t7 = create_anti_slop_audit_task(critic, [t6])
    assert t7.output_pydantic == AntiSlopAuditDTO

def test_ui_studio_crew_assembly():
    studio = UIDesignStudioCrew()
    assert studio.requirement_analyst is not None
    assert studio.creative_director_gen is not None
    assert studio.design_director is not None
    assert studio.information_architect is not None
    assert studio.code_engineer is not None
    assert studio.ui_specialist is not None
    assert studio.anti_slop_critic is not None

if __name__ == "__main__":
    test_foundation_tools()
    test_component_catalog()
    test_ui_agents_creation()
    test_ui_tasks_creation()
    test_ui_studio_crew_assembly()
    print("All UI Design Studio Crew tests passed successfully!")


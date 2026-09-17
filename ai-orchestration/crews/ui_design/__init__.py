from .agents import (
    create_requirement_analyst_agent,
    create_creative_direction_generator_agent,
    create_design_director_agent,
    create_information_architect_agent,
    create_ui_component_specialist_agent,
    create_visual_patcher_agent,
    create_anti_slop_critic_agent,
)
from .tasks import (
    create_requirement_analysis_task,
    create_creative_directions_task,
    create_design_director_task,
    create_media_strategy_task,
    create_layout_planning_task,
    create_bespoke_ui_synthesis_task,
    create_anti_slop_audit_task,
    create_visual_patch_task,
)
from .crew import UIDesignStudioCrew

# Backward compatibility alias
create_ui_synthesis_task = create_bespoke_ui_synthesis_task

__all__ = [
    "create_requirement_analyst_agent",
    "create_creative_direction_generator_agent",
    "create_design_director_agent",
    "create_information_architect_agent",
    "create_ui_component_specialist_agent",
    "create_visual_patcher_agent",
    "create_anti_slop_critic_agent",
    "create_requirement_analysis_task",
    "create_creative_directions_task",
    "create_design_director_task",
    "create_media_strategy_task",
    "create_layout_planning_task",
    "create_bespoke_ui_synthesis_task",
    "create_ui_synthesis_task",
    "create_anti_slop_audit_task",
    "create_visual_patch_task",
    "UIDesignStudioCrew",
]


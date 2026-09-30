import os
import sys
from typing import Dict, Any, Optional

# Ensure the internal scripts directory is in sys.path
_SCRIPTS_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "scripts")
if _SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, _SCRIPTS_DIR)

from design_system import generate_design_system
from core import search, search_stack

def get_design_system(
    query: str,
    project_name: str = "Project",
    variance: Optional[int] = None,
    motion: Optional[int] = None,
    density: Optional[int] = None,
) -> Dict[str, Any]:
    """
    Instantly returns a domain-specific design system recommendation (colors, typography,
    layout pattern, key effects, anti-patterns, constraints) from UI/UX Pro Max in <0.02s.
    """
    res = generate_design_system(
        query=query,
        project_name=project_name,
        variance=variance,
        motion=motion,
        density=density,
    )
    return res.get("design_system", {})

def query_design_knowledge(
    query: str,
    domain: Optional[str] = None,
    stack: Optional[str] = None,
    max_results: int = 3,
) -> Dict[str, Any]:
    """Queries specific domain or stack rules from the UI/UX Pro Max database."""
    if stack:
        return search_stack(query=query, stack_name=stack, max_results=max_results)
    return search(query=query, domain=domain, max_results=max_results)

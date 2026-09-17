from typing import Dict, Any, Optional

FOUNDATIONS: Dict[str, Dict[str, Any]] = {
    "ramp": {
        "name": "Ramp Clean",
        "canvas_bg": "#f8fafc",
        "surface": "#ffffff",
        "border": "#e2e8f0",
        "text": "#0f172a",
        "accent": "#10b981",
        "radius": "8px",
        "style_descriptor": "High contrast, neutral cool slate, clean 1px borders, dense financial typography"
    },
    "calcom": {
        "name": "Cal.com Calm",
        "canvas_bg": "#f9fafb",
        "surface": "#ffffff",
        "border": "#e5e7eb",
        "text": "#111827",
        "accent": "#18181b",
        "radius": "8px",
        "style_descriptor": "Calm scheduling minimalism, generous whitespace, subtle neutral borders"
    },
    "raycast": {
        "name": "Raycast Keyboard",
        "canvas_bg": "#0c0d0e",
        "surface": "#161719",
        "border": "#27282b",
        "text": "#f4f4f5",
        "accent": "#ff6363",
        "radius": "10px",
        "style_descriptor": "Deep dark developer terminal, monospace keyboard badges, electric coral accents"
    },
    "railway": {
        "name": "Railway Terminal",
        "canvas_bg": "#0b0d0e",
        "surface": "#13111c",
        "border": "#28203d",
        "text": "#f3e8ff",
        "accent": "#c084fc",
        "radius": "8px",
        "style_descriptor": "Dark violet cloud infrastructure, terminal telemetry logs, purple accents"
    },
    "attio": {
        "name": "Attio Fluid",
        "canvas_bg": "#ffffff",
        "surface": "#f4f4f5",
        "border": "#e4e4e7",
        "text": "#18181b",
        "accent": "#3b82f6",
        "radius": "6px",
        "style_descriptor": "Modern data-dense CRM layout, electric blue accents, compact card padding"
    },
    "mintlify": {
        "name": "Mintlify Knowledge",
        "canvas_bg": "#0f172a",
        "surface": "#1e293b",
        "border": "#334155",
        "text": "#f8fafc",
        "accent": "#10b981",
        "radius": "8px",
        "style_descriptor": "Modern developer documentation style, dark surfaces, emerald accents, crisp code blocks"
    }
}

def get_foundation_tokens(foundation_key: str) -> Dict[str, Any]:
    """Returns the design foundation specifications and token tokens."""
    key = foundation_key.lower().strip()
    return FOUNDATIONS.get(key, FOUNDATIONS["ramp"])

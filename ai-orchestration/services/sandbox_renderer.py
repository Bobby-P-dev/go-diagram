"""
Sandbox Headless Browser Renderer
Renders bespoke Tailwind HTML in a headless Chrome/Chromium browser,
capturing real screenshots for multimodal Visual Design Critic evaluation.
"""

import os
import json
import re
import shutil
import base64
import tempfile
import subprocess
import logging
from typing import Optional, Dict, Any

logger = logging.getLogger("SandboxRenderer")

def find_browser_binary() -> Optional[str]:
    """Locates an available Chrome or Chromium binary."""
    explicit_path = os.getenv("CHROME_PATH")
    if explicit_path and os.path.exists(explicit_path):
        return explicit_path

    candidates = [
        "/usr/bin/chromium",
        "/usr/bin/chromium-browser",
        "/usr/bin/google-chrome-stable",
        "/usr/bin/google-chrome",
    ]
    for c in candidates:
        if os.path.exists(c) and os.access(c, os.X_OK):
            return c

    for name in ["chromium", "chromium-browser", "google-chrome-stable", "google-chrome"]:
        found = shutil.which(name)
        if found:
            return found

    return None

def wrap_with_full_document(html_content: str, theme: Optional[Dict[str, Any]] = None) -> str:
    """Match the canvas fragment baseline while preserving authored full documents.

    Fonts, styles, and layout authored in the artifact take precedence over these
    defaults. Review must not introduce a dark canvas or serif pairing of its own.
    """
    if re.search(r"<!doctype\s+html|<html(?:\s|>)", html_content, re.IGNORECASE):
        return html_content
    theme = theme or {}
    is_dark = theme.get("mode") == "dark"
    primary = theme.get("primary")
    colors = {"brand": primary, "primary": primary} if isinstance(primary, str) and primary else {}
    config = json.dumps({"darkMode": "class", "theme": {"extend": {"colors": colors}}}).replace("<", "\\u003c")
    dark_class = ' class="dark"' if is_dark else ""
    background = "#020617" if is_dark else "transparent"
    foreground = "#f1f5f9" if is_dark else "inherit"
    return f"""<!DOCTYPE html>
<html lang="en"{dark_class}>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>UI viewport review</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script>tailwind.config = {config};</script>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <style>
    body {{
      margin: 0; padding: 0; min-height: 100vh;
      background-color: {background}; color: {foreground};
    }}
  </style>
</head>
<body{dark_class}>
{html_content}
</body>
</html>"""

def render_html_to_screenshot(
    html_content: str,
    width: int = 1024,
    height: int = 720,
    timeout_sec: int = 15,
    theme: Optional[Dict[str, Any]] = None,
) -> Optional[str]:
    """
    Renders HTML inside headless Chrome/Chromium and returns base64 PNG data URI.
    Returns None if browser binary is unavailable or rendering fails.
    """
    if not html_content or not html_content.strip():
        logger.warning("Empty HTML content provided for screenshot rendering.")
        return None

    browser_bin = find_browser_binary()
    if not browser_bin:
        logger.warning("No Chrome or Chromium binary found. Visual review is unavailable.")
        return None

    # The caller supplies actual artifact viewport dimensions, not a marketing
    # desktop width. Bound process resources even for malformed metadata.
    width = max(240, min(int(width), 3840))
    height = max(240, min(int(height), 2160))
    timeout_sec = max(1, min(int(timeout_sec), 30))
    full_html = wrap_with_full_document(html_content, theme)

    tmp_html = None
    tmp_png = None
    try:
        # Create temp HTML file
        with tempfile.NamedTemporaryFile(suffix=".html", delete=False, mode="w", encoding="utf-8") as f:
            f.write(full_html)
            tmp_html = f.name

        # Create temp PNG target path
        with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as f:
            tmp_png = f.name

        cmd = [
            browser_bin,
            "--headless=new",
            "--disable-gpu",
            "--no-sandbox",
            "--disable-dev-shm-usage",
            "--hide-scrollbars",
            "--force-device-scale-factor=1",
            "--run-all-compositor-stages-before-draw",
            # Allow CDN styles, fonts, and image decoding a bounded settling window.
            # This is best-effort readiness, not proof that every remote asset loaded.
            "--virtual-time-budget=3000",
            f"--timeout={max(500, (timeout_sec - 1) * 1000)}",
            f"--window-size={width},{height}",
            f"--screenshot={tmp_png}",
            f"file://{tmp_html}",
        ]

        logger.info(f"Running headless render: {browser_bin} (viewport: {width}x{height})")
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout_sec)

        if res.returncode != 0:
            logger.error(f"Chrome render error (code {res.returncode}): {res.stderr.decode('utf-8', errors='ignore')}")
            return None

        if os.path.exists(tmp_png) and os.path.getsize(tmp_png) > 100:
            with open(tmp_png, "rb") as pf:
                png_bytes = pf.read()
            b64 = base64.b64encode(png_bytes).decode("utf-8")
            logger.info(f"Successfully captured screenshot: {len(png_bytes)} bytes")
            return f"data:image/png;base64,{b64}"
        else:
            logger.warning("Rendered screenshot file is empty or missing.")
            return None

    except subprocess.TimeoutExpired:
        logger.error(f"Headless browser render timed out after {timeout_sec}s.")
        return None
    except Exception as e:
        logger.error(f"Failed to render screenshot: {e}", exc_info=True)
        return None
    finally:
        if tmp_html and os.path.exists(tmp_html):
            try:
                os.remove(tmp_html)
            except Exception:
                pass
        if tmp_png and os.path.exists(tmp_png):
            try:
                os.remove(tmp_png)
            except Exception:
                pass

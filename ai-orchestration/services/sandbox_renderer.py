"""
Sandbox Headless Browser Renderer
Renders bespoke Tailwind HTML in a headless Chrome/Chromium browser,
capturing real screenshots for multimodal Visual Design Critic evaluation.
"""

import os
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

def wrap_with_full_document(html_content: str) -> str:
    """Wraps HTML fragment into a complete self-contained document with Tailwind CDN & fonts."""
    if "<!DOCTYPE html>" in html_content or "<html" in html_content:
        return html_content

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Sandbox UI Render</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800;900&family=Playfair+Display:ital,wght@0,500;0,700;1,500&family=JetBrains+Mono:wght@400;500;700&display=swap" rel="stylesheet">
  <script>
    tailwind.config = {{
      theme: {{
        extend: {{
          fontFamily: {{
            sans: ['Inter', 'sans-serif'],
            serif: ['Playfair Display', 'serif'],
            mono: ['JetBrains Mono', 'monospace'],
          }},
        }}
      }}
    }}
  </script>
  <style>
    body {{
      margin: 0;
      padding: 0;
      -webkit-font-smoothing: antialiased;
    }}
  </style>
</head>
<body class="bg-slate-950 text-slate-100 min-h-screen">
{html_content}
</body>
</html>"""

def render_html_to_screenshot(
    html_content: str,
    width: int = 1440,
    height: int = 900,
    timeout_sec: int = 15,
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
        logger.warning("No Chrome or Chromium binary found. Visual Critic will fallback to structured inspection.")
        return None

    full_html = wrap_with_full_document(html_content)

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

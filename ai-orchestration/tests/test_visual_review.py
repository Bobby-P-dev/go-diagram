"""Regression checks for evidence-based review and canvas-aligned screenshots."""
import json
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import MagicMock, patch
import unittest


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from services import sandbox_renderer as renderer
from services import visual_critic_service as critic


class ProviderTestCase(unittest.TestCase):
    def setUp(self):
        client = MagicMock()
        client.__enter__.return_value = client
        factory = MagicMock(return_value=client)
        self.provider = factory, client
        patches = [
            patch.object(critic, "settings", SimpleNamespace(openai_api_key="test-key", openai_base_url="https://vision.example/v1", openai_model="test-vision")),
            patch.dict(sys.modules, {"openai": SimpleNamespace(OpenAI=factory)}),
        ]
        for item in patches:
            item.start()
            self.addCleanup(item.stop)


def response_for(client, payload):
    client.chat.completions.create.return_value = SimpleNamespace(choices=[
        SimpleNamespace(message=SimpleNamespace(content=json.dumps(payload)))
    ])


def complete_review(**updates):
    result = dict(status="pass", overall_visual_score=8.5, composition_score=8.5,
                  visual_hierarchy_score=8.5, whitespace_balance_score=8.5,
                  distinctiveness_score=8.5, has_excessive_empty_space=False,
                  has_generic_template_feel=False, critique_summary="Visible viewport is legible.", issues=[])
    result.update(updates)
    return result


class VisualReviewTests(ProviderTestCase):
    def test_missing_screenshot_is_unavailable_without_provider_call(self):
        provider = self.provider
        factory, _ = provider
        result = critic.critique_screenshot("", "compact login")
        assert result.status == "unavailable"
        assert result.overall_visual_score is None
        factory.assert_not_called()


    def test_failed_vision_does_not_invent_pass_or_leak_error(self):
        provider = self.provider
        factory, client = provider
        client.chat.completions.create.side_effect = RuntimeError("private-provider-detail")
        result = critic.critique_screenshot("data:image/png;base64,AAAA", "compact login")
        assert result.status == "unavailable"
        assert result.overall_visual_score is None
        assert "private-provider-detail" not in result.critique_summary
        assert factory.call_args.kwargs["timeout"] == 25.0
        assert factory.call_args.kwargs["max_retries"] == 0


    def test_incomplete_or_invalid_review_is_unavailable(self):
        _, client = self.provider
        for payload in [
            {"status": "pass"}, complete_review(overall_visual_score=None),
            complete_review(overall_visual_score=100), complete_review(overall_visual_score=True),
            complete_review(overall_visual_score=float("nan")),
        ]:
            with self.subTest(payload=payload):
                response_for(client, payload)
                result = critic.critique_screenshot("data:image/png;base64,AAAA", "dashboard")
                assert result.status == "unavailable"
                assert result.overall_visual_score is None


    def test_review_cannot_pass_with_low_score_or_high_issue(self):
        _, client = self.provider
        for updates in [
            {"overall_visual_score": 7.0},
            {"issues": [{"target": "submit", "type": "contrast", "severity": "high",
                         "problem": "Label invisible", "fix_direction": "Increase text contrast"}]},
        ]:
            with self.subTest(updates=updates):
                response_for(client, complete_review(**updates))
                result = critic.critique_screenshot("data:image/png;base64,AAAA", "dashboard")
                assert result.status == "revise"


    def test_review_preserves_original_request_and_accepts_evidenced_pass(self):
        provider = self.provider
        _, client = provider
        response_for(client, complete_review())
        prompt = "Layar login Indonesia monokrom tanpa foto"
        result = critic.critique_screenshot("data:image/png;base64,AAAA", prompt, "light")
        assert result.status == "pass"
        sent = client.chat.completions.create.call_args.kwargs["messages"]
        assert prompt in sent[1]["content"][0]["text"]
        assert "application screen" in sent[0]["content"]
        assert "Do not demand a hero" in sent[0]["content"]


    def test_empty_html_is_unavailable_without_provider_call(self):
        provider = self.provider
        factory, _ = provider
        result = critic.critique_html("", "compact login")
        assert result.status == "unavailable"
        assert result.overall_visual_score is None
        factory.assert_not_called()


    def test_html_review_preserves_request_and_triggers_revision(self):
        _, client = self.provider
        response_for(client, complete_review(overall_visual_score=6.5, issues=[
            {"target": "sec-hero", "type": "composition", "severity": "high", "problem": "Rigid layout", "fix_direction": "Vary grid"}
        ]))
        result = critic.critique_html("<main><div>Test</div></main>", "dashboard", "dark")
        assert result.status == "revise"
        assert result.overall_visual_score == 6.5
        assert len(result.issues) == 1



    def test_fragment_render_does_not_force_dark_theme_or_serif_font(self):
        document = renderer.wrap_with_full_document('<main class="font-serif">Editorial</main>')
        assert 'class="dark"' not in document
        assert "Playfair" not in document
        assert 'background-color: transparent' in document


    def test_dark_theme_and_brand_match_canvas_and_config_is_script_safe(self):
        document = renderer.wrap_with_full_document("<main>Hello</main>", {"mode": "dark", "primary": "</script>"})
        assert '<html lang="en" class="dark">' in document
        assert '"brand": "\\u003c/script>"' in document


    def test_complete_document_is_preserved(self):
        original = '<!doctype html><HTML lang="id"><head><style>body{font-family:serif}</style></head><body>Hi</body></HTML>'
        assert renderer.wrap_with_full_document(original, {"mode": "dark"}) == original


    def test_renderer_uses_requested_viewport_and_cleans_temp_files(self):
        observed = {}

        def run(command, **kwargs):
            observed["command"] = command
            observed["timeout"] = kwargs["timeout"]
            html_path = Path(command[-1].removeprefix("file://"))
            png_path = Path(next(x.split("=", 1)[1] for x in command if x.startswith("--screenshot=")))
            observed["paths"] = [html_path, png_path]
            observed["html"] = html_path.read_text()
            png_path.write_bytes(b"\x89PNG\r\n\x1a\n" + b"x" * 200)
            return SimpleNamespace(returncode=0)

        with patch.object(renderer, "find_browser_binary", return_value="/fake/chromium"), patch.object(renderer.subprocess, "run", run):
            result = renderer.render_html_to_screenshot("<main>Light UI</main>", width=375, height=812, theme={"mode": "light"})
        assert result.startswith("data:image/png;base64,")
        assert "--window-size=375,812" in observed["command"]
        assert "--virtual-time-budget=3000" in observed["command"]
        assert observed["timeout"] == 15
        assert 'class="dark"' not in observed["html"]
        assert all(not path.exists() for path in observed["paths"])

if __name__ == "__main__":
    unittest.main()

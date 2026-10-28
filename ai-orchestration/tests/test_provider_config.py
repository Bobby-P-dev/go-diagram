import importlib.util
import os
from pathlib import Path
import unittest
from unittest.mock import patch


CONFIG_PATH = Path(__file__).resolve().parents[1] / "config.py"


class ProviderConfigTests(unittest.TestCase):
    def load_settings(self, environment):
        with patch.dict(os.environ, environment, clear=True), patch("dotenv.load_dotenv"):
            spec = importlib.util.spec_from_file_location("isolated_provider_config", CONFIG_PATH)
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            return module.settings

    def test_meta_key_aliases(self):
        for alias in ("MODEL_API_KEY", "META_API_KEY"):
            with self.subTest(alias=alias):
                settings = self.load_settings({
                    alias: "mock-meta-key",
                    "OPEN_AI_MODEL": "muse-spark-1.3",
                    "OPEN_AI_BASE_URL": "https://api.meta.ai/v1",
                })
                self.assertEqual(settings.ai_provider, "openai")
                self.assertEqual(settings.openai_api_key, "mock-meta-key")
                self.assertEqual(settings.openai_model, "muse-spark-1.3")
                self.assertEqual(settings.openai_base_url, "https://api.meta.ai/v1")

    def test_existing_alias_precedence_matches_go(self):
        settings = self.load_settings({
            "MODEL_API_KEY": "mock-meta-key",
            "OPEN_AI_API_KEY": "mock-old-key",
            "OPENAI_API_KEY": "mock-preferred-key",
            "OPEN_AI_MODEL": "other-model",
            "OPENAI_MODEL": "preferred-model",
            "OPEN_AI_BASE_URL": "https://other.invalid/v1",
            "OPENAI_BASE_URL": "https://preferred.invalid/v1",
        })
        self.assertEqual(settings.openai_api_key, "mock-preferred-key")
        self.assertEqual(settings.openai_model, "preferred-model")
        self.assertEqual(settings.openai_base_url, "https://preferred.invalid/v1")

    def test_secrets_have_no_fallback(self):
        settings = self.load_settings({})
        self.assertEqual(settings.openai_api_key, "")
        self.assertEqual(settings.internal_service_key, "")


if __name__ == "__main__":
    unittest.main()

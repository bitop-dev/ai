import os
import unittest


class Phase0DocumentationTests(unittest.TestCase):
    def setUp(self):
        repo_root = os.path.dirname(os.path.dirname(__file__))
        self.phase0_path = os.path.join(repo_root, "tasks", "phase-0.md")

    def test_phase0_doc_exists(self):
        self.assertTrue(os.path.exists(self.phase0_path))

    def test_phase0_doc_has_layout_section(self):
        with open(self.phase0_path, "r", encoding="utf-8") as handle:
            content = handle.read()

        self.assertIn("## Proposed Directory Layout", content)

    def test_phase0_doc_has_naming_conventions(self):
        with open(self.phase0_path, "r", encoding="utf-8") as handle:
            content = handle.read()

        self.assertIn("## Naming Conventions", content)
        self.assertIn("<module>/pkg/<name>", content)

    def test_phase0_doc_has_package_mapping_inventory(self):
        with open(self.phase0_path, "r", encoding="utf-8") as handle:
            content = handle.read()

        expected_entries = [
            "`packages/ai`",
            "`packages/provider`",
            "`packages/provider-utils`",
            "`packages/gateway`",
            "`packages/mcp`",
            "`packages/test-server`",
            "`packages/valibot`",
            "`packages/devtools`",
            "`packages/codemod`",
            "`packages/amazon-bedrock`",
            "`packages/anthropic`",
            "`packages/assemblyai`",
            "`packages/azure`",
            "`packages/baseten`",
            "`packages/black-forest-labs`",
            "`packages/cerebras`",
            "`packages/cohere`",
            "`packages/deepgram`",
            "`packages/deepinfra`",
            "`packages/deepseek`",
            "`packages/elevenlabs`",
            "`packages/fal`",
            "`packages/fireworks`",
            "`packages/gladia`",
            "`packages/google`",
            "`packages/google-vertex`",
            "`packages/groq`",
            "`packages/hume`",
            "`packages/lmnt`",
            "`packages/luma`",
            "`packages/mistral`",
            "`packages/openai`",
            "`packages/openai-compatible`",
            "`packages/perplexity`",
            "`packages/prodia`",
            "`packages/replicate`",
            "`packages/revai`",
            "`packages/togetherai`",
            "`packages/vercel`",
            "`packages/xai`",
            "`packages/huggingface`",
            "`packages/langchain`",
            "`packages/llamaindex`",
            "`packages/angular`",
            "`packages/react`",
            "`packages/rsc`",
            "`packages/svelte`",
            "`packages/vue`",
            "Inventory validated against `ai-sdk-6/packages`",
        ]

        for entry in expected_entries:
            self.assertIn(entry, content)


if __name__ == "__main__":
    unittest.main()

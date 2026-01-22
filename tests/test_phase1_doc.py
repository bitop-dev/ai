import os
import unittest


class Phase1DocumentationTests(unittest.TestCase):
    def setUp(self):
        repo_root = os.path.dirname(os.path.dirname(__file__))
        self.phase1_path = os.path.join(repo_root, "tasks", "phase-1.md")

    def test_phase1_doc_exists(self):
        self.assertTrue(os.path.exists(self.phase1_path))

    def test_phase1_doc_has_module_identity(self):
        with open(self.phase1_path, "r", encoding="utf-8") as handle:
            content = handle.read()

        self.assertIn("### Module Identity", content)
        self.assertIn("github.com/vercel/ai-sdk-go", content)
        self.assertIn("semantic versioning with `v0`", content)

    def test_phase1_doc_has_module_naming(self):
        with open(self.phase1_path, "r", encoding="utf-8") as handle:
            content = handle.read()

        self.assertIn("<module>/pkg/<name>", content)
        self.assertIn("<module>/pkg/providers/<provider>", content)


if __name__ == "__main__":
    unittest.main()

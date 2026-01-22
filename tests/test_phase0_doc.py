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


if __name__ == "__main__":
    unittest.main()

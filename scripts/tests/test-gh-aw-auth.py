from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


class WorkflowAuthTests(unittest.TestCase):
    def test_sources_and_generated_job_permissions(self):
        paths = sorted((ROOT / ".github/workflows").glob("*.lock.yml"))
        self.assertTrue(paths)
        for path in paths:
            with self.subTest(workflow=path.name):
                source = path.with_suffix("").with_suffix(".md").read_text().split("---", 2)[1]
                self.assertIn("    type: github-oidc\n    provider: anthropic\n", source)
                for block, token in (
                    ("checkout:\n", "  github-token: ${{ secrets.GITHUB_TOKEN }}"),
                    ("safe-outputs:\n", "  github-token: ${{ secrets.GITHUB_TOKEN }}"),
                    ("  github:\n", "    github-token: ${{ secrets.GITHUB_TOKEN }}"),
                ):
                    self.assertIn(block + token, source)
                text = path.read_text()
                jobs = dict(re.findall(r"(?ms)^  (\w+):\n(.*?)(?=^  \w+:\n|\Z)", text.split("\njobs:\n", 1)[1]))
                permissions = re.search(r"(?m)^    permissions:\n((?:      [^\n]+\n)+)", jobs["agent"])[1]
                for line in permissions.splitlines():
                    self.assertTrue(line.endswith(": read") or line.strip() == "id-token: write", line)
                safe = re.search(r"(?m)^    permissions:\n((?:      [^\n]+\n)+)", jobs["safe_outputs"])[1]
                expected = "contents" if path.name == "release.lock.yml" else "issues"
                self.assertIn(f"      {expected}: write\n", safe)
                if path.name in ("weekly-repo-status.lock.yml", "kongctl-feature-user-agent.lock.yml"):
                    self.assertIn("      discussions: write\n", safe)
                conclusion = re.search(r"(?m)^    permissions:\n((?:      [^\n]+\n)+)", jobs["conclusion"])[1]
                self.assertIn("      issues: write\n", conclusion)
                self.assertIn("      actions: read\n", conclusion)

    def test_benchmark_caller_allows_oidc_in_reusable_workflow(self):
        source = (ROOT / ".github/workflows/declarative-benchmark.yaml").read_text()
        caller = source.split("    uses: ./.github/workflows/benchmark-regression-analysis.lock.yml\n", 1)[1]
        permissions = caller.split("    permissions:\n", 1)[1].split("    with:\n", 1)[0]
        self.assertIn("      id-token: write\n", permissions)


if __name__ == "__main__":
    unittest.main()

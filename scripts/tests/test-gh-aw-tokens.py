import importlib.util
import json
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("tokens", ROOT / "scripts/patch-gh-aw-tokens.py")
tokens = importlib.util.module_from_spec(spec)
spec.loader.exec_module(tokens)


def generated(body, secrets=None, version="v0.86.2"):
    metadata = json.dumps({"compiler_version": version, "agent_id": "claude"})
    manifest = json.dumps({"secrets": secrets or list(tokens.OBSOLETE), "actions": []})
    return f"# gh-aw-metadata: {metadata}\n# gh-aw-manifest: {manifest}\n{body}"


class TokenOverlayTests(unittest.TestCase):
    def test_removes_unused_probes_and_preserves_runtime_auth(self):
        original = generated('''jobs:
  activation:
    outputs:
      oauth_token_check_failed: ${{ steps.check-oauth-tokens.outputs.oauth_token_check_failed == 'true' }}
    steps:
      - name: Check for OAuth tokens
        id: check-oauth-tokens
        run: bash "${RUNNER_TEMP}/gh-aw/actions/check_oauth_tokens.sh"
        env:
          COPILOT_GITHUB_TOKEN: ${{ secrets.COPILOT_GITHUB_TOKEN }}
          GH_AW_GITHUB_TOKEN: ${{ secrets.GH_AW_GITHUB_TOKEN }}
          GH_AW_GITHUB_MCP_SERVER_TOKEN: ${{ secrets.GH_AW_GITHUB_MCP_SERVER_TOKEN }}
      - name: Daily guardrail
        env:
          GH_AW_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
      - name: Determine automatic lockdown mode for GitHub MCP Server
        env:
          GH_AW_GITHUB_TOKEN: ${{ secrets.GH_AW_GITHUB_TOKEN }}
          GH_AW_GITHUB_MCP_SERVER_TOKEN: ${{ secrets.GH_AW_GITHUB_MCP_SERVER_TOKEN }}
        with:
          script: determineAutomaticLockdown(github, context, core);
      - name: GitHub MCP
        env:
          GITHUB_MCP_SERVER_TOKEN: ${{ secrets.GITHUB_TOKEN }}
''')
        updated = tokens.normalize(original)
        self.assertNotIn("check-oauth-tokens", updated)
        self.assertNotIn("        env:\n        with:", updated)
        self.assertIn('oauth_token_check_failed: "false"', updated)
        self.assertIn("GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}", updated)
        self.assertIn("GITHUB_MCP_SERVER_TOKEN: ${{ secrets.GITHUB_TOKEN }}", updated)
        self.assertIn("determineAutomaticLockdown(github, context, core)", updated)
        self.assertEqual(tokens.normalize(updated), updated)

    def test_preserves_other_secrets_and_redaction(self):
        names = [*tokens.OBSOLETE, "GITHUB_TOKEN", "KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT"]
        body = "# Secrets used:\n" + "".join(f"#   - {name}\n" for name in names)
        body += "on:\n  workflow_call:\n    secrets:\n"
        body += "".join(f"      {name}:\n        required: false\n" for name in names[:-2])
        body += "      OTHER_SECRET:\n        required: true\n"
        body += "          GH_AW_SECRET_NAMES: '" + ",".join(names) + "'\n"
        body += "".join(f"          SECRET_{name}: ${{{{ secrets.{name} }}}}\n" for name in names)
        updated = tokens.normalize(generated(body, names))
        self.assertIn("OTHER_SECRET:\n        required: true", updated)
        for name in names[-2:]:
            self.assertIn(f"SECRET_{name}: ${{{{ secrets.{name} }}}}", updated)
        manifest = json.loads(updated.splitlines()[1].removeprefix("# gh-aw-manifest: "))
        self.assertEqual(manifest["secrets"], names[-2:])

    def test_preserves_explicit_lockdown_policy(self):
        original = generated('''      - name: Determine automatic lockdown mode for GitHub MCP Server
        id: determine-automatic-lockdown
        uses: actions/github-script@pinned
        env:
          GH_AW_GITHUB_TOKEN: ${{ secrets.GH_AW_GITHUB_TOKEN }}
          GH_AW_GITHUB_MCP_SERVER_TOKEN: ${{ secrets.GH_AW_GITHUB_MCP_SERVER_TOKEN }}
          GH_AW_GITHUB_MIN_INTEGRITY: 'none'
        with:
          script: determineAutomaticLockdown(github, context, core);
''')
        updated = tokens.normalize(original)
        self.assertIn("        env:\n          GH_AW_GITHUB_MIN_INTEGRITY: 'none'\n", updated)

    def test_refuses_unexpected_consumers_and_compiler_versions(self):
        for body in (
            "github-token: ${{ secrets.GH_AW_GITHUB_TOKEN || secrets.GITHUB_TOKEN }}\n",
            "COPILOT_GITHUB_TOKEN: ${{ secrets.ANOTHER_TOKEN }}\n",
            "          GH_AW_GITHUB_TOKEN: ${{ secrets.GH_AW_GITHUB_TOKEN }}\n",
        ):
            with self.subTest(body=body), self.assertRaisesRegex(ValueError, "unexpected remaining"):
                tokens.normalize(generated(body))
        with self.assertRaisesRegex(ValueError, "review the token overlay"):
            tokens.normalize(generated("", version="v1.0.1"))

    def test_repository_workflows_have_no_obsolete_credential_references(self):
        obsolete = (*tokens.OBSOLETE, "PAT", "GH_TOKEN_PRIVATE_READ", "GET_KONGHQ_COM_TOKEN", "TAP_GITHUB_TOKEN")
        for path in (ROOT / ".github/workflows").iterdir():
            if path.suffix not in (".yaml", ".yml", ".md"):
                continue
            text = path.read_text()
            for name in obsolete:
                # Cover expression dot/bracket syntax and reusable declarations.
                pattern = rf"secrets\s*(?:\.\s*{name}\b|\[\s*['\"]{name}['\"]\s*\])|^\s*{name}:\s*$"
                self.assertIsNone(re.search(pattern, text, re.MULTILINE), f"{path}: {name}")

    def test_sources_and_generated_job_permissions(self):
        paths = sorted((ROOT / ".github/workflows").glob("*.lock.yml"))
        self.assertEqual(len(paths), 8)
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
                self.assertEqual(tokens.normalize(text), text)
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

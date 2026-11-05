#!/usr/bin/env python3
"""Remove unconditional credential probes emitted by gh-aw v0.86.2.

Authentication itself is configured in the Markdown sources. This generator
overlay handles only compiler-injected metadata and unused credential probes;
unexpected credential consumers fail rather than silently changing their auth.
"""

import argparse
import json
from pathlib import Path
import re


OBSOLETE = (
    "GH_AW_GITHUB_TOKEN",
    "GH_AW_GITHUB_MCP_SERVER_TOKEN",
    "COPILOT_GITHUB_TOKEN",
)


def normalize(text):
    if not text.startswith("# gh-aw-metadata: "):
        raise ValueError("expected a gh-aw generated lock file")
    metadata = json.loads(text.splitlines()[0].removeprefix("# gh-aw-metadata: "))
    if metadata["compiler_version"] != "v0.86.2":
        raise ValueError("review the token overlay before changing gh-aw versions")
    if metadata.get("agent_id") != "claude":
        raise ValueError("the token overlay only supports Claude workflows")

    # These workflows use Claude with Anthropic OIDC, never Copilot auth.
    # Keep the activation output contract used by conclusion/failure handlers.
    oauth_env = "".join(
        f"          {name}: ${{{{ secrets.{name} }}}}\n"
        for name in ("COPILOT_GITHUB_TOKEN", "GH_AW_GITHUB_TOKEN", "GH_AW_GITHUB_MCP_SERVER_TOKEN")
    )
    text = re.sub(
        r"(?m)^      - name: Check for OAuth tokens\n"
        r"        id: check-oauth-tokens\n"
        r'        run: bash "\$\{RUNNER_TEMP\}/gh-aw/actions/check_oauth_tokens.sh"\n'
        "        env:\n" + re.escape(oauth_env),
        "",
        text,
    )
    text = text.replace(
        "${{ steps.check-oauth-tokens.outputs.oauth_token_check_failed == 'true' }}",
        '"false"',
    )
    text = text.replace(
        '          echo "::error::COPILOT_GITHUB_TOKEN must be configured in the CALLER repository\'s secrets."',
        '          echo "::error::Check Anthropic OIDC federation and caller GITHUB_TOKEN permissions."',
    )

    # The guardrail supports GITHUB_TOKEN directly. Lockdown does not read
    # either of the optional credential variables emitted by the compiler.
    text = text.replace(
        "          GH_AW_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}\n",
        "          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}\n",
    )
    text = re.sub(
        r"(?m)(^      - name: Determine automatic lockdown mode for GitHub MCP Server\n"
        r"(?:        (?:id|uses):[^\n]+\n)*)"
        r"        env:\n"
        r"          GH_AW_GITHUB_TOKEN: \$\{\{ secrets.GH_AW_GITHUB_TOKEN \}\}\n"
        r"          GH_AW_GITHUB_MCP_SERVER_TOKEN: \$\{\{ secrets.GH_AW_GITHUB_MCP_SERVER_TOKEN \}\}\n",
        r"\1        env:\n",
        text,
    )
    text = text.replace("        env:\n        with:\n", "        with:\n")
    for name in OBSOLETE:
        text = text.replace(f"          SECRET_{name}: ${{{{ secrets.{name} }}}}\n", "")
        text = text.replace(f"      {name}:\n        required: false\n", "")
        text = text.replace(f"#   - {name}\n", "")

    def redact_names(match):
        names = [name for name in match[1].split(",") if name not in OBSOLETE]
        return "          GH_AW_SECRET_NAMES: '" + ",".join(names) + "'"

    text = re.sub(r"(?m)^          GH_AW_SECRET_NAMES: '([^']*)'", redact_names, text)

    def manifest(match):
        data = json.loads(match[1])
        data["secrets"] = [name for name in data["secrets"] if name not in OBSOLETE]
        return "# gh-aw-manifest: " + json.dumps(data, separators=(",", ":"))

    text = re.sub(r"(?m)^# gh-aw-manifest: (.*)$", manifest, text)
    for name in OBSOLETE:
        if name in text:
            raise ValueError(f"unexpected remaining {name} consumer; configure its source token")
    return text


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="fail if regeneration is needed")
    parser.add_argument("paths", nargs="*", type=Path)
    args = parser.parse_args()
    paths = args.paths or sorted(Path(".github/workflows").glob("*.lock.yml"))
    if not paths:
        raise SystemExit("no generated agentic workflows found")
    for path in paths:
        original = path.read_text()
        try:
            updated = normalize(original)
        except ValueError as error:
            raise SystemExit(f"{path}: {error}") from error
        if updated != original:
            if args.check:
                raise SystemExit(f"{path}: run make compile-gh-aw")
            path.write_text(updated)
            print(f"patched token metadata in {path}")


if __name__ == "__main__":
    main()

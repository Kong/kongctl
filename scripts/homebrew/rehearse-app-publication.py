#!/usr/bin/env python3
"""Exercise tap publication with marker-only changes to the existing release."""

import base64
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys


REPO = "Kong/homebrew-kongctl"
TARGETS = {"cask": "Casks/kongctl.rb", "formula": "Formula/kongctl.rb"}


def gh(*args, payload=None):
    result = subprocess.run(
        ["gh", *args],
        input=None if payload is None else json.dumps(payload),
        text=True, capture_output=True, check=True,
    )
    return json.loads(result.stdout)


def marker():
    run_id = os.environ["GITHUB_RUN_ID"]
    if not run_id.isdigit():
        raise ValueError("Invalid rehearsal run ID")
    return f"# kongctl-app-auth-rehearsal: {run_id}\n"


def prepare():
    directory = Path(os.environ["REHEARSAL_DIR"])
    directory.mkdir(parents=True, exist_ok=False)
    versions = {}
    for kind, target in TARGETS.items():
        data = gh("api", f"repos/{REPO}/contents/{target}?ref=main")
        original = base64.b64decode(data["content"]).decode()
        if "# kongctl-app-auth-rehearsal:" in original:
            raise ValueError("A previous rehearsal marker needs cleanup first")
        pattern = r'version "([0-9]+\.[0-9]+\.[0-9]+)"' if kind == "cask" else (
            r"releases/download/v([0-9]+\.[0-9]+\.[0-9]+)/"
        )
        matches = set(re.findall(pattern, original))
        if len(matches) != 1:
            raise ValueError(f"Cannot identify the existing {kind} version")
        versions[kind] = matches.pop()
        (directory / f"{kind}.rb").write_text(marker() + original)
    branch = f"release/kongctl-{versions['formula']}"
    # Do not reuse a real release's existing PR or branch.
    branches = gh("api", "--paginate", "--slurp", f"repos/{REPO}/branches?per_page=100")
    if any(item["name"] == branch for page in branches for item in page):
        raise ValueError(f"Rehearsal would collide with existing branch {branch}")
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        for kind, version in versions.items():
            output.write(f"{kind}_version={version}\n")
    hooks = directory / "hooks"
    hooks.mkdir()
    hook = hooks / "pre-commit"
    hook.write_text(f"#!/bin/sh\nexec python3 {shlex.quote(str(Path(__file__).resolve()))} guard\n")
    hook.chmod(0o755)
    subprocess.run(["git", "config", "--global", "core.hooksPath", str(hooks)], check=True)


def guard():
    files = subprocess.check_output(["git", "diff", "--cached", "--name-only"], text=True).splitlines()
    if len(files) != 1 or files[0] not in TARGETS.values():
        raise ValueError("Rehearsal can change only one tap recipe at a time")
    target = files[0]
    before = subprocess.check_output(["git", "show", f"HEAD:{target}"], text=True)
    after = subprocess.check_output(["git", "show", f":{target}"], text=True)
    if after != marker() + before:
        raise ValueError("Rehearsal commit must only prepend its marker; refusing recipe changes")


def cleanup():
    for target in TARGETS.values():
        for attempt in range(3):
            data = gh("api", f"repos/{REPO}/contents/{target}?ref=main")
            current = base64.b64decode(data["content"]).decode()
            if marker() not in current:
                break
            cleaned = current.replace(marker(), "")
            try:
                gh("api", "--method", "PUT", f"repos/{REPO}/contents/{target}", "--input", "-", payload={
                    "message": f"test: remove App auth rehearsal marker ({os.environ['GITHUB_RUN_ID']})",
                    "content": base64.b64encode(cleaned.encode()).decode(),
                    "sha": data["sha"], "branch": "main",
                })
            except subprocess.CalledProcessError:
                if attempt == 2:
                    raise
                continue
            print(f"Removed rehearsal marker from {target}")
            break


if __name__ == "__main__":
    {"prepare": prepare, "guard": guard, "cleanup": cleanup}[sys.argv[1]]()

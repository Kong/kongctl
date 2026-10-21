#!/usr/bin/env python3
"""Bind each release handoff to its source and an exact SHA-256 inventory."""

import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys


def require(condition, message):
    if not condition:
        raise ValueError(message)


def source():
    commit = os.environ["RELEASE_COMMIT"]
    tag = os.environ["RELEASE_TAG"]
    require(len(commit) == 40 and all(c in "0123456789abcdef" for c in commit),
            "Invalid release commit")
    require(subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip() == commit,
            "Unexpected checkout commit")
    require(subprocess.check_output(
        ["git", "rev-parse", f"refs/tags/{tag}^{{commit}}"], text=True
    ).strip() == commit, "Release tag does not match source")
    if os.environ["RELEASE_BUILD_MODE"] == "full":
        require(os.environ["GITHUB_REPOSITORY"] == "Kong/kongctl", "Unexpected repository")
        require(os.environ["GITHUB_REF"] == "refs/heads/main", "Release must run from main")
        subprocess.run(
            ["git", "merge-base", "--is-ancestor", commit, "refs/remotes/origin/main"],
            check=True,
        )
    return {"repository": os.environ["GITHUB_REPOSITORY"], "commit": commit,
            "tag": tag, "mode": os.environ["RELEASE_BUILD_MODE"]}


def inventory(root):
    files = {}
    for path in sorted(root.rglob("*")):
        require(not path.is_symlink(), f"Symlink in handoff: {path}")
        if path.is_dir():
            continue
        require(path.is_file(), f"Non-file in handoff: {path}")
        name = path.relative_to(root).as_posix()
        if name != "handoff.json":
            with path.open("rb") as stream:
                files[name] = hashlib.file_digest(stream, "sha256").hexdigest()
    require(files, "Empty handoff")
    return files


if __name__ == "__main__":
    operation, directory = sys.argv[1:]
    root = Path(directory)
    manifest = root / "handoff.json"
    expected = {"schema": 1, "source": source(), "files": inventory(root)}
    if operation == "record":
        manifest.write_text(json.dumps(expected, indent=2) + "\n")
    elif operation == "verify":
        require(not manifest.is_symlink(), "Symlinked manifest")
        require(json.loads(manifest.read_text()) == expected, "Release handoff mismatch")
    else:
        raise ValueError(operation)

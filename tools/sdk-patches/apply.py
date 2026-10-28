#!/usr/bin/env python3
"""Install kongctl's SDK patch bundle and apply its post-generation fixes."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys


BUNDLE = Path(__file__).resolve().parent
SNAPSHOT = "kongctl-patches"
POST_GENERATE = f"\tpython3 {SNAPSHOT}/apply.py post-generate .\n"


def configure_workflow(text, manifest):
    """Preserve upstream overlays while replacing this bundle's entries."""
    names = set(manifest["overlays"])
    section = re.search(r"^        overlays:\n(?:            - location: .*\n)*", text, re.M)
    entries = []
    if section:
        for line in section.group().splitlines()[1:]:
            location = line.split("location: ", 1)[1]
            if location.startswith(f"./{SNAPSHOT}/overlays/"):
                continue
            if location.startswith("./.speakeasy/overlays/") and Path(location).name in names:
                continue
            entries.append(line + "\n")
    entries.extend(f"            - location: ./{SNAPSHOT}/overlays/{name}\n" for name in manifest["overlays"])
    replacement = "        overlays:\n" + "".join(entries)
    if section:
        text = text[:section.start()] + replacement + text[section.end():]
    else:
        text, count = re.subn(r"^        registry:\n", replacement + "        registry:\n", text, flags=re.M)
        if count != 1:
            raise ValueError("expected one Konnect registry block in workflow.yaml")
    text, count = re.subn(r"^speakeasyVersion: .*", "speakeasyVersion: " + manifest["speakeasy_version"], text, flags=re.M)
    if count != 1:
        raise ValueError("expected one Speakeasy version in workflow.yaml")
    return text


def replace_tag(text, patch):
    before, after = patch["before"], patch["after"]
    optional_field = patch.get("optional_field")
    if optional_field and not re.search(r"^\s*" + re.escape(optional_field) + r"\s", text, re.M):
        return text
    if text.count(before) == 1 and after not in text:
        return text.replace(before, after, 1)
    if before not in text and text.count(after) == 1:
        return text
    raise ValueError(f"{patch['file']}: expected exactly one original or patched field; SDK schema may have changed")


def post_generate(sdk, manifest):
    replacements = []
    for patch in manifest["post_generation"]:
        path = sdk / patch["file"]
        replacements.append((path, replace_tag(path.read_text(), patch)))
    for path, text in replacements:
        path.write_text(text)


def prepare(sdk, manifest):
    if sdk == BUNDLE or BUNDLE.is_relative_to(sdk):
        raise ValueError("prepare must run from the kongctl source bundle, targeting a separate SDK checkout")
    revision = subprocess.check_output(["git", "-C", str(BUNDLE), "rev-parse", "HEAD"], text=True).strip()
    status = subprocess.check_output(["git", "-C", str(BUNDLE), "status", "--porcelain", "--", "."], text=True)
    if status:
        raise ValueError("commit the SDK patch bundle in kongctl before preparing an SDK snapshot")
    workflow = sdk / ".speakeasy/workflow.yaml"
    workflow_text = configure_workflow(workflow.read_text(), manifest)
    makefile = sdk / "Makefile"
    make_text = makefile.read_text()
    anchor = "\t$(MAKE) _generate.omitempty\n"
    if make_text.count(anchor) != 1:
        raise ValueError("expected one upstream post-generation hook in SDK Makefile")
    if POST_GENERATE not in make_text:
        make_text = make_text.replace(anchor, anchor + POST_GENERATE, 1)
    generation = r"^generate\.sdk\.speakeasy: speakeasy(?: validate\.kongctl-overlays)?$"
    make_text, count = re.subn(generation, "generate.sdk.speakeasy: speakeasy validate.kongctl-overlays", make_text, flags=re.M)
    if count != 1:
        raise ValueError("expected one Speakeasy generation target in SDK Makefile")
    if ".PHONY: validate.kongctl-overlays\n" not in make_text:
        make_text += (
            "\n.PHONY: validate.kongctl-overlays\nvalidate.kongctl-overlays:\n"
            "\t@for overlay in $(sort $(wildcard kongctl-patches/overlays/*.yaml)); do \\\n"
            "\t\tspeakeasy overlay validate --overlay $$overlay; \\\n"
            "\t\tspeakeasy overlay apply --strict --schema $(OPENAPI_FILE) --overlay $$overlay --out /dev/null; \\\n"
            "\tdone\n"
        )
    else:
        make_text = make_text.replace("$(wildcard .speakeasy/overlays/*.yaml)", "$(wildcard kongctl-patches/overlays/*.yaml)")
    mise = sdk / ".mise/config.toml"
    mise_text, count = re.subn(r'^version = "[^"]+"$', f'version = "{manifest["speakeasy_version"]}"', mise.read_text(), flags=re.M)
    if count != 1:
        raise ValueError("expected one pinned Speakeasy tool in .mise/config.toml")

    files = ["apply.py", "manifest.json", "README.md"]
    files += ["overlays/" + name for name in manifest["overlays"]]
    files += list(manifest["tests"])
    hashes = {}
    for name in files:
        source, target = BUNDLE / name, sdk / SNAPSHOT / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        hashes[name] = hashlib.sha256(source.read_bytes()).hexdigest()
    provenance = {"repository": "Kong/kongctl", "commit": revision, "path": "tools/sdk-patches", "sha256": hashes}
    (sdk / SNAPSHOT / "provenance.json").write_text(json.dumps(provenance, indent=2) + "\n")
    for source, target in manifest["tests"].items():
        shutil.copyfile(BUNDLE / source, sdk / target)
    # Remove the previous kongctl-owned copies after installing the snapshot.
    for name in manifest["overlays"]:
        (sdk / ".speakeasy/overlays" / name).unlink(missing_ok=True)
    workflow.write_text(workflow_text)
    makefile.write_text(make_text)
    mise.write_text(mise_text)
    print(f"Prepared SDK patch snapshot from Kong/kongctl@{revision}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["prepare", "post-generate"])
    parser.add_argument("sdk", type=Path, help="SDK checkout directory")
    args = parser.parse_args()
    manifest = json.loads((BUNDLE / "manifest.json").read_text())
    try:
        if args.command == "prepare":
            prepare(args.sdk.resolve(), manifest)
        else:
            post_generate(args.sdk.resolve(), manifest)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        print(f"SDK patch failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

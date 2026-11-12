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
    section = re.search(r"^        overlays:\n(?:(?: {12,}.*|[ \t]*)\n)*", text, re.M)
    entries = []
    if section:
        blocks = re.findall(r"^            -[^\n]*\n(?: {13,}.*\n)*", section.group(), re.M)
        for block in blocks:
            match = re.search(r"^\s*(?:-\s+)?location:\s*(.+?)\s*$", block, re.M)
            if not match:
                raise ValueError("expected a location in each SDK overlay entry")
            location = match.group(1).strip("\"'")
            if location.startswith(f"./{SNAPSHOT}/overlays/"):
                continue
            if location.startswith("./.speakeasy/overlays/") and Path(location).name in names:
                continue
            entries.append(block)
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
    if text.count(after) == 1 and before not in text.replace(after, "", 1):
        return text
    raise ValueError(f"{patch['file']}: expected exactly one original or patched field; SDK schema may have changed")


FIELD = re.compile(r'^\t(\w+)\s+([^`\n]+?)\s+`([^`]+)`$', re.M)
WRAPPER = "optionalnullable.OptionalNullable["


def typed_presence(text, policy):
    """Keep generated field types while preserving absence, null and values."""
    models = policy["models"]
    found = set(re.findall(r"^type (\w+) struct ", text, re.M))
    if found != set(models):
        raise ValueError(f"{policy['file']}: config model inventory changed")
    docs = {}
    for name, model in models.items():
        pattern = r"^type " + re.escape(name) + r" struct \{(.*?)^\}"
        match = re.search(pattern, text, re.M | re.S)
        body = match[1]
        fields = []
        optional = {}
        for field in FIELD.finditer(body):
            field_name, typ, tags = field.groups()
            if field_name == "AdditionalProperties":
                if typ.strip() != "map[string]any" or tags != 'additionalProperties:"true" json:"-"':
                    raise ValueError(f"{name}: unexpected additional properties field")
                continue
            key_match = re.search(r'json:"([^",]+)', tags)
            if key_match is None or key_match[1] == "-":
                raise ValueError(f"{name}.{field_name}: missing or unsupported json tag")
            key = key_match[1]
            original = typ.strip()
            if original.startswith(WRAPPER):
                inner = original[len(WRAPPER):-1]
                original = inner if inner.startswith(("[]", "map[")) else "*" + inner
            fields.append([field_name, original, key])
            if key not in model["required"]:
                if not original.startswith(("*", "[]", "map[")):
                    raise ValueError(f"{name}.{field_name}: optional field shape changed")
                inner = original.removeprefix("*")
                optional[field_name] = (original, WRAPPER + inner + "]", key)
        digest = hashlib.sha256(json.dumps(fields, separators=(",", ":")).encode()).hexdigest()
        if digest != model["fields_sha256"]:
            raise ValueError(
                f"{name}: generated fields changed; SDK schema may have changed; "
                f"expected fields_sha256={model['fields_sha256']}, computed fields_sha256={digest}; "
                f"review normalized fields: {json.dumps(fields, separators=(',', ':'))}"
            )
        required = re.search(
            r"func \(\w+ \*" + re.escape(name) +
            r"\) UnmarshalJSON.*?utils.UnmarshalJSON\([^\n]+false, ([^\n]+)\);", text, re.S,
        )
        if required is None or re.findall(r'"([^"]+)"', required[1]) != model["required"]:
            raise ValueError(f"{name}: required fields changed")

        def patch_field(field):
            field_name, _, tags = field.groups()
            if field_name not in optional:
                return field[0]
            _, wrapped, key = optional[field_name]
            tags = re.sub(r'default:"[^"]*"\s*', "", tags)
            tags = re.sub(r'json:"[^"]+"', f'json:"{key},omitempty"', tags)
            return f"\t{field_name} {wrapped} `{tags}`"

        body = FIELD.sub(patch_field, body)
        if "AdditionalProperties" not in {field[1] for field in FIELD.finditer(body)}:
            body += '\tAdditionalProperties map[string]any `additionalProperties:"true" json:"-"`\n'
        text = text[:match.start(1)] + body + text[match.end(1):]
        for field_name, (original, _, _) in optional.items():
            getter = re.search(
                r"func \((\w+) \*" + re.escape(name) + r"\) Get" + re.escape(field_name) +
                r"\(\) ([^\n{]+) \{(.*?)^\}", text, re.M | re.S,
            )
            if getter is None or getter[2].strip() != original:
                raise ValueError(f"{name}.Get{field_name}: getter shape changed")
            receiver = getter[1]
            start = f"if {receiver} == nil {{ return nil }}"
            old = start + f" return {receiver}.{field_name}"
            new = start + f" value, _ := {receiver}.{field_name}.Get()"
            if original.startswith("*"):
                new += " return value"
            else:
                new += " if value == nil { return nil }; return *value"
            compact = lambda value: re.sub(r"\s+", "", value).replace(";", "")
            if compact(getter[3]) not in (compact(old), compact(new)):
                raise ValueError(f"{name}.Get{field_name}: getter body changed")
            new = new.replace(" } value", " }\nvalue").replace("() return", "()\nreturn")
            new = new.replace("() if", "()\nif").replace(" }; return", " }\nreturn")
            text = text[:getter.start(3)] + "\n" + new + "\n" + text[getter.end(3):]
        docs[name] = optional
    text = add_go_import(text, "github.com/Kong/sdk-konnect-go/optionalnullable")
    text = subprocess.check_output(["gofmt"], input=text, text=True)
    return text, docs


def add_go_import(text, path):
    if re.search(r'^\s*(?:import\s+)?"' + re.escape(path) + r'"\s*$', text, re.M):
        return text
    block = re.search(r'^import\s*\(\s*\n', text, re.M)
    if block:
        return text[:block.end()] + f'\t"{path}"\n' + text[block.end():]
    single = re.search(r'^import[ \t]+((?:[\w.]+[ \t]+)?"[^"\n]+")\s*$', text, re.M)
    if single:
        return text[:single.start()] + f'import (\n\t{single[1]}\n\t"{path}"\n)\n' + text[single.end():]
    raise ValueError(f"cannot add Go import {path}: unsupported import declaration")


def typed_presence_docs(text, optional):
    sections = list(re.finditer(r'^## Fields\s*\n.*?(?=^#{1,2} |\Z)', text, re.M | re.S))
    if len(sections) != 1:
        raise ValueError("expected one config documentation Fields section")
    section = sections[0]
    tables = list(re.finditer(r'^\|[^\n]*\n(?:^\|[^\n]*(?:\n|$))+', section[0], re.M))
    if not tables:
        raise ValueError("missing config documentation Fields table")
    table_match = tables[0]
    table = table_match[0]
    for name, (original, wrapped, _) in optional.items():
        pattern = r"^(\|\s*`" + re.escape(name) + r"`\s*\|)([^|]*)(\|.*)$"

        def replace_type(match):
            link = re.search(r'\]\(([^)]+)\)', match[2])
            value = f"`{wrapped}`"
            inner = original.lstrip("*[]")
            if link:
                value = f"[{value}]({link[1]})"
            elif re.fullmatch(r"[A-Z]\w*", inner):
                value = f"[{value}](../../models/components/{inner.lower()}.md)"
            return match[1] + " " + value + " " + match[3]

        table, count = re.subn(pattern, replace_type, table, flags=re.M)
        if count != 1:
            raise ValueError(f"missing or duplicate config documentation field {name}")
    if "`AdditionalProperties`" not in table:
        table = table.rstrip("\n") + "\n| `AdditionalProperties` | map[string]any | :heavy_minus_sign: | Additional API fields preserved alongside typed fields. |\n"
    updated = section[0][:table_match.start()] + table + section[0][table_match.end():]
    return text[:section.start()] + updated + text[section.end():]


def post_generate(sdk, manifest):
    replacements = {}
    for patch in manifest["post_generation"]:
        path = sdk / patch["file"]
        if patch.get("optional_file") and not path.exists():
            continue
        text = replacements[path] if path in replacements else path.read_text()
        replacements[path] = replace_tag(text, patch)
    for patch in manifest.get("post_generation_doc_sections", []):
        path = sdk / patch["file"]
        if patch.get("optional_file") and not path.exists():
            continue
        text = replacements[path] if path in replacements else path.read_text()
        replacements[path] = replace_doc_section(text, patch)
    for policy in manifest.get("typed_presence", []):
        path = sdk / policy["file"]
        source = replacements[path] if path in replacements else path.read_text()
        replacements[path], models = typed_presence(source, policy)
        if directory := policy.get("docs_directory"):
            for name, optional in models.items():
                doc = sdk / directory / (name.lower() + ".md")
                replacements[doc] = typed_presence_docs(doc.read_text(), optional)
    for path, text in replacements.items():
        path.write_text(text)


def replace_doc_section(text, patch):
    pattern = r"^## " + re.escape(patch["section"]) + r"\n.*?(?=^## |\Z)"
    matches = list(re.finditer(pattern, text, re.M | re.S))
    if len(matches) != 1:
        raise ValueError(f"{patch['file']}: expected one documentation section {patch['section']}")
    section = matches[0]
    updated = replace_tag(section.group(), patch)
    imports = list(re.finditer(r"^import[ \t]*\(\n.*?^\)", updated, re.M | re.S))
    if len(imports) != 1:
        raise ValueError(f"{patch['file']}: expected one Go example import block in {patch['section']}")
    block = imports[0]
    new_imports = block.group()
    for name in patch["imports"]:
        if f'"{name}"' not in new_imports:
            new_imports = re.sub(r"^import[ \t]*\(\n", f'import (\n\t"{name}"\n', new_imports, count=1)
    updated = updated[:block.start()] + new_imports + updated[block.end():]
    return text[:section.start()] + updated + text[section.end():]


def configure_makefile(text, manifest):
    anchor = "\t$(MAKE) _generate.omitempty\n"
    if text.count(anchor) != 1:
        raise ValueError("expected one upstream post-generation hook in SDK Makefile")
    if POST_GENERATE not in text:
        text = text.replace(anchor, anchor + POST_GENERATE, 1)
    generation = r"^generate\.sdk\.speakeasy: speakeasy(?: validate\.kongctl-overlays)?$"
    text, count = re.subn(generation, "generate.sdk.speakeasy: speakeasy validate.kongctl-overlays", text, flags=re.M)
    if count != 1:
        raise ValueError("expected one Speakeasy generation target in SDK Makefile")
    if ".PHONY: validate.kongctl-overlays\n" not in text:
        text += (
            "\n.PHONY: validate.kongctl-overlays\nvalidate.kongctl-overlays:\n"
            "\t@for overlay in $(sort $(wildcard kongctl-patches/overlays/*.yaml)); do \\\n"
            "\t\tspeakeasy overlay validate --overlay $$overlay; \\\n"
            "\t\tspeakeasy overlay apply --strict --schema $(OPENAPI_FILE) --overlay $$overlay --out /dev/null; \\\n"
            "\tdone\n"
        )
    else:
        text = text.replace("$(wildcard .speakeasy/overlays/*.yaml)", "$(wildcard kongctl-patches/overlays/*.yaml)")
    if manifest.get("typed_presence"):
        test_hook = "\t$(MAKE) test.kongctl-patches\n"
        if test_hook not in text:
            text = text.replace(POST_GENERATE, POST_GENERATE + test_hook, 1)
        if ".PHONY: test.kongctl-patches\n" not in text:
            text += (
                "\n.PHONY: test.kongctl-patches\ntest.kongctl-patches:\n"
                "\tCGO_ENABLED=0 go test -count=1 -tags=kongctl_sdk_patched .\n"
            )
    return text


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
    make_text = configure_makefile(makefile.read_text(), manifest)
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

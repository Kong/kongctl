#!/usr/bin/env python3
"""Estimate token usage for files and skills using tiktoken.

This script provides:
- General file token estimates via `--file`
- Skill token estimates via `--skill`
- Optional skill breakdown (`--split-skill`) into:
  - name+description
  - full frontmatter block
  - body
  - full file
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

try:
    import tiktoken
except ImportError as exc:  # pragma: no cover - runtime dependency check
    raise SystemExit(
        "tiktoken is required. Install with: pip install tiktoken"
    ) from exc


def resolve_encoding(model: str):
    """Resolve tokenizer encoding for a model, falling back when unknown."""
    try:
        return tiktoken.encoding_for_model(model), False
    except KeyError:
        try:
            return tiktoken.get_encoding("cl100k_base"), True
        except Exception:
            raise RuntimeError(
                "Unable to load tiktoken encoding (cl100k_base). "
                "Ensure network access on first run or use a pre-populated tiktoken cache."
            )
    except Exception:
        try:
            return tiktoken.get_encoding("cl100k_base"), True
        except Exception:
            raise RuntimeError(
                f"Unable to load tiktoken encoding for model '{model}'. "
                "Ensure network access on first run or use a pre-populated tiktoken cache."
            )


def count_tokens(text: str, encoding) -> int:
    """Count tokens for text using the provided tiktoken encoding."""
    return len(encoding.encode(text))


def resolve_skill_md(skill_path: Path) -> Path:
    """Resolve a skill directory or SKILL.md path to the SKILL.md file."""
    if skill_path.is_dir():
        skill_md = skill_path / "SKILL.md"
    else:
        skill_md = skill_path

    if not skill_md.exists():
        raise FileNotFoundError(f"Skill file not found: {skill_md}")
    if skill_md.name != "SKILL.md":
        raise ValueError(
            f"Expected SKILL.md path or skill directory, got: {skill_path}"
        )
    return skill_md


def split_frontmatter(content: str) -> tuple[str, str]:
    """Split markdown into frontmatter block and body."""
    match = re.match(r"^---\r?\n(.*?)\r?\n---\r?\n?", content, flags=re.DOTALL)
    if not match:
        return "", content

    frontmatter = content[: match.end()]
    body = content[match.end() :]
    return frontmatter, body


def extract_name_description(frontmatter: str) -> str:
    """Extract only name and description text from frontmatter.

    This parser intentionally stays lightweight and does not require YAML libs.
    It supports common folded description formatting used in skills.
    """
    if not frontmatter:
        return ""

    inner_match = re.match(r"^---\r?\n(.*?)\r?\n---\r?\n?$", frontmatter, flags=re.DOTALL)
    if not inner_match:
        return ""
    inner = inner_match.group(1)
    lines = inner.splitlines()

    name = ""
    description = ""

    for idx, line in enumerate(lines):
        if line.startswith("name:") and not name:
            name = line.split(":", 1)[1].strip()
            continue

        if line.startswith("description:") and not description:
            value = line.split(":", 1)[1].strip()
            parts = [value] if value else []

            next_idx = idx + 1
            while next_idx < len(lines):
                next_line = lines[next_idx]
                if next_line.startswith((" ", "\t")):
                    parts.append(next_line.strip())
                    next_idx += 1
                    continue
                break

            description = " ".join(p for p in parts if p).strip()

    return f"name: {name}\ndescription: {description}\n"


def print_header(model: str, encoding_name: str, fallback: bool) -> None:
    """Print metadata shared by all output modes."""
    print(f"model={model}")
    print(f"encoding={encoding_name}")
    if fallback:
        print("encoding_fallback=true")


def run_file_mode(file_path: Path, model: str) -> int:
    """Estimate token usage for a single file."""
    if not file_path.exists():
        raise FileNotFoundError(f"File not found: {file_path}")

    text = file_path.read_text()
    encoding, fallback = resolve_encoding(model)
    print_header(model, encoding.name, fallback)
    print(f"path={file_path.resolve()}")
    print(f"tokens={count_tokens(text, encoding)}")
    return 0


def run_skill_mode(skill_arg: Path, model: str, split_skill: bool) -> int:
    """Estimate token usage for skill content."""
    skill_md = resolve_skill_md(skill_arg)
    content = skill_md.read_text()
    frontmatter, body = split_frontmatter(content)

    encoding, fallback = resolve_encoding(model)
    print_header(model, encoding.name, fallback)
    print(f"path={skill_md.resolve()}")

    if not split_skill:
        print(f"tokens.full_skill={count_tokens(content, encoding)}")
        return 0

    name_description = extract_name_description(frontmatter)
    print(f"tokens.name_description={count_tokens(name_description, encoding)}")
    print(f"tokens.frontmatter_block={count_tokens(frontmatter, encoding)}")
    print(f"tokens.body={count_tokens(body, encoding)}")
    print(f"tokens.full_skill={count_tokens(content, encoding)}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Estimate token counts with tiktoken for files or skills."
    )
    parser.add_argument(
        "--model",
        default="gpt-4o",
        help="Model name used to resolve token encoding (default: gpt-4o).",
    )
    parser.add_argument(
        "--file",
        type=Path,
        help="Path to any text file for general token estimation.",
    )
    parser.add_argument(
        "--skill",
        type=Path,
        help="Path to a skill directory or SKILL.md file.",
    )
    parser.add_argument(
        "--split-skill",
        action="store_true",
        help=(
            "When used with --skill, report name+description/frontmatter/body/full "
            "token estimates."
        ),
    )

    args = parser.parse_args()

    if bool(args.file) == bool(args.skill):
        parser.error("Provide exactly one of --file or --skill.")

    if args.split_skill and not args.skill:
        parser.error("--split-skill requires --skill.")

    try:
        if args.file:
            return run_file_mode(args.file, args.model)
        return run_skill_mode(args.skill, args.model, args.split_skill)
    except Exception as exc:  # pragma: no cover - CLI error reporting
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())

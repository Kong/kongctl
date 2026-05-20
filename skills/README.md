# kongctl Skills

This directory contains human-maintained skills for coding agents that work
with `kongctl`.

## Included Skills

- `kongctl-declarative`
  - Generate declarative config and manage plan/apply/sync/delete/adopt flows.
- `kongctl-extension-builder`
  - Create, validate, and test local kongctl CLI extensions.

## Install in Agent Tooling

Use `kongctl install skills` from the root of the repository where your agent
will work:

```sh
kongctl install skills
```

To preview the files and symlinks before writing them, run:

```sh
kongctl install skills --dry-run
```

By default, the installer writes skill files to `.kongctl/skills/` and creates
symlinks for supported agent tooling:

- `.agents/skills/kongctl-declarative`
- `.agents/skills/kongctl-extension-builder`
- `.claude/skills/kongctl-declarative`
- `.claude/skills/kongctl-extension-builder`

Use `--path` to choose a different directory for installed skill files:

```sh
kongctl install skills --path .tools/kongctl/skills
```

Keep the skill folder names unchanged so each `SKILL.md` is discoverable.

## Manual Install

If the installer is unavailable, copy or symlink each skill directory into your
tool's skills directory.

### Claude Code

- Target path: `.claude/skills/`
- Example:
  - `ln -s ../../skills/kongctl-declarative .claude/skills/kongctl-declarative`
  - `ln -s ../../skills/kongctl-extension-builder .claude/skills/kongctl-extension-builder`

### Codex, Cursor, opencode

- Target path: `.agents/skills/` (some setups use `.agents/skills/`)
- Example:
  - `ln -s ../../skills/kongctl-declarative .agents/skills/kongctl-declarative`
  - `ln -s ../../skills/kongctl-extension-builder .agents/skills/kongctl-extension-builder`

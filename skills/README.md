# kongctl Skills

This directory contains human-maintained skills for coding agents that work
with `kongctl`.

## Included Skills

- `kongctl-query`
  - Read-only discovery and inspection of Konnect resources.
- `kongctl-declarative`
  - Generate declarative config and manage plan/apply/sync/delete/adopt flows.

## Install in Agent Tooling

Copy or symlink each skill directory into your tool's skills directory.

### Claude Code

- Target path: `.claude/skills/`
- Example:
  - `ln -s ../../skills/kongctl-query .claude/skills/kongctl-query`
  - `ln -s ../../skills/kongctl-declarative .claude/skills/kongctl-declarative`

### Codex or Cursor

- Target path: `.agent/skills/` (some setups use `.agents/skills/`)
- Example:
  - `ln -s ../../skills/kongctl-query .agent/skills/kongctl-query`
  - `ln -s ../../skills/kongctl-declarative .agent/skills/kongctl-declarative`

Keep the skill folder names unchanged so each `SKILL.md` is discoverable.

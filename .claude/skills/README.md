# Claude Code Skills Note

Claude Code currently works with individual skill symlinks.

Current layout in this repo:

- `.claude/skills/kongctl-query -> ../../skills/kongctl-query`

Tracking issue:

- <https://github.com/anthropics/claude-code/issues/20755>

Current recommendation:

- Use per-skill symlinks under `.claude/skills/`.
- Do not rely on an aggregate directory symlink such as
  `.claude/skills/kongctl-skills -> ../../skills`.

If Claude Code behavior changes in a future release, re-test aggregate symlinks.

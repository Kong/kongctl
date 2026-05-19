# kongctl Pulse

kongctl Pulse is a generated status dashboard for the repository's current
GitHub milestones. It produces:

- a portfolio page at `docs/reports/milestones/index.html`
- one `<Milestone> Pulse` page per current milestone
- a Markdown digest at `docs/reports/milestones/latest.md`
- JSON report data in `docs/reports/milestones/data/`, including the latest
  full snapshot and compact chart history

Generate the report locally with:

```sh
scripts/milestone-pulse.sh --repo Kong/kongctl
```

Use `--state all` to include closed milestones in addition to current open
milestones.

The generated pages use the official Kong logo assets already checked into this
repository and do not modify or recolor the logo.

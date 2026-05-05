# kongctl Pulse

kongctl Pulse is a generated status dashboard for the repository's current
GitHub milestones. It produces:

- a portfolio page at `docs/reports/milestones/index.html`
- one `<Milestone> Pulse` page per current milestone
- a Markdown digest at `docs/reports/milestones/latest.md`
- daily JSON snapshots in `docs/reports/milestones/data/`

The scheduled `kongctl Pulse` GitHub Actions workflow refreshes these files
daily. Run it locally with:

```sh
scripts/milestone-pulse.sh --repo Kong/kongctl
```

Use `--state all` to include closed milestones in addition to current open
milestones.

The generated pages use the official Kong logo assets already checked into this
repository and do not modify or recolor the logo.

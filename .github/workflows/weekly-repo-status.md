---
description: |
  This workflow creates a weekly repo status report. It gathers recent repository
  activity (issues, PRs, discussions, releases, code changes) and generates
  engaging GitHub discussions with productivity insights, community highlights,
  and project recommendations.

on:
  schedule: weekly
  workflow_dispatch:

permissions:
  contents: read
  issues: read
  pull-requests: read

network: defaults

tools:
  github:
    # If in a public repo, setting `lockdown: false` allows
    # reading issues, pull requests and comments from 3rd-parties
    # If in a private repo this has no particular effect.
    lockdown: false

safe-outputs:
  create-discussion:
    title-prefix: "[ai] "        # prefix for titles
    category: "announcements"    # category slug, name, or ID (use lowercase, prefer announcement-capable)
    expires: 3                   # auto-close after 3 days (or false to disable)
    max: 3                       # max discussions (default: 1)
    fallback-to-issue: true      # fallback to issue creation on permission errors (default: true)
source: githubnext/agentics/workflows/daily-repo-status.md@69b5e3ae5fa7f35fa555b0a22aee14c36ab57ebb
---

# Weekly Repo Status

Create an upbeat weekly status report for the repo as a GitHub Discussion.

## What to include

- Recent repository activity (issues, PRs, discussions, releases, code changes)
- Progress tracking, goal reminders and highlights
- Project status and recommendations
- Actionable next steps for maintainers

## Style

- Be positive, encouraging, and helpful 🌟
- Use emojis lightly for engagement
- Keep it concise - adjust length based on actual activity

## Process

1. Gather recent activity from the repository
2. Study the repository, its issues and its pull requests
3. Create a new GitHub discussion with your findings and insights

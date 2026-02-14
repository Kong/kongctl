---
on:
  slash_command:
    name: triage
    events: [issues, issue_comment]  # Only in issue bodies and issue comments
  reaction: eyes

imports:
  - shared/issue-triage-fm.md

timeout-minutes: 10
source: githubnext/agentics/workflows/issue-triage.md@69b5e3ae5fa7f35fa555b0a22aee14c36ab57ebb
---

{{#import shared/issue-triage.md}}

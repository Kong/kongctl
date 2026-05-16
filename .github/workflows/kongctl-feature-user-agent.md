---
name: Kongctl Feature User Agent
description: |
  Runs a manual feature-user evaluation of one advertised kongctl workflow
  against a disposable Konnect org and files actionable friction only.
on:
  workflow_dispatch:
permissions:
  contents: read
  issues: read
checkout:
  fetch-depth: 0
engine:
  id: copilot
  model: claude-opus-4.6
strict: true
timeout-minutes: 60
environment: kongctl-user-agent-eval
concurrency:
  group: konnect-feature-user-agent
  cancel-in-progress: false
network:
  allowed:
    - defaults
    - github
    - "*.api.konghq.com"
    - developer.konghq.com
    - docs.konghq.com
tools:
  web-fetch:
  github:
    toolsets: [issues]
    lockdown: false
safe-outputs:
  mentions: false
  allowed-github-references:
    - repo
  noop:
    report-as-issue: false
    max: 1
  create-issue:
    title-prefix: "[agent-eval] "
    labels:
      - automation
      - agentic-workflows
      - konnect
    expires: 30d
    max: 1
pre-agent-steps:
  - name: Build kongctl and reset feature-user org
    env:
      KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT: ${{ secrets.KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT }}
      KONGCTL_FEATURE_USER_AGENT_KONNECT_BASE_URL: ${{ vars.KONGCTL_FEATURE_USER_AGENT_KONNECT_BASE_URL || 'https://us.api.konghq.com' }}
    run: |
      set -euo pipefail

      run_dir="/tmp/gh-aw/kongctl-feature-user-agent"
      evidence_dir="/tmp/gh-aw/kongctl-feature-user-agent/sanitized"
      auth_env="${run_dir}/auth.env"
      mkdir -p "${evidence_dir}"
      umask 077
      {
        printf 'KONGCTL_DEFAULT_KONNECT_PAT=%q\n' "${KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT}"
        printf 'KONGCTL_DEFAULT_KONNECT_BASE_URL=%q\n' "${KONGCTL_FEATURE_USER_AGENT_KONNECT_BASE_URL}"
      } > "${auth_env}"

      export KONGCTL_DEFAULT_KONNECT_PAT="${KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT}"
      export KONGCTL_DEFAULT_KONNECT_BASE_URL="${KONGCTL_FEATURE_USER_AGENT_KONNECT_BASE_URL}"
      export KONGCTL_E2E_KONNECT_PAT="${KONGCTL_FEATURE_USER_AGENT_KONNECT_PAT}"
      export KONGCTL_E2E_KONNECT_BASE_URL="${KONGCTL_FEATURE_USER_AGENT_KONNECT_BASE_URL}"
      export KONGCTL_E2E_RESET=1
      export KONGCTL_E2E_LOG_LEVEL=debug
      export KONGCTL_E2E_CONSOLE_LOG_LEVEL=warn

      cat > "${evidence_dir}/README.md" <<'EOF'
      # Kongctl Feature User Agent Evidence

      This directory is reserved for sanitized evaluation artifacts only.
      Raw command logs, HTTP traces, headers, secrets, and stable Konnect
      identifiers must not be written here.
      EOF

      make build-ci
      test -x ./kongctl
      make reset-org
post-steps:
  - name: Check sanitized feature-user artifacts
    if: always()
    run: |
      set -euo pipefail

      evidence_dir="/tmp/gh-aw/kongctl-feature-user-agent/sanitized"
      if [ ! -d "${evidence_dir}" ]; then
        exit 0
      fi

      if find "${evidence_dir}" -type f -size +1048576c | grep -q .; then
        echo "::error::Sanitized artifact files must be 1 MiB or smaller."
        exit 1
      fi

      unsafe_pattern='([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|Bearer[[:space:]]+[A-Za-z0-9._~+/-]+=*|Authorization:|X-Api-Key:|KONGCTL_[A-Z0-9_]*PAT=|https://[^[:space:]]*[?&](token|signature|X-Amz-Signature)=)'
      if grep -R -E -q "${unsafe_pattern}" "${evidence_dir}"; then
        echo "::error::Sanitized artifacts contain values that look unsafe. Redact or omit them before upload."
        exit 1
      fi
  - name: Upload sanitized feature-user artifacts
    if: always()
    uses: actions/upload-artifact@v7.0.1
    with:
      name: kongctl-feature-user-agent-sanitized
      path: /tmp/gh-aw/kongctl-feature-user-agent/sanitized
      if-no-files-found: ignore
      retention-days: 14
tracker-id: kongctl-feature-user-agent
---

# Kongctl Feature User Agent

You are a feature user evaluating `kongctl` as a command-line product.

Your job is to discover an advertised `kongctl` feature, choose one natural
use case, exercise it against the disposable Konnect org prepared for this run,
capture sanitized evidence, and file feedback only when the friction is
concrete and reproducible.

Emit exactly one completion safe output:

- Use `create_issue` when you found actionable friction.
- Use `noop` when no actionable friction is found.

Do not emit more than one `create_issue` or `noop`. Do not use GitHub write
APIs directly.

## Runtime Context

- Repository: `${{ github.repository }}`
- Workspace: `${{ github.workspace }}`
- Run URL: `${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}`
- Built binary: `./kongctl`
- Auth env file: `/tmp/gh-aw/kongctl-feature-user-agent/auth.env`
- Sanitized artifact directory:
  `/tmp/gh-aw/kongctl-feature-user-agent/sanitized`

The workflow has already built `./kongctl` with `make build-ci` and reset the
dedicated Konnect org with the existing e2e reset helper.

## Editable Agent Guidance

- Use `KONGCTL_DEFAULT_KONNECT_PAT`; do not run `kongctl login`.
- Before running `kongctl` commands, load auth with:
  `set -a; . /tmp/gh-aw/kongctl-feature-user-agent/auth.env; set +a`.
- Never print, quote, copy, summarize, or upload the auth env file or its
  contents.
- Treat the Konnect org as disposable.
- Discover features from `README.md`, `docs/`, `kongctl --help`, command help,
  and public Kong developer docs.
- Pick one advertised feature or workflow without persona steering, feature
  selection memory, repo memory, or history-based steering.
- Prefer one realistic, bounded workflow over broad coverage.
- Capture commands, exit codes, sanitized stdout/stderr excerpts, generated
  config, expected vs. actual behavior, and cleanup result.
- Attempt cleanup when created resources are easy to identify.
- File an issue only for concrete, reproducible friction.
- Emit `noop` when no actionable friction is found.

## Evaluation Process

1. Read the local docs and command help just enough to identify advertised
   workflows.
2. Select exactly one feature workflow and state the reason in your private
   notes or sanitized evidence.
3. Exercise the workflow using `./kongctl` and PAT-based environment auth.
4. Keep command output excerpts short. Prefer command shapes, exit codes, and
   non-identifying error text.
5. If you create resources and can identify them safely, attempt cleanup.
6. Write sanitized evidence files only under:
   `/tmp/gh-aw/kongctl-feature-user-agent/sanitized`.
7. Before final output, run a sanitization check over the issue body and every
   file in the sanitized artifact directory. Rewrite or omit unsafe content.
8. Emit exactly one safe output.

## Redaction Rules

Do not include any of the following in GitHub issues, summaries, or uploaded
artifacts:

- Konnect PATs, bearer tokens, refresh tokens, API keys, cookies, auth headers,
  private keys, certificates, or signed URLs.
- Organization IDs, account IDs, user IDs, team IDs, system account IDs, portal
  IDs, API IDs, control plane IDs, or other stable Konnect UUIDs.
- Email addresses, names of real users, usernames, organization names, team
  names, domains, or URLs that identify the eval org or account.
- Full raw HTTP trace logs, request/response headers, or raw JSON bodies unless
  sanitized first.

Use these masking conventions:

- Replace tokens and keys with `[REDACTED_SECRET]`.
- Replace stable resource IDs and UUIDs with `[REDACTED_ID]`.
- Replace emails and user names with `[REDACTED_IDENTITY]`.
- Replace org/account names or domains with `[REDACTED_ORG]`.
- Keep generic command shapes and non-identifying error text.

Immediately before any `create_issue`, `noop`, or artifact-producing file is
finalized, review the issue body and sanitized artifact files for secrets,
identity data, stable IDs, raw traces, and unsafe URLs. Rewrite or omit unsafe
content rather than trying to preserve it.

## Issue Requirements

Only call `create_issue` for concrete, reproducible friction. The issue must
include:

- The advertised feature/workflow that was evaluated.
- The expected behavior from docs or command help.
- The actual behavior observed.
- Minimal sanitized reproduction steps.
- Sanitized command shapes and exit codes.
- Short sanitized stdout/stderr excerpts when useful.
- Cleanup attempted and result.
- Why this is actionable for maintainers.

If the workflow is successful, the result is ambiguous, or the only observations
are subjective preferences, call `noop` with a concise summary instead.

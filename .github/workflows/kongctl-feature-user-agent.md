---
name: User Agent Eval
description: |
  Runs a scheduled or manual feature-user evaluation of one advertised kongctl
  workflow against a disposable Konnect org, filing actionable friction or
  publishing successful evaluations as expiring discussions.
on:
  schedule:
    # GitHub schedules are UTC-only. 01:00 UTC Tuesday-Saturday maps to
    # Monday-Friday evenings in US/Central: 8 PM during daylight time and
    # 7 PM during standard time.
    - cron: "0 1 * * 2-6"
  workflow_dispatch:
permissions:
  contents: read
  issues: read
checkout:
  fetch-depth: 0
engine:
  id: copilot
  model: claude-opus-4.6
features:
  action-mode: "action"
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
  cache-memory:
    key: >-
      kongctl-feature-user-agent-${{ github.repository_owner }}-${{ github.event.repository.name }}-${{ github.workflow }}
    allowed-extensions: [".json"]
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
  create-discussion:
    title-prefix: "[agent-eval] "
    category: "general"
    labels:
      - automation
      - agentic-workflows
      - konnect
    expires: 30d
    max: 1
    fallback-to-issue: false
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
  - name: Redact UUIDs in sanitized feature-user artifacts
    if: always()
    run: |
      set -euo pipefail

      evidence_dir="/tmp/gh-aw/kongctl-feature-user-agent/sanitized"
      if [ ! -d "${evidence_dir}" ]; then
        exit 0
      fi

      uuid_pattern='[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-'
      uuid_pattern="${uuid_pattern}"'[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-'
      uuid_pattern="${uuid_pattern}"'[0-9a-fA-F]{12}'

      find "${evidence_dir}" -type f \
        \( -name '*.md' -o -name '*.json' -o -name '*.jsonl' -o \
          -name '*.txt' \) \
        -print0 |
      while IFS= read -r -d '' file; do
        perl -0pi -e "s/${uuid_pattern}/[REDACTED_ID]/g" "${file}"
      done
  - name: Append feature-user evaluation summary
    if: always()
    run: |
      set -euo pipefail

      evidence_dir="/tmp/gh-aw/kongctl-feature-user-agent/sanitized"
      summary_file="${evidence_dir}/evaluation-summary.md"

      {
        echo "## User Agent Eval"
        echo
        echo "- Run: ${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
        echo "- Sanitized artifacts: \`kongctl-feature-user-agent-sanitized\`"
        echo
      } >> "${GITHUB_STEP_SUMMARY}"

      if [ ! -f "${summary_file}" ]; then
        {
          echo "No sanitized evaluation summary was produced."
          echo
          echo "Check the agent logs and sanitized artifact upload for details."
        } >> "${GITHUB_STEP_SUMMARY}"
        exit 0
      fi

      unsafe_pattern='([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|Bearer[[:space:]]+[A-Za-z0-9._~+/-]+=*|Authorization:|X-Api-Key:|KONGCTL_[A-Z0-9_]*PAT=|https://[^[:space:]]*[?&](token|signature|X-Amz-Signature)=)'
      if grep -E -q "${unsafe_pattern}" "${summary_file}"; then
        echo "::error::Evaluation summary contains values that look unsafe. Redact or omit them before upload."
        exit 1
      fi

      cat "${summary_file}" >> "${GITHUB_STEP_SUMMARY}"
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
  - name: Hydrate replay prompt into final report safe output
    if: always()
    run: |
      set -euo pipefail

      agent_output="/tmp/gh-aw/agent_output.json"
      replay_prompt="/tmp/gh-aw/kongctl-feature-user-agent/sanitized/selected-use-case-prompt.md"

      if [ ! -f "${agent_output}" ] || [ ! -f "${replay_prompt}" ]; then
        exit 0
      fi

      unsafe_pattern='([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}|[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|Bearer[[:space:]]+[A-Za-z0-9._~+/-]+=*|Authorization:|X-Api-Key:|KONGCTL_[A-Z0-9_]*PAT=|https://[^[:space:]]*[?&](token|signature|X-Amz-Signature)=)'
      if grep -E -q "${unsafe_pattern}" "${replay_prompt}"; then
        echo "::error::Replay prompt contains values that look unsafe. Redact or omit them before safe output creation."
        exit 1
      fi

      node <<'NODE'
      const fs = require("fs");

      const agentOutputPath = "/tmp/gh-aw/agent_output.json";
      const replayPromptPath = "/tmp/gh-aw/kongctl-feature-user-agent/sanitized/selected-use-case-prompt.md";
      const artifactName = "kongctl-feature-user-agent-sanitized";
      const replayPromptArtifactPath = "selected-use-case-prompt.md";
      const maxInlinePromptLength = 30000;
      const maxBodyLength = 64000;
      const runURL = process.env.GITHUB_SERVER_URL &&
        process.env.GITHUB_REPOSITORY &&
        process.env.GITHUB_RUN_ID
        ? `${process.env.GITHUB_SERVER_URL}/${process.env.GITHUB_REPOSITORY}/actions/runs/${process.env.GITHUB_RUN_ID}`
        : "";
      const artifactLocation = runURL
        ? `the \`${artifactName}\` artifact from ${runURL}`
        : `the \`${artifactName}\` workflow run artifact`;

      const data = JSON.parse(fs.readFileSync(agentOutputPath, "utf8"));
      const rawReplayPrompt = fs.readFileSync(replayPromptPath, "utf8").trim();
      if (!rawReplayPrompt) {
        process.exit(0);
      }

      function clippedReplayPrompt(maxLength) {
        if (rawReplayPrompt.length <= maxLength) {
          return rawReplayPrompt;
        }

        return `${rawReplayPrompt.slice(0, maxLength).trimEnd()}

      [Replay prompt truncated in safe output; full sanitized prompt is available in ${artifactLocation} as \`${replayPromptArtifactPath}\`.]`;
      }

      function markdownFenceFor(content) {
        let fence = "```";
        while (content.includes(fence)) {
          fence += "`";
        }
        return fence;
      }

      function replayPromptSection(maxPromptLength = maxInlinePromptLength) {
        const replayPrompt = clippedReplayPrompt(maxPromptLength);
        const fence = markdownFenceFor(replayPrompt);

        return `## Replay Prompt

      Sanitized replay prompt excerpt from \`${replayPromptArtifactPath}\` in the \`${artifactName}\` artifact:

      ${fence}markdown
      ${replayPrompt}
      ${fence}`;
      }

      function hasInlineReplayPrompt(body) {
        const match = body.match(/^## Replay Prompt\s*$/im);
        if (!match) {
          return false;
        }

        const rest = body.slice(match.index);
        const next = rest.slice(match[0].length).search(/\n##\s+/);
        const section = next === -1 ? rest : rest.slice(0, match[0].length + next);

        return section.includes("```") && !/See artifact:\s*\/tmp\/gh-aw\/kongctl-feature-user-agent\/sanitized\/selected-use-case-prompt\.md/i.test(section);
      }

      function hydrateBody(body) {
        if (hasInlineReplayPrompt(body)) {
          return body;
        }

        function buildHydratedBody(maxPromptLength) {
          const section = replayPromptSection(maxPromptLength);
          if (/^## Replay Prompt\s*$/im.test(body)) {
            return body.replace(
              /(^|\r?\n)## Replay Prompt[^\n]*\r?\n[\s\S]*?(?=\r?\n##\s+|$)/i,
              (_, prefix) => `${prefix}${section}`,
            );
          }

          return `${body.trimEnd()}

      ${section}`;
        }

        const hydrated = buildHydratedBody(maxInlinePromptLength);
        if (hydrated.length <= maxBodyLength) {
          return hydrated;
        }

        const overflow = hydrated.length - maxBodyLength;
        const smallerPromptLength = Math.max(1000, maxInlinePromptLength - overflow - 500);

        return buildHydratedBody(smallerPromptLength);
      }

      function finalReportArgs(value) {
        if (!value || typeof value !== "object" || Array.isArray(value)) {
          return undefined;
        }

        if (value.create_issue && typeof value.create_issue === "object") {
          return value.create_issue;
        }

        if (value.create_discussion && typeof value.create_discussion === "object") {
          return value.create_discussion;
        }

        const toolName = String(value.name || value.tool || value.tool_name || value.type || "").toLowerCase();
        const isFinalReport = toolName === "create_issue" ||
          toolName.endsWith(".create_issue") ||
          toolName === "create_discussion" ||
          toolName.endsWith(".create_discussion");
        if (!isFinalReport) {
          return undefined;
        }

        for (const key of ["arguments", "args", "input", "parameters"]) {
          if (value[key] && typeof value[key] === "object" && typeof value[key].body === "string") {
            return value[key];
          }
        }

        if (typeof value.body === "string") {
          return value;
        }

        return undefined;
      }

      let hydrated = 0;
      function visit(value) {
        if (!value || typeof value !== "object") {
          return;
        }

        const args = finalReportArgs(value);
        if (args && typeof args.body === "string") {
          const nextBody = hydrateBody(args.body);
          if (nextBody !== args.body) {
            args.body = nextBody;
            hydrated += 1;
          }
        }

        for (const child of Array.isArray(value) ? value : Object.values(value)) {
          visit(child);
        }
      }

      visit(data);

      if (hydrated > 0) {
        fs.writeFileSync(agentOutputPath, `${JSON.stringify(data, null, 2)}\n`);
        console.log(`Hydrated replay prompt into ${hydrated} final report safe output item(s).`);
      }
      NODE
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

# User Agent Eval

You are a feature user evaluating `kongctl` as a command-line product.

Your job is to choose one advertised `kongctl` feature workflow, exercise it
against the disposable Konnect org prepared for this run, capture sanitized
evidence, and either file concrete reproducible friction or publish a
successful evaluation report.

Emit exactly one completion safe output:

- Use `create_issue` when you found actionable friction.
- Use `create_discussion` when the evaluated workflow succeeds and you have a
  useful sanitized success report.
- Use `noop` only when no meaningful report can be produced, the result is
  incomplete or ambiguous, or the workflow should intentionally stay silent.

Do not emit more than one `create_issue`, `create_discussion`, or `noop`. Do
not use GitHub write APIs directly.

Non-issue observations are allowed, but they are supporting evidence, not a
separate safe output. Write them only to the sanitized observation artifact
described below, then include them in the final report decision.

## Runtime Context

- Repository: `${{ github.repository }}`
- Workspace: `${{ github.workspace }}`
- Run URL: `${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}`
- Run seed: `${{ github.run_id }}`
- Agent engine: `copilot`
- Agent model: `claude-opus-4.6`
- Runtime version env: `GH_AW_VERSION` when available.
- Built binary: `./kongctl`
- Auth env file: `/tmp/gh-aw/kongctl-feature-user-agent/auth.env`
- Sanitized artifact directory:
  `/tmp/gh-aw/kongctl-feature-user-agent/sanitized`
- Cache-memory state file:
  `/tmp/gh-aw/cache-memory/kongctl-feature-user-agent-state.json`

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
- Use recent default-branch code changes and the persistent state file to
  choose a feature workflow before falling back to the run-seeded procedure.
- Do not choose the most prominent, obvious, or familiar workflow unless the
  selection process below lands on it.
- Use only the cache-memory state file for feature-selection memory. Do not
  use persona steering, repo memory, or other history-based steering.
- Prefer one realistic, bounded workflow over broad coverage.
- Capture commands, exit codes, sanitized stdout/stderr excerpts, generated
  config, expected vs. actual behavior, and cleanup result.
- After selecting the workflow and before running workflow-specific `kongctl`
  commands, write a sanitized replay prompt to:
  `/tmp/gh-aw/kongctl-feature-user-agent/sanitized/selected-use-case-prompt.md`.
- Always write a sanitized `evaluation-summary.md` file in the sanitized
  artifact directory.
- Always write a sanitized `command-ledger.json` file in the sanitized artifact
  directory.
- Write `observation-summary.md` only when you find useful non-issue product
  feedback.
- Attempt cleanup when created resources are easy to identify.
- File an issue only for concrete, reproducible friction.
- Publish a discussion when the evaluated workflow succeeds and the evidence is
  meaningful.
- Emit `noop` only when no meaningful report can be produced, the result is
  incomplete or ambiguous, or the workflow should intentionally stay silent.

## Persistent State

Use cache-memory exactly as lightweight workflow state.

- Read and write only JSON files under `/tmp/gh-aw/cache-memory/`.
- Use this state file:
  `/tmp/gh-aw/cache-memory/kongctl-feature-user-agent-state.json`.
- If the file does not exist, initialize it.
- Do not store secrets, raw command output, Konnect IDs, or large artifacts.

Use this schema:

```json
{
  "version": 1,
  "recent_exercises": [
    {
      "title": "stable workflow title",
      "result": "create_issue|create_discussion|noop",
      "run_id": "github run id",
      "selected_at": "ISO-8601 UTC timestamp"
    }
  ],
  "processed_recent_shas": []
}
```

Trim `recent_exercises` to the 20 most recent entries and
`processed_recent_shas` to the 50 most recent SHAs before finishing.

## Canonical Feature Matrix

Start from this stable feature matrix, then add any clearly advertised workflow
discovered from current docs or command help:

- `adopt declarative resources`
- `apply declarative api configuration`
- `create personal access token`
- `create system account access token`
- `delete declarative resources`
- `diff declarative configuration`
- `dump declarative configuration`
- `dump terraform import blocks`
- `explain declarative resource schema`
- `get apis listing and details`
- `get gateway control planes`
- `get organization and teams`
- `lint configuration files`
- `list portals`
- `manage analytics dashboards`
- `plan declarative configuration`
- `scaffold declarative resource`
- `sync declarative configuration`

Keep candidate titles stable and lowercase so run-seeded selection is
repeatable.

## Recent Code Signal

Before seeded selection, inspect recent default-branch activity from roughly
the last 72 hours using local git history.

Map recent paths to candidate workflows:

- `internal/cmd/root/verbs/**`: prefer workflows for the touched verb.
- `internal/cmd/root/products/konnect/**`: prefer the touched product resource.
- `internal/declarative/**`: prefer apply, plan, diff, sync, dump, delete,
  adopt, scaffold, explain, or the touched resource type.
- `internal/konnect/**`: prefer workflows that exercise the touched API helper,
  command integration, auth, or HTTP behavior.
- `docs/**` and `README.md`: prefer the advertised workflow whose docs changed.
- Ignore generated lockfiles, unrelated CI-only changes, and test-only changes
  unless they clearly point at a product workflow.

If recent changes map to one or more viable workflows, prefer the best mapped
workflow that is not already present in `recent_exercises`. If all mapped
workflows were recently exercised, choose the least recent mapped workflow.

If no recent change maps cleanly to a viable workflow, fall back to seeded
selection over the canonical matrix plus discovered additions.

## Depth Probe And Command Budget

After the selected workflow's basic happy path succeeds, run exactly one depth
probe from this fixed priority order, choosing the first probe that fits the
workflow:

1. A documented example variant.
2. Output format coverage such as text, JSON, YAML, token, or jq.
3. Invalid input or not-found behavior.
4. Idempotency or repeated execution behavior.
5. Read-after-write verification.
6. Cleanup verification.

Limit workflow-specific `./kongctl` execution to 8 commands total, excluding
repository reads, command-help reads, and one required cleanup command. Stop
early once you have one actionable issue or one useful observation.

## Evidence Artifacts

Write `command-ledger.json` as a JSON array. Each entry must use this shape:

```json
{
  "command_shape": "./kongctl ...",
  "purpose": "why this command was run",
  "exit_code": 0,
  "stdout_excerpt": "short sanitized excerpt or summary",
  "stderr_excerpt": "short sanitized excerpt or summary",
  "assessment": "passed|failed|informational",
  "cleanup": "not-needed|attempted|completed|failed"
}
```

Use command shapes, not raw commands containing secrets, stable IDs, or account
identity data. Keep excerpts short and sanitized.

Before writing any evidence file, replace every UUID-shaped value with
`[REDACTED_ID]`. This includes real resource IDs, stable Konnect IDs, and
synthetic or deliberately non-existent IDs used for not-found tests. In
`command-ledger.json`, command shapes must use placeholders rather than raw
resource IDs.

Write `observation-summary.md` only for useful feedback that is not concrete
enough for a product issue. Use these sections:

- `Observation Summary`
- `Why It Matters`
- `Evidence`
- `Suggested Follow-Up`
- `Safe Output`

In `evaluation-summary.md` and any successful evaluation discussion, include a
`Model Adaptation / Recovery` section that answers:

- Did the first attempted command work?
- Did you inspect help output and change commands?
- Did you retry with a different resource, namespace, flag, or output format?
- Did you encounter expected friction but recover successfully?
- Were failed commands user-facing product friction or normal exploration?

## Evaluation Process

1. Initialize and read the persistent state file.
2. Read the local docs and command help just enough to confirm the canonical
   feature matrix and any newly advertised workflows.
3. Inspect recent default-branch changes from roughly the last 72 hours and
   map them to viable feature workflows.
4. Select exactly one candidate:
   - Prefer a viable recent-code candidate that was not recently exercised.
   - Otherwise use the least recently exercised recent-code candidate.
   - Otherwise sort all viable candidates by stable lowercase title and compute
     selected index as `run seed modulo candidate count`.
   - If the selected candidate proves impossible after deeper inspection, skip
     it, select the next candidate cyclically, and document why it was skipped.
   - Do not re-rank candidates by model preference after sorting.
5. Write the sanitized selected-use-case replay prompt described below.
6. Exercise the selected workflow using `./kongctl` and PAT-based environment
   auth.
7. If the basic path succeeds, run one bounded depth probe from the fixed probe
   list.
8. Keep command output excerpts short. Prefer command shapes, exit codes, and
   non-identifying error text.
9. If you create resources and can identify them safely, attempt cleanup.
10. Write sanitized evidence files only under:
   `/tmp/gh-aw/kongctl-feature-user-agent/sanitized`.
11. Write `command-ledger.json`.
12. Write `/tmp/gh-aw/kongctl-feature-user-agent/sanitized/evaluation-summary.md`
   with these sections:
   - `Agent Runtime`: the engine, model, `GH_AW_VERSION` value when available,
     run URL, and note that gh-aw appends exact workflow engine/version metadata
     to created issues or discussions.
   - `Recent Code Signal`: changed paths considered and mapped workflow, or
     why seeded fallback was used.
   - `Run Seed`: the seed value and candidate count.
   - `Candidate Set`: the sorted candidate titles, selected index, and any
     skipped candidate.
   - `Feature Workflow Selected`: the use case derived from the docs/help.
   - `Why This Workflow`: which input assets advertised it, and how the
     run-seeded selection chose it.
   - `Commands Attempted`: command shapes and exit codes.
   - `Model Adaptation / Recovery`: whether the first command worked, what
     changed after help output or failed attempts, whether alternate resources,
     namespaces, flags, or formats were used, and whether failures represented
     product friction or normal exploration.
   - `Depth Probe`: which probe ran, or why none fit.
   - `Observed Result`: what happened, including short sanitized excerpts.
   - `Success Criteria`: how you decided the workflow succeeded or failed.
   - `Friction Assessment`: why you did or did not find actionable friction.
   - `Cleanup`: cleanup attempted and result.
   - `Command Ledger`: the path to `command-ledger.json`.
   - `Observation`: whether `observation-summary.md` was written, and why.
   - `Selected Use-Case Prompt`: the path to the replay prompt artifact and a
     compact excerpt that is useful for a future rerun.
   - `Safe Output`: whether you emitted `create_issue`, `create_discussion`,
     or `noop`, and why.
13. Update the persistent state file with the selected title, result, run ID,
   timestamp, and recent SHAs considered.
14. Before final output, run a sanitization check over the safe output body and
   every file in the sanitized artifact directory. Rewrite or omit unsafe
   content.
15. Emit exactly one safe output using the decision tree at the top of this
    prompt.

## Selected Use-Case Replay Prompt

The selected-use-case replay prompt should help a maintainer rerun the same
evaluated user intent after a fix, with the same model or a different model. It
is not the full workflow prompt and must not include the pre-use-case candidate
discovery or run-seeded selection instructions.

Write the replay prompt after the selected workflow is known and before
workflow-specific tool or `kongctl` execution begins. Include:

- Agent runtime target: engine `copilot`, model `claude-opus-4.6` or the
  `COPILOT_MODEL` value when available, `GH_AW_VERSION` when available, and the
  run URL for traceability.
- The selected workflow title, advertised source, user intent, and expected
  behavior.
- The exact bounded task to perform against `./kongctl`, including safe command
  shapes, preconditions, success criteria, evidence to collect, and cleanup
  expectations.
- The same auth, artifact directory, and redaction constraints needed to run
  safely in this workflow.

Exclude:

- The full candidate set.
- The run-seeded selection algorithm.
- Any hidden chain-of-thought, private reasoning, secrets, stable Konnect IDs,
  raw HTTP traces, account identity data, or auth file contents.

## Redaction Rules

Do not include any of the following in GitHub issues, discussions, summaries,
or uploaded artifacts:

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

Immediately before any `create_issue`, `create_discussion`, `noop`, or
artifact-producing file is finalized, review the safe output body and sanitized
artifact files for secrets, identity data, stable IDs, raw traces, and unsafe
URLs. Rewrite or omit unsafe content rather than trying to preserve it.

## Final Output Requirements

Only call `create_issue` for concrete, reproducible friction. The issue must
include:

- Agent runtime details: engine, model, `GH_AW_VERSION` when available, run URL,
  and workflow engine/version metadata when available.
- The selected-use-case replay prompt, either in full or as a compact sanitized
  excerpt, plus the artifact path
  `/tmp/gh-aw/kongctl-feature-user-agent/sanitized/selected-use-case-prompt.md`.
  Do not provide only an artifact path; the issue body must contain enough of
  the replay prompt for a maintainer to understand and rerun the use case
  without downloading artifacts.
- The advertised feature/workflow that was evaluated.
- The expected behavior from docs or command help.
- The actual behavior observed.
- Minimal sanitized reproduction steps.
- Sanitized command shapes and exit codes.
- Short sanitized stdout/stderr excerpts when useful.
- Cleanup attempted and result.
- Why this is actionable for maintainers.

Call `create_discussion` when the evaluated workflow succeeds and you have a
meaningful sanitized success report. The discussion must include:

- `Summary`: short outcome statement.
- `Agent Runtime`: engine, model, `GH_AW_VERSION` when available, and run URL.
- `Selected Use Case`: workflow title, why it was selected, and advertised
  source from docs, help, or code signal.
- `Replay Prompt`: compact sanitized excerpt from
  `/tmp/gh-aw/kongctl-feature-user-agent/sanitized/selected-use-case-prompt.md`.
- `Commands Attempted`: table with command shape, purpose, exit code, and
  assessment.
- `Model Adaptation / Recovery`: whether and how the model changed commands,
  resources, namespace, flags, or output format after initial attempts.
- `Depth Probe`: which probe ran and why.
- `Observed Result`: what happened, including short sanitized excerpts when
  useful.
- `Success Criteria`: how success was determined.
- `Cleanup`: resources created, cleanup command shape, and cleanup result.
- `Artifacts`: references to `evaluation-summary.md`, `command-ledger.json`,
  `selected-use-case-prompt.md`, and `observation-summary.md` when written.
- `Safe Output Decision`: why this is a successful evaluation discussion rather
  than an issue.

The discussion body must be useful without downloading artifacts. Artifact
references are supporting links, not a substitute for the required sections.

If the workflow result is ambiguous, the evaluation is incomplete, no
meaningful sanitized report can be produced, or the run should intentionally
stay silent, call `noop` with a concise summary instead.

If the workflow succeeds but reveals useful non-issue product feedback, write
`observation-summary.md`, mention it in `evaluation-summary.md`, update the
state file with result `create_discussion`, and include the observation in the
discussion.

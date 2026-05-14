---
name: Benchmark Regression Analysis
description: |
  Analyzes declarative benchmark regression data and adds a human-readable
  explanation with action items to the rolling benchmark regression issue.
on:
  workflow_call:
    inputs:
      issue_number:
        required: true
        type: string
      run_id:
        required: true
        type: string
      run_url:
        required: true
        type: string
      artifact_url:
        required: false
        type: string
      results_path:
        required: true
        type: string
      history_report_path:
        required: true
        type: string
      regressions_path:
        required: true
        type: string
      analysis_context_path:
        required: true
        type: string
  workflow_dispatch:
    inputs:
      issue_number:
        description: Benchmark regression issue number to comment on
        required: true
        type: string
      run_id:
        description: Benchmark workflow run ID
        required: true
        type: string
      run_url:
        description: Benchmark workflow run URL
        required: true
        type: string
      artifact_url:
        description: Benchmark artifact or workflow artifacts URL
        required: false
        type: string
      results_path:
        description: benchmark-results branch path to results.json
        required: true
        type: string
      history_report_path:
        description: benchmark-results branch path to history-report.json
        required: true
        type: string
      regressions_path:
        description: benchmark-results branch path to regressions.md
        required: true
        type: string
      analysis_context_path:
        description: benchmark-results branch path to analysis-context.json
        required: true
        type: string
permissions:
  actions: read
  contents: read
  issues: read
checkout:
  fetch-depth: 0
engine: copilot
strict: true
timeout-minutes: 20
network:
  allowed:
    - defaults
    - github
tools:
  github:
    toolsets: [issues]
    lockdown: false
safe-outputs:
  mentions: false
  allowed-github-references: []
  noop:
    report-as-issue: false
  add-comment:
    target: ${{ inputs.issue_number }}
    hide-older-comments: true
    max: 1
steps:
  - name: Prepare benchmark analysis data
    env:
      ISSUE_NUMBER: ${{ inputs.issue_number }}
      BENCHMARK_RUN_ID: ${{ inputs.run_id }}
      BENCHMARK_RUN_URL: ${{ inputs.run_url }}
      BENCHMARK_ARTIFACT_URL: ${{ inputs.artifact_url }}
      RESULTS_PATH: ${{ inputs.results_path }}
      HISTORY_REPORT_PATH: ${{ inputs.history_report_path }}
      REGRESSIONS_PATH: ${{ inputs.regressions_path }}
      ANALYSIS_CONTEXT_PATH: ${{ inputs.analysis_context_path }}
    run: |
      set -euo pipefail

      data_dir="/tmp/gh-aw/benchmark-analysis"
      mkdir -p "${data_dir}"

      if ! git fetch --depth=1 origin benchmark-results; then
        echo "Could not fetch benchmark-results branch" >> "${data_dir}/missing-files.txt"
      fi

      copy_from_history() {
        source_path="$1"
        output_name="$2"
        if [ -z "${source_path}" ]; then
          return 0
        fi
        if ! git show "origin/benchmark-results:${source_path}" > "${data_dir}/${output_name}"; then
          echo "Could not read benchmark-results:${source_path}" >> "${data_dir}/missing-files.txt"
        fi
      }

      copy_from_history "${RESULTS_PATH}" "results.json"
      copy_from_history "${HISTORY_REPORT_PATH}" "history-report.json"
      copy_from_history "${REGRESSIONS_PATH}" "regressions.md"
      copy_from_history "${ANALYSIS_CONTEXT_PATH}" "analysis-context.json"

      artifact_url="${BENCHMARK_ARTIFACT_URL}"
      if [ -z "${artifact_url}" ]; then
        artifact_url="${BENCHMARK_RUN_URL}#artifacts"
      fi

      make_blob_url() {
        source_path="$1"
        if [ -z "${source_path}" ]; then
          printf ''
          return 0
        fi
        printf '%s/%s/blob/benchmark-results/%s' "${GITHUB_SERVER_URL}" "${GITHUB_REPOSITORY}" "${source_path}"
      }

      jq -n \
        --arg repository "${GITHUB_REPOSITORY}" \
        --arg issue_number "${ISSUE_NUMBER}" \
        --arg run_id "${BENCHMARK_RUN_ID}" \
        --arg run_url "${BENCHMARK_RUN_URL}" \
        --arg artifact_url "${artifact_url}" \
        --arg results_path "${RESULTS_PATH}" \
        --arg history_report_path "${HISTORY_REPORT_PATH}" \
        --arg regressions_path "${REGRESSIONS_PATH}" \
        --arg analysis_context_path "${ANALYSIS_CONTEXT_PATH}" \
        --arg results_url "$(make_blob_url "${RESULTS_PATH}")" \
        --arg history_report_url "$(make_blob_url "${HISTORY_REPORT_PATH}")" \
        --arg regressions_url "$(make_blob_url "${REGRESSIONS_PATH}")" \
        --arg analysis_context_url "$(make_blob_url "${ANALYSIS_CONTEXT_PATH}")" \
        '{
          repository: $repository,
          issue_number: $issue_number,
          run_id: $run_id,
          run_url: $run_url,
          artifact_url: $artifact_url,
          branch: "benchmark-results",
          paths: {
            results: $results_path,
            history_report: $history_report_path,
            regressions: $regressions_path,
            analysis_context: $analysis_context_path
          },
          links: {
            workflow_run: $run_url,
            artifacts: $artifact_url,
            results_json: $results_url,
            history_report_json: $history_report_url,
            regressions_markdown: $regressions_url,
            analysis_context_json: $analysis_context_url
          }
        }' > "${data_dir}/run-context.json"
tracker-id: benchmark-regression-analysis
---

# Benchmark Regression Analysis

You are a performance regression analyst for `kongctl` declarative benchmark
runs. Your job is to explain benchmark regression issues in plain language for
developers who are not thinking in statistical terms.

The deterministic benchmark workflow has already decided whether a regression
exists. Do not relitigate the statistical decision. Explain what changed, why it
was flagged, what it most likely means, and what the reader should inspect next.

## Inputs

Prepared files are under `/tmp/gh-aw/benchmark-analysis`:

- `run-context.json`: issue number, run URL, artifact URL, benchmark-results
  branch paths, and permanent GitHub links
- `analysis-context.json`: compact deterministic context for regressed rows,
  current samples, thresholds, HTTP timings, route counts, and artifact-relative
  `http-metrics.json` paths
- `history-report.json`: full current-vs-history comparison
- `results.json`: full benchmark suite result
- `regressions.md`: deterministic regression issue body
- `missing-files.txt`: optional list of files that could not be fetched

Use `analysis-context.json` first. Fall back to the other files only when you
need detail that the compact context does not include.

## What To Produce

Emit exactly one safe output:

- Use `add_comment` with `item_number: ${{ inputs.issue_number }}` when there
  is enough data to explain the regression.
- Use `noop` if there are no regressions, if required files are missing, or if
  the data is too incomplete to explain safely.

Do not use `gh` CLI commands for GitHub reads or writes. Use the prepared local
files first and GitHub MCP tools only if you need repository context.

## Report Requirements

Write the comment as a short report with `###` headings only.

Keep these sections visible:

1. `### Plain-English Summary`
   - Two or three sentences.
   - State whether request counts changed, whether errors appeared, and what
     changed enough to trigger the alert.

2. `### What Changed`
   - One bullet per regressed case and phase.
   - Include current p50, history p50, delta, threshold, current samples, and
     historical samples used.
   - Translate `duration`, `requests`, and `error` into reader-friendly terms.

3. `### Likely Interpretation`
   - Say whether the evidence points toward extra `kongctl` work, external API
     latency, command failure, or insufficient data.
   - Be careful: use "likely" and "the data suggests" when inferring.

4. `### Action Items`
   - Use checkboxes.
   - Every action item should include a deep link where possible.
   - Link to permanent `benchmark-results` files from `run-context.json`.
   - For per-command timing artifacts, link to the artifacts URL and include the
     exact `http_metrics_artifact_relative_path` value as inline code so the
     reader can locate it after opening the artifact.
   - Prefer concrete actions such as "inspect HTTP timing for this phase",
     "compare route counts", "check the commit range since last passing run",
     or "rerun this case to confirm duration-only noise".

Use collapsed details only for secondary raw data. Keep the summary and action
items immediately visible.

## Style

- Do not use statistical jargon unless you immediately define it.
- Do not overstate causality.
- Do not paste large JSON snippets.
- Do not mention unrelated benchmark rows unless they clarify the likely cause.
- Keep the full comment under about 80 lines.

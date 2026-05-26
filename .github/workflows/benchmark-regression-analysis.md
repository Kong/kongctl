---
name: Benchmark Regression Analysis
description: |
  Analyzes declarative benchmark regression data and adds a human-readable
  explanation with action items to the rolling benchmark regression issue.
on:
  schedule:
    # GitHub schedules are UTC-only. 06:00 UTC Sunday-Thursday maps to
    # midnight US/Central during standard time and 1 AM during daylight time.
    - cron: "0 6 * * 0-4"
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
    target: "*"
    discussions: false
    pull-requests: false
    hide-older-comments: true
    max: 1
steps:
  - name: Prepare benchmark analysis data
    env:
      GH_TOKEN: ${{ github.token }}
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

      if [ -z "${RESULTS_PATH}" ]; then
        RESULTS_PATH="latest/results.json"
      fi
      if [ -z "${HISTORY_REPORT_PATH}" ]; then
        HISTORY_REPORT_PATH="latest/history-report.json"
      fi
      if [ -z "${REGRESSIONS_PATH}" ]; then
        REGRESSIONS_PATH="latest/regressions.md"
      fi
      if [ -z "${ANALYSIS_CONTEXT_PATH}" ]; then
        ANALYSIS_CONTEXT_PATH="latest/analysis-context.json"
      fi

      run_records_dir="${RESULTS_PATH%/*}"
      if [ "${run_records_dir}" = "${RESULTS_PATH}" ]; then
        run_records_dir="latest"
      fi
      METADATA_PATH="${run_records_dir}/metadata.json"
      REGRESSIONS_JSON_PATH="${run_records_dir}/regressions.json"

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

      copy_optional_from_history() {
        source_path="$1"
        output_name="$2"
        if [ -z "${source_path}" ]; then
          return 0
        fi
        git show "origin/benchmark-results:${source_path}" > "${data_dir}/${output_name}" 2>/dev/null || true
      }

      copy_from_history "${RESULTS_PATH}" "results.json"
      copy_from_history "${HISTORY_REPORT_PATH}" "history-report.json"
      copy_from_history "${REGRESSIONS_PATH}" "regressions.md"
      copy_from_history "${ANALYSIS_CONTEXT_PATH}" "analysis-context.json"
      copy_optional_from_history "${METADATA_PATH}" "metadata.json"
      copy_optional_from_history "${REGRESSIONS_JSON_PATH}" "regressions.json"

      if [ -f "${data_dir}/metadata.json" ]; then
        if [ -z "${BENCHMARK_RUN_ID}" ]; then
          BENCHMARK_RUN_ID="$(jq -r '.run_id // ""' "${data_dir}/metadata.json")"
        fi
        if [ -z "${BENCHMARK_RUN_URL}" ]; then
          BENCHMARK_RUN_URL="$(jq -r '.run_url // ""' "${data_dir}/metadata.json")"
        fi
      fi

      if [ -z "${ISSUE_NUMBER}" ]; then
        title="[benchmark-regression] Declarative benchmark regressions"
        ISSUE_NUMBER="$(gh issue list \
          --state open \
          --search "${title} in:title" \
          --json number \
          --jq '.[0].number // empty' 2>/dev/null || true)"
        if [ -z "${ISSUE_NUMBER}" ]; then
          echo "Could not find an open benchmark regression issue" >> "${data_dir}/missing-files.txt"
        fi
      fi

      artifact_url="${BENCHMARK_ARTIFACT_URL}"
      if [ -z "${artifact_url}" ] && [ -n "${BENCHMARK_RUN_URL}" ]; then
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
        --arg metadata_path "${METADATA_PATH}" \
        --arg regressions_json_path "${REGRESSIONS_JSON_PATH}" \
        --arg results_url "$(make_blob_url "${RESULTS_PATH}")" \
        --arg history_report_url "$(make_blob_url "${HISTORY_REPORT_PATH}")" \
        --arg regressions_url "$(make_blob_url "${REGRESSIONS_PATH}")" \
        --arg analysis_context_url "$(make_blob_url "${ANALYSIS_CONTEXT_PATH}")" \
        --arg metadata_url "$(make_blob_url "${METADATA_PATH}")" \
        --arg regressions_json_url "$(make_blob_url "${REGRESSIONS_JSON_PATH}")" \
        '{
          repository: $repository,
          issue_number: (if $issue_number == "" then null else $issue_number end),
          run_id: $run_id,
          run_url: $run_url,
          artifact_url: $artifact_url,
          branch: "benchmark-results",
          paths: {
            results: $results_path,
            history_report: $history_report_path,
            regressions: $regressions_path,
            analysis_context: $analysis_context_path,
            metadata: $metadata_path,
            regressions_json: $regressions_json_path
          },
          links: {
            workflow_run: $run_url,
            artifacts: $artifact_url,
            results_json: $results_url,
            history_report_json: $history_report_url,
            regressions_markdown: $regressions_url,
            analysis_context_json: $analysis_context_url,
            metadata_json: $metadata_url,
            regressions_json: $regressions_json_url
          }
        }' > "${data_dir}/run-context.json"
post-steps:
  - name: Validate benchmark analysis comment target
    if: always()
    run: |
      set -euo pipefail

      data_dir="/tmp/gh-aw/benchmark-analysis"
      run_context="${data_dir}/run-context.json"
      agent_output="/tmp/gh-aw/agent_output.json"
      safe_outputs="${RUNNER_TEMP}/gh-aw/safeoutputs/outputs.jsonl"

      if [ ! -f "${safe_outputs}" ] && [ ! -f "${agent_output}" ]; then
        exit 0
      fi

      issue_number=""
      if [ -f "${run_context}" ]; then
        issue_number="$(jq -r '.issue_number // ""' "${run_context}")"
      fi

      filtered_outputs="${data_dir}/safeoutputs.validated.jsonl"
      invalid_comment_count=0

      if [ -f "${safe_outputs}" ]; then
        invalid_comment_count="$(
          jq -s --arg issue_number "${issue_number}" \
            '[.[] | select(.type == "add_comment" and ($issue_number == "" or ((.item_number // .issue_number // "") | tostring) != $issue_number))] | length' \
            "${safe_outputs}"
        )"

        if [ "${invalid_comment_count}" -gt 0 ]; then
          jq -c --arg issue_number "${issue_number}" \
            'select(.type != "add_comment" or ($issue_number != "" and ((.item_number // .issue_number // "") | tostring) == $issue_number))' \
            "${safe_outputs}" > "${filtered_outputs}"

          if [ ! -s "${filtered_outputs}" ]; then
            jq -cn --arg message \
              "Skipped benchmark analysis comment because the requested target did not match the resolved benchmark regression issue." \
              '{type: "noop", message: $message}' >> "${filtered_outputs}"
          fi

          mv "${filtered_outputs}" "${safe_outputs}"
        fi
      fi

      if [ -f "${agent_output}" ]; then
        agent_invalid_comment_count="$(
          jq --arg issue_number "${issue_number}" \
            '[.items[]? | select(.type == "add_comment" and ($issue_number == "" or ((.item_number // .issue_number // "") | tostring) != $issue_number))] | length' \
            "${agent_output}"
        )"
        if [ "${agent_invalid_comment_count}" -eq 0 ]; then
          exit 0
        fi

        filtered_agent_output="${data_dir}/agent_output.validated.json"
        jq --arg issue_number "${issue_number}" --arg message \
          "Skipped benchmark analysis comment because the requested target did not match the resolved benchmark regression issue." \
          '
          def valid_output:
            .type != "add_comment" or
            ($issue_number != "" and ((.item_number // .issue_number // "") | tostring) == $issue_number);

          .items = ([.items[]? | select(valid_output)]) |
          if (.items | length) == 0 then
            .items = [{type: "noop", message: $message}]
          else
            .
          end
          ' "${agent_output}" > "${filtered_agent_output}"
        mv "${filtered_agent_output}" "${agent_output}"
      fi
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
- `regressions.json`: machine-readable deterministic regression status
- `metadata.json`: benchmark-results metadata for the benchmark run
- `missing-files.txt`: optional list of files that could not be fetched

Use `analysis-context.json` first. Fall back to the other files only when you
need detail that the compact context does not include.

## What To Produce

Emit exactly one safe output:

- Use `add_comment` with `item_number` set to the `issue_number` from
  `run-context.json` when there is enough data to explain the regression.
- Use `noop` if there are no regressions, if required files are missing, or if
  the data is too incomplete to explain safely. Also use `noop` if
  `run-context.json` has an empty, null, or missing `issue_number`.

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

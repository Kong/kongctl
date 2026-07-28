#!/usr/bin/env bash

e2e_extract_go_results() {
  local run_log="$1"
  local status="$2"
  local output_file="$3"

  sed -n "s/^[[:space:]]*--- ${status}: Test_Scenarios\/\(.*\) (.*/\1/p" "${run_log}" |
    sed 's#^test/e2e/scenarios/##' |
    sed 's#^scenarios/##' |
    sort -u >"${output_file}"
}

e2e_extract_beta_failures() {
  local artifacts_dir="$1"
  local output_file="$2"
  local failures_dir="${artifacts_dir}/beta-failures"
  local record

  : >"${output_file}"
  if [ ! -d "${failures_dir}" ]; then
    return 0
  fi

  while IFS= read -r record; do
    jq -er '
      select(
        (.scenario | type == "string" and length > 0) and
        .maturity == "beta" and
        .mode == "warn" and
        (.error | type == "string" and length > 0)
      ) |
      .scenario
    ' "${record}" >>"${output_file}"
  done < <(find "${failures_dir}" -name failure.json -type f | sort)

  sort -u "${output_file}" -o "${output_file}"
}

e2e_subtract_results() {
  local input_file="$1"
  local excluded_file="$2"
  local output_file="$3"

  comm -23 <(sort -u "${input_file}") <(sort -u "${excluded_file}") >"${output_file}"
}

e2e_escape_workflow_command() {
  local value="$1"
  value="${value//'%'/'%25'}"
  value="${value//$'\r'/'%0D'}"
  value="${value//$'\n'/'%0A'}"
  printf '%s' "${value}"
}

e2e_emit_beta_annotations() {
  local artifacts_dir="$1"
  local failures_dir="${artifacts_dir}/beta-failures"
  local record scenario message cleanup_message

  if [ ! -d "${failures_dir}" ]; then
    return 0
  fi

  while IFS= read -r record; do
    scenario="$(jq -er '.scenario' "${record}")"
    message="$(jq -er '.error' "${record}")"
    printf '::warning file=test/e2e/scenarios/%s,title=Beta E2E failure::%s\n' \
      "${scenario}" \
      "$(e2e_escape_workflow_command "${message}")"

    cleanup_message="$(jq -r '.cleanup_error // empty' "${record}")"
    if [ -n "${cleanup_message}" ]; then
      printf '::warning file=test/e2e/scenarios/%s,title=Beta E2E cleanup failure::%s\n' \
        "${scenario}" \
        "$(e2e_escape_workflow_command "${cleanup_message}")"
    fi
  done < <(find "${failures_dir}" -name failure.json -type f | sort)
}

e2e_blocking_failure_reason() {
  local failed_total="$1"
  local nonzero_exit_total="$2"
  local failure_reason=""

  if [ "${failed_total}" -gt 0 ]; then
    failure_reason="${failed_total} scenario(s) failed"
  fi
  if [ "${nonzero_exit_total}" -gt 0 ]; then
    if [ -n "${failure_reason}" ]; then
      failure_reason="${failure_reason}; "
    fi
    failure_reason="${failure_reason}${nonzero_exit_total} shard(s) exited non-zero"
  fi

  printf '%s' "${failure_reason}"
}

e2e_result_coverage_error() {
  local assigned_file="$1"
  local observed_file="$2"
  local assigned_sorted observed_sorted duplicate_results

  assigned_sorted="$(mktemp)"
  observed_sorted="$(mktemp)"
  sort "${assigned_file}" >"${assigned_sorted}"
  sort "${observed_file}" >"${observed_sorted}"

  duplicate_results="$(uniq -d "${observed_sorted}" || true)"
  if [ -n "${duplicate_results}" ]; then
    printf 'Scenarios appeared in multiple result categories: %s' "${duplicate_results}"
    rm -f "${assigned_sorted}" "${observed_sorted}"
    return 0
  fi
  if ! diff -q "${assigned_sorted}" "${observed_sorted}" >/dev/null; then
    printf 'Scenario result coverage does not match shard assignments.'
  fi

  rm -f "${assigned_sorted}" "${observed_sorted}"
}

#!/usr/bin/env bash
# shellcheck disable=SC2154 # Globals are supplied by smoke-test.sh.
# Sourced by smoke-test.sh; uses its isolated fixtures, reporting, and cleanup.

check_adoption() {
  local directory="$fixtures_dir/$resource_key" output preview
  local -a selector=()
  case "$resource_key" in
    portal) selector=(portal "$resource_name") ;;
    ai_gateway) selector=(ai-gateway models --gateway-name "$resource_name") ;;
    *) return ;;
  esac
  preview="$directory/adoption-preview.yaml"
  output="$directory/adopted.yaml"

  start_check "adopt-preview-${resource_key}" "$resource_key" "adoption"
  local -a selection=()
  if [[ "$resource_key" == "ai_gateway" ]]; then
    selection=(--name-pattern 'smoke-*')
  fi
  run_kongctl adopt "${selector[@]}" "${selection[@]}" --overwrite-namespace \
    --namespace "${resource_namespace}-preview" --dry-run --declarative-output "$preview" -o json
  require_success
  assert_adoption_result preview "${resource_namespace}-preview"
  assert_text_contains "$preview" "# DRY RUN:"
  pass_current
  # A preview aimed at another namespace must leave both ownership and config intact.
  check_zero_plans "$resource_updated" "adoption-preview"

  start_check "adopt-${resource_key}" "$resource_key" "adoption"
  if [[ "$resource_key" == "ai_gateway" ]]; then
    selection=(--all)
  fi
  run_kongctl adopt "${selector[@]}" "${selection[@]}" --overwrite-namespace \
    --namespace "$resource_namespace" --declarative-output "$output" -o json
  require_success
  assert_adoption_result actual "$resource_namespace"
  assert_text_contains "$output" "$resource_namespace"
  if [[ "$resource_key" == "portal" ]]; then
    assert_text_contains "$output" "portals:"
    assert_text_contains "$output" "$resource_name"
  else
    assert_text_contains "$output" "smoke-model"
    assert_text_contains "$output" "_external"
  fi
  pass_current
  check_zero_plans "$resource_updated" "adopted"

  # Apply mode verifies the exported selection without pruning omitted children.
  start_check "adopt-output-${resource_key}" "$resource_key" "adoption"
  local plan="$plans_dir/${resource_key}-adopt-output.json"
  run_kongctl plan -f "$output" --mode apply --require-namespace "$resource_namespace" --output-file "$plan"
  require_success
  assert_zero_plan "$plan"
  pass_current
}

assert_adoption_result() {
  if ! python3 "$script_dir/smoke/assert-adoption.py" "$current_stdout" "$1" \
    "$resource_key" "$resource_name" "$resource_namespace" "$2"; then
    fail_current "unexpected adoption result"
  fi
}

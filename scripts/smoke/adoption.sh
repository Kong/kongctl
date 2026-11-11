#!/usr/bin/env bash
# shellcheck disable=SC2154 # Globals are supplied by smoke-test.sh.
# Sourced by smoke-test.sh; uses its isolated fixtures, reporting, and cleanup.

adoption_pending_restore=""

check_adoption() {
  local directory="$fixtures_dir/$resource_key" output preview
  local temporary_namespace="${resource_namespace}-preview"
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
  assert_text_contains "$preview" "$temporary_namespace"
  pass_current
  # A preview aimed at another namespace must leave both ownership and config intact.
  check_zero_plans "$resource_updated" "adoption-preview"

  start_check "adopt-${resource_key}" "$resource_key" "adoption"
  if [[ "$resource_key" == "ai_gateway" ]]; then
    selection=(--all)
  fi
  # Register restoration before the request, including failures after a remote write.
  adoption_pending_restore="$resource_key"
  run_kongctl adopt "${selector[@]}" "${selection[@]}" --overwrite-namespace \
    --namespace "$temporary_namespace" -o json
  require_success
  assert_adoption_result actual "$temporary_namespace"
  pass_current
  check_adoption_owner "$temporary_namespace" moved

  start_check "adopt-restore-${resource_key}" "$resource_key" "adoption"
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
  check_adoption_owner "$resource_namespace" restored
  adoption_pending_restore=""
  check_zero_plans "$resource_updated" "adopted"

  # Apply mode verifies the exported selection without pruning omitted children.
  start_check "adopt-output-${resource_key}" "$resource_key" "adoption"
  local plan="$plans_dir/${resource_key}-adopt-output.json"
  run_kongctl plan -f "$output" --mode apply --require-namespace "$resource_namespace" --output-file "$plan"
  require_success
  assert_zero_plan "$plan"
  pass_current
}

check_adoption_owner() {
  start_check "adopt-owner-${resource_key}-$2" "$resource_key" "adoption"
  if [[ "$resource_key" == "portal" ]]; then
    run_kongctl "${detail_args[@]}"
    require_success
    assert_object_field "labels.KONGCTL-namespace" "$1"
  else
    run_kongctl get ai-gateway models --gateway-name "$resource_name" --model-name smoke-model -o json
    require_success
    assert_object_field "managed_by.kongctl-namespace" "$1"
  fi
  pass_current
}

restore_adoption_ownership() {
  [[ "$adoption_pending_restore" == "$resource_key" ]] || return 0
  local -a args=(adopt portal "$resource_name")
  if [[ "$resource_key" == "ai_gateway" ]]; then
    args=(adopt ai-gateway models --all --gateway-name "$resource_name")
  fi
  start_check "adopt-restore-${resource_key}-cleanup" "$resource_key" "cleanup"
  run_kongctl "${args[@]}" --overwrite-namespace --namespace "$resource_namespace" -o json
  if [[ "$current_exit" -ne 0 ]]; then
    record_current failed "could not restore adoption namespace before cleanup"
    echo " FAIL"
    return 1
  fi
  adoption_pending_restore=""
  pass_current
}

assert_adoption_result() {
  if ! python3 "$script_dir/smoke/assert-adoption.py" "$current_stdout" "$1" \
    "$resource_key" "$resource_name" "$resource_namespace" "$2"; then
    fail_current "unexpected adoption result"
  fi
}

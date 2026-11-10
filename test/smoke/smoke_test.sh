#!/usr/bin/env bash
set -euo pipefail

# Operator defaults must not alter confirmation and fault-injection cases.
unset KONGCTL_SMOKE_YES FAKE_FAIL_ON FAKE_FAIL_DELETE FAKE_BAD_SCAFFOLD FAKE_FAULT

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SMOKE_SCRIPT="$ROOT/scripts/smoke-test.sh"

tmp_base="${KONGCTL_SMOKE_TEST_TMPDIR:-}"
if [[ -z "$tmp_base" ]]; then
  tmp_base="$(go env GOCACHE 2>/dev/null || true)"
fi
if [[ -z "$tmp_base" ]]; then
  tmp_base="${TMPDIR:-/tmp}"
fi
mkdir -p "$tmp_base"
TMP_ROOT="$(mktemp -d "${tmp_base%/}/kongctl-smoke-tests.XXXXXX")"
trap 'rm -rf "$TMP_ROOT"' EXIT

fail() {
  echo "not ok - $1" >&2
  if [[ -n "${2:-}" && -f "$2" ]]; then
    sed -n '1,180p' "$2" >&2
  fi
  exit 1
}

pass() {
  echo "ok - $1"
}

assert_contains() {
  local file="$1"
  local value="$2"
  local message="$3"
  grep -Fq -- "$value" "$file" || fail "$message" "$file"
}

assert_not_contains() {
  local file="$1"
  local value="$2"
  local message="$3"
  if grep -Fq -- "$value" "$file"; then
    fail "$message" "$file"
  fi
}

assert_json() {
  local file="$1"
  local expression="$2"
  local message="$3"
  python3 - "$file" "$expression" <<'PY' || fail "$message" "$file"
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    value = json.load(handle)
if not eval(sys.argv[2], {"__builtins__": {}}, {"value": value, "len": len, "any": any}):
    raise SystemExit(1)
PY
}

write_fake_kongctl() {
  local path="$1"
  cp "$ROOT/test/smoke/fake_kongctl.py" "$path"
  chmod 755 "$path"
}

new_case() {
  local name="$1"
  CASE_DIR="$TMP_ROOT/$name"
  mkdir -p "$CASE_DIR/artifacts"
  FAKE_BIN="$CASE_DIR/kongctl"
  FAKE_LOG="$CASE_DIR/commands.log"
  FAKE_STATE="$CASE_DIR/state.json"
  OUTPUT="$CASE_DIR/output.log"
  : >"$FAKE_LOG"
  printf '{}\n' >"$FAKE_STATE"
  write_fake_kongctl "$FAKE_BIN"
}

run_smoke() {
  set +e
  FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" "$@" >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  RUN_DIR="$(find "$CASE_DIR/artifacts" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
}

test_quick_uses_explain_and_scaffold() {
  new_case quick
  run_smoke --quick --expect-version v1.2.3 --expect-commit abcdef
  [[ "$STATUS" -eq 0 ]] || fail "quick smoke succeeds" "$OUTPUT"
  assert_contains "$FAKE_LOG" "explain api -o json" "quick runs explain"
  assert_contains "$FAKE_LOG" "scaffold api" "quick runs scaffold"
  assert_contains "$RUN_DIR/fixtures/api/initial.yaml" "versions:" "API fixture uses nested version scaffold"
  assert_contains "$RUN_DIR/fixtures/api/initial.yaml" "documents:" "API fixture uses nested document scaffold"
  assert_contains "$RUN_DIR/fixtures/api/specs/smoke-api.yaml" "version: v1.0.0" \
    "generated OpenAPI version matches the scaffolded API version"
  assert_contains "$RUN_DIR/fixtures/control_plane/initial.yaml" "CLUSTER_TYPE_CONTROL_PLANE" \
    "control-plane fixture is enriched"
  local key namespace_var
  for key in api portal control_plane ai_gateway event_gateway; do
    namespace_var="KONGCTL_SMOKE_NAMESPACE_${key^^}"
    assert_contains "$RUN_DIR/fixtures/$key/initial.yaml" "    namespace: !env $namespace_var" \
      "$key create fixture inherits an environment namespace"
    assert_contains "$RUN_DIR/fixtures/$key/updated.yaml" "      namespace: !env {var: $namespace_var}" \
      "$key update fixture has an explicit environment namespace"
    assert_not_contains "$RUN_DIR/fixtures/$key/updated.yaml" "_defaults:" \
      "$key update namespace cannot fall back to a file default"
  done
  assert_not_contains "$FAKE_LOG" "adopt " "quick does not adopt"
  assert_not_contains "$FAKE_LOG" "apply --plan" "quick does not apply"
  assert_not_contains "$FAKE_LOG" "delete " "quick does not delete"
  assert_json "$RUN_DIR/report.json" "value['status'] == 'passed' and value['mode'] == 'quick'" \
    "quick report is valid"
  pass "quick mode derives fixtures from explain and scaffold"
}

test_full_lifecycle() {
  new_case full
  KONGCTL_SMOKE_NAMESPACE_API=unrelated-operator-namespace run_smoke --yes
  [[ "$STATUS" -eq 0 ]] || fail "full smoke succeeds" "$OUTPUT"
  assert_json "$FAKE_STATE" "value == {}" "full run cleans every resource"
  assert_contains "$FAKE_LOG" "adopt portal" "full run adopts a root"
  assert_contains "$FAKE_LOG" "adopt ai-gateway models" "full run adopts children"
  assert_contains "$FAKE_LOG" "--name-pattern smoke-*" "preview uses filtered children"
  assert_contains "$FAKE_LOG" "--all --overwrite-namespace" "adoption uses all children"
  assert_contains "$FAKE_LOG" "delete --plan" "full run executes delete plans"
  assert_contains "$FAKE_LOG" "list ai-gateways -o json" "full run exercises imperative list"
  assert_contains "$FAKE_LOG" "get gateway control-plane" "full run exercises imperative get"
  assert_contains "$FAKE_LOG" "--include-child-resources" "dump requests nested child resources"
  local report_condition="value['status'] == 'passed' and value['cleanup']['status'] == 'passed' "
  report_condition+="and len(value['resources']) == 7 and value['known_issues'] == [] "
  report_condition+="and value['known_issue_checks'] == []"
  assert_json "$RUN_DIR/report.json" "$report_condition" "full report records lifecycle and cleanup"
  python3 - "$RUN_DIR" <<'PYTEST'
import json
import pathlib
import sys

directory = pathlib.Path(sys.argv[1])
report = json.loads((directory / "report.json").read_text())
for resource in report["resources"]:
    key, namespace = resource["key"], resource["namespace"]
    for stage in ("create", "updated-apply", "updated-sync", "delete"):
        suffix = "" if stage in ("create", "delete") else "-zero"
        plan = json.loads((directory / "plans" / f"{key}-{stage}{suffix}.json").read_text())
        assert plan["_desired"]["_namespace"] == namespace, (key, stage)
        assert all(change["namespace"] == namespace for change in plan["changes"])
    if key != "identity_principal":
        dumped = (directory / "dumps" / f"{key}.yaml").read_text()
        assert "!env" not in dumped, key
        assert f'namespace: "{namespace}"' in dumped, key
PYTEST
  python3 - "$FAKE_LOG" <<'PYTEST'
import json
import pathlib
import sys

records = [json.loads(line) for line in
           pathlib.Path(sys.argv[1]).with_suffix(".namespaces.jsonl").read_text().splitlines()]
assert {record["verb"] for record in records} == {"apply", "sync", "delete"}
for record in records:
    assert record["environment"] and record["environment"] != record["planned"], record
PYTEST
  pass "full lifecycle covers resources and per-resource delete"
}

test_apply_failure_stops_lifecycle() {
  new_case unrecognized-apply-failure
  set +e
  FAKE_FAIL_ON="apply --plan" FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" --yes >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  RUN_DIR="$(find "$CASE_DIR/artifacts" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  [[ "$STATUS" -eq 1 ]] || fail "unrecognized apply failure returns one" "$OUTPUT"
  assert_not_contains "$FAKE_LOG" "list portals -o json" "unexpected failure stops the lifecycle"
  pass "unexpected apply failure stops the lifecycle"
}

test_failure_cleans_up() {
  new_case failure-cleanup
  set +e
  KONGCTL_SMOKE_YES=1 FAKE_FAIL_ON="sync -f" FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  RUN_DIR="$(find "$CASE_DIR/artifacts" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  [[ "$STATUS" -eq 1 ]] || fail "injected failure returns one" "$OUTPUT"
  assert_json "$FAKE_STATE" "value == {}" "failed run automatically cleans resources"
  assert_contains "$FAKE_LOG" "delete -f" "failed run uses declarative delete cleanup"
  assert_json "$RUN_DIR/report.json" \
    "value['status'] == 'failed' and value['cleanup']['status'] == 'passed'" \
    "failure report records successful cleanup"
  pass "failure triggers targeted cleanup"
}

test_keep_on_failure() {
  new_case keep-on-failure
  set +e
  FAKE_FAIL_ON="sync -f" FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" --yes --keep-on-failure \
    >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  RUN_DIR="$(find "$CASE_DIR/artifacts" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  [[ "$STATUS" -eq 1 ]] || fail "retained failure returns one" "$OUTPUT"
  assert_json "$FAKE_STATE" "'api' in value" "keep-on-failure retains created API"
  assert_json "$RUN_DIR/report.json" "value['cleanup']['status'] == 'retained'" \
    "report marks retained cleanup"
  pass "keep-on-failure retains diagnostic resources"
}

test_cleanup_failure_is_reported() {
  new_case cleanup-failure
  set +e
  FAKE_FAIL_ON="sync -f" FAKE_FAIL_DELETE=1 FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" --yes >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  RUN_DIR="$(find "$CASE_DIR/artifacts" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  [[ "$STATUS" -eq 1 ]] || fail "cleanup failure returns one" "$OUTPUT"
  assert_json "$FAKE_STATE" "'api' in value" "failed cleanup leaves resource state visible"
  assert_json "$RUN_DIR/report.json" "value['cleanup']['status'] == 'failed'" \
    "report marks cleanup failure"
  pass "cleanup failures remain hard failures"
}

test_prompt_refusal_is_non_mutating() {
  new_case prompt-refusal
  set +e
  printf 'n\n' | FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  [[ "$STATUS" -eq 1 ]] || fail "prompt refusal returns one" "$OUTPUT"
  assert_not_contains "$FAKE_LOG" "plan -f" "prompt refusal occurs before planning"
  assert_json "$FAKE_STATE" "value == {}" "prompt refusal does not mutate state"
  pass "confirmation refusal is non-mutating"
}

test_scaffold_contract_failure_is_safe() {
  new_case bad-scaffold
  set +e
  FAKE_BAD_SCAFFOLD="portal" FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" --quick >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  [[ "$STATUS" -eq 1 ]] || fail "bad scaffold fails" "$OUTPUT"
  assert_not_contains "$FAKE_LOG" "plan -f" "contract failure occurs before planning"
  assert_json "$FAKE_STATE" "value == {}" "contract failure does not mutate state"
  pass "explain and scaffold disagreement fails safely"
}

test_default_ai_gateway_23_lifecycle() {
  new_case runtime23
  run_smoke --yes --resources ai_gateway
  [[ "$STATUS" -eq 0 ]] || fail "runtime 2.3 smoke succeeds" "$OUTPUT"
  assert_contains "$RUN_DIR/fixtures/ai_gateway/updated.yaml" '"cost": 3.5' "runtime costs are updated"
  assert_contains "$RUN_DIR/fixtures/ai_gateway/updated.yaml" '"ttl_ms": 0' "zero cache TTL is covered"
  assert_contains "$RUN_DIR/fixtures/ai_gateway/initial.yaml" 'min_runtime_version: "2.3"' \
    "AI Gateway requires runtime 2.3"
  assert_not_contains "$RUN_DIR/fixtures/ai_gateway/initial.yaml" '"expressions"' \
    "rate-limiting policy does not declare unsupported expressions"
  assert_not_contains "$RUN_DIR/fixtures/ai_gateway/updated.yaml" '"expressions"' \
    "rate-limiting policy update does not declare unsupported expressions"
  assert_contains "$RUN_DIR/fixtures/ai_gateway/updated.yaml" '"tool": "lookup", "limit": 10' \
    "tool rate limits are updated"
  assert_contains "$RUN_DIR/fixtures/ai_gateway/updated.yaml" '!ref smoke-store-ref#name' \
    "model references its datastore by name"
  assert_json "$RUN_DIR/plans/ai_gateway-replacement.json" \
    "any(c['resource_type'] == 'ai_gateway_model' and c['action'] == 'UPDATE' for c in value['changes'])" \
    "provider replacement updates the semantic model"
  python3 - "$RUN_DIR/fixtures/ai_gateway/replacement.yaml" <<'PYTEST' || fail "semantic provider references are replaced"
import json
import pathlib
import sys
text = pathlib.Path(sys.argv[1]).read_text()
model = json.loads(next(line.removeprefix("    models: ") for line in text.splitlines()
                        if line.startswith("    models: ")).replace("!ref smoke-policy-ref#name", '"policy"')
                   .replace("!ref smoke-store-ref#name", '"store"')
                   .replace("!env {var: KONGCTL_SMOKE_NAMESPACE_AI_GATEWAY}", '"namespace"'))[0]
assert model["config"]["balancer"]["embeddings"]["provider"] == "smoke-provider-replacement"
assert len(model["targets"]) == 2
assert all(t["provider"] == "smoke-provider-replacement" for t in model["targets"])
PYTEST
  assert_contains "$FAKE_LOG" "get ai-gateway datastores" "lifecycle verifies datastore reads"
  assert_contains "$FAKE_LOG" "ai_gateway-replacement.json" "replacement uses a saved plan"
  assert_json "$FAKE_STATE" "value == {}" "runtime suite cleans up"
  assert_contains "$FAKE_LOG" "get ai-gateway mcp-servers" "default lifecycle verifies MCP fields"
  pass "default AI Gateway lifecycle includes runtime 2.3"
}

test_fault_is_rejected() {
  local fault="$1" check="$2" mode="${3:---yes}"
  local resources="${4:-api,portal,control_plane,ai_gateway,event_gateway}"
  new_case "$fault"
  export FAKE_FAULT="$fault"
  run_smoke "$mode" --resources "$resources"
  unset FAKE_FAULT
  [[ "$STATUS" -eq 1 ]] || fail "$fault must fail" "$OUTPUT"
  assert_json "$RUN_DIR/report.json" \
    "any(c['name'] == '$check' and c['status'] == 'failed' for c in value['checks'])" \
    "$fault fails at the expected check"
  assert_json "$FAKE_STATE" "value == {}" "$fault leaves no resources"
  pass "$fault is rejected"
}

test_resource_selection() {
  new_case selected
  run_smoke --yes --resources ai_gateway
  [[ "$STATUS" -eq 0 ]] || fail "selected resource lifecycle succeeds" "$OUTPUT"
  assert_json "$RUN_DIR/report.json" \
    "len(value['resources']) == 1 and value['resources'][0]['key'] == 'ai_gateway'" \
    "report lists only selected resources"
  assert_not_contains "$FAKE_LOG" "scaffold portal" "unselected fixtures are not prepared"
  assert_json "$FAKE_STATE" "value == {}" "selected resource is cleaned"
  for selection in 'api,unknown' 'api,api' 'api,' ',api' 'api,,portal'; do
    run_smoke --quick --resources "$selection"
    [[ "$STATUS" -eq 2 ]] || fail "invalid selection $selection is rejected" "$OUTPUT"
  done
  pass "resource selection and validation"
}

test_fake_datastore_lookup() {
  new_case datastore-lookup
  python3 - "$FAKE_STATE" <<'PYTEST'
import json
import pathlib
import sys
pathlib.Path(sys.argv[1]).write_text(json.dumps({"ai_gateway": {"datastores": [
    {"name": "another-store"}, {"name": "smoke-store"},
]}}))
PYTEST
  FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$FAKE_BIN" get ai-gateway datastores --gateway-name gateway -o json >"$CASE_DIR/list.json"
  assert_json "$CASE_DIR/list.json" "len(value) == 2" "fake lists all datastores"
  FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$FAKE_BIN" get ai-gateway datastores --gateway-name gateway smoke-store -o json >"$CASE_DIR/detail.json"
  assert_json "$CASE_DIR/detail.json" "value['name'] == 'smoke-store'" "fake matches the requested datastore"
  if FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$FAKE_BIN" get ai-gateway datastores --gateway-name gateway missing-store -o json \
      >"$CASE_DIR/missing.json" 2>"$CASE_DIR/missing.stderr"; then
    fail "unknown datastore must fail"
  fi
  assert_contains "$CASE_DIR/missing.stderr" "datastore not found" "unknown datastore reports not found"
  pass "fake datastore list and exact name lookup"
}

test_numeric_readback_format() {
  new_case numeric-format
  export FAKE_FAULT=numeric-format
  run_smoke --yes --resources ai_gateway
  unset FAKE_FAULT
  [[ "$STATUS" -eq 0 ]] || fail "equivalent numeric readback formats succeed" "$OUTPUT"
  assert_json "$FAKE_STATE" "value == {}" "numeric format case cleans up"
  pass "equivalent numeric readback formats"
}

test_reordered_mcp_readback() {
  new_case reordered-mcp
  export FAKE_FAULT=reordered-mcp
  run_smoke --yes --resources ai_gateway
  unset FAKE_FAULT
  [[ "$STATUS" -eq 0 ]] || fail "MCP readback matches limits and tools by identity" "$OUTPUT"
  assert_json "$FAKE_STATE" "value == {}" "reordered MCP case cleans up"
  pass "MCP readback is independent of array order"
}

test_identity_lifecycle() {
  new_case identity
  run_smoke --yes --resources identity_principal,identity_auth_server
  [[ "$STATUS" -eq 0 ]] || fail "Identity-only smoke succeeds" "$OUTPUT"
  assert_json "$FAKE_STATE" "value == {}" "Identity resources are cleaned up"
  assert_contains "$FAKE_LOG" "--display-name smoke-person-" "principal discovery uses exact display name"
  assert_contains "$FAKE_LOG" "get identity principal-identities" "custom identity readback runs"
  assert_contains "$FAKE_LOG" "get identity auth-server-claim" "claim readback runs"
  assert_contains "$FAKE_LOG" "list identity directories" "existing directory inventory is checked"
  assert_not_contains "$FAKE_LOG" "scaffold identity_directory" "smoke does not manage a directory"
  assert_contains "$RUN_DIR/fixtures/identity_principal/initial.yaml" '"_external"' \
    "principal fixture references an external directory"
  assert_not_contains "$RUN_DIR/fixtures/identity_principal/initial.yaml" "allow_all_control_planes" \
    "principal fixture cannot change directory settings"
  assert_contains "$RUN_DIR/fixtures/identity_auth_server/updated.yaml" "!ref smoke-read#id" \
    "claim references its scope"
  assert_json "$RUN_DIR/report.json" "value['status'] == 'passed' and len(value['resources']) == 2" \
    "Identity-only report includes both families"
  pass "Identity-only lifecycle covers principals, identities, scopes, and claims"
}

test_quick_uses_explain_and_scaffold
test_full_lifecycle
test_apply_failure_stops_lifecycle
test_failure_cleans_up
test_keep_on_failure
test_cleanup_failure_is_reported
test_prompt_refusal_is_non_mutating
test_scaffold_contract_failure_is_safe
test_default_ai_gateway_23_lifecycle
test_fault_is_rejected bad-adoption-preview adopt-preview-portal --yes portal
test_fault_is_rejected bad-list list-api-quick --quick
test_fault_is_rejected lost-tags patch-preserves-tags --quick
test_fault_is_rejected false-zero plan-zero-api-updated-apply
test_fault_is_rejected unsafe-order plan-replacement-ai_gateway
test_fault_is_rejected wrong-not-found get-absent-event_gateway
test_fault_is_rejected missing-child plan-create-api
test_fault_is_rejected bad-namespace plan-create-api
test_fault_is_rejected bad-dump plan-zero-portal-dump-sync
test_fault_is_rejected stale-mcp-limit get-updated-mcp --yes ai_gateway
test_fault_is_rejected missing-dump-datastore dump-ai_gateway --yes ai_gateway
test_fault_is_rejected wrong-threshold get-updated-model --yes ai_gateway
test_fault_is_rejected rebind-apply-namespace get-api --yes api
test_fault_is_rejected rebind-sync-namespace get-portal --yes portal
test_fault_is_rejected rebind-delete-namespace delete-api --yes api
test_fault_is_rejected wrong-dump-namespace plan-zero-api-dump-apply --yes api
test_fake_datastore_lookup
test_numeric_readback_format
test_reordered_mcp_readback
test_resource_selection
test_identity_lifecycle
test_fault_is_rejected directory-read-error list-identity-directories --yes identity_principal
test_fault_is_rejected stale-principal list-updated-principals --yes identity_principal
test_fault_is_rejected stale-claim get-updated-claim --yes identity_auth_server
python3 "$ROOT/test/smoke/plan_test.py" --temp-root "$TMP_ROOT"
python3 "$ROOT/test/smoke/namespace_test.py" --temp-root "$TMP_ROOT"

python3 "$ROOT/test/smoke/directory_test.py" --temp-root "$TMP_ROOT"

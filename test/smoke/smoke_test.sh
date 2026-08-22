#!/usr/bin/env bash
set -euo pipefail

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
  cat >"$path" <<'PY'
#!/usr/bin/env python3
import json
import os
import pathlib
import re
import sys

args = sys.argv[1:]
if args[:1] == ["--profile"]:
    args = args[2:]
log_path = pathlib.Path(os.environ["FAKE_KONGCTL_LOG"])
with log_path.open("a", encoding="utf-8") as handle:
    handle.write(" ".join(args) + "\n")

joined = " ".join(args)
fail_on = os.environ.get("FAKE_FAIL_ON", "")
if fail_on and fail_on in joined:
    print(f"injected failure for {fail_on}", file=sys.stderr)
    raise SystemExit(1)
if os.environ.get("FAKE_FAIL_DELETE") == "1" and args[:1] == ["delete"]:
    print("injected delete failure", file=sys.stderr)
    raise SystemExit(1)

state_path = pathlib.Path(os.environ["FAKE_KONGCTL_STATE"])
state = json.loads(state_path.read_text(encoding="utf-8")) if state_path.exists() else {}

def save_state():
    state_path.write_text(json.dumps(state, sort_keys=True), encoding="utf-8")

def option(name, default=""):
    if name not in args:
        return default
    return args[args.index(name) + 1]

def resource_from_path(path):
    value = str(path)
    for key in ("control_plane", "ai_gateway", "portal", "api"):
        if f"/{key}/" in value:
            return key
    raise RuntimeError(f"unknown fixture path: {value}")

def parse_fixture(path):
    text = pathlib.Path(path).read_text(encoding="utf-8")
    key = resource_from_path(path)
    root = {"api": "apis", "portal": "portals", "control_plane": "control_planes", "ai_gateway": "ai_gateways"}[key]
    def scalar(field):
        match = re.search(rf"^    {field}: (.+)$", text, re.MULTILINE)
        if not match:
            return ""
        raw = match.group(1)
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return raw.strip('"')
    ref_match = re.search(r"^  - ref: (.+)$", text, re.MULTILINE)
    ref = json.loads(ref_match.group(1))
    return key, {
        "ref": ref,
        "name": scalar("name") or ref,
        "display_name": scalar("display_name"),
        "description": scalar("description"),
        "document_title": bool(re.search(r"^        title: ", text, re.MULTILINE)),
        "root": root,
    }

def resource_from_command():
    if "control-plane" in args or "control-planes" in args:
        return "control_plane"
    if "ai-gateway" in args or "ai-gateways" in args:
        return "ai_gateway"
    if "portal" in args or "portals" in args:
        return "portal"
    return "api"

def print_execution(changes=1):
    print(json.dumps({"summary": {"status": "success", "failed": 0, "applied": changes, "total_changes": changes}}))

if args == ["--help"]:
    print("kongctl fake help")
elif args[:2] == ["version", "--full"]:
    print(json.dumps({"version": "1.2.3", "commit": "abcdef123456", "date": "2026-08-21T00:00:00Z"}))
elif args[:1] == ["explain"]:
    subject = args[1]
    roots = {"api": "apis", "portal": "portals", "control_plane": "control_planes", "ai_gateway": "ai_gateways",
             "api.versions": "api_versions", "api.documents": "api_documents"}
    properties = {
        "api": ["ref", "name", "description", "version", "slug"],
        "portal": ["ref", "name", "display_name", "description"],
        "control_plane": ["ref", "name", "description", "cluster_type"],
        "ai_gateway": ["ref", "name", "display_name", "description"],
        "api.versions": ["ref", "version", "spec"],
        "api.documents": ["ref", "content", "title", "slug"],
    }[subject]
    maturity = "beta" if subject == "ai_gateway" else "ga"
    print(json.dumps({
        "type": "object",
        "title": f"kongctl declarative schema: {subject}",
        "x-kongctl-root-key": roots[subject],
        "x-kongctl-maturity": {"level": maturity},
        "required": ["ref"],
        "properties": {field: {"type": "string"} for field in properties},
    }))
elif args[:1] == ["scaffold"]:
    subject = args[1]
    bad = os.environ.get("FAKE_BAD_SCAFFOLD", "")
    outputs = {
        "api": """apis:
  - ref: my-resource
    name: my-resource
    description: Example description
    version: v1.0.0
    slug: my-resource
    # labels: {}
""",
        "portal": """portals:
  - ref: my-resource
    name: my-resource
    display_name: My Resource
    description: Example description
""",
        "control_plane": """control_planes:
  - ref: my-resource
    name: my-resource
    description: Example description
    # cluster_type: value
""",
        "ai_gateway": """# Maturity: beta

ai_gateways:
  - ref: my-resource
    name: my-ai-gateway
    display_name: My AI Gateway
    # description:
""",
        "api.versions": """apis:
  - ref: my-resource
    versions:
      - version: v1.0.0
        spec: !file ./specs/api.yaml
        ref: my-resource
""",
        "api.documents": """apis:
  - ref: my-resource
    documents:
      - content: !file ./content.txt
        # title: value
        slug: my-resource
        ref: my-resource
""",
    }
    output = outputs[subject]
    if bad == subject:
        output = output.replace(output.splitlines()[0], "wrong_root:", 1)
    print(output, end="")
elif args[:1] == ["plan"]:
    fixture = option("-f")
    key, desired = parse_fixture(fixture)
    mode = option("--mode", "sync")
    action = "DELETE" if mode == "delete" else ("CREATE" if key not in state else "UPDATE")
    changes = []
    if mode == "delete" and key in state or mode != "delete" and state.get(key) != desired:
        changes = [{"action": action, "resource_type": key, "resource_ref": desired["ref"], "fields": desired}]
    plan = {"metadata": {"mode": mode}, "summary": {"total_changes": len(changes)}, "changes": changes}
    pathlib.Path(option("--output-file")).write_text(json.dumps(plan), encoding="utf-8")
    print(json.dumps(plan))
elif args[:1] == ["diff"]:
    plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
    print("\n".join(change["resource_ref"] for change in plan["changes"]) or "No changes")
elif args[:1] == ["apply"]:
    plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
    if os.environ.get("FAKE_KNOWN_ISSUE_1947") == "1" and any(
        change["resource_type"] == "api" and not change["fields"].get("document_title")
        for change in plan["changes"]
    ):
        print(json.dumps({
            "execution": {"errors": [{
                "error": "incompatible plan change 2 (\"3:c:api_document:guide\") "
                         "for CREATE api_document: title is required; regenerate the plan"
            }]},
            "summary": {"status": "error", "failed": 1, "applied": 0, "total_changes": 1},
        }))
        print("Error: execution completed with 1 errors", file=sys.stderr)
        raise SystemExit(1)
    for change in plan["changes"]:
        state[change["resource_type"]] = change["fields"]
    save_state()
    print_execution(len(plan["changes"]))
elif args[:1] == ["sync"]:
    key, desired = parse_fixture(option("-f"))
    state[key] = desired
    save_state()
    print_execution(1)
elif args[:2] == ["dump", "declarative"]:
    resources = option("--resources")
    keys = {
        "apis": "api", "portals": "portal", "control_planes": "control_plane", "ai_gateways": "ai_gateway",
    }
    key = keys[resources]
    item = state[key]
    text = f"{item['root']}:\n  - ref: {item['ref']}\n    name: {item['name']}\n"
    if key == "api":
        text += "    versions:\n      - version: v1.0.0\n    documents:\n      - slug: guide\n"
    pathlib.Path(option("--output-file")).write_text(text, encoding="utf-8")
elif args[:1] == ["delete"]:
    dry_run = "--dry-run" in args
    if "--plan" in args:
        plan = json.loads(pathlib.Path(option("--plan")).read_text(encoding="utf-8"))
        key = plan["changes"][0]["resource_type"]
    else:
        key = resource_from_path(option("-f"))
    changes = 1 if key in state else 0
    if not dry_run:
        state.pop(key, None)
        save_state()
    print_execution(changes)
elif args[:1] in (["get"], ["list"]):
    key = resource_from_command()
    item = state.get(key)
    if args[0] == "list":
        print(json.dumps([item] if item else []))
    elif not item:
        print("not found", file=sys.stderr)
        raise SystemExit(1)
    else:
        print(json.dumps(item))
else:
    print(f"unsupported fake command: {joined}", file=sys.stderr)
    raise SystemExit(2)
PY
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
    FAKE_KNOWN_ISSUE_1947="${FAKE_KNOWN_ISSUE_1947:-}" \
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
  assert_not_contains "$FAKE_LOG" "apply --plan" "quick does not apply"
  assert_not_contains "$FAKE_LOG" "delete " "quick does not delete"
  assert_json "$RUN_DIR/report.json" "value['status'] == 'passed' and value['mode'] == 'quick'" \
    "quick report is valid"
  pass "quick mode derives fixtures from explain and scaffold"
}

test_full_lifecycle() {
  new_case full
  run_smoke --yes
  [[ "$STATUS" -eq 0 ]] || fail "full smoke succeeds" "$OUTPUT"
  assert_json "$FAKE_STATE" "value == {}" "full run cleans every resource"
  assert_contains "$FAKE_LOG" "delete --plan" "full run executes delete plans"
  assert_contains "$FAKE_LOG" "list ai-gateways -o json" "full run exercises imperative list"
  assert_contains "$FAKE_LOG" "get gateway control-plane" "full run exercises imperative get"
  assert_contains "$FAKE_LOG" "--include-child-resources" "dump requests nested child resources"
  local report_condition="value['status'] == 'passed' and value['cleanup']['status'] == 'passed' "
  report_condition+="and len(value['resources']) == 4 and value['known_issues'] == [] "
  report_condition+="and len(value['known_issue_checks']) == 1 "
  report_condition+="and value['known_issue_checks'][0]['detected'] == False"
  assert_json "$RUN_DIR/report.json" "$report_condition" "full report records lifecycle and cleanup"
  assert_contains "$RUN_DIR/summary.txt" "#1947 not detected; recovery may be removable" \
    "fixed behavior flags the recovery as a removal candidate"
  pass "full lifecycle covers resources and per-resource delete"
}

test_known_issue_recovers_and_continues() {
  new_case known-issue
  FAKE_KNOWN_ISSUE_1947=1 run_smoke --yes
  [[ "$STATUS" -eq 1 ]] || fail "known issue keeps the run unsuccessful" "$OUTPUT"
  assert_contains "$OUTPUT" "KNOWN ISSUE #1947" "terminal output identifies the known issue"
  assert_contains "$OUTPUT" "recover-known-issue-1947" "terminal output identifies the recovery"
  assert_contains "$FAKE_LOG" "list portals -o json" "smoke testing continues after recovery"
  assert_json "$FAKE_STATE" "value == {}" "known-issue run still cleans every resource"
  assert_not_contains "$RUN_DIR/fixtures/api/initial.yaml" '        title: ' \
    "canonical fixture remains unchanged"
  assert_contains "$RUN_DIR/fixtures/api/initial.issue-1947.yaml" '        title: "Smoke API ' \
    "recovery is written to a separate fixture"
  local report_condition="value['status'] == 'failed' and value['cleanup']['status'] == 'passed' "
  report_condition+="and len(value['known_issues']) == 1 and len(value['known_issue_checks']) == 1 "
  report_condition+="and value['known_issues'][0]['issue'] == 1947 "
  report_condition+="and value['known_issues'][0]['recovery_status'] == 'succeeded' "
  report_condition+="and any(check['status'] == 'known_issue' for check in value['checks'])"
  assert_json "$RUN_DIR/report.json" "$report_condition" \
    "report records the detected issue and successful recovery"
  assert_contains "$RUN_DIR/summary.txt" "known issue: #1947 (recovery succeeded)" \
    "text summary records the known issue"
  pass "known issue recovery preserves failure status and continues coverage"
}

test_unrecognized_apply_failure_does_not_recover() {
  new_case unrecognized-apply-failure
  set +e
  FAKE_FAIL_ON="apply --plan" FAKE_KONGCTL_LOG="$FAKE_LOG" FAKE_KONGCTL_STATE="$FAKE_STATE" \
    "$SMOKE_SCRIPT" --binary "$FAKE_BIN" --artifacts-dir "$CASE_DIR/artifacts" --yes >"$OUTPUT" 2>&1
  STATUS=$?
  set -e
  RUN_DIR="$(find "$CASE_DIR/artifacts" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
  [[ "$STATUS" -eq 1 ]] || fail "unrecognized apply failure returns one" "$OUTPUT"
  assert_not_contains "$OUTPUT" "KNOWN ISSUE #1947" "unrecognized errors are not classified as #1947"
  assert_not_contains "$OUTPUT" "recover-known-issue" "unrecognized errors do not invoke recovery"
  assert_not_contains "$FAKE_LOG" "list portals -o json" "unexpected failure stops the lifecycle"
  assert_json "$RUN_DIR/report.json" "value['known_issues'] == []" \
    "unexpected failure is absent from known issues"
  pass "known-issue matching does not hide unrelated apply failures"
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

test_quick_uses_explain_and_scaffold
test_full_lifecycle
test_known_issue_recovers_and_continues
test_unrecognized_apply_failure_does_not_recover
test_failure_cleans_up
test_keep_on_failure
test_cleanup_failure_is_reported
test_prompt_refusal_is_non_mutating
test_scaffold_contract_failure_is_safe

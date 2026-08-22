#!/usr/bin/env bash
# shellcheck disable=SC2317 # Resource loaders and exit handlers are invoked indirectly.
set -uo pipefail

usage() {
  cat <<'EOF'
Smoke test an installed kongctl binary against a configured Konnect environment.

Usage:
  scripts/smoke-test.sh [options]

Options:
  --binary PATH             kongctl executable (default: KONGCTL_BIN or kongctl on PATH)
  --profile NAME            kongctl profile to use
  --quick                   Run non-mutating discovery, scaffold, and list checks
  --expect-version VERSION  Require the reported version (a leading v is ignored)
  --expect-commit SHA       Require the reported commit to start with SHA
  --artifacts-dir PATH      Parent directory for the timestamped run directory
  --yes                     Skip the full-suite confirmation prompt
  --keep-on-failure         Retain resources when the run fails or is interrupted
  -h, --help                Show this help

Environment:
  KONGCTL_BIN               Default binary override
  KONGCTL_SMOKE_YES         1, true, or yes skips the confirmation prompt

The full suite creates isolated APIs, portals, control planes, and AI gateways,
then deletes each resource with kongctl delete. Authentication is taken from the
selected profile and the existing kongctl environment. An exactly matched known
CLI issue may use a separate recovery fixture so later checks can run, but the
overall smoke test still fails and reports the issue.
EOF
}

die_usage() {
  echo "smoke-test: $*" >&2
  usage >&2
  exit 2
}

is_truthy() {
  case "${1:-}" in
    1 | true | TRUE | yes | YES)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

binary_arg="${KONGCTL_BIN:-}"
profile_arg=""
mode="full"
expect_version=""
expect_commit=""
artifacts_parent=""
assume_yes="false"
keep_on_failure="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      [[ $# -ge 2 ]] || die_usage "--binary requires a value"
      binary_arg="$2"
      shift 2
      ;;
    --profile)
      [[ $# -ge 2 ]] || die_usage "--profile requires a value"
      profile_arg="$2"
      shift 2
      ;;
    --quick)
      mode="quick"
      shift
      ;;
    --expect-version)
      [[ $# -ge 2 ]] || die_usage "--expect-version requires a value"
      expect_version="$2"
      shift 2
      ;;
    --expect-commit)
      [[ $# -ge 2 ]] || die_usage "--expect-commit requires a value"
      expect_commit="$2"
      shift 2
      ;;
    --artifacts-dir)
      [[ $# -ge 2 ]] || die_usage "--artifacts-dir requires a value"
      artifacts_parent="$2"
      shift 2
      ;;
    --yes)
      assume_yes="true"
      shift
      ;;
    --keep-on-failure)
      keep_on_failure="true"
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      die_usage "unknown option: $1"
      ;;
  esac
done

if ! command -v python3 >/dev/null 2>&1; then
  echo "smoke-test: python3 is required" >&2
  exit 2
fi

if [[ -z "$binary_arg" ]]; then
  binary_arg="$(command -v kongctl 2>/dev/null || true)"
elif [[ "$binary_arg" != */* ]]; then
  binary_arg="$(command -v "$binary_arg" 2>/dev/null || true)"
fi

if [[ -z "$binary_arg" || ! -x "$binary_arg" ]]; then
  echo "smoke-test: kongctl binary is not executable: ${binary_arg:-kongctl}" >&2
  exit 2
fi

binary_dir="$(cd "$(dirname "$binary_arg")" && pwd)"
kongctl_bin="$binary_dir/$(basename "$binary_arg")"

if [[ -n "$artifacts_parent" ]]; then
  mkdir -p "$artifacts_parent" || exit 2
  artifacts_parent="$(cd "$artifacts_parent" && pwd)"
else
  artifacts_parent="${TMPDIR:-/tmp}"
fi

run_stamp="$(date -u +%Y%m%dT%H%M%SZ)"
run_suffix="$(python3 -c 'import secrets; print(secrets.token_hex(3))')"
run_id="kctl-smoke-${run_stamp}-${run_suffix}"
artifacts_dir="$(mktemp -d "${artifacts_parent%/}/kongctl-smoke.XXXXXX")" || exit 2
commands_dir="$artifacts_dir/commands"
explain_dir="$artifacts_dir/explain"
scaffold_dir="$artifacts_dir/scaffolds"
fixtures_dir="$artifacts_dir/fixtures"
plans_dir="$artifacts_dir/plans"
dumps_dir="$artifacts_dir/dumps"
known_issues_dir="$artifacts_dir/known-issues"
results_file="$artifacts_dir/results.jsonl"
resources_file="$artifacts_dir/resources.jsonl"
mkdir -p "$commands_dir" "$explain_dir" "$scaffold_dir" "$fixtures_dir" "$plans_dir" "$dumps_dir" \
  "$known_issues_dir"
: >"$results_file"
: >"$resources_file"

declare -a profile_args=()
if [[ -n "$profile_arg" ]]; then
  profile_args=(--profile "$profile_arg")
fi
effective_profile="${profile_arg:-${KONGCTL_PROFILE:-default}}"

check_index=0
current_name=""
current_resource=""
current_phase=""
current_stdout=""
current_stderr=""
current_command=""
current_exit=0
current_started=0
current_duration=0
run_started="$(date +%s)"
run_failure=""
cleanup_state="not_needed"
cleanup_failed="false"
known_issue_detected="false"
active_known_issue=""
active_known_issue_step=""
in_exit="false"
cleaned_keys=" "
declare -a touched_resources=()

# Each ID names one exact failure matcher and one recovery function below. A
# recovery is considered only after the ordinary smoke-test operation fails.
known_issue_registry=(1947)

shell_join() {
  local item
  local output=""
  for item in "$@"; do
    printf -v item '%q' "$item"
    output+="${output:+ }${item}"
  done
  printf '%s\n' "$output"
}

start_check() {
  current_name="$1"
  current_resource="$2"
  current_phase="$3"
  check_index=$((check_index + 1))
  local seq
  printf -v seq '%03d' "$check_index"
  current_stdout="$commands_dir/${seq}-${current_name}.stdout"
  current_stderr="$commands_dir/${seq}-${current_name}.stderr"
  current_command=""
  current_exit=0
  current_started="$(date +%s)"
  current_duration=0
  : >"$current_stdout"
  : >"$current_stderr"
  printf '==> %-42s' "$current_name"
}

run_kongctl() {
  current_command="$(shell_join "$kongctl_bin" "${profile_args[@]}" "$@")"
  set +e
  "$kongctl_bin" "${profile_args[@]}" "$@" >"$current_stdout" 2>"$current_stderr"
  current_exit=$?
  set -e
  current_duration=$(($(date +%s) - current_started))
}

record_current() {
  local status="$1"
  local message="${2:-}"
  RESULT_NAME="$current_name" \
    RESULT_RESOURCE="$current_resource" \
    RESULT_PHASE="$current_phase" \
    RESULT_STATUS="$status" \
    RESULT_MESSAGE="$message" \
    RESULT_COMMAND="$current_command" \
    RESULT_EXIT="$current_exit" \
    RESULT_DURATION="$current_duration" \
    RESULT_STDOUT="$current_stdout" \
    RESULT_STDERR="$current_stderr" \
    python3 - "$results_file" <<'PY'
import json
import os
import sys

record = {
    "name": os.environ["RESULT_NAME"],
    "resource": os.environ["RESULT_RESOURCE"] or None,
    "phase": os.environ["RESULT_PHASE"],
    "status": os.environ["RESULT_STATUS"],
    "message": os.environ["RESULT_MESSAGE"] or None,
    "command": os.environ["RESULT_COMMAND"],
    "exit_code": int(os.environ["RESULT_EXIT"]),
    "duration_seconds": int(os.environ["RESULT_DURATION"]),
    "stdout_file": os.environ["RESULT_STDOUT"],
    "stderr_file": os.environ["RESULT_STDERR"],
}
with open(sys.argv[1], "a", encoding="utf-8") as handle:
    handle.write(json.dumps(record, sort_keys=True) + "\n")
PY
}

pass_current() {
  record_current "passed" "${1:-}"
  echo " PASS"
}

fail_current() {
  local message="$1"
  record_current "failed" "$message"
  echo " FAIL"
  echo "smoke-test: $current_name: $message" >&2
  if [[ -s "$current_stderr" ]]; then
    sed -n '1,80p' "$current_stderr" >&2
  fi
  run_failure="$current_name: $message"
  exit 1
}

known_issue_title() {
  case "$1" in
    1947)
      echo "API document scaffold omits the title required by apply"
      ;;
    *)
      return 1
      ;;
  esac
}

known_issue_url() {
  case "$1" in
    1947)
      echo "https://github.com/Kong/kongctl/issues/1947"
      ;;
    *)
      return 1
      ;;
  esac
}

known_issue_step() {
  case "$1" in
    1947)
      echo "apply-create-api"
      ;;
    *)
      return 1
      ;;
  esac
}

has_known_issue_1947_failure() {
  grep -Fq 'for CREATE api_document: title is required; regenerate the plan' "$current_stdout" || return 1
  python3 - "$explain_dir/api.documents.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    schema = json.load(handle)
if "title" not in schema.get("properties", {}):
    raise SystemExit(1)
if "title" in schema.get("required", []):
    raise SystemExit(1)
PY
}

match_known_issue_1947() {
  [[ "$current_name" == "apply-create-api" ]] || return 1
  has_known_issue_1947_failure
}

has_known_issue_failure() {
  case "$1" in
    1947)
      has_known_issue_1947_failure
      ;;
    *)
      return 1
      ;;
  esac
}

match_known_issue() {
  case "$1" in
    1947)
      match_known_issue_1947
      ;;
    *)
      return 1
      ;;
  esac
}

write_known_issue_state() {
  local issue="$1"
  local detected="$2"
  local recovery_status="$3"
  local original_fixture="${4:-}"
  local workaround_fixture="${5:-}"
  local evaluated_step="${6:-${active_known_issue_step:-$current_name}}"
  KNOWN_ISSUE_ID="$issue" \
    KNOWN_ISSUE_TITLE="$(known_issue_title "$issue")" \
    KNOWN_ISSUE_URL="$(known_issue_url "$issue")" \
    KNOWN_ISSUE_STEP="$evaluated_step" \
    KNOWN_ISSUE_DETECTED="$detected" \
    KNOWN_ISSUE_RECOVERY="$recovery_status" \
    KNOWN_ISSUE_ORIGINAL="$original_fixture" \
    KNOWN_ISSUE_WORKAROUND="$workaround_fixture" \
    python3 - "$known_issues_dir/${issue}.json" <<'PY'
import json
import os
import pathlib
import sys

record = {
    "issue": int(os.environ["KNOWN_ISSUE_ID"]),
    "title": os.environ["KNOWN_ISSUE_TITLE"],
    "url": os.environ["KNOWN_ISSUE_URL"],
    "step": os.environ["KNOWN_ISSUE_STEP"],
    "detected": os.environ["KNOWN_ISSUE_DETECTED"] == "true",
    "recovery_status": os.environ["KNOWN_ISSUE_RECOVERY"],
    "original_fixture": os.environ["KNOWN_ISSUE_ORIGINAL"] or None,
    "workaround_fixture": os.environ["KNOWN_ISSUE_WORKAROUND"] or None,
}
pathlib.Path(sys.argv[1]).write_text(json.dumps(record, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

recognize_current_known_issue() {
  local issue
  for issue in "${known_issue_registry[@]}"; do
    if match_known_issue "$issue"; then
      active_known_issue="$issue"
      active_known_issue_step="$current_name"
      known_issue_detected="true"
      record_current "known_issue" "Kong/kongctl#$issue: $(known_issue_title "$issue")"
      echo " KNOWN ISSUE #$issue"
      echo "smoke-test: recognized $(known_issue_url "$issue"); attempting recovery" >&2
      write_known_issue_state "$issue" "true" "pending" "$resource_initial"
      return 0
    fi
  done
  return 1
}

record_undetected_known_issues() {
  local step="$1"
  local fixture="$2"
  local issue
  for issue in "${known_issue_registry[@]}"; do
    if [[ "$(known_issue_step "$issue")" == "$step" && ! -f "$known_issues_dir/${issue}.json" ]]; then
      write_known_issue_state "$issue" "false" "not_needed" "$fixture" "" "$step"
    fi
  done
}

require_success() {
  if [[ "$current_exit" -ne 0 ]]; then
    fail_current "command exited with status $current_exit"
  fi
}

assert_version_output() {
  if ! python3 - "$current_stdout" "$expect_version" "$expect_commit" >"$artifacts_dir/binary.json" <<'PY'
import json
import sys

path, expected_version, expected_commit = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    value = json.load(handle)
for field in ("version", "commit", "date"):
    if not str(value.get(field, "")).strip():
        raise SystemExit(f"version output has no {field}")
normalize_version = lambda version: version[1:] if version.startswith("v") else version
if expected_version and normalize_version(value["version"]) != normalize_version(expected_version):
    raise SystemExit(f"version {value['version']!r} does not match {expected_version!r}")
if expected_commit and not value["commit"].startswith(expected_commit):
    raise SystemExit(f"commit {value['commit']!r} does not start with {expected_commit!r}")
print(json.dumps(value, indent=2, sort_keys=True))
PY
  then
    fail_current "invalid or unexpected version output"
  fi
}

assert_explain_output() {
  local type_name="$1"
  local root_key="$2"
  local maturity="$3"
  shift 3
  if ! python3 - "$current_stdout" "$type_name" "$root_key" "$maturity" "$@" <<'PY'
import json
import sys

path, resource_type, root_key, maturity, *fields = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    schema = json.load(handle)
if schema.get("type") != "object":
    raise SystemExit("schema type is not object")
if schema.get("title") != f"kongctl declarative schema: {resource_type}":
    raise SystemExit("unexpected schema title")
if schema.get("x-kongctl-root-key") != root_key:
    raise SystemExit("unexpected root key")
if "ref" not in schema.get("required", []):
    raise SystemExit("ref is not required")
properties = schema.get("properties", {})
missing = [field for field in fields if field not in properties]
if missing:
    raise SystemExit(f"missing schema properties: {', '.join(missing)}")
actual_maturity = schema.get("x-kongctl-maturity", {}).get("level")
if maturity and actual_maturity != maturity:
    raise SystemExit(f"maturity {actual_maturity!r} does not match {maturity!r}")
PY
  then
    fail_current "explain output did not match the expected resource contract"
  fi
}

transform_scaffold() {
  local type_name="$1"
  local root_key="$2"
  local namespace="$3"
  local ref="$4"
  local name="$5"
  local display_name="$6"
  local description="$7"
  local raw_file="$8"
  local schema_file="$9"
  local initial_file="${10}"
  local updated_file="${11}"
  local nested_versions="${12:-}"
  local nested_documents="${13:-}"

  python3 - "$type_name" "$root_key" "$namespace" "$ref" "$name" "$display_name" "$description" \
    "$raw_file" "$schema_file" "$initial_file" "$updated_file" "$run_id" \
    "$nested_versions" "$nested_documents" <<'PY'
import json
import pathlib
import re
import sys

(
    resource_type,
    expected_root,
    namespace,
    reference,
    name,
    display_name,
    description,
    raw_path,
    schema_path,
    initial_path,
    updated_path,
    run_id,
    versions_path,
    documents_path,
) = sys.argv[1:]

schema = json.loads(pathlib.Path(schema_path).read_text(encoding="utf-8"))
root_key = schema.get("x-kongctl-root-key")
if root_key != expected_root:
    raise SystemExit(f"explain root key {root_key!r} does not match {expected_root!r}")

raw_lines = pathlib.Path(raw_path).read_text(encoding="utf-8").splitlines()
active = [line for line in raw_lines if line.strip() and not line.lstrip().startswith("#")]
if not active or active[0] != f"{root_key}:":
    raise SystemExit("scaffold root does not match explain root key")
if sum(1 for line in active if re.match(r"^  - ref:", line)) != 1:
    raise SystemExit("scaffold must contain exactly one active root ref")

quoted = lambda value: json.dumps(value, ensure_ascii=False)
found = set()
rendered = []
for line in active:
    if re.match(r"^  - ref:", line):
        line = f"  - ref: {quoted(reference)}"
        found.add("ref")
    elif re.match(r"^    name:", line):
        line = f"    name: {quoted(name)}"
        found.add("name")
    elif re.match(r"^    display_name:", line):
        line = f"    display_name: {quoted(display_name)}"
        found.add("display_name")
    elif re.match(r"^    description:", line):
        line = f"    description: {quoted(description)}"
        found.add("description")
    elif re.match(r"^    slug:", line):
        line = f"    slug: {quoted(name)}"
    rendered.append(line)

if "ref" not in found:
    raise SystemExit("scaffold has no active ref")
if "name" in schema.get("properties", {}) and "name" not in found:
    raise SystemExit("scaffold has no active name")
if "description" in schema.get("properties", {}) and "description" not in found:
    rendered.append(f"    description: {quoted(description)}")
if "display_name" in schema.get("properties", {}) and "display_name" not in found:
    raise SystemExit("scaffold has no active display_name")
if resource_type == "control_plane":
    rendered.append('    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE"')

def nested_block(path, key, replacements):
    if not path:
        return []
    lines = pathlib.Path(path).read_text(encoding="utf-8").splitlines()
    lines = [line for line in lines if line.strip() and not line.lstrip().startswith("#")]
    marker = f"    {key}:"
    try:
        start = lines.index(marker)
    except ValueError as error:
        raise SystemExit(f"nested scaffold has no {key} block") from error
    block = lines[start:]
    output = []
    for line in block:
        for old, new in replacements:
            line = line.replace(old, new)
        output.append(line)
    return output

if resource_type == "api":
    rendered.extend(nested_block(versions_path, "versions", [
        ("my-resource", f"{reference}-v1"),
        ("./specs/api.yaml", "./specs/smoke-api.yaml"),
    ]))
    rendered.extend(nested_block(documents_path, "documents", [
        ("my-resource", f"{reference}-guide"),
        ("./content.txt", "./content/smoke-document.md"),
    ]))

rendered.extend([
    "    labels:",
    f"      smoke-run: {quoted(run_id)}",
    '      smoke-phase: "initial"',
])
header = ["_defaults:", "  kongctl:", f"    namespace: {quoted(namespace)}"]
initial = "\n".join(header + rendered) + "\n"
updated = initial.replace(quoted(description), quoted(description + " updated"), 1)
updated = updated.replace('smoke-phase: "initial"', 'smoke-phase: "updated"', 1)
pathlib.Path(initial_path).parent.mkdir(parents=True, exist_ok=True)
pathlib.Path(initial_path).write_text(initial, encoding="utf-8")
pathlib.Path(updated_path).write_text(updated, encoding="utf-8")
PY
}

assert_plan() {
  local plan_file="$1"
  local mode_name="$2"
  local action="$3"
  local resource_type="$4"
  local resource_ref="$5"
  if ! python3 - "$plan_file" "$mode_name" "$action" "$resource_type" "$resource_ref" <<'PY'
import json
import sys

path, mode, action, resource_type, resource_ref = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    plan = json.load(handle)
if plan.get("metadata", {}).get("mode") != mode:
    raise SystemExit("unexpected plan mode")
matches = [
    change for change in plan.get("changes", [])
    if change.get("action") == action
    and change.get("resource_type") == resource_type
    and change.get("resource_ref") == resource_ref
]
if not matches:
    raise SystemExit("expected resource change is absent")
PY
  then
    fail_current "plan did not contain the expected $action for $resource_ref"
  fi
}

assert_zero_plan() {
  local plan_file="$1"
  if ! python3 - "$plan_file" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    plan = json.load(handle)
if plan.get("summary", {}).get("total_changes") != 0:
    raise SystemExit("plan has changes")
PY
  then
    fail_current "expected a zero-change plan"
  fi
}

assert_execution() {
  local minimum="$1"
  if ! python3 - "$current_stdout" "$minimum" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    result = json.load(handle)
summary = result.get("summary", {})
if summary.get("status") != "success" or summary.get("failed", 0) != 0:
    raise SystemExit("execution was not successful")
if int(summary.get("total_changes", 0)) < int(sys.argv[2]):
    raise SystemExit("execution had fewer changes than expected")
PY
  then
    fail_current "structured execution report was not successful"
  fi
}

assert_object_field() {
  local field="$1"
  local expected="$2"
  if ! python3 - "$current_stdout" "$field" "$expected" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    value = json.load(handle)
current = value
for part in sys.argv[2].split("."):
    current = current[part]
if str(current) != sys.argv[3]:
    raise SystemExit(f"{current!r} != {sys.argv[3]!r}")
PY
  then
    fail_current "response field $field did not equal $expected"
  fi
}

assert_collection() {
  local field="$1"
  local expected="$2"
  local presence="$3"
  if ! python3 - "$current_stdout" "$field" "$expected" "$presence" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    values = json.load(handle)
if isinstance(values, dict):
    values = values.get("data", values.get("items", []))
found = any(str(item.get(sys.argv[2], "")) == sys.argv[3] for item in values)
if found != (sys.argv[4] == "present"):
    raise SystemExit("collection presence assertion failed")
PY
  then
    fail_current "collection did not report $expected as $presence"
  fi
}

assert_text_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$file"; then
    fail_current "output did not contain: $expected"
  fi
}

record_resource() {
  RESOURCE_KEY="$1" RESOURCE_TYPE="$2" RESOURCE_ROOT="$3" RESOURCE_NAME="$4" RESOURCE_REF="$5" \
    RESOURCE_NAMESPACE="$6" RESOURCE_INITIAL="$7" RESOURCE_UPDATED="$8" \
    python3 - "$resources_file" <<'PY'
import json
import os
import sys
record = {
    "key": os.environ["RESOURCE_KEY"],
    "type": os.environ["RESOURCE_TYPE"],
    "root_key": os.environ["RESOURCE_ROOT"],
    "name": os.environ["RESOURCE_NAME"],
    "ref": os.environ["RESOURCE_REF"],
    "namespace": os.environ["RESOURCE_NAMESPACE"],
    "initial_file": os.environ["RESOURCE_INITIAL"],
    "updated_file": os.environ["RESOURCE_UPDATED"],
}
with open(sys.argv[1], "a", encoding="utf-8") as handle:
    handle.write(json.dumps(record, sort_keys=True) + "\n")
PY
}

prepare_resource() {
  local key="$1"
  local type_name="$2"
  local root_key="$3"
  local maturity="$4"
  local name="$5"
  local display_name="$6"
  local description="$7"
  shift 7
  local namespace="kctl-smoke-${run_suffix}-${key//_/-}"
  local ref="$name"
  local raw_file="$scaffold_dir/${key}.raw.yaml"
  local schema_file="$explain_dir/${key}.json"
  local resource_dir="$fixtures_dir/$key"
  local initial_file="$resource_dir/initial.yaml"
  local updated_file="$resource_dir/updated.yaml"
  local versions_file=""
  local documents_file=""
  mkdir -p "$resource_dir"

  start_check "explain-${key}" "$key" "discovery"
  run_kongctl explain "$type_name" -o json
  require_success
  assert_explain_output "$type_name" "$root_key" "$maturity" "$@"
  cp "$current_stdout" "$schema_file"
  pass_current

  start_check "scaffold-${key}" "$key" "scaffold"
  run_kongctl scaffold "$type_name"
  require_success
  cp "$current_stdout" "$raw_file"
  assert_text_contains "$raw_file" "$root_key:"
  pass_current

  if [[ "$key" == "api" ]]; then
    start_check "explain-api-versions" "$key" "discovery"
    run_kongctl explain api.versions -o json
    require_success
    assert_explain_output "api.versions" "api_versions" "ga" version spec ref
    cp "$current_stdout" "$explain_dir/api.versions.json"
    pass_current

    start_check "scaffold-api-versions" "$key" "scaffold"
    run_kongctl scaffold api.versions
    require_success
    versions_file="$scaffold_dir/api.versions.raw.yaml"
    cp "$current_stdout" "$versions_file"
    assert_text_contains "$versions_file" "spec: !file"
    pass_current

    start_check "explain-api-documents" "$key" "discovery"
    run_kongctl explain api.documents -o json
    require_success
    assert_explain_output "api.documents" "api_documents" "ga" content slug ref
    cp "$current_stdout" "$explain_dir/api.documents.json"
    pass_current

    start_check "scaffold-api-documents" "$key" "scaffold"
    run_kongctl scaffold api.documents
    require_success
    documents_file="$scaffold_dir/api.documents.raw.yaml"
    cp "$current_stdout" "$documents_file"
    assert_text_contains "$documents_file" "content: !file"
    pass_current
  fi

  start_check "fixture-${key}" "$key" "scaffold"
  current_command="transform scaffold outputs into $initial_file and $updated_file"
  if ! transform_scaffold "$type_name" "$root_key" "$namespace" "$ref" "$name" "$display_name" \
    "$description" "$raw_file" "$schema_file" "$initial_file" "$updated_file" "$versions_file" "$documents_file"
  then
    fail_current "could not transform scaffold into a smoke fixture"
  fi
  if [[ "$key" == "api" ]]; then
    mkdir -p "$resource_dir/specs" "$resource_dir/content"
    cat >"$resource_dir/specs/smoke-api.yaml" <<EOF
openapi: 3.0.0
info:
  title: ${display_name}
  version: v1.0.0
paths: {}
EOF
    cat >"$resource_dir/content/smoke-document.md" <<EOF
# ${display_name}

Generated by ${run_id} from kongctl scaffold output.
EOF
  fi
  pass_current "generated $initial_file"

  printf -v "${key}_type" '%s' "$type_name"
  printf -v "${key}_root" '%s' "$root_key"
  printf -v "${key}_namespace" '%s' "$namespace"
  printf -v "${key}_ref" '%s' "$ref"
  printf -v "${key}_name" '%s' "$name"
  printf -v "${key}_description" '%s' "$description"
  printf -v "${key}_initial" '%s' "$initial_file"
  printf -v "${key}_updated" '%s' "$updated_file"
  record_resource "$key" "$type_name" "$root_key" "$name" "$ref" "$namespace" "$initial_file" "$updated_file"
}

# To add a resource type: prepare its scaffold near the bottom of this file, add
# its key to resource_registry, and define one loader like the functions below.
# The shared lifecycle and cleanup functions will then exercise it uniformly.
load_resource_api() {
  resource_key="api"; resource_type="$api_type"; resource_root="$api_root"; resource_namespace="$api_namespace"
  resource_ref="$api_ref"; resource_name="$api_name"; resource_description="$api_description"
  resource_initial="$api_initial"; resource_updated="$api_updated"; dump_type="apis"
  detail_args=(get api "$resource_name" -o json); list_args=(list apis -o json); match_field="name"
}

load_resource_portal() {
  resource_key="portal"; resource_type="$portal_type"; resource_root="$portal_root"
  resource_namespace="$portal_namespace"
  resource_ref="$portal_ref"; resource_name="$portal_name"; resource_description="$portal_description"
  resource_initial="$portal_initial"; resource_updated="$portal_updated"; dump_type="portals"
  detail_args=(get portal "$resource_name" -o json); list_args=(list portals -o json); match_field="name"
}

load_resource_control_plane() {
  resource_key="control_plane"; resource_type="$control_plane_type"; resource_root="$control_plane_root"
  resource_namespace="$control_plane_namespace"; resource_ref="$control_plane_ref"; resource_name="$control_plane_name"
  resource_description="$control_plane_description"; resource_initial="$control_plane_initial"
  resource_updated="$control_plane_updated"; dump_type="control_planes"
  detail_args=(get gateway control-plane "$resource_name" -o json)
  list_args=(list gateway control-planes -o json); match_field="name"
}

load_resource_ai_gateway() {
  resource_key="ai_gateway"; resource_type="$ai_gateway_type"; resource_root="$ai_gateway_root"
  resource_namespace="$ai_gateway_namespace"; resource_ref="$ai_gateway_ref"; resource_name="$ai_gateway_name"
  resource_description="$ai_gateway_description"; resource_initial="$ai_gateway_initial"
  resource_updated="$ai_gateway_updated"; dump_type="ai_gateways"
  detail_args=(get ai-gateway "$resource_name" -o json)
  list_args=(list ai-gateways -o json); match_field="name"
}

declare -a detail_args=()
declare -a list_args=()
resource_key=""; resource_type=""; resource_root=""; resource_namespace=""; resource_ref=""; resource_name=""
resource_description=""; resource_initial=""; resource_updated=""; dump_type=""; match_field=""
api_type=""; api_root=""; api_namespace=""; api_ref=""; api_name=""; api_description=""; api_initial=""; api_updated=""
portal_type=""; portal_root=""; portal_namespace=""; portal_ref=""; portal_name=""; portal_description=""
portal_initial=""; portal_updated=""
control_plane_type=""; control_plane_root=""; control_plane_namespace=""; control_plane_ref=""; control_plane_name=""
control_plane_description=""; control_plane_initial=""; control_plane_updated=""
ai_gateway_type=""; ai_gateway_root=""; ai_gateway_namespace=""; ai_gateway_ref=""; ai_gateway_name=""
ai_gateway_description=""; ai_gateway_initial=""; ai_gateway_updated=""

check_detail_present() {
  start_check "get-${resource_key}" "$resource_key" "read"
  run_kongctl "${detail_args[@]}"
  require_success
  assert_object_field "$match_field" "$resource_name"
  pass_current
}

check_list_presence() {
  local presence="$1"
  start_check "list-${resource_key}-${presence}" "$resource_key" "read"
  run_kongctl "${list_args[@]}"
  require_success
  assert_collection "$match_field" "$resource_name" "$presence"
  pass_current
}

recover_known_issue_1947() {
  local original_initial="$resource_initial"
  local original_updated="$resource_updated"
  local workaround_initial="${original_initial%.yaml}.issue-1947.yaml"
  local workaround_updated="${original_updated%.yaml}.issue-1947.yaml"

  python3 - "$original_initial" "$workaround_initial" "$original_updated" "$workaround_updated" \
    "Smoke API ${run_suffix} Guide" <<'PY'
import json
import pathlib
import sys

source_initial, target_initial, source_updated, target_updated, title = sys.argv[1:]
marker = "      - content: !file ./content/smoke-document.md"
title_line = f"        title: {json.dumps(title)}"
for source, target in ((source_initial, target_initial), (source_updated, target_updated)):
    text = pathlib.Path(source).read_text(encoding="utf-8")
    if title_line in text:
        raise SystemExit(f"canonical fixture already contains issue #1947 recovery: {source}")
    if text.count(marker) != 1:
        raise SystemExit(f"could not identify the API document in {source}")
    recovered = text.replace(marker, marker + "\n" + title_line, 1)
    pathlib.Path(target).write_text(recovered, encoding="utf-8")
print(target_initial)
print(target_updated)
PY
  local status=$?
  if [[ "$status" -ne 0 ]]; then
    return "$status"
  fi

  resource_initial="$workaround_initial"
  resource_updated="$workaround_updated"
  api_initial="$workaround_initial"
  api_updated="$workaround_updated"
}

recover_known_issue() {
  case "$1" in
    1947)
      recover_known_issue_1947
      ;;
    *)
      return 1
      ;;
  esac
}

smoke_resource() {
  local key="$1"
  "load_resource_${key}"
  local create_plan="$plans_dir/${key}-create.json"
  local zero_plan="$plans_dir/${key}-zero.json"
  local dump_file="$dumps_dir/${key}.yaml"

  start_check "collision-${key}" "$key" "preflight"
  run_kongctl "${list_args[@]}"
  require_success
  assert_collection "$match_field" "$resource_name" "absent"
  pass_current

  start_check "plan-create-${key}" "$key" "create"
  run_kongctl plan -f "$resource_initial" --mode apply --require-namespace "$resource_namespace" \
    --output-file "$create_plan"
  require_success
  assert_plan "$create_plan" "apply" "CREATE" "$resource_type" "$resource_ref"
  pass_current

  start_check "diff-create-${key}" "$key" "create"
  run_kongctl diff --plan "$create_plan"
  require_success
  assert_text_contains "$current_stdout" "$resource_ref"
  pass_current

  touched_resources+=("$key")
  start_check "apply-create-${key}" "$key" "create"
  run_kongctl apply --plan "$create_plan" --auto-approve -o json
  if [[ "$current_exit" -eq 0 ]]; then
    assert_execution 1
    pass_current
    record_undetected_known_issues "apply-create-${key}" "$resource_initial"
  elif recognize_current_known_issue; then
    local issue="$active_known_issue"
    local original_fixture="$resource_initial"

    start_check "recover-known-issue-${issue}" "$key" "recovery"
    current_command="create a separate fixture for known issue #${issue}"
    set +e
    recover_known_issue "$issue" >"$current_stdout" 2>"$current_stderr"
    current_exit=$?
    set -e
    current_duration=$(($(date +%s) - current_started))
    if [[ "$current_exit" -ne 0 ]]; then
      write_known_issue_state "$issue" "true" "failed" "$original_fixture"
      require_success
    fi
    pass_current "temporary recovery for $(known_issue_url "$issue")"

    create_plan="$plans_dir/${key}-create.issue-${issue}.json"
    start_check "plan-create-${key}-after-${issue}" "$key" "recovery"
    run_kongctl plan -f "$resource_initial" --mode apply --require-namespace "$resource_namespace" \
      --output-file "$create_plan"
    require_success
    assert_plan "$create_plan" "apply" "CREATE" "$resource_type" "$resource_ref"
    pass_current

    start_check "diff-create-${key}-after-${issue}" "$key" "recovery"
    run_kongctl diff --plan "$create_plan"
    require_success
    assert_text_contains "$current_stdout" "$resource_ref"
    pass_current

    start_check "apply-create-${key}-after-${issue}" "$key" "recovery"
    run_kongctl apply --plan "$create_plan" --auto-approve -o json
    if [[ "$current_exit" -ne 0 ]]; then
      if has_known_issue_failure "$issue"; then
        write_known_issue_state "$issue" "true" "failed" "$original_fixture" "$resource_initial"
      else
        write_known_issue_state "$issue" "true" "succeeded" "$original_fixture" "$resource_initial"
      fi
      require_success
    fi
    assert_execution 1
    pass_current
    write_known_issue_state "$issue" "true" "succeeded" "$original_fixture" "$resource_initial"
  else
    require_success
  fi

  check_detail_present
  check_list_presence "present"

  start_check "sync-update-${key}" "$key" "update"
  run_kongctl sync -f "$resource_updated" --require-namespace "$resource_namespace" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current

  start_check "get-updated-${key}" "$key" "update"
  run_kongctl "${detail_args[@]}"
  require_success
  assert_object_field "description" "$resource_description updated"
  pass_current

  start_check "plan-zero-${key}" "$key" "update"
  run_kongctl plan -f "$resource_updated" --mode apply --require-namespace "$resource_namespace" \
    --output-file "$zero_plan"
  require_success
  assert_zero_plan "$zero_plan"
  pass_current

  start_check "dump-${key}" "$key" "dump"
  run_kongctl dump declarative --resources "$dump_type" --filter-name "$resource_name" \
    --default-namespace "$resource_namespace" --include-child-resources --output-file "$dump_file"
  require_success
  assert_text_contains "$dump_file" "$resource_root:"
  assert_text_contains "$dump_file" "$resource_name"
  if [[ "$key" == "api" ]]; then
    assert_text_contains "$dump_file" "versions:"
    assert_text_contains "$dump_file" "documents:"
  fi
  pass_current
}

best_effort_cleanup_resource() {
  local key="$1"
  "load_resource_${key}"
  start_check "cleanup-${key}" "$key" "cleanup"
  run_kongctl delete -f "$resource_updated" --require-namespace "$resource_namespace" --auto-approve -o json
  if [[ "$current_exit" -eq 0 ]]; then
    record_current "passed" "best-effort cleanup"
    echo " PASS"
    cleaned_keys+="$key "
  else
    record_current "failed" "best-effort delete exited with status $current_exit"
    echo " FAIL"
    cleanup_failed="true"
  fi
}

cleanup_resource() {
  local key="$1"
  "load_resource_${key}"
  local delete_plan="$plans_dir/${key}-delete.json"

  start_check "delete-dry-run-${key}" "$key" "cleanup"
  run_kongctl delete -f "$resource_updated" --require-namespace "$resource_namespace" --dry-run -o json
  require_success
  assert_execution 1
  pass_current

  check_detail_present

  start_check "plan-delete-${key}" "$key" "cleanup"
  run_kongctl plan -f "$resource_updated" --mode delete --require-namespace "$resource_namespace" \
    --output-file "$delete_plan"
  require_success
  assert_plan "$delete_plan" "delete" "DELETE" "$resource_type" "$resource_ref"
  pass_current

  start_check "diff-delete-${key}" "$key" "cleanup"
  run_kongctl diff --plan "$delete_plan"
  require_success
  assert_text_contains "$current_stdout" "$resource_ref"
  pass_current

  start_check "delete-${key}" "$key" "cleanup"
  run_kongctl delete --plan "$delete_plan" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current

  start_check "get-absent-${key}" "$key" "cleanup"
  run_kongctl "${detail_args[@]}"
  if [[ "$current_exit" -eq 0 ]]; then
    fail_current "resource still exists after delete"
  fi
  pass_current "expected not-found response"
  check_list_presence "absent"
  cleaned_keys+="$key "
}

is_cleaned() {
  [[ "$cleaned_keys" == *" $1 "* ]]
}

cleanup_all() {
  local style="$1"
  local i key
  cleanup_state="running"
  for ((i = ${#touched_resources[@]} - 1; i >= 0; i--)); do
    key="${touched_resources[$i]}"
    if is_cleaned "$key"; then
      continue
    fi
    if [[ "$style" == "strict" ]]; then
      cleanup_resource "$key"
    else
      best_effort_cleanup_resource "$key"
    fi
  done
  if [[ "$cleanup_failed" == "true" ]]; then
    cleanup_state="failed"
  else
    cleanup_state="passed"
  fi
}

write_report() {
  local exit_code="$1"
  local finished
  finished="$(date +%s)"
  REPORT_RUN_ID="$run_id" REPORT_MODE="$mode" REPORT_BINARY="$kongctl_bin" REPORT_PROFILE="$effective_profile" \
    REPORT_ARTIFACTS="$artifacts_dir" REPORT_CLEANUP="$cleanup_state" REPORT_EXIT="$exit_code" \
    REPORT_STARTED="$run_started" REPORT_FINISHED="$finished" REPORT_FAILURE="$run_failure" \
    python3 - "$results_file" "$resources_file" "$known_issues_dir" "$artifacts_dir/binary.json" \
    "$artifacts_dir/report.json" "$artifacts_dir/summary.txt" <<'PY'
import json
import os
import pathlib
import sys

results_path, resources_path, known_issues_path, binary_path, report_path, summary_path = map(
    pathlib.Path, sys.argv[1:]
)

def read_jsonl(path):
    if not path.exists():
        return []
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line]

checks = read_jsonl(results_path)
resources = read_jsonl(resources_path)
known_issue_checks = [
    json.loads(path.read_text(encoding="utf-8"))
    for path in sorted(known_issues_path.glob("*.json"))
]
known_issues = [issue for issue in known_issue_checks if issue["detected"]]
binary = {}
if binary_path.exists():
    binary = json.loads(binary_path.read_text(encoding="utf-8"))
binary["path"] = os.environ["REPORT_BINARY"]
exit_code = int(os.environ["REPORT_EXIT"])
report = {
    "schema_version": 2,
    "run_id": os.environ["REPORT_RUN_ID"],
    "mode": os.environ["REPORT_MODE"],
    "status": "passed" if exit_code == 0 else "failed",
    "exit_code": exit_code,
    "failure": os.environ["REPORT_FAILURE"] or None,
    "profile": os.environ["REPORT_PROFILE"],
    "binary": binary,
    "artifacts_dir": os.environ["REPORT_ARTIFACTS"],
    "started_epoch": int(os.environ["REPORT_STARTED"]),
    "finished_epoch": int(os.environ["REPORT_FINISHED"]),
    "duration_seconds": int(os.environ["REPORT_FINISHED"]) - int(os.environ["REPORT_STARTED"]),
    "cleanup": {"status": os.environ["REPORT_CLEANUP"]},
    "known_issue_checks": known_issue_checks,
    "known_issues": known_issues,
    "resources": resources,
    "checks": checks,
}
report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
passed = sum(check["status"] == "passed" for check in checks)
failed = sum(check["status"] == "failed" for check in checks)
known = sum(check["status"] == "known_issue" for check in checks)
lines = [
    f"kongctl smoke test: {report['status'].upper()}",
    f"mode: {report['mode']}",
    f"binary: {binary.get('path', '')}",
    f"version: {binary.get('version', 'unknown')}",
    f"commit: {binary.get('commit', 'unknown')}",
    f"profile: {report['profile']}",
    f"checks: {passed} passed, {failed} failed, {known} known issues",
    f"cleanup: {report['cleanup']['status']}",
]
for issue in known_issues:
    lines.append(
        f"known issue: #{issue['issue']} (recovery {issue['recovery_status']}) {issue['url']}"
    )
for issue in known_issue_checks:
    if not issue["detected"]:
        lines.append(
            f"known issue check: #{issue['issue']} not detected; recovery may be removable {issue['url']}"
        )
lines.append(f"artifacts: {report['artifacts_dir']}")
summary_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
PY
}

on_exit() {
  local code="$1"
  if [[ "$in_exit" == "true" ]]; then
    return
  fi
  in_exit="true"
  trap - EXIT INT TERM
  if [[ "$code" -ne 0 && "$mode" == "full" ]] &&
    [[ ${#touched_resources[@]} -gt 0 && "$cleanup_state" != "passed" ]]; then
    if [[ "$keep_on_failure" == "true" ]]; then
      cleanup_state="retained"
    else
      cleanup_all "best-effort"
    fi
  fi
  write_report "$code" || true
  echo
  cat "$artifacts_dir/summary.txt" 2>/dev/null || true
  exit "$code"
}

trap 'on_exit $?' EXIT
trap 'run_failure="interrupted"; exit 130' INT
trap 'run_failure="terminated"; exit 143' TERM

start_check "version" "" "preflight"
run_kongctl version --full -o json
require_success
assert_version_output
pass_current

start_check "help" "" "preflight"
run_kongctl --help
require_success
assert_text_contains "$current_stdout" "kongctl"
pass_current

resource_token="${run_suffix}"
prepare_resource "api" "api" "apis" "ga" "smoke-api-${resource_token}" "Smoke API ${resource_token}" \
  "kongctl smoke API" name description version slug
prepare_resource "portal" "portal" "portals" "ga" "smoke-portal-${resource_token}" "Smoke Portal ${resource_token}" \
  "kongctl smoke portal" name display_name description
prepare_resource "control_plane" "control_plane" "control_planes" "ga" "smoke-cp-${resource_token}" \
  "Smoke Control Plane ${resource_token}" "kongctl smoke control plane" name description cluster_type
prepare_resource "ai_gateway" "ai_gateway" "ai_gateways" "beta" "smoke-aigw-${resource_token}" \
  "Smoke AI Gateway ${resource_token}" "kongctl smoke AI gateway" name display_name description

resource_registry=(api portal control_plane ai_gateway)

if [[ "$mode" == "quick" ]]; then
  for key in "${resource_registry[@]}"; do
    "load_resource_${key}"
    start_check "list-${key}-quick" "$key" "read"
    run_kongctl "${list_args[@]}"
    require_success
    pass_current
  done
  cleanup_state="not_needed"
  exit 0
fi

if [[ "$assume_yes" != "true" ]] && ! is_truthy "${KONGCTL_SMOKE_YES:-}"; then
  binary_version="$(
    python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' \
      "$artifacts_dir/binary.json"
  )"
  echo
  echo "Binary:    $kongctl_bin ($binary_version)"
  echo "Profile:   $effective_profile"
  echo "Resources: APIs, portals, control planes, AI gateways"
  echo "Artifacts: $artifacts_dir"
  printf 'Continue with remote resource creation? [y/N] '
  read -r answer
  case "$answer" in
    y | Y | yes | YES)
      ;;
    *)
      run_failure="operator declined confirmation"
      exit 1
      ;;
  esac
fi

for key in "${resource_registry[@]}"; do
  smoke_resource "$key"
done

cleanup_all "strict"
if [[ "$known_issue_detected" == "true" ]]; then
  run_failure="known CLI issues were detected; see the known-issues section of the report"
  exit 1
fi
exit 0

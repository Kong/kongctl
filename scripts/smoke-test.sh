#!/usr/bin/env bash
# shellcheck disable=SC2317,SC2329 # Resource loaders and exit handlers are invoked indirectly.
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
  --resources LIST          Comma-separated subset of api,portal,control_plane,
                            ai_gateway,event_gateway,identity_principal,
                            identity_auth_server (default: all seven)
  --expect-version VERSION  Require the reported version (a leading v is ignored)
  --expect-commit SHA       Require the reported commit to start with SHA
  --artifacts-dir PATH      Parent directory for the timestamped run directory
  --yes                     Skip the full-suite confirmation prompt
  --keep-on-failure         Retain resources when the run fails or is interrupted
  -h, --help                Show this help

Environment:
  KONGCTL_BIN               Default binary override
  KONGCTL_SMOKE_YES         1, true, or yes skips the confirmation prompt

The full suite creates isolated APIs, portals, control planes, Event Gateways,
AI gateways, Identity principals, and Auth Servers with representative children,
then deletes each resource with kongctl delete. Authentication is taken from the
selected profile and the existing kongctl environment. If Identity has no
directory, full mode bootstraps a protected default directory and retains it
after cleanup. Quick mode never bootstraps resources.
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
resource_selection="api,portal,control_plane,ai_gateway,event_gateway,identity_principal,identity_auth_server"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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
    --resources)
      [[ $# -ge 2 && -n "$2" ]] || die_usage "--resources requires a non-empty list"
      resource_selection="$2"
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

declare -a resource_registry=()
IFS=',' read -r -a resource_registry <<<"$resource_selection"
[[ "$resource_selection" != *, ]] || die_usage "--resources cannot contain empty entries"
selected_keys=" "
for key in "${resource_registry[@]}"; do
  case "$key" in
    api | portal | control_plane | ai_gateway | event_gateway | identity_principal | identity_auth_server) ;;
    *) die_usage "unknown resource: $key" ;;
  esac
  [[ "$selected_keys" != *" $key "* ]] || die_usage "duplicate resource: $key"
  selected_keys+="$key "
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
results_file="$artifacts_dir/results.jsonl"
resources_file="$artifacts_dir/resources.jsonl"
mkdir -p "$commands_dir" "$explain_dir" "$scaffold_dir" "$fixtures_dir" "$plans_dir" "$dumps_dir"
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
in_exit="false"
cleaned_keys=" "
declare -a touched_resources=()

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

run_saved_plan_with_changed_namespace() {
  local namespace_var="KONGCTL_SMOKE_NAMESPACE_${resource_key^^}"
  local "$namespace_var"
  printf -v "$namespace_var" '%s' "kctl-smoke-${run_suffix}-changed"
  export "${namespace_var?}"
  run_kongctl "$@"
  current_command="$(shell_join env "$namespace_var=${!namespace_var}") $current_command"
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
# Identity scaffolds use the grouped identity: envelope. Normalize indentation
# for the shared transformer; fixtures.py restores the envelope after enrichment.
if resource_type.startswith("identity_"):
    if not raw_lines or raw_lines[0] != "identity:":
        raise SystemExit("identity scaffold is missing its envelope")
    raw_lines = [line[2:] for line in raw_lines[1:]]
    root_key = root_key.removeprefix("identity.")
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
namespace_var = f"KONGCTL_SMOKE_NAMESPACE_{resource_type.upper()}"
header = ["_defaults:", "  kongctl:", f"    namespace: !env {namespace_var}"]
initial = "\n".join(header + rendered) + "\n"
updated = initial.replace(quoted(description), quoted(description + " updated"), 1)
updated = updated.replace('smoke-phase: "initial"', 'smoke-phase: "updated"', 1)
# Updates exercise explicit metadata without a file default masking resolution.
updated = "\n".join(updated.splitlines()[len(header):]) + "\n"
updated += f"    kongctl:\n      namespace: !env {{var: {namespace_var}}}\n"
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
  if ! python3 - "$plan_file" "$mode_name" "$action" "$resource_type" "$resource_ref" \
    "$resource_namespace" <<'PY'
import json
import sys

path, mode, action, resource_type, resource_ref, namespace = sys.argv[1:]
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
if len(matches) != 1:
    raise SystemExit("expected exactly one resource change")
changes = plan.get("changes", [])
if plan.get("summary", {}).get("total_changes") != len(changes):
    raise SystemExit("plan summary does not match changes")
if any(change.get("action") != action for change in changes):
    raise SystemExit("plan contains unexpected actions")
if any(change.get("namespace") != namespace for change in changes):
    raise SystemExit("plan contains a change outside the smoke namespace")

PY
  then
    fail_current "plan did not contain the expected $action for $resource_ref"
  fi
  local stage="create"
  if [[ "$action" == "DELETE" && "$resource_type" == identity_* ]]; then
    stage="delete"
  fi
  if [[ "$action" == "CREATE" || "$stage" == "delete" ]]; then
    if ! python3 "$script_dir/smoke/assert-plan.py" "$plan_file" "$resource_type" "$stage" \
      "$resource_namespace" 2>>"$current_stderr"; then
      fail_current "unexpected $stage resources or dependency order"
    fi
  fi
}

assert_zero_plan() {
  local plan_file="$1"
  if ! python3 - "$plan_file" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    plan = json.load(handle)
if plan.get("summary", {}).get("total_changes") != 0 or plan.get("changes") != []:
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
  local comparison="${3:-text}"
  if ! python3 - "$current_stdout" "$field" "$expected" "$comparison" <<'PY'
import json
import math
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    value = json.load(handle)
current = value
for part in sys.argv[2].split("."):
    if isinstance(current, list) and "=" in part:
        key, expected_value = part.split("=", 1)
        matches = [item for item in current if item.get(key) == expected_value]
        if len(matches) != 1:
            raise SystemExit(f"expected exactly one array entry with {part}")
        current = matches[0]
    else:
        current = current[int(part)] if isinstance(current, list) else current[part]
expected, comparison = sys.argv[3:]
if comparison == "text":
    matches = str(current) == expected
else:
    numeric = isinstance(current, (int, float)) and not isinstance(current, bool)
    if comparison == "integer":
        matches = numeric and current == int(expected)
    elif comparison == "number":
        matches = numeric and math.isclose(current, float(expected), rel_tol=1e-12, abs_tol=1e-12)
    else:
        raise SystemExit(f"unknown comparison: {comparison}")
if not matches:
    raise SystemExit(f"{current!r} != {expected!r}")
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
    values = values.get("data", values.get("items"))
if not isinstance(values, list) or any(not isinstance(item, dict) for item in values):
    raise SystemExit("expected a collection of objects")
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
  local namespace_var="KONGCTL_SMOKE_NAMESPACE_${key^^}"
  printf -v "$namespace_var" '%s' "$namespace"
  export "${namespace_var?}"
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
  assert_text_contains "$raw_file" "${root_key##*.}:"
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
  if ! python3 "$script_dir/smoke/fixtures.py" "$key" "$resource_dir"; then
    fail_current "could not enrich smoke fixtures"
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

load_resource_event_gateway() {
  resource_key="event_gateway"; resource_type="$event_gateway_type"; resource_root="$event_gateway_root"
  resource_namespace="$event_gateway_namespace"; resource_ref="$event_gateway_ref"
  resource_name="$event_gateway_name"; resource_description="$event_gateway_description"
  resource_initial="$event_gateway_initial"; resource_updated="$event_gateway_updated"; dump_type="event_gateways"
  detail_args=(get event-gateway "$resource_name" -o json)
  list_args=(list konnect event-gateways -o json); match_field="name"
}

load_resource_identity_principal() {
  resource_key="identity_principal"; resource_type="$identity_principal_type"; resource_root="$identity_principal_root"
  resource_namespace="$identity_principal_namespace"; resource_ref="$identity_principal_ref"; resource_name="$identity_principal_name"
  resource_description="$identity_principal_description"; resource_initial="$identity_principal_initial"
  resource_updated="$identity_principal_updated"; dump_type=""
  detail_args=(get identity principal --directory-name "$identity_directory_name" "$identity_principal_id" -o json)
  list_args=(list identity principals --directory-name "$identity_directory_name" -o json); match_field="display_name"
}

load_resource_identity_auth_server() {
  resource_key="identity_auth_server"; resource_type="$identity_auth_server_type"; resource_root="$identity_auth_server_root"
  resource_namespace="$identity_auth_server_namespace"; resource_ref="$identity_auth_server_ref"; resource_name="$identity_auth_server_name"
  resource_description="$identity_auth_server_description"; resource_initial="$identity_auth_server_initial"
  resource_updated="$identity_auth_server_updated"; dump_type="identity.auth_servers"
  detail_args=(get identity auth-server "$resource_name" -o json)
  list_args=(list identity auth-server -o json); match_field="name"
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
event_gateway_type=""; event_gateway_root=""; event_gateway_namespace=""; event_gateway_ref=""
event_gateway_name=""; event_gateway_description=""; event_gateway_initial=""; event_gateway_updated=""

identity_principal_id=""
identity_principal_type=""; identity_principal_root=""; identity_principal_namespace=""; identity_principal_ref=""
identity_principal_name=""; identity_principal_description=""; identity_principal_initial=""; identity_principal_updated=""
identity_auth_server_type=""; identity_auth_server_root=""; identity_auth_server_namespace=""; identity_auth_server_ref=""
identity_auth_server_name=""; identity_auth_server_description=""
identity_auth_server_initial=""; identity_auth_server_updated=""

identity_directory_name="default"
identity_directory_id=""

# A directory is shared organization infrastructure, not a disposable smoke root.
discover_identity_directory() {
  local require_present="${1:-false}"
  start_check "list-identity-directories" "identity_principal" "preflight"
  run_kongctl list identity directories -o json
  require_success
  if ! python3 - "$current_stdout" "$artifacts_dir/identity-directory.json" "$require_present" <<'PYDIRECTORY'
import json
import pathlib
import sys
import uuid

values = json.loads(pathlib.Path(sys.argv[1]).read_text())
if isinstance(values, dict):
    values = values.get("data", values.get("items"))
if not isinstance(values, list):
    raise SystemExit("expected a directory collection")
for value in values:
    if not isinstance(value, dict) or not isinstance(value.get("name"), str) or not value["name"]:
        raise SystemExit("directory has no name")
    uuid.UUID(value["id"])
if not values and sys.argv[3] == "true":
    raise SystemExit("no directory found after bootstrap")
selected = min(values, key=lambda value: (value["name"] != "default", value["name"], value["id"])) if values else {
    "name": "default", "id": "",
}
pathlib.Path(sys.argv[2]).write_text(json.dumps({"name": selected["name"], "id": selected["id"]}) + "\n")
PYDIRECTORY
  then
    fail_current "could not select an Identity directory"
  fi
  identity_directory_name="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["name"])' \
    "$artifacts_dir/identity-directory.json")"
  identity_directory_id="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["id"])' \
    "$artifacts_dir/identity-directory.json")"
  export KONGCTL_SMOKE_IDENTITY_DIRECTORY_NAME="$identity_directory_name"
  pass_current
}

prepare_identity_directory_bootstrap() {
  start_check "fixture-identity-directory-bootstrap" "identity_principal" "scaffold"
  current_command="write $fixtures_dir/identity-directory-bootstrap.yaml"
  if ! python3 - "$fixtures_dir/identity-directory-bootstrap.yaml" \
    >"$current_stdout" 2>"$current_stderr" <<'PYBOOTSTRAP'
import json
import pathlib
import sys

manifest = {"_defaults": {"kongctl": {"namespace": "kctl-smoke-identity-bootstrap"}}, "identity": {"directories": [{
    "ref": "default", "name": "default", "allow_all_control_planes": True,
    "kongctl": {"protected": True},
}]}}
pathlib.Path(sys.argv[1]).write_text(json.dumps(manifest, indent=2) + "\n")
PYBOOTSTRAP
  then
    current_exit=1
    fail_current "could not write the Identity bootstrap fixture"
  fi
  pass_current
}

bootstrap_identity_directory() {
  local plan_file="$plans_dir/identity-directory-bootstrap.json"
  start_check "plan-bootstrap-identity-directory" "identity_principal" "bootstrap"
  run_kongctl plan -f "$fixtures_dir/identity-directory-bootstrap.yaml" --mode apply \
    --require-namespace kctl-smoke-identity-bootstrap --output-file "$plan_file"
  require_success
  if ! python3 - "$plan_file" <<'PYBOOTSTRAPPLAN'
import json
import sys

plan = json.load(open(sys.argv[1]))
changes = plan.get("changes", [])
if plan.get("metadata", {}).get("mode") != "apply" or plan.get("summary", {}).get("total_changes") != 1:
    raise SystemExit("expected exactly one bootstrap create")
if len(changes) != 1:
    raise SystemExit("bootstrap plan must contain only the directory")
change = changes[0]
if any(change.get(key) != value for key, value in {
    "action": "CREATE", "resource_type": "identity_directory", "resource_ref": "default",
    "namespace": "kctl-smoke-identity-bootstrap", "protection": True,
}.items()):
    raise SystemExit("unexpected directory bootstrap change")
if change.get("fields", {}).get("name") != "default" or change["fields"].get("allow_all_control_planes") is not True:
    raise SystemExit("bootstrap must create default with allow_all_control_planes enabled")
PYBOOTSTRAPPLAN
  then
    fail_current "unsafe Identity directory bootstrap plan"
  fi
  pass_current

  start_check "bootstrap-identity-directory" "identity_principal" "bootstrap"
  run_kongctl apply --plan "$plan_file" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current "retained protected directory; excluded from smoke cleanup"
  discover_identity_directory true
}

select_smoke_principal() {
  start_check "select-smoke-principal" "$resource_key" "read"
  run_kongctl get identity principals --directory-name "$identity_directory_name" --display-name "$resource_name" -o json
  require_success
  if ! identity_principal_id="$(python3 -c '
import json, sys, uuid
values = json.load(open(sys.argv[1]))
assert len(values) == 1
print(uuid.UUID(values[0]["id"]))
' "$current_stdout")"; then
    fail_current "expected exactly one smoke principal with a UUID"
  fi
  detail_args=(get identity principal --directory-name "$identity_directory_name" "$identity_principal_id" -o json)
  pass_current
}

check_detail_present() {
  start_check "get-${resource_key}" "$resource_key" "read"
  run_kongctl "${detail_args[@]}"
  require_success
  assert_object_field "$match_field" "$resource_name"
  if [[ "$resource_key" == "identity_principal" ]]; then
    assert_object_field "managed_by.kongctl-namespace" "$resource_namespace"
  else
    assert_object_field "labels.KONGCTL-namespace" "$resource_namespace"
  fi
  pass_current
}

check_local_features() {
  local fixture="$fixtures_dir/tagged.yaml"
  cat >"$fixture" <<'YAML'
resource:
  name: before
  parent: !ref smoke-parent
  content: !file ./does-not-exist.txt
  environment: !env KONGCTL_SMOKE_UNSET
  secret: !secret {source: !env KONGCTL_SMOKE_UNSET}
YAML
  start_check "patch-preserves-tags" "" "local"
  run_kongctl patch file "$fixture" -s '$.resource' -v 'name:"after"' -o text
  require_success
  local expected
  for expected in 'name: after' '!ref smoke-parent' '!file ./does-not-exist.txt' \
    '!env KONGCTL_SMOKE_UNSET' '!secret'; do
    assert_text_contains "$current_stdout" "$expected"
  done
  assert_text_contains "$fixture" "name: before"
  pass_current

  start_check "explain-provider-auth" "ai_gateway" "discovery"
  run_kongctl explain ai_gateway.model_providers --extended -o text
  require_success
  assert_text_contains "$current_stdout" "type=openai"
  assert_text_contains "$current_stdout" "type=azure"
  assert_text_contains "$current_stdout" "allowed: basic"
  pass_current

  start_check "explain-vault-auth" "ai_gateway" "discovery"
  run_kongctl explain ai_gateway.vaults --extended -o text
  require_success
  assert_text_contains "$current_stdout" "auth_method=approle"
  assert_text_contains "$current_stdout" "auth_method=kubernetes"
  pass_current

  local config_file="$fixtures_dir/config file.yaml"
  printf 'smoke-env:\n  output: json\n' >"$config_file"
  start_check "config-file-environment" "" "local"
  KONGCTL_CONFIG_FILE="$config_file" run_kongctl get profiles --profile smoke-env -o json
  current_command="KONGCTL_CONFIG_FILE=$(shell_join "$config_file") $current_command"
  require_success
  assert_text_contains "$current_stdout" '"smoke-env"'
  pass_current

  start_check "config-file-flag-precedence" "" "local"
  KONGCTL_CONFIG_FILE="$fixtures_dir/missing.yaml" run_kongctl get profiles \
    --profile smoke-env --config-file "$config_file" -o json
  current_command="KONGCTL_CONFIG_FILE=$(shell_join "$fixtures_dir/missing.yaml") $current_command"
  require_success
  assert_text_contains "$current_stdout" '"smoke-env"'
  pass_current

  start_check "config-file-missing" "" "local"
  KONGCTL_CONFIG_FILE="$fixtures_dir/missing.yaml" run_kongctl get profiles --profile smoke-env -o json
  current_command="KONGCTL_CONFIG_FILE=$(shell_join "$fixtures_dir/missing.yaml") $current_command"
  if [[ "$current_exit" -eq 0 ]]; then
    fail_current "missing custom config unexpectedly succeeded"
  fi
  assert_text_contains "$current_stderr" "provided config file path does not exist"
  pass_current
}

# shellcheck source=scripts/smoke/adoption.sh
source "$script_dir/smoke/adoption.sh"

check_list_presence() {
  local presence="$1"
  start_check "list-${resource_key}-${presence}" "$resource_key" "read"
  run_kongctl "${list_args[@]}"
  require_success
  assert_collection "$match_field" "$resource_name" "$presence"
  pass_current
}

smoke_resource() {
  local key="$1"
  "load_resource_${key}"
  local create_plan="$plans_dir/${key}-create.json"
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
  run_saved_plan_with_changed_namespace apply --plan "$create_plan" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current

  if [[ "$key" == "identity_principal" ]]; then
    select_smoke_principal
  fi
  check_detail_present
  check_list_presence "present"

  start_check "list-text-${key}" "$key" "read"
  run_kongctl "${list_args[@]}" -o text --no-trunc
  require_success
  if [[ "$key" == "ai_gateway" ]]; then
    assert_text_contains "$current_stdout" "Smoke AI Gateway ${run_suffix}"
  else
    assert_text_contains "$current_stdout" "$resource_name"
  fi
  pass_current

  start_check "sync-update-${key}" "$key" "update"
  run_kongctl sync -f "$resource_updated" --require-namespace "$resource_namespace" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current

  start_check "get-updated-${key}" "$key" "update"
  run_kongctl "${detail_args[@]}"
  require_success
  assert_object_field "description" "$resource_description updated"
  assert_object_field "labels.smoke-phase" "updated"
  pass_current

  check_updated_children

  check_zero_plans "$resource_updated" "updated"
  check_adoption

  if [[ "$key" != "identity_principal" ]]; then
    start_check "dump-${key}" "$key" "dump"
    run_kongctl dump declarative --resources "$dump_type" --filter-name "$resource_name" \
      --default-namespace "$resource_namespace" --include-child-resources --output-file "$dump_file"
    require_success
    assert_text_contains "$dump_file" "${resource_root##*.}:"
    assert_text_contains "$dump_file" "$resource_name"
    if [[ "$key" == "identity_auth_server" ]]; then
      assert_text_contains "$dump_file" "scopes:"
      assert_text_contains "$dump_file" "claims:"
    fi
    if [[ "$key" == "api" ]]; then
      assert_text_contains "$dump_file" "versions:"
      assert_text_contains "$dump_file" "documents:"
    fi
    if [[ "$key" == "ai_gateway" ]]; then
      assert_text_contains "$dump_file" "datastores:"
      assert_text_contains "$dump_file" "smoke-store"
      # The model also names the datastore; require the datastore definition's type.
      if ! grep -Eq 'type"?:[[:space:]]*"?vectordb' "$dump_file"; then
        fail_current "dump did not include the vector datastore definition"
      fi
    fi
    pass_current
    check_zero_plans "$dump_file" "dump"
  fi

  if [[ "$key" == "event_gateway" ]]; then
    start_check "dump-root-only-${key}" "$key" "dump"
    run_kongctl dump declarative --resources "$dump_type" --filter-name "$resource_name" \
      --default-namespace "$resource_namespace"
    require_success
    if grep -Eq '^[[:space:]]*(backend_clusters|virtual_clusters):' "$current_stdout"; then
      fail_current "default dump unexpectedly includes children"
    fi
    assert_text_contains "$current_stdout" "$resource_name"
    pass_current
  fi

  if [[ "$key" == "ai_gateway" ]]; then
    check_child_transition "$fixtures_dir/$key/replacement.yaml" "replacement"
  fi
  if [[ -f "$fixtures_dir/$key/pruned.yaml" ]]; then
    check_child_transition "$fixtures_dir/$key/pruned.yaml" "prune"
    # Restore children so root deletion also exercises populated roots.
    start_check "restore-children-${key}" "$key" "update"
    run_kongctl sync -f "$resource_updated" --require-namespace "$resource_namespace" --auto-approve -o json
    require_success
    assert_execution 1
    pass_current
    check_zero_plans "$resource_updated" "restored"
  fi
}

check_zero_plans() {
  local fixture="$1" stage="$2" plan_mode plan_file
  for plan_mode in apply sync; do
    plan_file="$plans_dir/${resource_key}-${stage}-${plan_mode}-zero.json"
    start_check "plan-zero-${resource_key}-${stage}-${plan_mode}" "$resource_key" "idempotency"
    run_kongctl plan -f "$fixture" --mode "$plan_mode" --require-namespace "$resource_namespace" \
      --output-file "$plan_file"
    require_success
    assert_zero_plan "$plan_file"
    pass_current
  done
}

check_updated_children() {
  if [[ "$resource_key" == "identity_principal" ]]; then
    start_check "list-updated-principals" "$resource_key" "read"
    run_kongctl get identity principals --directory-name "$identity_directory_name" --display-name "$resource_name" -o json
    require_success
    assert_object_field "0.display_name" "$resource_name"
    assert_object_field "0.description" "$resource_description updated"
    assert_object_field "0.metadata.department" "platform"
    assert_object_field "0.metadata.attempts" "4" integer
    assert_object_field "0.metadata.trusted" "True"
    local principal_id
    if ! principal_id="$(python3 -c '
import json, sys, uuid
values = json.load(open(sys.argv[1]))
assert len(values) == 1
print(uuid.UUID(values[0]["id"]))
' "$current_stdout")"; then
      fail_current "expected exactly one principal with a UUID"
    fi
    pass_current

    start_check "get-updated-principal" "$resource_key" "read"
    run_kongctl get identity principal --directory-name "$identity_directory_name" "$principal_id" -o json
    require_success
    assert_object_field "id" "$principal_id"
    assert_object_field "type" "user"
    assert_object_field "managed_by.kongctl-namespace" "$resource_namespace"
    pass_current

    start_check "list-updated-identities" "$resource_key" "read"
    run_kongctl get identity principal-identities --directory-name "$identity_directory_name" --principal-id "$principal_id" -o json
    require_success
    assert_object_field "0.type" "custom"
    assert_object_field "0.key" "employee_id"
    assert_object_field "0.value" "${resource_ref}-employee-updated"
    pass_current
  elif [[ "$resource_key" == "identity_auth_server" ]]; then
    start_check "get-updated-auth-server" "$resource_key" "read"
    run_kongctl "${detail_args[@]}"
    require_success
    assert_object_field "audience" "https://updated.smoke.example.test"
    pass_current

    start_check "get-updated-scope" "$resource_key" "read"
    run_kongctl get identity auth-server-scope --auth-server-name "$resource_name" smoke-read -o json
    require_success
    assert_object_field "name" "smoke-read"
    assert_object_field "description" "Smoke read scope updated"
    assert_object_field "default" "True"
    local scope_id
    if ! scope_id="$(python3 -c 'import json, sys, uuid; print(uuid.UUID(json.load(open(sys.argv[1]))["id"]))' \
      "$current_stdout")"; then
      fail_current "scope has no valid UUID"
    fi
    pass_current

    start_check "get-updated-claim" "$resource_key" "read"
    run_kongctl get identity auth-server-claim --auth-server-name "$resource_name" smoke-team -o json
    require_success
    assert_object_field "name" "smoke-team"
    assert_object_field "value" '"platform"'
    assert_object_field "include_in_scopes.0" "$scope_id"
    pass_current
  elif [[ "$resource_key" == "ai_gateway" ]]; then
    start_check "list-datastores" "$resource_key" "read"
    run_kongctl get ai-gateway datastores --gateway-name "$resource_name" -o json
    require_success
    assert_collection "name" "smoke-store" "present"
    pass_current

    start_check "get-updated-datastore" "$resource_key" "read"
    run_kongctl get ai-gateway datastores --gateway-name "$resource_name" smoke-store -o json
    require_success
    assert_object_field "name" "smoke-store"
    assert_object_field "display_name" "Smoke store updated"
    assert_object_field "type" "vectordb"
    assert_object_field "config.host" "database.example.com"
    assert_object_field "config.database" "smoke"
    pass_current

    start_check "get-updated-model" "$resource_key" "read"
    run_kongctl get ai-gateway models --gateway-name "$resource_name" --model-name smoke-model -o json
    require_success
    assert_object_field "name" "smoke-model"
    assert_object_field "display_name" "Smoke model updated"
    assert_object_field "targets.0.provider" "smoke-provider"
    assert_object_field "policies.0" "smoke-policy"
    assert_object_field "datastores.0.name" "smoke-store"
    assert_object_field "config.balancer.algorithm" "semantic"
    assert_object_field "config.balancer.embeddings.provider" "smoke-provider"
    assert_object_field "config.balancer.embeddings.name" "text-embedding-3-small"
    assert_object_field "config.balancer.embeddings.config.type" "openai"
    assert_object_field "config.balancer.vectordb.type" "pgvector"
    assert_object_field "config.balancer.vectordb.dimensions" "1536" integer
    assert_object_field "config.balancer.vectordb.distance_metric" "cosine"
    assert_object_field "config.balancer.vectordb.threshold" "0.7" number
    assert_object_field "targets.0.semantic_description" "General business questions and detailed explanations."
    assert_object_field "targets.1.name" "gpt-4o-mini"
    assert_object_field "targets.1.provider" "smoke-provider"
    assert_object_field "targets.1.semantic_description" "CATCHALL"
    assert_object_field "targets.0.config.input_cost_list.0.cost" "3.5"
    assert_object_field "targets.0.config.output_cost_list.0.cost" "10"
    assert_object_field "targets.0.config.cache_read_cost_list.0.cost" "0.5"
    assert_object_field "config.route.model.values.1" "smoke-alias"
    pass_current

    start_check "get-updated-policy" "$resource_key" "read"
    run_kongctl get ai-gateway policies --gateway-name "$resource_name" smoke-policy -o json
    require_success
    assert_object_field "config.minute" "20"
    assert_object_field "condition" "http.method == 'POST'"
    pass_current

    start_check "get-updated-mcp" "$resource_key" "read"
    run_kongctl get ai-gateway mcp-servers --gateway-name "$resource_name" smoke-mcp -o json
    require_success
    assert_object_field "rate_limiting.limits.method=tools/list.limit" "20"
    assert_object_field "rate_limiting.limits.method=tools/list.window_size" "60"
    assert_object_field "rate_limiting.limits.tool=lookup.limit" "10"
    assert_object_field "rate_limiting.limits.tool=lookup.window_size" "1"
    assert_object_field "access.acls.allow_when.0" "http.headers.x_smoke_user == 'allowed'"
    assert_object_field "access.default_tool_acls.deny_when.0" "http.headers.x_smoke_blocked == 'true'"
    assert_object_field "tools.name=lookup.access.acls.allow_when.0" "http.headers.x_smoke_tool == 'allowed'"
    assert_object_field "config.allowed_versions.0" "2025-11-25"
    assert_object_field "config.cache.tools_list.ttl_ms" "0"
    assert_object_field "config.cache.discover.ttl_ms" "120000"
    assert_object_field "config.cache.discover.cache_scope" "public"
    pass_current
  elif [[ "$resource_key" == "portal" ]]; then
    start_check "get-updated-snippet" "$resource_key" "read"
    run_kongctl get portal snippets --portal-name "$resource_name" smoke-snippet -o json
    require_success
    assert_object_field "content" "Smoke snippet updated content"
    pass_current
  fi
}

check_child_transition() {
  local fixture="$1" stage="$2" plan_file="$plans_dir/${resource_key}-$2.json"
  start_check "plan-${stage}-${resource_key}" "$resource_key" "update"
  run_kongctl plan -f "$fixture" --mode sync --require-namespace "$resource_namespace" --output-file "$plan_file"
  require_success
  if ! python3 "$script_dir/smoke/assert-plan.py" "$plan_file" "$resource_key" "$stage" \
    "$resource_namespace" 2>>"$current_stderr"; then
    fail_current "unexpected child transition or dependency order"
  fi
  pass_current
  start_check "sync-${stage}-${resource_key}" "$resource_key" "update"
  run_saved_plan_with_changed_namespace sync --plan "$plan_file" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current
  check_detail_present
  check_zero_plans "$fixture" "$stage"
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
  run_saved_plan_with_changed_namespace delete --plan "$delete_plan" --auto-approve -o json
  require_success
  assert_execution 1
  pass_current

  start_check "get-absent-${key}" "$key" "cleanup"
  run_kongctl "${detail_args[@]}"
  if [[ "$current_exit" -eq 0 ]]; then
    fail_current "resource still exists after delete"
  fi
  if ! grep -Eiq 'not found' "$current_stderr"; then
    fail_current "get failed without a not-found diagnostic"
  fi
  pass_current "expected not-found response"
  check_list_presence "absent"
  if [[ "$key" == "identity_principal" ]]; then
    start_check "retain-default-directory" "$key" "cleanup"
    run_kongctl get identity directory "$identity_directory_name" -o json
    require_success
    assert_object_field "name" "$identity_directory_name"
    pass_current
  fi
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
    python3 - "$results_file" "$resources_file" "$artifacts_dir/binary.json" \
    "$artifacts_dir/report.json" "$artifacts_dir/summary.txt" <<'PY'
import json
import os
import pathlib
import sys

results_path, resources_path, binary_path, report_path, summary_path = map(
    pathlib.Path, sys.argv[1:]
)

def read_jsonl(path):
    if not path.exists():
        return []
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line]

checks = read_jsonl(results_path)
resources = read_jsonl(resources_path)
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
    "known_issue_checks": [],
    "known_issues": [],
    "resources": resources,
    "checks": checks,
}
report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
passed = sum(check["status"] == "passed" for check in checks)
failed = sum(check["status"] == "failed" for check in checks)
known = sum(check["status"] == "known_issue" for check in checks)
skipped = sum(check["status"] == "skipped" for check in checks)
lines = [
    f"kongctl smoke test: {report['status'].upper()}",
    f"mode: {report['mode']}",
    f"binary: {binary.get('path', '')}",
    f"version: {binary.get('version', 'unknown')}",
    f"commit: {binary.get('commit', 'unknown')}",
    f"profile: {report['profile']}",
    f"checks: {passed} passed, {failed} failed, {known} known issues, {skipped} skipped",
    f"cleanup: {report['cleanup']['status']}",
]
if report["failure"]:
    lines.append(f"failure: {report['failure']}")
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

check_local_features

resource_token="${run_suffix}"
if [[ "$selected_keys" == *" api "* ]]; then
  prepare_resource "api" "api" "apis" "ga" "smoke-api-${resource_token}" "Smoke API ${resource_token}" \
    "kongctl smoke API" name description version slug
fi
if [[ "$selected_keys" == *" portal "* ]]; then
  prepare_resource "portal" "portal" "portals" "ga" "smoke-portal-${resource_token}" "Smoke Portal ${resource_token}" \
    "kongctl smoke portal" name display_name description
fi
if [[ "$selected_keys" == *" control_plane "* ]]; then
  prepare_resource "control_plane" "control_plane" "control_planes" "ga" "smoke-cp-${resource_token}" \
    "Smoke Control Plane ${resource_token}" "kongctl smoke control plane" name description cluster_type
fi
if [[ "$selected_keys" == *" ai_gateway "* ]]; then
  prepare_resource "ai_gateway" "ai_gateway" "ai_gateways" "ga" "smoke-aigw-${resource_token}" \
    "Smoke AI Gateway ${resource_token}" "kongctl smoke AI gateway" name display_name description
fi
if [[ "$selected_keys" == *" event_gateway "* ]]; then
  prepare_resource "event_gateway" "event_gateway" "event_gateways" "" "smoke-egw-${resource_token}" \
    "Smoke Event Gateway ${resource_token}" "kongctl smoke Event Gateway" name description
fi

if [[ "$selected_keys" == *" identity_principal "* ]]; then
  discover_identity_directory
  if [[ -z "$identity_directory_id" ]]; then
    prepare_identity_directory_bootstrap
  fi
  prepare_resource "identity_principal" "identity_principal" "identity.principals" "ga" "smoke-person-${resource_token}" \
    "smoke-person-${resource_token}" "kongctl smoke principal" display_name directory
fi
if [[ "$selected_keys" == *" identity_auth_server "* ]]; then
  prepare_resource "identity_auth_server" "identity_auth_server" "identity.auth_servers" "ga" "smoke-issuer-${resource_token}" \
    "" "kongctl smoke Auth Server" name audience
fi

if [[ "$mode" == "quick" ]]; then
  for key in "${resource_registry[@]}"; do
    "load_resource_${key}"
    start_check "list-${key}-quick" "$key" "read"
    if [[ "$key" == "identity_principal" && -z "$identity_directory_id" ]]; then
      current_command="skip principal list: no Identity directory exists"
      record_current "skipped" "full mode will bootstrap a directory after confirmation"
      echo " SKIP (no directory; full mode will bootstrap)"
      continue
    fi
    run_kongctl "${list_args[@]}"
    require_success
    assert_collection "$match_field" "$resource_name" "absent"
    pass_current
  done
  cleanup_state="not_needed"
  exit 0
fi

if [[ "$selected_keys" == *" identity_principal "* && -z "$identity_directory_id" ]]; then
  echo "Identity: creates a protected default directory (allow_all_control_planes=true)."
  echo "Identity: retains this directory after cleanup, including failed runs."
fi

if [[ "$assume_yes" != "true" ]] && ! is_truthy "${KONGCTL_SMOKE_YES:-}"; then
  binary_version="$(
    python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' \
      "$artifacts_dir/binary.json"
  )"
  echo
  echo "Binary:    $kongctl_bin ($binary_version)"
  echo "Profile:   $effective_profile"
  echo "Resources: $resource_selection"
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

if [[ "$selected_keys" == *" identity_principal "* && -z "$identity_directory_id" ]]; then
  bootstrap_identity_directory
fi

for key in "${resource_registry[@]}"; do
  smoke_resource "$key"
done

cleanup_all "strict"
exit 0

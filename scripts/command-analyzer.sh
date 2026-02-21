#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Run a kongctl command with HTTP logging enabled and print a summary.

Usage:
  scripts/command-analyzer.sh [options] -- <kongctl-args...>
  scripts/command-analyzer.sh [options] "<kongctl-args string>"

Options:
  --log-file PATH     Log file path to use (default: auto temp file)
  --log-level LEVEL   Log level to inject if missing (default: debug)
  --method METHOD     Pass through to http-log-summary.sh --method
  --route REGEX       Pass through to http-log-summary.sh --route
  --top N             Pass through to http-log-summary.sh --top
  -h, --help          Show this help

Examples:
  scripts/command-analyzer.sh "apply -f docs/examples/declarative/basic/api.yaml --auto-approve"
  scripts/command-analyzer.sh -- apply -f docs/examples/declarative/basic/api.yaml --auto-approve
  scripts/command-analyzer.sh --method GET --route '^/v2/portals$' -- \
    apply -f docs/examples/declarative/basic/api.yaml --auto-approve
EOF
}

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)
kongctl_bin="${KONGCTL_BIN:-$repo_dir/kongctl}"
summary_script="$repo_dir/scripts/http-log-summary.sh"

log_file=""
inject_log_level="debug"
summary_method=""
summary_route=""
summary_top=""
cmd_string=""
declare -a cmd_args=()

extract_flag_value() {
  local flag="$1"
  shift
  local -a values=("$@")
  local i
  for ((i = 0; i < ${#values[@]}; i++)); do
    if [[ "${values[$i]}" == "$flag" ]]; then
      if (( i + 1 < ${#values[@]} )); then
        printf '%s\n' "${values[$((i + 1))]}"
        return 0
      fi
      return 1
    fi
    if [[ "${values[$i]}" == "$flag="* ]]; then
      printf '%s\n' "${values[$i]#*=}"
      return 0
    fi
  done
  return 1
}

has_flag() {
  local flag="$1"
  shift
  local -a values=("$@")
  local item
  for item in "${values[@]}"; do
    if [[ "$item" == "$flag" || "$item" == "$flag="* ]]; then
      return 0
    fi
  done
  return 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --log-file)
      if [[ $# -lt 2 ]]; then
        echo "--log-file requires a value" >&2
        exit 1
      fi
      log_file="$2"
      shift 2
      ;;
    --log-level)
      if [[ $# -lt 2 ]]; then
        echo "--log-level requires a value" >&2
        exit 1
      fi
      inject_log_level="$2"
      shift 2
      ;;
    --method)
      if [[ $# -lt 2 ]]; then
        echo "--method requires a value" >&2
        exit 1
      fi
      summary_method="$2"
      shift 2
      ;;
    --route)
      if [[ $# -lt 2 ]]; then
        echo "--route requires a value" >&2
        exit 1
      fi
      summary_route="$2"
      shift 2
      ;;
    --top)
      if [[ $# -lt 2 ]]; then
        echo "--top requires a value" >&2
        exit 1
      fi
      summary_top="$2"
      shift 2
      ;;
    --)
      shift
      if [[ $# -eq 0 ]]; then
        echo "No kongctl command arguments were provided after --" >&2
        usage
        exit 1
      fi
      cmd_args=("$@")
      break
      ;;
    *)
      if [[ -n "$cmd_string" ]]; then
        echo "Only one command string can be provided" >&2
        usage
        exit 1
      fi
      cmd_string="$1"
      shift
      ;;
  esac
done

if [[ -n "$cmd_string" && ${#cmd_args[@]} -gt 0 ]]; then
  echo "Provide either a command string or args after --, not both" >&2
  exit 1
fi

if [[ -n "$cmd_string" ]]; then
  # shellcheck disable=SC2206
  eval "cmd_args=($cmd_string)"
fi

if [[ ${#cmd_args[@]} -eq 0 ]]; then
  usage
  exit 1
fi

if [[ ! -x "$kongctl_bin" ]]; then
  echo "kongctl binary is not executable: $kongctl_bin" >&2
  exit 1
fi

if [[ ! -x "$summary_script" ]]; then
  echo "summary script is not executable: $summary_script" >&2
  exit 1
fi

first_arg="${cmd_args[0]}"
if [[ "$first_arg" == "kongctl" || "$first_arg" == "./kongctl" || "$(basename "$first_arg")" == "kongctl" ]]; then
  cmd_args=("${cmd_args[@]:1}")
fi

if [[ ${#cmd_args[@]} -eq 0 ]]; then
  echo "No kongctl subcommand arguments remain after removing kongctl binary token" >&2
  exit 1
fi

has_log_file=false
has_log_level=false
if has_flag "--log-file" "${cmd_args[@]}"; then
  has_log_file=true
fi
if has_flag "--log-level" "${cmd_args[@]}"; then
  has_log_level=true
fi

if [[ "$has_log_file" == "true" && -n "$log_file" ]]; then
  echo "Command already includes --log-file; remove it or omit script --log-file" >&2
  exit 1
fi

if [[ "$has_log_file" == "false" ]]; then
  if [[ -z "$log_file" ]]; then
    log_file=$(mktemp /tmp/kongctl-http.XXXX.log)
  fi
  cmd_args+=("--log-file" "$log_file")
else
  log_file="$(extract_flag_value "--log-file" "${cmd_args[@]}")"
fi

if [[ "$has_log_level" == "false" ]]; then
  cmd_args+=("--log-level" "$inject_log_level")
fi

start_ms=$(awk -v t="$EPOCHREALTIME" 'BEGIN { printf "%.0f", t * 1000 }')

set +e
(
  cd "$repo_dir"
  "$kongctl_bin" "${cmd_args[@]}"
)
command_status=$?
set -e

end_ms=$(awk -v t="$EPOCHREALTIME" 'BEGIN { printf "%.0f", t * 1000 }')
elapsed_ms=$((end_ms - start_ms))
elapsed_seconds=$(awk -v ms="$elapsed_ms" 'BEGIN { printf "%.3f", ms / 1000 }')

echo
echo "Command metrics"
echo "  exit_code: ${command_status}"
echo "  elapsed: ${elapsed_ms} ms (${elapsed_seconds} s)"
echo "  log_file: ${log_file}"
echo

summary_cmd=("$summary_script" "$log_file")
if [[ -n "$summary_method" ]]; then
  summary_cmd+=("--method" "$summary_method")
fi
if [[ -n "$summary_route" ]]; then
  summary_cmd+=("--route" "$summary_route")
fi
if [[ -n "$summary_top" ]]; then
  summary_cmd+=("--top" "$summary_top")
fi

"${summary_cmd[@]}"

exit "$command_status"

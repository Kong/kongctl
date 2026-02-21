#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Summarize kongctl SDK HTTP logs from a log file.

Usage:
  scripts/http-log-summary.sh <log-file> [--method METHOD] [--route REGEX] [--top N]

Options:
  --method METHOD   Filter to a specific HTTP method (example: GET)
  --route REGEX     Filter to routes matching this regex (example: '^/v2/portals$')
  --top N           Number of method/route rows to print (default: 20)
  -h, --help        Show this help

Examples:
  scripts/http-log-summary.sh /tmp/kongctl.log
  scripts/http-log-summary.sh /tmp/kongctl.log --method GET --route '^/v2/portals$'
EOF
}

log_file=""
method_filter=""
route_filter=""
top_n=20

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --method)
      if [[ $# -lt 2 ]]; then
        echo "--method requires a value" >&2
        exit 1
      fi
      method_filter="$2"
      shift 2
      ;;
    --route)
      if [[ $# -lt 2 ]]; then
        echo "--route requires a value" >&2
        exit 1
      fi
      route_filter="$2"
      shift 2
      ;;
    --top)
      if [[ $# -lt 2 ]]; then
        echo "--top requires a value" >&2
        exit 1
      fi
      top_n="$2"
      shift 2
      ;;
    --*)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
    *)
      if [[ -n "$log_file" ]]; then
        echo "Only one log file can be provided" >&2
        usage
        exit 1
      fi
      log_file="$1"
      shift
      ;;
  esac
done

if [[ -z "$log_file" ]]; then
  usage
  exit 1
fi

if [[ ! -f "$log_file" ]]; then
  echo "Log file not found: $log_file" >&2
  exit 1
fi

if ! [[ "$top_n" =~ ^[0-9]+$ ]] || [[ "$top_n" == "0" ]]; then
  echo "--top must be a positive integer" >&2
  exit 1
fi

method_filter=$(printf '%s' "$method_filter" | tr '[:lower:]' '[:upper:]')

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

request_lines="$tmp_dir/requests.log"
response_lines="$tmp_dir/responses.log"
error_lines="$tmp_dir/errors.log"

if command -v rg >/dev/null 2>&1; then
  rg 'log_type=http_request' "$log_file" >"$request_lines" || true
  rg 'log_type=http_response' "$log_file" >"$response_lines" || true
  rg 'log_type=http_error' "$log_file" >"$error_lines" || true
else
  grep -E 'log_type=http_request' "$log_file" >"$request_lines" || true
  grep -E 'log_type=http_response' "$log_file" >"$response_lines" || true
  grep -E 'log_type=http_error' "$log_file" >"$error_lines" || true
fi

count_lines() {
  local file_path="$1"
  wc -l <"$file_path" | tr -d '[:space:]'
}

extract_method_route() {
  local file_path="$1"
  awk -v method_filter="$method_filter" -v route_filter="$route_filter" '
function extract(line, key, pattern, value) {
  pattern = key "=([^ ]+|\"[^\"]+\")"
  if (match(line, pattern)) {
    value = substr(line, RSTART + length(key) + 1, RLENGTH - length(key) - 1)
    gsub(/^"/, "", value)
    gsub(/"$/, "", value)
    return value
  }
  return ""
}
{
  method = toupper(extract($0, "method"))
  route = extract($0, "route")
  if (method == "" || route == "") {
    next
  }
  if (method_filter != "" && method != method_filter) {
    next
  }
  if (route_filter != "" && route !~ route_filter) {
    next
  }
  print method " " route
}
' "$file_path"
}

extract_status() {
  awk -v method_filter="$method_filter" -v route_filter="$route_filter" '
function extract(line, key, pattern, value) {
  pattern = key "=([^ ]+|\"[^\"]+\")"
  if (match(line, pattern)) {
    value = substr(line, RSTART + length(key) + 1, RLENGTH - length(key) - 1)
    gsub(/^"/, "", value)
    gsub(/"$/, "", value)
    return value
  }
  return ""
}
{
  method = toupper(extract($0, "method"))
  route = extract($0, "route")
  status = extract($0, "status_code")
  if (status == "") {
    next
  }
  if (method_filter != "" && method != method_filter) {
    next
  }
  if (route_filter != "" && route !~ route_filter) {
    next
  }
  print status
}
' "$response_lines"
}

extract_duration_metrics() {
  local file_path="$1"
  awk -v method_filter="$method_filter" -v route_filter="$route_filter" '
function extract(line, key, pattern, value) {
  pattern = key "=([^ ]+|\"[^\"]+\")"
  if (match(line, pattern)) {
    value = substr(line, RSTART + length(key) + 1, RLENGTH - length(key) - 1)
    gsub(/^"/, "", value)
    gsub(/"$/, "", value)
    return value
  }
  return ""
}
function duration_ms(duration, raw, unit, value) {
  raw = duration
  gsub(/µs/, "us", raw)
  gsub(/μs/, "us", raw)
  if (raw ~ /^[0-9]+(\.[0-9]+)?ns$/) {
    value = substr(raw, 1, length(raw) - 2) + 0
    return value / 1000000
  }
  if (raw ~ /^[0-9]+(\.[0-9]+)?us$/) {
    value = substr(raw, 1, length(raw) - 2) + 0
    return value / 1000
  }
  if (raw ~ /^[0-9]+(\.[0-9]+)?ms$/) {
    value = substr(raw, 1, length(raw) - 2) + 0
    return value
  }
  if (raw ~ /^[0-9]+(\.[0-9]+)?s$/) {
    value = substr(raw, 1, length(raw) - 1) + 0
    return value * 1000
  }
  if (raw ~ /^[0-9]+(\.[0-9]+)?m$/) {
    value = substr(raw, 1, length(raw) - 1) + 0
    return value * 60000
  }
  if (raw ~ /^[0-9]+(\.[0-9]+)?h$/) {
    value = substr(raw, 1, length(raw) - 1) + 0
    return value * 3600000
  }
  return -1
}
{
  method = toupper(extract($0, "method"))
  route = extract($0, "route")
  duration = extract($0, "duration")
  if (duration == "") {
    next
  }
  if (method_filter != "" && method != method_filter) {
    next
  }
  if (route_filter != "" && route !~ route_filter) {
    next
  }
  ms = duration_ms(duration)
  if (ms < 0) {
    next
  }
  count++
  sum += ms
  if (count == 1 || ms < min) {
    min = ms
  }
  if (count == 1 || ms > max) {
    max = ms
  }
}
END {
  if (count == 0) {
    print "0 0 0 0 0"
    exit
  }
  print count, sum, min, max, (sum / count)
}
' "$file_path"
}

fmt_duration() {
  local ms="$1"
  awk -v ms="$ms" 'BEGIN { printf "%.3f ms (%.3f s)", ms + 0, (ms + 0) / 1000 }'
}

print_timing_block() {
  local label="$1"
  local count="$2"
  local sum_ms="$3"
  local avg_ms="$4"
  local min_ms="$5"
  local max_ms="$6"

  if [[ "$count" == "0" ]]; then
    echo "  $label: no matching duration entries"
    return
  fi

  echo "  $label:"
  echo "    count: $count"
  echo "    total: $(fmt_duration "$sum_ms")"
  echo "    avg:   $(fmt_duration "$avg_ms")"
  echo "    min:   $(fmt_duration "$min_ms")"
  echo "    max:   $(fmt_duration "$max_ms")"
}

total_requests=$(count_lines "$request_lines")
total_responses=$(count_lines "$response_lines")
total_errors=$(count_lines "$error_lines")
filtered_requests=$(extract_method_route "$request_lines" | wc -l | tr -d '[:space:]')
filtered_responses=$(extract_method_route "$response_lines" | wc -l | tr -d '[:space:]')
read -r \
  response_duration_count \
  response_duration_sum \
  response_duration_min \
  response_duration_max \
  response_duration_avg \
  <<<"$(extract_duration_metrics "$response_lines")"
read -r \
  error_duration_count \
  error_duration_sum \
  error_duration_min \
  error_duration_max \
  error_duration_avg \
  <<<"$(extract_duration_metrics "$error_lines")"
read -r \
  combined_duration_count \
  combined_duration_sum \
  combined_duration_min \
  combined_duration_max \
  combined_duration_avg \
  <<<"$(awk \
    -v c1="$response_duration_count" \
    -v s1="$response_duration_sum" \
    -v min1="$response_duration_min" \
    -v max1="$response_duration_max" \
    -v c2="$error_duration_count" \
    -v s2="$error_duration_sum" \
    -v min2="$error_duration_min" \
    -v max2="$error_duration_max" '
BEGIN {
  count = c1 + c2
  sum = s1 + s2
  if (count == 0) {
    print "0 0 0 0 0"
    exit
  }

  if (c1 > 0 && c2 > 0) {
    min = (min1 < min2) ? min1 : min2
    max = (max1 > max2) ? max1 : max2
  } else if (c1 > 0) {
    min = min1
    max = max1
  } else {
    min = min2
    max = max2
  }

  print count, sum, min, max, (sum / count)
}
')"

echo "Log file: $log_file"
echo "Filters: method=${method_filter:-<none>} route=${route_filter:-<none>}"
echo
echo "Totals"
echo "  http_request lines:  $total_requests"
echo "  http_response lines: $total_responses"
echo "  http_error lines:    $total_errors"
echo "  filtered requests:   $filtered_requests"
echo "  filtered responses:  $filtered_responses"
echo
echo "Request Counts by Method+Route (top $top_n)"
request_counts=$(extract_method_route "$request_lines" | sort | uniq -c | sort -nr | head -n "$top_n" || true)
if [[ -z "$request_counts" ]]; then
  echo "  no matching request entries"
else
  printf '%s\n' "$request_counts" | awk '{printf "  %6d  %-8s  %s\n", $1, $2, $3}'
fi
echo
echo "Response Status Counts"
status_counts=$(extract_status | sort | uniq -c | sort -nr || true)
if [[ -z "$status_counts" ]]; then
  echo "  no matching response entries"
else
  printf '%s\n' "$status_counts" | awk '{printf "  %6d  %s\n", $1, $2}'
fi
echo
echo "Timing (from duration field, filtered)"
print_timing_block "responses" \
  "$response_duration_count" \
  "$response_duration_sum" \
  "$response_duration_avg" \
  "$response_duration_min" \
  "$response_duration_max"
print_timing_block "errors" \
  "$error_duration_count" \
  "$error_duration_sum" \
  "$error_duration_avg" \
  "$error_duration_min" \
  "$error_duration_max"
print_timing_block "combined" \
  "$combined_duration_count" \
  "$combined_duration_sum" \
  "$combined_duration_avg" \
  "$combined_duration_min" \
  "$combined_duration_max"

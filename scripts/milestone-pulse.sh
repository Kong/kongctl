#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/milestone-pulse.sh [options]

Generate static kongctl Pulse dashboards for all current GitHub milestones.

Options:
  --repo OWNER/REPO       GitHub repository. Defaults to $GITHUB_REPOSITORY or
                          the current gh repository.
  --state STATE           Milestone state to include. Defaults to "open".
                          Use "all" to include closed milestones too.
  --output-dir DIR        Output directory. Defaults to docs/reports/milestones.
  --snapshot-date DATE    Snapshot date in YYYY-MM-DD format. Defaults to UTC today.
  --generated-at TIME     ISO-8601 generation timestamp. Defaults to UTC now.
  -h, --help              Show this help.

Environment:
  GH_TOKEN or GITHUB_TOKEN must be set when gh is not already authenticated.
EOF
}

require_command() {
  local command_name="$1"

  if ! command -v "${command_name}" >/dev/null 2>&1; then
    echo "required command not found: ${command_name}" >&2
    exit 1
  fi
}

require_option_value() {
  local option_name="$1"
  local remaining_args="$2"

  if [ "${remaining_args}" -lt 2 ]; then
    echo "${option_name} requires a value" >&2
    usage >&2
    exit 1
  fi
}

emit_script_json() {
  local source_file="$1"

  jq -c '.' "${source_file}" | sed 's#</#<\\/#g'
}

repo="${GITHUB_REPOSITORY:-}"
milestone_state="${MILESTONE_PULSE_STATE:-open}"
output_dir="${MILESTONE_PULSE_OUTPUT_DIR:-docs/reports/milestones}"
snapshot_date="${MILESTONE_PULSE_SNAPSHOT_DATE:-$(date -u +%F)}"
generated_at="${MILESTONE_PULSE_GENERATED_AT:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      require_option_value "$1" "$#"
      repo="${2:-}"
      shift 2
      ;;
    --state)
      require_option_value "$1" "$#"
      milestone_state="${2:-}"
      shift 2
      ;;
    --output-dir)
      require_option_value "$1" "$#"
      output_dir="${2:-}"
      shift 2
      ;;
    --snapshot-date)
      require_option_value "$1" "$#"
      snapshot_date="${2:-}"
      shift 2
      ;;
    --generated-at)
      require_option_value "$1" "$#"
      generated_at="${2:-}"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

require_command gh
require_command jq

if [ -z "${repo}" ]; then
  repo="$(gh repo view --json nameWithOwner --jq .nameWithOwner)"
fi

if [ -z "${repo}" ]; then
  echo "repository is required; pass --repo OWNER/REPO or set GITHUB_REPOSITORY" >&2
  exit 1
fi

case "${milestone_state}" in
  open | closed | all)
    ;;
  *)
    echo "milestone state must be one of: open, closed, all" >&2
    exit 1
    ;;
esac

if ! [[ "${snapshot_date}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  echo "snapshot date must use YYYY-MM-DD format: ${snapshot_date}" >&2
  exit 1
fi

data_dir="${output_dir}/data"
tmp_root="${MILESTONE_PULSE_TMP_DIR:-.tmp/milestone-pulse}"
mkdir -p "${data_dir}" "${tmp_root}"
work_dir="$(mktemp -d "${tmp_root}/run.XXXXXX")"
trap 'rm -rf "${work_dir}"' EXIT

milestones_file="${work_dir}/milestones.json"
issues_dir="${work_dir}/issues"
prs_dir="${work_dir}/prs"
issues_map_file="${work_dir}/issues-map.json"
shipped_map_file="${work_dir}/shipped-map.json"
snapshot_file="${data_dir}/${snapshot_date}.json"
latest_file="${data_dir}/latest.json"
history_file="${work_dir}/history.json"
mkdir -p "${issues_dir}" "${prs_dir}"

echo "Fetching ${milestone_state} milestones for ${repo}..."
gh api --paginate --slurp \
  "repos/${repo}/milestones?state=${milestone_state}&sort=due_on&direction=asc&per_page=100" \
  | jq 'add // []' > "${milestones_file}"

jq -c '.[] | {number, title}' "${milestones_file}" | while IFS= read -r milestone; do
  number="$(jq -r '.number' <<< "${milestone}")"
  title="$(jq -r '.title' <<< "${milestone}")"
  echo "Fetching milestone #${number}: ${title}"
  gh api --paginate --slurp \
    "repos/${repo}/issues?state=all&milestone=${number}&per_page=100" \
    | jq '
        add // []
        | map({
            number,
            title,
            state,
            html_url,
            created_at,
            updated_at,
            closed_at,
            kind: (if has("pull_request") then "pull_request" else "issue" end),
            merged_at: null,
            base_ref: null,
            author: null,
            linked_issue_numbers: [],
            labels: [.labels[]?.name],
            assignees: [.assignees[]?.login]
          })
      ' > "${issues_dir}/${number}.base.json"

  pr_work_dir="${prs_dir}/${number}"
  pr_candidates="${pr_work_dir}/candidate-prs.tsv"
  mkdir -p "${pr_work_dir}"
  : > "${pr_candidates}"

  jq -r '.[] | select(.kind == "pull_request") | [.number, ""] | @tsv' \
    "${issues_dir}/${number}.base.json" >> "${pr_candidates}"

  jq -r '.[] | select(.kind == "issue" and .state == "closed") | .number' \
    "${issues_dir}/${number}.base.json" \
    | while IFS= read -r issue_number; do
      if [ -z "${issue_number}" ]; then
        continue
      fi
      gh api --paginate --slurp "repos/${repo}/issues/${issue_number}/timeline?per_page=100" \
        | jq -r --arg issue_number "${issue_number}" '
            add // []
            | .[]
            | select((.source.issue.pull_request.url? // "") != "")
            | [.source.issue.number, $issue_number]
            | @tsv
          ' >> "${pr_candidates}"
    done

  if [ -s "${pr_candidates}" ]; then
    cut -f 1 "${pr_candidates}" | sort -n -u | while IFS= read -r pr_number; do
      if [ -z "${pr_number}" ]; then
        continue
      fi

      linked_issues_file="${pr_work_dir}/${pr_number}.linked"
      awk -F '\t' -v pr_number="${pr_number}" '$1 == pr_number && $2 != "" { print $2 }' \
        "${pr_candidates}" \
        | sort -n -u \
        | jq -R 'tonumber' \
        | jq -s '.' > "${linked_issues_file}"

      gh api "repos/${repo}/pulls/${pr_number}" \
        | jq --slurpfile linked_issues "${linked_issues_file}" '{
            number,
            title,
            html_url,
            merged_at,
            base_ref: .base.ref,
            author: .user.login,
            linked_issue_numbers: $linked_issues[0]
          }' > "${pr_work_dir}/${pr_number}.json"
    done
  fi

  pr_details="$(find "${pr_work_dir}" -maxdepth 1 -type f -name "*.json" -print | sort)"
  if [ -n "${pr_details}" ]; then
    # shellcheck disable=SC2086
    jq -s '.' ${pr_details} > "${prs_dir}/${number}.json"
  else
    jq -n '[]' > "${prs_dir}/${number}.json"
  fi

  jq --slurpfile prs "${prs_dir}/${number}.json" '
    map(
      . as $item
      | if .kind == "pull_request" then
          ($prs[0] | map(select(.number == $item.number)) | .[0]) as $pr
          | if $pr == null then
              .
            else
              . + {
                merged_at: $pr.merged_at,
                base_ref: $pr.base_ref,
                author: $pr.author,
                linked_issue_numbers: $pr.linked_issue_numbers
              }
            end
        else
          .
        end
    )
  ' "${issues_dir}/${number}.base.json" > "${issues_dir}/${number}.json"
done

jq -n '{}' > "${issues_map_file}"
for issue_file in "${issues_dir}"/*.json; do
  if [ ! -f "${issue_file}" ]; then
    continue
  fi
  number="$(basename "${issue_file}" .json)"
  if ! [[ "${number}" =~ ^[0-9]+$ ]]; then
    continue
  fi
  tmp_map="${work_dir}/issues-map-${number}.json"
  jq --arg number "${number}" --slurpfile issues "${issue_file}" \
    '. + {($number): $issues[0]}' "${issues_map_file}" > "${tmp_map}"
  mv "${tmp_map}" "${issues_map_file}"
done

jq -n '{}' > "${shipped_map_file}"
for pr_file in "${prs_dir}"/*.json; do
  if [ ! -f "${pr_file}" ]; then
    continue
  fi
  number="$(basename "${pr_file}" .json)"
  if ! [[ "${number}" =~ ^[0-9]+$ ]]; then
    continue
  fi
  tmp_map="${work_dir}/shipped-map-${number}.json"
  jq --arg number "${number}" --slurpfile prs "${pr_file}" \
    '. + {($number): ($prs[0] | map(select(.merged_at != null and .base_ref == "main")) | sort_by(.merged_at) | reverse)}' \
    "${shipped_map_file}" > "${tmp_map}"
  mv "${tmp_map}" "${shipped_map_file}"
done

jq -n \
  --arg repo "${repo}" \
  --arg milestone_state "${milestone_state}" \
  --arg generated_at "${generated_at}" \
  --arg snapshot_date "${snapshot_date}" \
  --slurpfile milestones "${milestones_file}" \
  --slurpfile issues_map "${issues_map_file}" \
  --slurpfile shipped_map "${shipped_map_file}" '
  def slug:
    ascii_downcase
    | gsub("[^a-z0-9]+"; "-")
    | gsub("(^-+|-+$)"; "");

  def days_until($date):
    if $date == null or $date == "" then null
    else
      (((($date | fromdateiso8601) - ($generated_at | fromdateiso8601)) / 86400) | ceil)
    end;

  def priority($items; $label):
    [$items[] | select(.state == "open" and (.labels | index($label)))] | length;

  {
    repo: $repo,
    milestone_state: $milestone_state,
    generated_at: $generated_at,
    snapshot_date: $snapshot_date,
    milestones: (
      $milestones[0]
      | map(
          . as $milestone
          | ($issues_map[0][($milestone.number | tostring)] // []) as $items
          | ($shipped_map[0][($milestone.number | tostring)] // []) as $shipped_items
          | ($items | map(select(.state == "open"))) as $open_items
          | ($items | map(select(.state == "closed"))) as $closed_items
          | {
              number: $milestone.number,
              title: $milestone.title,
              slug: (($milestone.title | slug) as $slug | if $slug == "" then "milestone-\($milestone.number)" else $slug end),
              description: $milestone.description,
              state: $milestone.state,
              html_url: $milestone.html_url,
              created_at: $milestone.created_at,
              updated_at: $milestone.updated_at,
              due_on: $milestone.due_on,
              closed_at: $milestone.closed_at,
              days_remaining: days_until($milestone.due_on),
              items: ($items | sort_by(.state, .updated_at) | reverse),
              shipped_items: ($shipped_items | sort_by(.merged_at) | reverse),
              total_items: ($items | length),
              open_items: ($open_items | length),
              closed_items: ($closed_items | length),
              issue_items: ([$items[] | select(.kind == "issue")] | length),
              pull_request_items: ([$items[] | select(.kind == "pull_request")] | length),
              open_high_priority: priority($items; "high-priority"),
              open_medium_priority: priority($items; "medium-priority"),
              open_bugs: priority($items; "bug"),
              open_unassigned: ([$open_items[] | select((.assignees | length) == 0)] | length),
              stale_open: ([
                $open_items[]
                | select(
                    ((($generated_at | fromdateiso8601) - (.updated_at | fromdateiso8601)) / 86400) >= 14
                  )
                ] | length),
              completion_percent: (
                if ($items | length) == 0 then 0
                else ((($closed_items | length) * 10000 / ($items | length)) | round / 100)
                end
              )
            }
        )
    )
  }
  | .totals = {
      milestones: (.milestones | length),
      total_items: ([.milestones[].total_items] | add // 0),
      open_items: ([.milestones[].open_items] | add // 0),
      closed_items: ([.milestones[].closed_items] | add // 0),
      open_high_priority: ([.milestones[].open_high_priority] | add // 0),
      open_bugs: ([.milestones[].open_bugs] | add // 0),
      open_unassigned: ([.milestones[].open_unassigned] | add // 0),
      stale_open: ([.milestones[].stale_open] | add // 0)
    }
  | .totals.completion_percent = (
      if .totals.total_items == 0 then 0
      else ((.totals.closed_items * 10000 / .totals.total_items) | round / 100)
      end
    )
  ' > "${snapshot_file}"

cp "${snapshot_file}" "${latest_file}"

history_files="$(find "${data_dir}" -maxdepth 1 -type f -name '????-??-??.json' -print | sort)"
if [ -n "${history_files}" ]; then
  # shellcheck disable=SC2086
  jq -s '
    sort_by(.snapshot_date)
    | map({
        snapshot_date,
        generated_at,
        totals,
        milestones: [
          .milestones[]
          | {
              number,
              title,
              slug,
              total_items,
              open_items,
              closed_items,
              open_high_priority,
              open_bugs,
              completion_percent
            }
        ]
      })
  ' ${history_files} > "${history_file}"
else
  jq -n '[]' > "${history_file}"
fi

write_html() {
  local target="$1"
  local view="$2"
  local milestone_number="$3"
  local asset_prefix="$4"

  mkdir -p "$(dirname "${target}")"
  {
    cat <<'HTML_HEAD'
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>kongctl Pulse</title>
HTML_HEAD
    printf '  <link rel="icon" href="%sbrand/logo/dark/Kong-Logomark.svg" type="image/svg+xml">\n' "${asset_prefix}"
    cat <<'HTML_HEAD'
  <style>
    :root {
      color-scheme: dark;
      --kong-dark: #0b0d12;
      --kong-graphite: #35363a;
      --kong-cloud: #f7f9fc;
      --kong-green: #ccff00;
      --bg: var(--kong-dark);
      --panel: #141820;
      --panel-raised: #1b202a;
      --ink: var(--kong-cloud);
      --muted: #aab4c0;
      --line: rgba(247, 249, 252, 0.14);
      --green: #ccff00;
      --teal: #20d9c3;
      --gold: #f0b429;
      --red: #ff6b5f;
      --blue: #8da2ff;
      --shadow: 0 18px 45px rgba(0, 0, 0, 0.34);
    }

    * {
      box-sizing: border-box;
    }

    body {
      margin: 0;
      background:
        radial-gradient(circle at 12% 0%, rgba(204, 255, 0, 0.1), transparent 28rem),
        linear-gradient(180deg, #0b0d12 0%, #121722 48%, #0b0d12 100%);
      color: var(--ink);
      font: 14px/1.5 Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }

    a {
      color: inherit;
    }

    .shell {
      min-height: 100vh;
    }

    .topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 24px;
      padding: 18px clamp(20px, 4vw, 56px);
      border-bottom: 3px solid var(--kong-green);
      background: rgba(11, 13, 18, 0.92);
      backdrop-filter: blur(12px);
      position: sticky;
      top: 0;
      z-index: 5;
    }

    .brand {
      display: flex;
      align-items: center;
      gap: 16px;
      min-width: 0;
    }

    .brand img {
      display: block;
      width: 112px;
      height: auto;
    }

    .brand small,
    .timestamp {
      color: rgba(247, 249, 252, 0.78);
      white-space: nowrap;
    }

    main {
      padding: 30px clamp(20px, 4vw, 56px) 56px;
    }

    .hero {
      display: grid;
      grid-template-columns: minmax(0, 1.3fr) minmax(280px, 0.7fr);
      gap: 24px;
      align-items: stretch;
      margin-bottom: 24px;
    }

    .portfolio-hero {
      grid-template-columns: 1fr;
    }

    h1,
    h2,
    h3,
    p {
      margin-top: 0;
    }

    h1 {
      margin-bottom: 10px;
      font-size: clamp(32px, 5vw, 62px);
      line-height: 1;
      letter-spacing: 0;
    }

    h2 {
      margin-bottom: 16px;
      font-size: 20px;
      letter-spacing: 0;
    }

    h3 {
      margin-bottom: 8px;
      font-size: 15px;
      letter-spacing: 0;
    }

    .lede {
      max-width: 850px;
      color: var(--muted);
      font-size: 18px;
    }

    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
    }

    .hero-copy {
      padding: clamp(24px, 4vw, 42px);
      background:
        linear-gradient(135deg, rgba(204, 255, 0, 0.1), transparent 34%),
        var(--kong-dark);
      color: var(--kong-cloud);
      border-color: rgba(204, 255, 0, 0.36);
    }

    .hero-side {
      display: grid;
      gap: 12px;
      padding: 18px;
      background: var(--panel-raised);
      border-color: rgba(204, 255, 0, 0.24);
      color: var(--kong-cloud);
    }

    .hero-copy .lede,
    .hero-copy .timestamp,
    .hero-copy p,
    .hero-side .label {
      color: rgba(247, 249, 252, 0.76);
    }

    .hero-copy a {
      color: var(--kong-cloud);
    }

    .hero-side .metric {
      background: rgba(11, 13, 18, 0.7);
      border-color: rgba(247, 249, 252, 0.14);
      box-shadow: none;
    }

    .metric-grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 14px;
      margin-bottom: 24px;
    }

    .metric {
      padding: 18px;
    }

    .metric .value {
      display: block;
      font-size: clamp(24px, 3vw, 36px);
      font-weight: 800;
      line-height: 1;
      color: var(--green);
      text-shadow: 0 0 18px rgba(204, 255, 0, 0.12);
    }

    .metric .label {
      display: block;
      margin-top: 8px;
      color: var(--muted);
    }

    .layout {
      display: grid;
      grid-template-columns: minmax(0, 1.1fr) minmax(320px, 0.9fr);
      gap: 24px;
      align-items: start;
    }

    .burn-workstream {
      display: grid;
      grid-template-columns: minmax(0, 3fr) minmax(260px, 1fr);
      gap: 24px;
      align-items: stretch;
    }

    .section {
      padding: 22px;
      margin-bottom: 24px;
    }

    .progress-track {
      height: 16px;
      overflow: hidden;
      border-radius: 999px;
      background: rgba(247, 249, 252, 0.12);
    }

    .progress-fill {
      height: 100%;
      min-width: 2px;
      border-radius: inherit;
      background: var(--kong-green);
      box-shadow: 0 0 0 1px rgba(11, 13, 18, 0.1) inset;
    }

    .progress-row {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      margin-top: 10px;
      color: var(--muted);
    }

    .chart {
      width: 100%;
      min-height: 220px;
      overflow: hidden;
    }

    .chart-with-legend {
      display: grid;
      grid-template-columns: minmax(0, 1fr) max-content;
      gap: 18px;
      align-items: start;
    }

    .chart svg {
      display: block;
      width: 100%;
      height: 240px;
    }

    .chart-legend {
      display: grid;
      gap: 8px;
      min-width: 118px;
      padding-top: 10px;
    }

    .chart-legend .pill {
      justify-content: flex-start;
      width: 100%;
    }

    .card-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(270px, 1fr));
      gap: 16px;
    }

    .milestone-card {
      display: grid;
      gap: 14px;
      padding: 18px;
      text-decoration: none;
      transition: transform 160ms ease, border-color 160ms ease;
    }

    .milestone-card:hover {
      border-color: #aec1cc;
      background: #202632;
      transform: translateY(-2px);
    }

    .pill-row {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    .pill {
      display: inline-flex;
      align-items: center;
      min-height: 24px;
      padding: 3px 9px;
      border-radius: 999px;
      border: 1px solid var(--line);
      background: rgba(247, 249, 252, 0.06);
      color: var(--muted);
      font-size: 12px;
      white-space: nowrap;
    }

    .pill.red {
      color: #ffaca5;
      border-color: rgba(255, 107, 95, 0.36);
      background: rgba(255, 107, 95, 0.12);
    }

    .pill.gold {
      color: #ffd789;
      border-color: rgba(240, 180, 41, 0.38);
      background: rgba(240, 180, 41, 0.14);
    }

    .pill.green {
      color: var(--kong-green);
      border-color: rgba(204, 255, 0, 0.34);
      background: rgba(204, 255, 0, 0.1);
    }

    .issue-list {
      display: grid;
      gap: 10px;
      margin: 0;
      padding: 0;
      list-style: none;
    }

    .issue {
      padding: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: rgba(247, 249, 252, 0.045);
    }

    .issue a {
      display: block;
      font-weight: 700;
      text-decoration: none;
    }

    .issue a:hover {
      text-decoration: underline;
    }

    .issue-meta {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-top: 8px;
      color: var(--muted);
      font-size: 12px;
    }

    table {
      width: 100%;
      border-collapse: collapse;
    }

    th,
    td {
      padding: 10px 8px;
      border-bottom: 1px solid var(--line);
      text-align: left;
      vertical-align: top;
    }

    th {
      color: var(--muted);
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.06em;
    }

    .empty {
      padding: 20px;
      border: 1px dashed var(--line);
      border-radius: 8px;
      color: var(--muted);
      background: rgba(247, 249, 252, 0.045);
    }

    .footer {
      color: var(--muted);
      padding-top: 8px;
    }

    @media (max-width: 980px) {
      .hero,
      .layout,
      .burn-workstream {
        grid-template-columns: 1fr;
      }

      .chart-with-legend {
        grid-template-columns: 1fr;
      }

      .chart-legend {
        display: flex;
        flex-wrap: wrap;
        min-width: 0;
        padding-top: 0;
      }

      .chart-legend .pill {
        width: auto;
      }

      .metric-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }

    @media (max-width: 620px) {
      .topbar {
        align-items: flex-start;
        flex-direction: column;
      }

      .timestamp {
        white-space: normal;
      }

      .metric-grid {
        grid-template-columns: 1fr;
      }

      h1 {
        font-size: 34px;
      }
    }
  </style>
</head>
<body>
  <div class="shell">
    <header class="topbar">
      <div class="brand">
HTML_HEAD
    printf '        <img src="%sbrand/logo/dark/Kong-Logotype.svg" alt="Kong">\n' "${asset_prefix}"
    cat <<'HTML_BODY'
        <small>kongctl Pulse</small>
      </div>
      <div class="timestamp" id="timestamp"></div>
    </header>
    <main id="app"></main>
  </div>
  <script>
HTML_BODY
    printf '    window.PULSE_DATA = '
    emit_script_json "${latest_file}"
    printf ';\n'
    printf '    window.PULSE_HISTORY = '
    emit_script_json "${history_file}"
    printf ';\n'
    printf '    window.PULSE_VIEW = %s;\n' "$(jq -n --arg view "${view}" '$view')"
    printf '    window.PULSE_MILESTONE_NUMBER = %s;\n' "$(jq -n --arg number "${milestone_number}" '$number')"
    cat <<'HTML_SCRIPT'

    const data = window.PULSE_DATA;
    const history = window.PULSE_HISTORY;
    const view = window.PULSE_VIEW;
    const milestoneNumber = Number(window.PULSE_MILESTONE_NUMBER);

    const app = document.getElementById('app');
    const timestamp = document.getElementById('timestamp');
    timestamp.textContent = `Generated ${formatDateTime(data.generated_at)} from ${data.repo}`;

    function esc(value) {
      return String(value ?? '').replace(/[&<>"']/g, (char) => ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#39;'
      })[char]);
    }

    function formatDate(value) {
      if (!value) return 'No due date';
      return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(value));
    }

    function formatDateTime(value) {
      return new Intl.DateTimeFormat(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        timeZoneName: 'short'
      }).format(new Date(value));
    }

    function percent(value) {
      return `${Number(value || 0).toFixed(Number(value || 0) % 1 === 0 ? 0 : 1)}%`;
    }

    function daysLabel(days) {
      if (days === null || days === undefined) return 'No due date';
      if (days < 0) return `${Math.abs(days)} days past due`;
      if (days === 0) return 'Due today';
      return `${days} days remaining`;
    }

    function metric(value, label) {
      return `<div class="metric panel"><span class="value">${esc(value)}</span><span class="label">${esc(label)}</span></div>`;
    }

    function progressBar(done, total) {
      const pct = total > 0 ? Math.round(done * 1000 / total) / 10 : 0;
      return `
        <div class="progress-track" aria-label="${esc(pct)} percent complete">
          <div class="progress-fill" style="width:${Math.max(0, Math.min(100, pct))}%"></div>
        </div>
        <div class="progress-row">
          <span>${esc(done)} closed</span>
          <strong>${esc(percent(pct))}</strong>
          <span>${esc(total - done)} open</span>
        </div>
      `;
    }

    function trendForMilestone(milestone) {
      return history
        .map((snapshot) => {
          const item = snapshot.milestones.find((entry) => entry.number === milestone.number);
          if (!item) return null;
          return { date: snapshot.snapshot_date, ...item };
        })
        .filter(Boolean);
    }

    function chart(points, series) {
      if (!points.length) {
        return '<div class="empty">Trend data will appear after the first scheduled refresh.</div>';
      }

      const width = 760;
      const height = 240;
      const pad = 34;
      const maxY = Math.max(1, ...points.flatMap((point) => series.map((item) => Number(point[item.key] || 0))));
      const x = (index) => points.length === 1 ? width / 2 : pad + (index * (width - pad * 2) / (points.length - 1));
      const y = (value) => height - pad - (Number(value || 0) * (height - pad * 2) / maxY);

      const paths = series.map((item) => {
        const d = points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${x(index).toFixed(1)} ${y(point[item.key]).toFixed(1)}`).join(' ');
        const dots = points.map((point, index) => `<circle cx="${x(index).toFixed(1)}" cy="${y(point[item.key]).toFixed(1)}" r="3.5" fill="${item.color}"></circle>`).join('');
        return `<path d="${d}" fill="none" stroke="${item.color}" stroke-width="3" stroke-linecap="round"></path>${dots}`;
      }).join('');

      const last = points[points.length - 1];
      const labels = series.map((item) => `<span class="pill"><span style="color:${item.color};font-weight:800">&bull;</span>&nbsp;${esc(item.label)} ${esc(last[item.key] || 0)}</span>`).join('');
      const axis = `
        <line x1="${pad}" y1="${height - pad}" x2="${width - pad}" y2="${height - pad}" stroke="rgba(247,249,252,0.18)"></line>
        <line x1="${pad}" y1="${pad}" x2="${pad}" y2="${height - pad}" stroke="rgba(247,249,252,0.18)"></line>
        <text x="${pad}" y="${pad - 10}" fill="#aab4c0" font-size="12">${esc(maxY)}</text>
        <text x="${pad}" y="${height - 8}" fill="#aab4c0" font-size="12">${esc(points[0].date)}</text>
        <text x="${width - pad}" y="${height - 8}" fill="#aab4c0" font-size="12" text-anchor="end">${esc(last.date)}</text>
      `;

      return `<div class="chart-with-legend"><svg viewBox="0 0 ${width} ${height}" role="img" aria-label="Milestone trend chart">${axis}${paths}</svg><div class="chart-legend">${labels}</div></div>`;
    }

    function issueList(items, emptyText, limit = 10) {
      const limited = items.slice(0, limit);
      if (!limited.length) return `<div class="empty">${esc(emptyText)}</div>`;
      return `
        <ul class="issue-list">
          ${limited.map((item) => `
            <li class="issue">
              <a href="${esc(item.html_url)}">#${esc(item.number)} ${esc(item.title)}</a>
              <div class="issue-meta">
                <span>${esc(item.kind === 'pull_request' ? 'PR' : 'Issue')}</span>
                <span>${esc(item.state)}</span>
                <span>updated ${esc(formatDate(item.updated_at))}</span>
                ${item.assignees.length ? `<span>${esc(item.assignees.join(', '))}</span>` : '<span>unassigned</span>'}
              </div>
              <div class="pill-row" style="margin-top:8px">
                ${item.labels.slice(0, 6).map((label) => `<span class="pill">${esc(label)}</span>`).join('')}
              </div>
            </li>
          `).join('')}
        </ul>
      `;
    }

    function mergedList(items, emptyText, limit = 8) {
      const limited = items.slice(0, limit);
      if (!limited.length) return `<div class="empty">${esc(emptyText)}</div>`;
      return `
        <ul class="issue-list">
          ${limited.map((item) => `
            <li class="issue">
              <a href="${esc(item.html_url)}">#${esc(item.number)} ${esc(item.title)}</a>
              <div class="issue-meta">
                <span>merged ${esc(formatDate(item.merged_at))}</span>
                <span>to ${esc(item.base_ref || 'main')}</span>
                ${item.linked_issue_numbers && item.linked_issue_numbers.length ? `<span>ships #${esc(item.linked_issue_numbers.join(', #'))}</span>` : ''}
                ${item.author ? `<span>${esc(item.author)}</span>` : ''}
              </div>
            </li>
          `).join('')}
        </ul>
      `;
    }

    function labelSummary(items) {
      const ignored = new Set(['triaged', 'do-not-triage']);
      const counts = new Map();
      items.forEach((item) => {
        item.labels.forEach((label) => {
          if (!ignored.has(label)) counts.set(label, (counts.get(label) || 0) + 1);
        });
      });
      return [...counts.entries()]
        .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
        .slice(0, 10)
        .map(([label, count]) => `<span class="pill">${esc(label)} ${esc(count)}</span>`)
        .join('');
    }

    function milestonePulse(milestone, options = {}) {
      const openItems = milestone.items.filter((item) => item.state === 'open');
      const shippedItems = milestone.shipped_items || [];
      const highPriority = openItems.filter((item) => item.labels.includes('high-priority'));
      const unassigned = openItems.filter((item) => item.assignees.length === 0);
      const milestoneTrend = trendForMilestone(milestone);
      const backLink = options.showBackLink ? '<p><a href="../">&larr; kongctl Pulse index</a></p>' : '';

      return `
        <section class="hero" id="${esc(milestone.slug)}">
          <div class="hero-copy panel">
            ${backLink}
            <h1>${esc(milestone.title)} Pulse</h1>
            <p class="lede">${esc(milestone.description || 'Progress, open risk, and work remaining for this milestone.')}</p>
            <div class="pill-row">
              <span class="pill green">${esc(percent(milestone.completion_percent))} complete</span>
              <span class="pill">${esc(daysLabel(milestone.days_remaining))}</span>
              <a class="pill" href="${esc(milestone.html_url)}">GitHub milestone</a>
            </div>
          </div>
          <aside class="hero-side panel">
            ${metric(milestone.total_items, 'Items in this milestone')}
            ${metric(milestone.closed_items, 'Closed')}
            ${metric(milestone.open_items, 'Open')}
          </aside>
        </section>

        <section class="burn-workstream">
          <section class="section panel">
            <h2>Burnup And Burndown</h2>
            ${progressBar(milestone.closed_items, milestone.total_items)}
            <div class="chart" style="margin-top:18px">
              ${chart(milestoneTrend, [
                { key: 'total_items', label: 'Scope', color: '#8da2ff' },
                { key: 'closed_items', label: 'Closed', color: '#ccff00' },
                { key: 'open_items', label: 'Open', color: '#ff6b5f' }
              ])}
            </div>
          </section>

          <section class="section panel">
            <h2>Open Workstream Mix</h2>
            <div class="pill-row">${labelSummary(openItems) || '<span class="pill green">No open labels</span>'}</div>
          </section>
        </section>

        <section class="layout">
          <section class="section panel">
            <h2>Recently Merged</h2>
            ${mergedList(shippedItems, 'Merged milestone PRs will appear here as the milestone progresses.', 8)}
          </section>

          <section class="section panel">
            <h2>Needs Attention</h2>
            <h3>High priority</h3>
            ${issueList(highPriority, 'No open high-priority items.', 6)}
            <h3 style="margin-top:18px">Unassigned</h3>
            ${issueList(unassigned, 'No unassigned open items.', 6)}
          </section>
        </section>
      `;
    }

    function renderIndex() {
      const milestones = data.milestones;

      app.innerHTML = `
        <section class="hero portfolio-hero">
          <div class="hero-copy panel">
            <h1>kongctl Pulse</h1>
            <p class="lede"><code>kongctl</code> progress, burn trends, open risks and more... updated daily.</p>
          </div>
        </section>

        ${milestones.map((milestone) => milestonePulse(milestone)).join('')}

        <p class="footer">Generated by <code>scripts/milestone-pulse.sh</code>. Daily snapshots live in <code>docs/reports/milestones/data</code>.</p>
      `;
    }

    function renderMilestone() {
      const milestone = data.milestones.find((item) => item.number === milestoneNumber);
      if (!milestone) {
        app.innerHTML = '<section class="section panel"><h1>Milestone not found</h1><p class="lede">This Pulse was generated for a milestone that is not in the latest snapshot.</p></section>';
        return;
      }

      document.title = `${milestone.title} Pulse`;
      app.innerHTML = `
        ${milestonePulse(milestone, { showBackLink: true })}
        <p class="footer">This Pulse refreshes from GitHub milestone data when the scheduled workflow runs.</p>
      `;
    }

    if (view === 'milestone') {
      renderMilestone();
    } else {
      renderIndex();
    }
  </script>
</body>
</html>
HTML_SCRIPT
  } > "${target}"
}

write_markdown() {
  local target="$1"
  local milestone_number="${2:-}"

  mkdir -p "$(dirname "${target}")"
  if [ -z "${milestone_number}" ]; then
    jq -r '
      "# kongctl Pulse\n",
      "Generated: \(.generated_at)",
      "Repository: \(.repo)",
      "Report scope: \(.milestone_state) milestones\n",
      "## Portfolio\n",
      "- Milestones: \(.totals.milestones)",
      "- Portfolio progress: \(.totals.completion_percent)%",
      "- Items across milestones: \(.totals.closed_items) closed / \(.totals.total_items) total",
      "- Open items across milestones: \(.totals.open_items)",
      "- Open high-priority: \(.totals.open_high_priority)",
      "- Open unassigned: \(.totals.open_unassigned)",
      "- Stale open 14d+: \(.totals.stale_open)\n",
      "## Current Milestones\n",
      "| Milestone | Progress | Open | Closed | Due | Pulse |",
      "| --- | ---: | ---: | ---: | --- | --- |",
      (.milestones[]
        | "| [\(.title)](\(.html_url)) | \(.completion_percent)% | \(.open_items) | \(.closed_items) | \(.due_on // "none") | [Pulse](\(.slug)/) |"
      )
    ' "${latest_file}" > "${target}"
    return
  fi

  jq -r --argjson milestone_number "${milestone_number}" --arg generated_at "${generated_at}" '
    .milestones[]
    | select(.number == $milestone_number)
    | "# \(.title) Pulse\n",
      "Generated: \($generated_at // empty)",
      "GitHub milestone: \(.html_url)\n",
      "## Summary\n",
      "- Progress: \(.completion_percent)%",
      "- Items: \(.closed_items) closed / \(.total_items) total",
      "- Open items: \(.open_items)",
      "- Due: \(.due_on // "none")",
      "- Days remaining: \(.days_remaining // "none")",
      "- Open high-priority: \(.open_high_priority)",
      "- Open bugs: \(.open_bugs)",
      "- Open unassigned: \(.open_unassigned)",
      "- Stale open 14d+: \(.stale_open)\n",
      "## Recently Merged\n",
      (if ((.shipped_items // []) | length) == 0 then
        "No milestone PRs have been merged yet."
      else
        ((.shipped_items // [])[0:8][]
          | "- [#\(.number) \(.title)](\(.html_url)) - merged \(.merged_at // "unknown")" +
            (if ((.linked_issue_numbers // []) | length) > 0 then ", ships #\((.linked_issue_numbers // []) | join(", #"))" else "" end)
        )
      end)
  ' "${latest_file}" > "${target}"
}

write_html "${output_dir}/index.html" "index" "" "../../../"
write_markdown "${output_dir}/latest.md"

jq -c '.milestones[] | {number, slug}' "${latest_file}" | while IFS= read -r milestone; do
  number="$(jq -r '.number' <<< "${milestone}")"
  slug="$(jq -r '.slug' <<< "${milestone}")"
  write_html "${output_dir}/${slug}/index.html" "milestone" "${number}" "../../../../"
  write_markdown "${output_dir}/${slug}/latest.md" "${number}"
done

echo "kongctl Pulse generated:"
echo "  ${output_dir}/index.html"
echo "  ${output_dir}/latest.md"
echo "  ${latest_file}"

#!/usr/bin/env python3
"""Download and summarize failed E2E GitHub Actions shard artifacts."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


DEFAULT_WORKFLOW = "e2e.yaml"
DEFAULT_ARTIFACT_PREFIX = "e2e-artifacts-"
TRANSIENT_PATTERNS = (
    "context deadline exceeded",
    "client.timeout",
    "connection reset",
    "connection refused",
    "connection timed out",
    "dial tcp",
    "eof",
    "i/o timeout",
    "net/http: timeout",
    "no such host",
    "service unavailable",
    "temporary failure",
    "tls handshake timeout",
    "too many requests",
    "unexpected eof",
    " 429 ",
    " 500 ",
    " 502 ",
    " 503 ",
    " 504 ",
    "status=429",
    "status=500",
    "status=502",
    "status=503",
    "status=504",
    "status_code\":429",
    "status_code\":500",
    "status_code\":502",
    "status_code\":503",
    "status_code\":504",
)


@dataclass
class RunInfo:
    database_id: int
    run_number: int | None
    run_attempt: int | None
    name: str
    workflow_name: str
    display_title: str
    path: str
    status: str
    conclusion: str
    event: str
    head_branch: str
    head_sha: str
    html_url: str


@dataclass
class PRInfo:
    number: int
    title: str
    url: str
    head_ref: str
    head_sha: str
    base_ref: str


@dataclass
class Artifact:
    artifact_id: int
    name: str
    expired: bool
    size_in_bytes: int
    created_at: str
    updated_at: str

    @property
    def org_name(self) -> str:
        match = re.match(r"^e2e-artifacts-\d+-(.+)$", self.name)
        if match:
            return match.group(1)
        return self.name


@dataclass
class Job:
    name: str
    conclusion: str
    status: str
    html_url: str

    @property
    def org_name(self) -> str | None:
        match = re.match(r"^E2E \((.+)\)$", self.name)
        if match:
            return match.group(1)
        return None


@dataclass
class ScenarioFailure:
    name: str
    normalized_name: str
    test_dir: Path | None = None
    log_excerpt: str = ""
    failed_commands: list["CommandFailure"] = field(default_factory=list)
    classification: str = "unknown"


@dataclass
class CommandFailure:
    path: Path
    command: str = ""
    exit_code: int | None = None
    timed_out: bool = False
    duration: str = ""
    stderr: str = ""
    stdout: str = ""
    execution_errors: str = ""
    log_hints: str = ""
    http_hints: str = ""


@dataclass
class ShardResult:
    artifact_dir: Path
    org_name: str
    run_attempt: int | None
    shard_index: int | None
    shard_total: int | None
    exit_code: int | None
    duration_seconds: int | None
    passed_count: int
    failed_count: int
    skipped_count: int
    observed_count: int
    failed_scenarios: list[ScenarioFailure]
    selected: bool = True


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    repo_root = git_repo_root()

    if args.artifacts_dir:
        artifact_dirs = [Path(p).expanduser().resolve() for p in args.artifacts_dir]
        run_info = None
        pr_info = None
        download_dir = common_parent(artifact_dirs)
    else:
        ensure_gh()
        repo = args.repo or infer_repo()
        pr_info = None
        if args.pr:
            repo, pr_number = parse_pr_ref(repo, args.pr)
            pr_info = fetch_pr(repo, pr_number)
            run_info = resolve_pr_run(repo, args.workflow, pr_info, args.max_pages)
        else:
            run_info = resolve_run(repo, args.workflow, args.run, args.max_pages)
        jobs = fetch_jobs(repo, run_info.database_id, args.attempt or run_info.run_attempt)
        artifacts = fetch_artifacts(repo, run_info.database_id)
        selected_artifacts = select_artifacts(
            artifacts=artifacts,
            jobs=jobs,
            org_filters=args.org,
            artifact_filters=args.artifact,
            all_shards=args.all_shards,
        )
        download_dir = resolve_download_dir(args.download_dir, repo_root, run_info, args.attempt)
        if not selected_artifacts:
            if args.org or args.artifact or args.all_shards:
                return fail("no matching E2E artifacts found for this run")
            artifact_dirs = []
        else:
            artifact_dirs = download_artifacts(repo, run_info.database_id, selected_artifacts, download_dir)

    if artifact_dirs:
        shard_results = analyze_artifacts(
            artifact_dirs,
            requested_attempt=args.attempt,
            include_success=args.include_success,
            max_snippet_lines=args.max_snippet_lines,
        )
    else:
        shard_results = []
    report = render_report(run_info, pr_info, download_dir, shard_results)

    if args.report_file:
        report_file = Path(args.report_file).expanduser()
        if not report_file.is_absolute():
            report_file = (Path.cwd() / report_file).resolve()
    else:
        report_file = download_dir / "e2e-ci-diagnosis.md"
    report_file.parent.mkdir(parents=True, exist_ok=True)
    report_file.write_text(report + "\n", encoding="utf-8")

    print(report)
    print()
    print(f"Report: {report_file}")

    has_failures = any(result.failed_count > 0 or (result.exit_code or 0) != 0 for result in shard_results)
    return 1 if has_failures and args.fail_on_failure else 0


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Resolve an E2E GitHub Actions run, download failed shard artifacts, "
            "and summarize failed scenarios from the harness artifacts."
        )
    )
    parser.add_argument("--run", help="workflow run number, run id, run URL, or 'latest'")
    parser.add_argument("--pr", help="pull request number or URL; resolves the latest E2E run for the PR")
    parser.add_argument("--workflow", default=DEFAULT_WORKFLOW, help=f"workflow file/name/id (default: {DEFAULT_WORKFLOW})")
    parser.add_argument("--repo", help="GitHub repository in owner/name form (default: inferred from origin)")
    parser.add_argument("--org", action="append", default=[], help="matrix org/shard name to download; repeatable")
    parser.add_argument("--artifact", action="append", default=[], help="exact artifact name to download; repeatable")
    parser.add_argument("--all-shards", action="store_true", help="download all E2E shard artifacts instead of failed shards")
    parser.add_argument("--attempt", type=int, help="workflow attempt to report; default is latest attempt from artifacts")
    parser.add_argument(
        "--artifacts-dir",
        action="append",
        help="analyze an already downloaded artifact directory instead of calling GitHub; repeatable",
    )
    parser.add_argument("--download-dir", type=Path, help="directory where artifacts are downloaded")
    parser.add_argument("--report-file", help="write markdown report here (default: <download-dir>/e2e-ci-diagnosis.md)")
    parser.add_argument("--include-success", action="store_true", help="include successful shard summaries")
    parser.add_argument("--max-pages", type=int, default=20, help="workflow run list pages to search for run numbers")
    parser.add_argument("--max-snippet-lines", type=int, default=24, help="max lines per stderr/log excerpt")
    parser.add_argument("--fail-on-failure", action="store_true", help="exit 1 when the report contains E2E failures")
    args = parser.parse_args(argv)

    if not args.artifacts_dir and not args.run and not args.pr:
        parser.error("--run or --pr is required unless --artifacts-dir is provided")
    if args.run and args.pr:
        parser.error("--run and --pr cannot both be provided")
    return args


def resolve_download_dir(download_dir: Path | None, repo_root: Path, run_info: RunInfo, attempt: int | None) -> Path:
    if download_dir is None:
        run_label = run_info.run_number or run_info.database_id
        run_attempt = attempt or run_info.run_attempt
        suffix = f"-attempt-{run_attempt}" if run_attempt else ""
        return repo_root / ".e2e-artifacts" / "ci" / f"e2e-run-{run_label}{suffix}"

    resolved = Path(download_dir).expanduser()
    if not resolved.is_absolute():
        return (Path.cwd() / resolved).resolve()
    return resolved


def ensure_gh() -> None:
    if shutil.which("gh") is None:
        raise SystemExit("gh is required; install GitHub CLI and authenticate with `gh auth login`")


def git_repo_root() -> Path:
    result = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if result.returncode == 0 and result.stdout.strip():
        return Path(result.stdout.strip())
    return Path.cwd()


def infer_repo() -> str:
    result = subprocess.run(
        ["git", "config", "--get", "remote.origin.url"],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode != 0:
        raise SystemExit("unable to infer repo from git remote; pass --repo owner/name")
    remote = result.stdout.strip()
    patterns = (
        r"git@github\.com:(?P<repo>[^/]+/[^.]+)(?:\.git)?$",
        r"https://github\.com/(?P<repo>[^/]+/[^.]+)(?:\.git)?$",
        r"ssh://git@github\.com/(?P<repo>[^/]+/[^.]+)(?:\.git)?$",
    )
    for pattern in patterns:
        match = re.match(pattern, remote)
        if match:
            return match.group("repo")
    raise SystemExit(f"unable to infer GitHub owner/name from remote: {remote}")


def parse_pr_ref(default_repo: str, pr_ref: str) -> tuple[str, int]:
    clean = pr_ref.strip()
    url_match = re.search(r"github\.com/(?P<repo>[^/]+/[^/]+)/pull/(?P<number>\d+)", clean)
    if url_match:
        return url_match.group("repo"), int(url_match.group("number"))
    if clean.isdigit():
        return default_repo, int(clean)
    raise SystemExit(f"unable to parse pull request reference {pr_ref!r}; use a PR number or URL")


def fetch_pr(repo: str, pr_number: int) -> PRInfo:
    data = gh_api_json(repo, f"/pulls/{pr_number}")
    head = data.get("head") or {}
    base = data.get("base") or {}
    head_sha = str(head.get("sha") or "").strip()
    if not head_sha:
        raise SystemExit(f"pull request #{pr_number} did not include a head SHA")
    return PRInfo(
        number=int(data.get("number") or pr_number),
        title=str(data.get("title") or ""),
        url=str(data.get("html_url") or ""),
        head_ref=str(head.get("ref") or ""),
        head_sha=head_sha,
        base_ref=str(base.get("ref") or ""),
    )


def resolve_run(repo: str, workflow: str, run_ref: str, max_pages: int) -> RunInfo:
    run_id = parse_run_url(run_ref)
    if run_id is not None:
        return fetch_run(repo, run_id)

    if run_ref.isdigit() and len(run_ref) > 6:
        try:
            return fetch_run(repo, int(run_ref))
        except RuntimeError:
            pass

    for page in range(1, max_pages + 1):
        data = gh_api_json(
            repo,
            f"/actions/workflows/{workflow}/runs?per_page=100&page={page}",
        )
        runs = data.get("workflow_runs", [])
        if not runs:
            break
        if run_ref == "latest":
            return run_from_json(runs[0])
        for item in runs:
            if str(item.get("run_number")) == run_ref or str(item.get("id")) == run_ref:
                return run_from_json(item)
    raise SystemExit(f"unable to resolve workflow run {run_ref!r} for workflow {workflow!r}")


def resolve_pr_run(repo: str, workflow: str, pr_info: PRInfo, max_pages: int) -> RunInfo:
    runs = resolve_pr_runs_from_check_runs(repo, workflow, pr_info)
    if runs:
        return latest_run(runs)

    runs = resolve_pr_runs_from_workflow_runs(repo, workflow, pr_info, max_pages)
    if runs:
        return latest_run(runs)

    raise SystemExit(
        f"unable to resolve an E2E workflow run for PR #{pr_info.number} at {pr_info.head_sha[:12]}"
    )


def resolve_pr_runs_from_check_runs(repo: str, workflow: str, pr_info: PRInfo) -> list[RunInfo]:
    check_runs = gh_api_pages(repo, f"/commits/{pr_info.head_sha}/check-runs?per_page=100", "check_runs")
    run_ids: set[int] = set()
    for check_run in check_runs:
        name = str(check_run.get("name") or "")
        if not is_e2e_check_name(name):
            continue
        run_id = action_run_id_from_url(str(check_run.get("details_url") or check_run.get("html_url") or ""))
        if run_id is not None:
            run_ids.add(run_id)

    runs = []
    for run_id in sorted(run_ids):
        run = fetch_run(repo, run_id)
        if workflow_matches(run, workflow):
            runs.append(run)
    return runs


def resolve_pr_runs_from_workflow_runs(repo: str, workflow: str, pr_info: PRInfo, max_pages: int) -> list[RunInfo]:
    matched: list[RunInfo] = []
    for page in range(1, max_pages + 1):
        data = gh_api_json(
            repo,
            f"/actions/workflows/{workflow}/runs?event=pull_request&per_page=100&page={page}",
        )
        runs = data.get("workflow_runs", [])
        if not runs:
            break
        for item in runs:
            if workflow_run_matches_pr(item, pr_info):
                matched.append(run_from_json(item))
        if matched:
            break
    return matched


def workflow_run_matches_pr(data: dict[str, Any], pr_info: PRInfo) -> bool:
    if str(data.get("head_sha") or "") == pr_info.head_sha:
        return True
    for item in data.get("pull_requests") or []:
        if to_int(item.get("number")) == pr_info.number:
            return True
    return False


def latest_run(runs: list[RunInfo]) -> RunInfo:
    return sorted(
        runs,
        key=lambda run: (run.run_number or 0, run.run_attempt or 0, run.database_id),
        reverse=True,
    )[0]


def is_e2e_check_name(name: str) -> bool:
    return name == "E2E" or name.startswith("E2E ")


def action_run_id_from_url(value: str) -> int | None:
    match = re.search(r"/actions/runs/(\d+)", value)
    if match:
        return int(match.group(1))
    return None


def workflow_matches(run: RunInfo, workflow: str) -> bool:
    want = workflow.strip().casefold()
    if not want:
        return True
    candidates = {
        run.name.casefold(),
        run.workflow_name.casefold(),
        run.path.casefold(),
        Path(run.path).name.casefold() if run.path else "",
    }
    return want in candidates


def parse_run_url(value: str) -> int | None:
    match = re.search(r"/actions/runs/(\d+)", value)
    if match:
        return int(match.group(1))
    return None


def fetch_run(repo: str, run_id: int) -> RunInfo:
    return run_from_json(gh_api_json(repo, f"/actions/runs/{run_id}"))


def run_from_json(data: dict[str, Any]) -> RunInfo:
    return RunInfo(
        database_id=int(data.get("database_id") or data.get("id")),
        run_number=to_int(data.get("run_number")),
        run_attempt=to_int(data.get("run_attempt")),
        name=str(data.get("name") or ""),
        workflow_name=str(data.get("workflow_name") or data.get("name") or ""),
        display_title=str(data.get("display_title") or ""),
        path=str(data.get("path") or ""),
        status=str(data.get("status") or ""),
        conclusion=str(data.get("conclusion") or ""),
        event=str(data.get("event") or ""),
        head_branch=str(data.get("head_branch") or ""),
        head_sha=str(data.get("head_sha") or ""),
        html_url=str(data.get("html_url") or ""),
    )


def fetch_jobs(repo: str, run_id: int, attempt: int | None) -> list[Job]:
    if attempt:
        path = f"/actions/runs/{run_id}/attempts/{attempt}/jobs?per_page=100"
    else:
        path = f"/actions/runs/{run_id}/jobs?filter=latest&per_page=100"
    jobs: list[Job] = []
    for item in gh_api_pages(repo, path, "jobs"):
        jobs.append(
            Job(
                name=str(item.get("name") or ""),
                conclusion=str(item.get("conclusion") or ""),
                status=str(item.get("status") or ""),
                html_url=str(item.get("html_url") or ""),
            )
        )
    return jobs


def fetch_artifacts(repo: str, run_id: int) -> list[Artifact]:
    artifacts: list[Artifact] = []
    for item in gh_api_pages(repo, f"/actions/runs/{run_id}/artifacts?per_page=100", "artifacts"):
        artifacts.append(
            Artifact(
                artifact_id=int(item["id"]),
                name=str(item.get("name") or ""),
                expired=bool(item.get("expired")),
                size_in_bytes=int(item.get("size_in_bytes") or 0),
                created_at=str(item.get("created_at") or ""),
                updated_at=str(item.get("updated_at") or ""),
            )
        )
    return artifacts


def gh_api_pages(repo: str, path: str, key: str) -> list[dict[str, Any]]:
    page = 1
    out: list[dict[str, Any]] = []
    while True:
        separator = "&" if "?" in path else "?"
        data = gh_api_json(repo, f"{path}{separator}page={page}")
        items = data.get(key, [])
        out.extend(items)
        if len(items) < 100:
            return out
        page += 1


def gh_api_json(repo: str, path: str) -> dict[str, Any]:
    result = subprocess.run(
        ["gh", "api", "-H", "Accept: application/vnd.github+json", f"/repos/{repo}{path}"],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip()
        raise RuntimeError(f"gh api {path} failed: {detail}")
    return json.loads(result.stdout)


def select_artifacts(
    artifacts: list[Artifact],
    jobs: list[Job],
    org_filters: list[str],
    artifact_filters: list[str],
    all_shards: bool,
) -> list[Artifact]:
    e2e_artifacts = [
        artifact
        for artifact in artifacts
        if artifact.name.startswith(DEFAULT_ARTIFACT_PREFIX) and not artifact.expired
    ]
    by_name = dedupe_artifacts_by_name(e2e_artifacts)
    e2e_artifacts = list(by_name.values())

    if artifact_filters:
        wanted = set(artifact_filters)
        return [artifact for artifact in e2e_artifacts if artifact.name in wanted]

    if org_filters:
        wanted = set(org_filters)
        return [artifact for artifact in e2e_artifacts if artifact.org_name in wanted]

    if all_shards:
        return e2e_artifacts

    failed_orgs = {
        job.org_name
        for job in jobs
        if job.org_name and job.status == "completed" and job.conclusion not in {"success", "skipped"}
    }
    selected = [artifact for artifact in e2e_artifacts if artifact.org_name in failed_orgs]
    if jobs:
        return selected
    return e2e_artifacts


def dedupe_artifacts_by_name(artifacts: list[Artifact]) -> dict[str, Artifact]:
    by_name: dict[str, Artifact] = {}
    for artifact in sorted(artifacts, key=lambda item: item.updated_at):
        by_name[artifact.name] = artifact
    return by_name


def download_artifacts(repo: str, run_id: int, artifacts: list[Artifact], download_dir: Path) -> list[Path]:
    download_dir.mkdir(parents=True, exist_ok=True)
    artifact_dirs: list[Path] = []
    for artifact in artifacts:
        artifact_dir = download_dir / artifact.name
        artifact_dirs.append(artifact_dir)
        if artifact_dir.exists() and any(artifact_dir.iterdir()):
            continue
        artifact_dir.mkdir(parents=True, exist_ok=True)
        result = subprocess.run(
            [
                "gh",
                "run",
                "download",
                str(run_id),
                "--repo",
                repo,
                "--name",
                artifact.name,
                "--dir",
                str(artifact_dir),
            ],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        if result.returncode != 0:
            detail = result.stderr.strip() or result.stdout.strip()
            raise SystemExit(f"failed to download artifact {artifact.name}: {detail}")
    return artifact_dirs


def analyze_artifacts(
    artifact_dirs: list[Path],
    requested_attempt: int | None,
    include_success: bool,
    max_snippet_lines: int,
) -> list[ShardResult]:
    all_results: list[ShardResult] = []
    for artifact_dir in artifact_dirs:
        for results_file in artifact_dir.rglob("scenario-results.txt"):
            all_results.append(
                parse_shard_result(results_file, artifact_root_for_results(results_file, artifact_dir), max_snippet_lines)
            )

    if not all_results:
        raise SystemExit("no scenario-results.txt files found in downloaded artifacts")

    selected_attempt = requested_attempt
    if selected_attempt is None:
        attempts = [result.run_attempt for result in all_results if result.run_attempt is not None]
        selected_attempt = max(attempts) if attempts else None

    filtered: list[ShardResult] = []
    seen_keys: set[tuple[int | None, str]] = set()
    for result in sorted(all_results, key=lambda item: (item.run_attempt or 0, str(item.artifact_dir)), reverse=True):
        if selected_attempt is not None and result.run_attempt not in {None, selected_attempt}:
            continue
        key = (result.shard_index, result.org_name)
        if key in seen_keys:
            continue
        seen_keys.add(key)
        if include_success or result.failed_count > 0 or (result.exit_code or 0) != 0:
            filtered.append(result)

    return sorted(filtered, key=lambda item: (item.shard_index if item.shard_index is not None else 9999, item.org_name))


def parse_shard_result(results_file: Path, artifact_dir: Path, max_snippet_lines: int) -> ShardResult:
    data = results_file.read_text(encoding="utf-8", errors="replace")
    fields, sections = parse_results_file(data)
    org_name = fields.get("org_name") or infer_org_from_artifact_dir(artifact_dir)
    failed_names = [line for line in sections.get("failed", []) if line.strip()]
    run_dir = results_file.parent
    failures = [
        analyze_scenario_failure(run_dir, scenario_name, max_snippet_lines)
        for scenario_name in failed_names
    ]
    return ShardResult(
        artifact_dir=artifact_dir,
        org_name=org_name,
        run_attempt=to_int(fields.get("run_attempt")),
        shard_index=to_int(fields.get("shard_index")),
        shard_total=to_int(fields.get("shard_total")),
        exit_code=to_int(fields.get("exit_code")),
        duration_seconds=to_int(fields.get("duration_seconds")),
        passed_count=to_int(fields.get("passed_count")) or 0,
        failed_count=to_int(fields.get("failed_count")) or len(failures),
        skipped_count=to_int(fields.get("skipped_count")) or 0,
        observed_count=to_int(fields.get("observed_count")) or 0,
        failed_scenarios=failures,
    )


def artifact_root_for_results(results_file: Path, fallback: Path) -> Path:
    for parent in results_file.parents:
        if parent.name.startswith(DEFAULT_ARTIFACT_PREFIX):
            return parent
    return fallback


def parse_results_file(data: str) -> tuple[dict[str, str], dict[str, list[str]]]:
    fields: dict[str, str] = {}
    sections: dict[str, list[str]] = {}
    current_section: str | None = None
    for raw_line in data.splitlines():
        line = raw_line.rstrip()
        section_match = re.match(r"^\[([^\]]+)\]$", line)
        if section_match:
            current_section = section_match.group(1).strip().lower()
            sections.setdefault(current_section, [])
            continue
        if current_section:
            if line.strip():
                sections[current_section].append(line.strip())
            continue
        if "=" in line:
            key, value = line.split("=", 1)
            fields[key.strip()] = value.strip()
    return fields, sections


def analyze_scenario_failure(run_dir: Path, scenario_name: str, max_snippet_lines: int) -> ScenarioFailure:
    normalized = normalize_scenario_name(scenario_name)
    test_dir = find_test_dir(run_dir, scenario_name)
    log_excerpt = extract_failure_excerpt(run_dir / "run.log", scenario_name, max_snippet_lines)
    failed_commands = find_failed_commands(test_dir, max_snippet_lines) if test_dir else []
    signal_text = "\n".join(
        [log_excerpt]
        + [command.stderr for command in failed_commands]
        + [command.stdout for command in failed_commands]
        + [command.execution_errors for command in failed_commands]
        + [command.log_hints for command in failed_commands]
        + [command.http_hints for command in failed_commands]
    )
    return ScenarioFailure(
        name=scenario_name,
        normalized_name=normalized,
        test_dir=test_dir,
        log_excerpt=log_excerpt,
        failed_commands=failed_commands,
        classification=classify_failure(signal_text),
    )


def normalize_scenario_name(name: str) -> str:
    clean = name.strip().replace("\\", "/")
    clean = re.sub(r"^test/e2e/scenarios/", "", clean)
    clean = re.sub(r"^scenarios/", "", clean)
    return clean.rstrip("/")


def find_test_dir(run_dir: Path, scenario_name: str) -> Path | None:
    tests_dir = run_dir / "tests"
    if not tests_dir.is_dir():
        return None

    normalized = normalize_scenario_name(scenario_name)
    variants = {
        scenario_name.strip().replace("\\", "/"),
        normalized,
        f"scenarios/{normalized}",
        f"test/e2e/scenarios/{normalized}",
    }
    candidates = {sanitize_name(f"Test_Scenarios/{variant}") for variant in variants if variant}
    for candidate in candidates:
        path = tests_dir / candidate
        if path.is_dir():
            return path

    suffixes = {sanitize_name(variant).replace(".", "_") for variant in variants if variant}
    for path in tests_dir.iterdir():
        if not path.is_dir():
            continue
        path_key = path.name.replace(".", "_")
        if any(path_key.endswith(suffix) for suffix in suffixes):
            return path
    return None


def sanitize_name(name: str) -> str:
    out = name
    for char in ("/", "\\", " ", ":", "*", "?", '"', "<", ">", "|"):
        out = out.replace(char, "_")
    return out


def extract_failure_excerpt(run_log: Path, scenario_name: str, max_lines: int) -> str:
    if not run_log.is_file():
        return ""
    lines = run_log.read_text(encoding="utf-8", errors="replace").splitlines()
    normalized = normalize_scenario_name(scenario_name)
    match_terms = [
        scenario_name.strip(),
        normalized,
        f"scenarios/{normalized}",
        f"test/e2e/scenarios/{normalized}",
    ]
    start = None
    for index, line in enumerate(lines):
        if "--- FAIL: Test_Scenarios/" in line and any(term and term in line for term in match_terms):
            start = index
            break
    if start is None:
        for index, line in enumerate(lines):
            if any(term and term in line for term in match_terms) and "scenario failed:" in line:
                start = max(0, index - 3)
                break
    if start is None:
        return ""

    collected: list[str] = []
    for line in lines[start:]:
        if collected and re.match(r"^\s+--- (PASS|FAIL|SKIP): Test_Scenarios/", line):
            break
        if collected and line.startswith("FAIL\t"):
            break
        collected.append(line)
        if len(collected) >= max_lines:
            break
    return "\n".join(collected).strip()


def find_failed_commands(test_dir: Path, max_snippet_lines: int) -> list[CommandFailure]:
    failures: list[CommandFailure] = []
    for meta_file in sorted(test_dir.rglob("commands/*/meta.json")):
        try:
            meta = json.loads(meta_file.read_text(encoding="utf-8", errors="replace"))
        except json.JSONDecodeError:
            continue
        exit_code = to_int(meta.get("exit_code"))
        timed_out = bool(meta.get("timed_out"))
        if exit_code == 0 and not timed_out:
            continue
        command_dir = meta_file.parent
        failures.append(
            CommandFailure(
                path=command_dir,
                command=read_optional(command_dir / "command.txt", 6).strip(),
                exit_code=exit_code,
                timed_out=timed_out,
                duration=str(meta.get("duration") or ""),
                stderr=interesting_excerpt(command_dir / "stderr.txt", max_snippet_lines),
                stdout=interesting_excerpt(command_dir / "stdout.txt", max_snippet_lines),
                execution_errors=execution_errors(command_dir / "stdout.txt", max_snippet_lines),
                log_hints=log_hints(command_dir / "kongctl.log", max_snippet_lines),
                http_hints=http_hints(command_dir, max_snippet_lines),
            )
        )
    return failures


def read_optional(path: Path, max_lines: int) -> str:
    if not path.is_file():
        return ""
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    return "\n".join(lines[:max_lines])


def interesting_excerpt(path: Path, max_lines: int) -> str:
    if not path.is_file():
        return ""
    lines = [line.rstrip() for line in path.read_text(encoding="utf-8", errors="replace").splitlines()]
    lines = [line for line in lines if line.strip()]
    if len(lines) <= max_lines:
        return "\n".join(lines)

    scored = []
    for index, line in enumerate(lines):
        lower = line.lower()
        if any(token in lower for token in ("error", "failed", "timeout", "panic", "fatal", "status", "exception")):
            scored.append(index)
    if not scored:
        return "\n".join(lines[-max_lines:])

    window: list[str] = []
    seen: set[int] = set()
    context = max(1, max_lines // max(1, len(scored)) // 2)
    for index in scored:
        for cursor in range(max(0, index - context), min(len(lines), index + context + 1)):
            if cursor not in seen:
                seen.add(cursor)
                window.append(lines[cursor])
            if len(window) >= max_lines:
                return "\n".join(window)
    return "\n".join(window[:max_lines])


def execution_errors(path: Path, max_lines: int) -> str:
    if not path.is_file():
        return ""
    try:
        data = json.loads(path.read_text(encoding="utf-8", errors="replace"))
    except json.JSONDecodeError:
        return ""
    errors = data.get("execution", {}).get("errors", [])
    if not isinstance(errors, list):
        return ""
    lines: list[str] = []
    for item in errors:
        if not isinstance(item, dict):
            continue
        action = str(item.get("action") or "").strip()
        resource_type = str(item.get("resource_type") or "").strip()
        resource_ref = str(item.get("resource_ref") or item.get("resource_name") or "").strip()
        error = str(item.get("error") or "").strip()
        prefix = " ".join(part for part in (action, resource_type, resource_ref) if part)
        if prefix and error:
            lines.append(f"{prefix}: {error}")
        elif error:
            lines.append(error)
        if len(lines) >= max_lines:
            break
    return "\n".join(lines)


def log_hints(path: Path, max_lines: int) -> str:
    if not path.is_file():
        return ""
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    hints = []
    for line in lines:
        lower = line.lower()
        if "event=retry_policy" in lower or "retry_status_codes=" in lower:
            continue
        status_match = re.search(r"status_code=(\d{3})", lower)
        if status_match and int(status_match.group(1)) >= 400:
            hints.append(line.rstrip())
            continue
        if any(token in lower for token in ("level=error", "level=warn", "timeout", "retry attempt", "retrying")):
            hints.append(line.rstrip())
    if not hints:
        return ""
    return "\n".join(hints[-max_lines:])


def http_hints(command_dir: Path, max_lines: int) -> str:
    dump_dir = command_dir / "http-dumps"
    if not dump_dir.is_dir():
        return ""
    hints: list[str] = []
    for path in sorted(dump_dir.glob("*.txt")):
        text = path.read_text(encoding="utf-8", errors="replace")
        status = re.search(r"^HTTP/\S+\s+(\d+)", text, re.MULTILINE)
        if status and int(status.group(1)) >= 400:
            hints.append(f"{path.name}: HTTP {status.group(1)}")
        elif any(pattern.strip() and pattern.strip() in text.lower() for pattern in TRANSIENT_PATTERNS):
            hints.append(f"{path.name}: transient-looking network/service signal")
        if len(hints) >= max_lines:
            break
    return "\n".join(hints)


def classify_failure(text: str) -> str:
    relevant_lines = []
    for line in text.splitlines():
        lower_line = line.lower()
        if "event=retry_policy" in lower_line or "retry_status_codes=" in lower_line:
            continue
        relevant_lines.append(line)
    lower = f" {' '.join(relevant_lines).lower()} "

    if re.search(r"\b(status(?:_code)?[=\": ]+|http/[^ ]+ )(409)\b", lower) or " conflict" in lower:
        return "API conflict"
    if has_transient_signal(lower):
        return "likely transient network/service issue"
    if "expected" in lower and "but got" in lower:
        return "assertion mismatch"
    if "produced unparsable output" in lower or "failed to parse" in lower:
        return "output parsing failure"
    if "expected failure but succeeded" in lower:
        return "unexpected command success"
    if "exit status" in lower or "exit=" in lower:
        return "command failure"
    return "unknown"


def has_transient_signal(lower: str) -> bool:
    network_patterns = (
        "context deadline exceeded",
        "client.timeout",
        "connection reset",
        "connection refused",
        "connection timed out",
        "dial tcp",
        "eof",
        "i/o timeout",
        "net/http: timeout",
        "no such host",
        "service unavailable",
        "temporary failure",
        "tls handshake timeout",
        "too many requests",
        "unexpected eof",
    )
    if any(pattern in lower for pattern in network_patterns):
        return True
    status_matches = re.findall(r"\b(?:status(?:_code)?[=\": ]+|http/[^ ]+ )(\d{3})\b", lower)
    return any(code in {"429", "500", "502", "503", "504"} for code in status_matches)


def render_report(
    run_info: RunInfo | None,
    pr_info: PRInfo | None,
    download_dir: Path,
    shard_results: list[ShardResult],
) -> str:
    lines: list[str] = ["# E2E CI Diagnosis", ""]
    if pr_info:
        lines.append(f"- Pull request: #{pr_info.number} {pr_info.url}")
    if run_info:
        workflow_name = run_info.workflow_name or run_info.name or "workflow"
        if run_info.run_number:
            title = f"{workflow_name} #{run_info.run_number}"
        else:
            title = workflow_name
        lines.extend(
            [
                f"- Run: {title} ({run_info.database_id})",
                f"- Title: {run_info.display_title or 'unknown'}",
                f"- Attempt: {run_info.run_attempt or 'unknown'}",
                f"- Status: {run_info.status}/{run_info.conclusion or 'unknown'}",
                f"- Branch/SHA: {run_info.head_branch or 'unknown'} {run_info.head_sha[:12] if run_info.head_sha else ''}".rstrip(),
                f"- URL: {run_info.html_url}",
            ]
        )
    lines.append(f"- Artifacts: {download_dir}")
    lines.append("")

    if not shard_results:
        lines.append("No failed E2E shard artifacts were found in the selected artifacts.")
        return "\n".join(lines)

    lines.extend(["## Shards", ""])
    for result in shard_results:
        shard = "?"
        if result.shard_index is not None and result.shard_total is not None:
            shard = f"{result.shard_index}/{result.shard_total}"
        lines.append(f"### {result.org_name} (shard {shard})")
        lines.append("")
        lines.append(
            f"- Attempt: {result.run_attempt or 'unknown'}; exit: {result.exit_code}; "
            f"passed/failed/skipped: {result.passed_count}/{result.failed_count}/{result.skipped_count}; "
            f"duration: {result.duration_seconds or 'unknown'}s"
        )
        lines.append(f"- Artifact: {result.artifact_dir}")
        if not result.failed_scenarios:
            lines.append("- No failed scenarios listed in `scenario-results.txt`.")
            lines.append("")
            continue
        lines.append("")
        for scenario in result.failed_scenarios:
            lines.append(f"#### {scenario.normalized_name}")
            lines.append("")
            lines.append(f"- Classification: {scenario.classification}")
            if scenario.test_dir:
                lines.append(f"- Scenario artifacts: {scenario.test_dir}")
            if scenario.failed_commands:
                for command in scenario.failed_commands:
                    lines.append(f"- Failed command: {command.path}")
                    if command.command:
                        lines.extend(["", "```text", trim_block(command.command, 8), "```"])
                    lines.append(
                        f"  exit={command.exit_code} timed_out={str(command.timed_out).lower()} duration={command.duration or 'unknown'}"
                    )
                    if command.stderr:
                        lines.extend(["", "stderr:", "```text", trim_block(command.stderr, 24), "```"])
                    if command.execution_errors:
                        lines.extend(
                            ["", "execution errors:", "```text", trim_block(command.execution_errors, 24), "```"]
                        )
                    if command.stdout and not command.execution_errors:
                        lines.extend(["", "stdout hints:", "```text", trim_block(command.stdout, 24), "```"])
                    if command.log_hints:
                        lines.extend(["", "kongctl log hints:", "```text", trim_block(command.log_hints, 24), "```"])
                    if command.http_hints:
                        lines.extend(["", "HTTP dump hints:", "```text", trim_block(command.http_hints, 24), "```"])
            elif scenario.log_excerpt:
                lines.extend(["", "go test excerpt:", "```text", trim_block(scenario.log_excerpt, 24), "```"])
            else:
                lines.append("- No per-command failure artifacts found; inspect the scenario directory manually.")
            lines.append("")
    return "\n".join(lines).rstrip()


def trim_block(text: str, max_lines: int) -> str:
    lines = text.strip().splitlines()
    if len(lines) <= max_lines:
        return "\n".join(lines)
    omitted = len(lines) - max_lines
    return "\n".join(lines[:max_lines] + [f"... ({omitted} more lines omitted)"])


def infer_org_from_artifact_dir(path: Path) -> str:
    for part in [path.name, *[parent.name for parent in path.parents]]:
        match = re.match(r"^e2e-artifacts-\d+-(.+)$", part)
        if match:
            return match.group(1)
    return path.name


def common_parent(paths: list[Path]) -> Path:
    if not paths:
        return Path.cwd()
    common = os.path.commonpath([str(path) for path in paths])
    return Path(common)


def to_int(value: Any) -> int | None:
    if value is None:
        return None
    try:
        return int(str(value).strip())
    except (TypeError, ValueError):
        return None


def fail(message: str) -> int:
    print(f"error: {message}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv[1:]))
    except RuntimeError as err:
        raise SystemExit(str(err)) from err

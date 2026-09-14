from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("e2e_baseline.py")
SPEC = importlib.util.spec_from_file_location("e2e_baseline", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class E2EBaselineTest(unittest.TestCase):
    def test_refresh_keeps_history_and_reports_latest_window(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            observations, report = root / "observations.json", root / "report.md"
            saved = [self.run_record(i, f"2026-09-0{i}T00:00:00Z") for i in [1, 2]]
            fresh = [self.run_record(3, "2026-09-03T00:00:00Z")]
            MODULE.write_json(observations, MODULE.observation_document("kong/kongctl", saved))
            args = [str(SCRIPT), "--count", "2", "--scan", "10", "--observations", str(observations),
                    "--output", str(report), "--refresh"]
            with patch.object(sys, "argv", args), patch.object(MODULE, "collect_runs", return_value=fresh) as collect:
                self.assertEqual(0, MODULE.main())
            self.assertEqual(10, collect.call_args.args[1])
            self.assertEqual({1, 2}, collect.call_args.kwargs["excluded_run_ids"])
            self.assertEqual([3, 2, 1], [r["run_id"] for r in MODULE.load_observations(observations, "kong/kongctl")])
            text = report.read_text()
            self.assertIn("latest 2 of 3 retained observations", text)
            self.assertIn("https://example.test/runs/3", text)
            self.assertNotIn("https://example.test/runs/1", text)
            # Refresh with no new data retains history and yields the same report.
            with patch.object(sys, "argv", args), patch.object(MODULE, "collect_runs", return_value=[]):
                MODULE.main()
            self.assertEqual(text, report.read_text())
            # Historical target-limited collection remains opt-in unchanged.
            with patch.object(sys, "argv", args[:-1]), patch.object(MODULE, "collect_runs", return_value=[]) as collect:
                MODULE.main()
            self.assertEqual(0, collect.call_args.args[1])

    def test_refresh_requires_archive_and_rejects_frozen(self) -> None:
        for extra in [[], ["--frozen"]]:
            with patch.object(sys, "argv", [str(SCRIPT), "--refresh", "--output", "unused.md", *extra]):
                with self.assertRaises(SystemExit):
                    MODULE.main()

    def test_cache_categories_use_emitted_markers_not_script_or_duration(self) -> None:
        for value in ["exact", "fallback", "cold"]:
            log = f"2026-09-14T00:00:00Z Go build cache result (dependency-fallback-v1): {value}\n"
            self.assertEqual(value, MODULE.cache_result(log, True))
        for value, category in [("true", "exact"), ("false", "cold")]:
            log = f"2026-09-14T00:00:00Z Go build cache primary-key hit: {value}\n"
            self.assertEqual(category, MODULE.cache_result(log, False))
            self.assertEqual("unknown", MODULE.cache_result(log, True))
        self.assertEqual("unknown", MODULE.cache_result("printf 'Go build cache primary-key hit: true'\n", False))
        build = {"databaseId": 42, "steps": [{"name": "Report Go cache status"}]}
        with patch.object(MODULE.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "", "expired")):
            self.assertEqual("unknown", MODULE.read_cache_result("kong/kongctl", 1, build))
        records = [self.run_record(1, "2026-09-01T00:00:00Z")]
        report = MODULE.markdown_report("kong/kongctl", records, MODULE.summarize(records), 20)
        self.assertIn("| unknown | 1 |", report)
        records[0]["cache_result"] = "fallback"
        self.assertIn("| fallback | 1 |", MODULE.markdown_report("kong/kongctl", records, MODULE.summarize(records), 20))

    def test_cache_log_timeout_is_unknown_and_next_lookup_can_succeed(self) -> None:
        build = {"databaseId": 42, "steps": [{"name": "Report Go cache status"}]}
        log = "Go build cache primary-key hit: true\n"
        with patch.object(MODULE.subprocess, "run", side_effect=[
            subprocess.TimeoutExpired("gh", 120, output=log),
            subprocess.CompletedProcess([], 0, log, ""),
        ]) as run:
            self.assertEqual("unknown", MODULE.read_cache_result("kong/kongctl", 1, build))
            self.assertEqual("exact", MODULE.read_cache_result("kong/kongctl", 2, build))
        self.assertEqual(2, run.call_count)
        for call in run.call_args_list:
            self.assertEqual(120, call.kwargs["timeout"])

    def test_nearest_rank(self) -> None:
        values = list(range(1, 21))
        self.assertEqual(10, MODULE.nearest_rank(values, 0.50))
        self.assertEqual(15, MODULE.nearest_rank(values, 0.75))
        self.assertEqual(18, MODULE.nearest_rank(values, 0.90))

    def test_selects_requested_complete_attempt(self) -> None:
        metrics = [
            {"run_attempt": 1, "shard_index": 0, "shard_total": 2},
            {"run_attempt": 1, "shard_index": 1, "shard_total": 2},
            {"run_attempt": 2, "shard_index": 0, "shard_total": 2},
        ]
        selected = MODULE.select_complete_attempt(metrics, 1)
        self.assertEqual([1, 1], [metric["run_attempt"] for metric in selected])

    def test_rejects_incomplete_latest_attempt(self) -> None:
        metrics = [
            {"run_attempt": 1, "shard_index": 0, "shard_total": 2},
            {"run_attempt": 1, "shard_index": 1, "shard_total": 2},
            {"run_attempt": 2, "shard_index": 0, "shard_total": 2},
        ]
        self.assertEqual([], MODULE.select_complete_attempt(metrics, 2))

    def test_merge_runs_keeps_saved_runs_and_replaces_duplicates(self) -> None:
        saved = [self.run_record(1, "2026-09-01T00:00:00Z", attempt=1, marker="saved")]
        collected = [
            self.run_record(1, "2026-09-01T00:00:00Z", attempt=2, marker="collected"),
            self.run_record(2, "2026-09-02T00:00:00Z", attempt=1, marker="new"),
        ]

        merged = MODULE.merge_runs(saved, collected)

        self.assertEqual([2, 1], [run["run_id"] for run in merged])
        self.assertEqual("collected", merged[1]["marker"])
        self.assertEqual(2, merged[1]["run_attempt"])

    def test_observations_round_trip(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "observations.json"
            runs = [self.run_record(1, "2026-09-01T00:00:00Z")]
            MODULE.write_json(path, MODULE.observation_document("kong/kongctl", runs))

            self.assertEqual(runs, MODULE.load_observations(path, "kong/kongctl"))

    def test_load_observations_rejects_incompatible_schema(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "observations.json"
            path.write_text(
                json.dumps({"schema_version": 99, "repository": "kong/kongctl", "runs": []}),
                encoding="utf-8",
            )

            with self.assertRaisesRegex(ValueError, "unsupported schema_version"):
                MODULE.load_observations(path, "kong/kongctl")

    def test_load_observations_rejects_missing_run_fields(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "observations.json"
            path.write_text(
                json.dumps(
                    {
                        "schema_version": MODULE.OBSERVATION_SCHEMA_VERSION,
                        "repository": "kong/kongctl",
                        "cohort": "cache-enabled",
                        "runs": [{"run_id": 1}],
                    }
                ),
                encoding="utf-8",
            )

            with self.assertRaisesRegex(ValueError, "missing fields"):
                MODULE.load_observations(path, "kong/kongctl")

    def test_partial_report_is_explicit(self) -> None:
        runs = [self.run_record(1, "2026-09-01T00:00:00Z")]

        report = MODULE.markdown_report("kong/kongctl", runs, MODULE.summarize(runs), 20)

        self.assertIn("Full successful runs: 1 of 20", report)
        self.assertIn("Status: **collecting**", report)

    def test_rejects_wrong_cohort_and_mixed_saved_data(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "observations.json"
            runs = [self.run_record(1, "2026-09-01T00:00:00Z")]
            MODULE.write_json(path, MODULE.observation_document("kong/kongctl", runs))
            with self.assertRaisesRegex(ValueError, "cohort does not match"):
                MODULE.load_observations(path, "kong/kongctl", "uncached")
            runs[0]["cohort"] = "uncached"
            MODULE.write_json(path, MODULE.observation_document("kong/kongctl", runs))
            with self.assertRaisesRegex(ValueError, "mixed cohorts"):
                MODULE.load_observations(path, "kong/kongctl")

    def test_frozen_report_remains_preliminary(self) -> None:
        runs = [self.run_record(1, "2026-09-01T00:00:00Z")]
        report = MODULE.markdown_report("kong/kongctl", runs, MODULE.summarize(runs), 20, frozen=True)
        self.assertIn("Status: **frozen preliminary**", report)
        with patch.object(MODULE, "gh_json") as api:
            self.assertEqual([], MODULE.collect_runs("kong/kongctl", 0, 100))
            api.assert_not_called()

    def test_rerun_uses_attempt_creation_and_records_harness_cost(self) -> None:
        start = "2026-09-04T19:08:00Z"
        end = "2026-09-04T19:18:50Z"
        jobs = [
            {"name": name, "conclusion": "success", "startedAt": start, "completedAt": end, "steps": []}
            for name in [MODULE.BUILD_JOB, MODULE.HARNESS_JOB, MODULE.VERIFY_JOB,
                         MODULE.REQUIRED_JOB, MODULE.SCENARIO_JOB_PREFIX + "org"]
        ]
        for job, names in [(jobs[0], ["Setup Go", "Build kongctl", "Build scenario test binary",
                                    "Report Go cache status"]),
                           (jobs[1], ["Setup Go", MODULE.HARNESS_JOB])]:
            job["steps"] = [{"name": name, "startedAt": start, "completedAt": end} for name in names]
        candidate = {"databaseId": 1, "createdAt": "2026-09-04T18:35:56Z", "url": "https://example.test/1"}
        attempt = {"created_at": "2026-09-04T19:07:59Z", "head_sha": "abc", "conclusion": "success"}
        metrics = [{"konnect_environment": "com", "run_attempt": 3, "org_name": "org",
                    "execution_duration_seconds": 100, "selected_scenario_count": 1,
                    "scenario_durations": [], "reset": {}}]
        with patch.object(MODULE, "gh_json", side_effect=[
            [candidate], {"attempt": 3, "jobs": jobs}, attempt,
        ]) as api, patch.object(MODULE, "download_metrics", return_value=metrics):
            records = MODULE.collect_runs("kong/kongctl", 1, 100)
        record = records[0]
        self.assertEqual(651, record["queue_to_required_status_seconds"])
        self.assertEqual(1, record["workflow_admission_delay_seconds"])
        self.assertEqual(650, record["harness_job_seconds"])
        self.assertEqual(650, record["harness_setup_seconds"])
        self.assertEqual(650, record["harness_test_seconds"])
        self.assertEqual(candidate["createdAt"], record["original_created_at"])
        self.assertEqual("cache-enabled", record["cohort"])
        self.assertIn("/attempts/3", api.call_args.args[0][1])
        jobs[0]["startedAt"] = candidate["createdAt"]
        self.assertIsNone(MODULE.eligible_run(
            {**candidate, "createdAt": attempt["created_at"], "original_created_at": candidate["createdAt"],
             "head_sha": "abc"}, jobs, metrics,
        ))

    def test_other_cohort_is_excluded_before_artifact_download(self) -> None:
        jobs = [{"name": MODULE.BUILD_JOB, "steps": []}]
        with patch.object(MODULE, "gh_json", side_effect=[
            [{"databaseId": 1}], {"attempt": 1, "jobs": jobs},
        ]), patch.object(MODULE, "download_metrics") as download:
            self.assertEqual([], MODULE.collect_runs("kong/kongctl", 1, 100))
            download.assert_not_called()

    def test_allocation_compatibility_and_invalid_metadata(self) -> None:
        self.assertEqual("modulo-v1", MODULE.metric_allocation({"schema_version": 1}))
        for metric in ({"schema_version": 2}, {"schema_version": 99, "allocation_id": "modulo-v1"},
                       {"schema_version": 2, "allocation_id": "weighted-v1:bad"}):
            self.assertIsNone(MODULE.metric_allocation(metric))
        weighted = "weighted-v1:" + "a" * 64
        self.assertEqual(weighted, MODULE.metric_allocation({"schema_version": 2, "allocation_id": weighted}))
        replay = weighted + ":pr-replay:" + "b" * 64
        self.assertEqual(replay, MODULE.metric_allocation({"schema_version": 2, "allocation_id": replay}))
        self.assertIsNone(MODULE.metric_allocation({"schema_version": 2, "allocation_id": weighted + ":pr-replay:bad"}))

    def test_replay_subset_is_not_pooled_with_full_live_allocation(self) -> None:
        weighted = "weighted-v1:" + "a" * 64
        jobs = [{"name": MODULE.BUILD_JOB, "steps": [{"name": "Report Go cache status"}]}]
        with patch.object(MODULE, "gh_json", side_effect=[
            [{"databaseId": 1}], {"attempt": 1, "jobs": jobs},
        ]) as api, patch.object(MODULE, "download_metrics", return_value=[{
            "schema_version": 2, "allocation_id": weighted + ":pr-replay:" + "b" * 64,
        }]):
            self.assertEqual([], MODULE.collect_runs("kong/kongctl", 1, 100, allocation_id=weighted))
            self.assertEqual(2, api.call_count)

    def test_wrong_allocation_is_excluded_before_attempt_lookup(self) -> None:
        jobs = [{"name": MODULE.BUILD_JOB, "steps": [{"name": "Report Go cache status"}]}]
        with patch.object(MODULE, "gh_json", side_effect=[
            [{"databaseId": 1}], {"attempt": 1, "jobs": jobs},
        ]) as api, patch.object(MODULE, "download_metrics", return_value=[{"schema_version": 1}]):
            self.assertEqual([], MODULE.collect_runs("kong/kongctl", 1, 100, allocation_id="weighted-v1:" + "a" * 64))
            self.assertEqual(2, api.call_count)

    def test_saved_allocations_cannot_be_mixed(self) -> None:
        weighted = "weighted-v1:" + "a" * 64
        runs = [self.run_record(1, "2026-09-01T00:00:00Z")]
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "observations.json"
            MODULE.write_json(path, MODULE.observation_document("kong/kongctl", runs))
            with self.assertRaisesRegex(ValueError, "allocation does not match"):
                MODULE.load_observations(path, "kong/kongctl", allocation_id=weighted)
            runs[0]["allocation_id"] = weighted
            MODULE.write_json(path, MODULE.observation_document("kong/kongctl", runs, allocation_id=weighted))
            self.assertEqual(runs, MODULE.load_observations(path, "kong/kongctl", allocation_id=weighted))
            with self.assertRaisesRegex(ValueError, "allocation does not match"):
                MODULE.load_observations(path, "kong/kongctl", allocation_id="weighted-v1:" + "b" * 64)
            runs[0]["allocation_id"] = "modulo-v1"
            MODULE.write_json(path, MODULE.observation_document("kong/kongctl", runs, allocation_id=weighted))
            with self.assertRaisesRegex(ValueError, "mixed allocations"):
                MODULE.load_observations(path, "kong/kongctl", allocation_id=weighted)

    def test_mixed_shard_allocations_are_ineligible(self) -> None:
        metrics = [
            {"schema_version": 1, "konnect_environment": "com"},
            {"schema_version": 2, "konnect_environment": "com", "allocation_id": "weighted-v1:" + "a" * 64},
        ]
        self.assertIsNone(MODULE.eligible_run({}, [], metrics))

    def test_duplicate_shard_index_is_not_a_complete_attempt(self) -> None:
        metric = {"run_attempt": 1, "shard_index": 0, "shard_total": 1}
        self.assertEqual([], MODULE.select_complete_attempt([metric, metric], 1))

    @staticmethod
    def run_record(
        run_id: int,
        created_at: str,
        *,
        attempt: int = 1,
        marker: str = "",
    ) -> dict[str, object]:
        return {
            "cohort": "cache-enabled",
            "original_created_at": created_at,
            "head_sha": "abc",
            "harness_job_seconds": 3.0,
            "harness_setup_seconds": 1.0,
            "harness_test_seconds": 2.0,
            "build_setup_seconds": 1.0,
            "run_id": run_id,
            "run_attempt": attempt,
            "url": f"https://example.test/runs/{run_id}",
            "created_at": created_at,
            "workflow_admission_delay_seconds": 1.0,
            "queue_to_required_status_seconds": 2.0,
            "build_job_seconds": 3.0,
            "build_kongctl_seconds": 2.0,
            "build_scenario_binary_seconds": 1.0,
            "longest_shard_seconds": 4.0,
            "shard_spread_seconds": 1.0,
            "shard_admission_delay_seconds": {"org": 0.0},
            "shards": [
                {
                    "org_name": "org",
                    "selected_scenario_count": 1,
                    "execution_duration_seconds": 4,
                }
            ],
            "scenario_durations": [{"scenario": "apis/example", "duration_seconds": 1.0}],
            "reset": {"count": 1},
            "marker": marker,
        }


if __name__ == "__main__":
    unittest.main()

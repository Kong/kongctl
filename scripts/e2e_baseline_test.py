from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("e2e-baseline.py")
SPEC = importlib.util.spec_from_file_location("e2e_baseline", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class E2EBaselineTest(unittest.TestCase):
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

from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


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
                json.dumps({"schema_version": 2, "repository": "kong/kongctl", "runs": []}),
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
                        "schema_version": 1,
                        "repository": "kong/kongctl",
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

    @staticmethod
    def run_record(
        run_id: int,
        created_at: str,
        *,
        attempt: int = 1,
        marker: str = "",
    ) -> dict[str, object]:
        return {
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

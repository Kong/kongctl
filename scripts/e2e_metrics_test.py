from __future__ import annotations

import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("e2e-metrics.py")
SPEC = importlib.util.spec_from_file_location("e2e_metrics", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class E2EMetricsTest(unittest.TestCase):
    def test_collects_scenario_and_reset_metrics(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            (root / "assigned-scenarios.txt").write_text(
                "shard_index=1\nshard_total=2\n\na/scenario.yaml\nb/scenario.yaml\n",
                encoding="utf-8",
            )
            (root / "scenario-results.txt").write_text(
                "org_name=acceptance-2\nrun_attempt=3\nduration_seconds=42\n",
                encoding="utf-8",
            )
            (root / "run.log").write_text(
                "    --- PASS: Test_Scenarios/test/e2e/scenarios/a/scenario.yaml (1.25s)\n"
                "    --- FAIL: Test_Scenarios/test/e2e/scenarios/b/scenario.yaml (2.50s)\n",
                encoding="utf-8",
            )
            reset_dir = root / "commands" / "000-reset_org"
            reset_dir.mkdir(parents=True)
            (reset_dir / "observation.json").write_text(
                json.dumps(
                    {
                        "type": "reset_summary",
                        "executed": True,
                        "regions": [
                            {
                                "duration_ms": 500,
                                "details": [
                                    {
                                        "total": 2,
                                        "deleted": 1,
                                        "list_calls": 2,
                                        "list_duration_ms": 300,
                                        "delete_calls": 1,
                                        "delete_duration_ms": 100,
                                    }
                                ],
                            }
                        ],
                    }
                ),
                encoding="utf-8",
            )

            metrics = MODULE.collect_metrics(
                root,
                {"GITHUB_RUN_ID": "123", "KONGCTL_E2E_KONNECT_ENV": "com"},
            )

        self.assertEqual(2, metrics["selected_scenario_count"])
        self.assertEqual(42, metrics["execution_duration_seconds"])
        self.assertEqual("acceptance-2", metrics["org_name"])
        self.assertEqual(1.25, metrics["scenario_durations"][0]["duration_seconds"])
        self.assertEqual("fail", metrics["scenario_durations"][1]["result"])
        self.assertEqual(
            {
                "count": 1,
                "duration_ms": 500,
                "list_calls": 2,
                "list_duration_ms": 300,
                "resources_found": 2,
                "delete_calls": 1,
                "delete_duration_ms": 100,
                "resources_deleted": 1,
            },
            metrics["reset"],
        )

    def test_rejects_malformed_observation(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            reset_dir = root / "commands" / "000-reset_org"
            reset_dir.mkdir(parents=True)
            (reset_dir / "observation.json").write_text("not json", encoding="utf-8")

            with self.assertRaises(json.JSONDecodeError):
                MODULE.collect_reset_metrics(root)


if __name__ == "__main__":
    unittest.main()

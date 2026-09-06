from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SPEC = importlib.util.spec_from_file_location("e2e_weights", Path(__file__).with_name("e2e-weights.py"))
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class E2EWeightsTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.scenarios = self.root / "scenarios"
        for name in ("a", "b", "new"):
            path = self.scenarios / name / "scenario.yaml"
            path.parent.mkdir(parents=True)
            path.write_text("# fixture\n")

    def observations(self, name, runs):
        path = self.root / name
        path.write_text(json.dumps({"schema_version": 2, "repository": "kong/kongctl", "runs": runs}))
        return path

    @staticmethod
    def runs(count=10):
        return [{
            "run_id": i + 1, "run_attempt": 1,
            "scenario_durations": [
                {"scenario": "scenarios/a/scenario.yaml", "result": "pass", "duration_seconds": i + 1},
                {"scenario": "b/scenario.yaml", "result": "pass", "duration_seconds": 20},
                {"scenario": "removed/scenario.yaml", "result": "pass", "duration_seconds": 99},
            ],
        } for i in range(count)]

    def test_medians_fallback_and_deduplication(self):
        first = self.observations("first.json", self.runs())
        second = self.observations("second.json", self.runs())
        result = MODULE.generate([second, first, first], self.scenarios)
        self.assertEqual({"duration_ms": 5500, "samples": 10}, result["scenarios"]["a/scenario.yaml"])
        self.assertEqual(12750, result["default_duration_ms"])
        self.assertNotIn("removed/scenario.yaml", result["scenarios"])
        self.assertNotIn("new/scenario.yaml", result["scenarios"])
        self.assertEqual(result, MODULE.generate([first, second], self.scenarios))
        self.assertEqual(2, len(result["source_sha256"]))

    def test_latest_attempt_only(self):
        first = self.observations("first.json", self.runs())
        runs = self.runs()
        for run in runs:
            run["run_attempt"] = 2
            run["scenario_durations"][0]["duration_seconds"] = 30
        second = self.observations("second.json", runs)
        result = MODULE.generate([first, second], self.scenarios)
        self.assertEqual({"duration_ms": 30000, "samples": 10}, result["scenarios"]["a/scenario.yaml"])

    def test_insufficient_and_unusable_samples_use_uniform_fallback(self):
        runs = self.runs(9)
        extra = self.runs(6)
        for i, (run, value) in enumerate(zip(extra, [0, -1, float("nan"), float("inf"), 3, 3])):
            run["run_id"] = i + 20
            for record in run["scenario_durations"]:
                record["duration_seconds"] = value
                if i == 4:
                    record["result"] = "fail"
                if i == 5:
                    record["result"] = "skip"
        path = self.observations("observations.json", runs + extra)
        result = MODULE.generate([path], self.scenarios)
        self.assertEqual({}, result["scenarios"])
        self.assertEqual(1000, result["default_duration_ms"])

    def test_conflicting_duplicate_rejected(self):
        first = self.observations("first.json", self.runs())
        runs = self.runs()
        runs[0]["scenario_durations"][0]["duration_seconds"] = 100
        second = self.observations("second.json", runs)
        with self.assertRaisesRegex(ValueError, "conflicting"):
            MODULE.generate([first, second], self.scenarios)

    def test_duplicate_scenario_rejected(self):
        runs = self.runs()
        runs[0]["scenario_durations"].append(runs[0]["scenario_durations"][0])
        path = self.observations("observations.json", runs)
        with self.assertRaisesRegex(ValueError, "duplicate scenario"):
            MODULE.generate([path], self.scenarios)

    def test_invalid_input(self):
        path = self.observations("observations.json", [])
        path.write_text("{}")
        with self.assertRaisesRegex(ValueError, "unsupported"):
            MODULE.generate([path], self.scenarios)
        with self.assertRaisesRegex(ValueError, "no scenarios"):
            MODULE.generate([path], self.root / "missing")

    def test_malformed_records_rejected(self):
        for field, value in (("run_id", True), ("run_id", 0), ("run_attempt", "1"),
                             ("scenario_durations", None), ("scenario_durations", [{}])):
            with self.subTest(field=field, value=value):
                runs = self.runs()
                runs[0][field] = value
                path = self.observations("observations.json", runs)
                with self.assertRaises(ValueError):
                    MODULE.generate([path], self.scenarios)

    def test_out_of_range_duration_rejected(self):
        runs = self.runs()
        runs[0]["scenario_durations"][0]["duration_seconds"] = 10**400
        path = self.observations("observations.json", runs)
        with self.assertRaisesRegex(ValueError, "millisecond range"):
            MODULE.generate([path], self.scenarios)

    def test_cli_preserves_snapshot_when_input_is_invalid(self):
        path = self.observations("observations.json", [])
        path.write_text("null")
        output = self.root / "weights.json"
        output.write_text("preserved snapshot\n")
        result = subprocess.run(
            [sys.executable, str(Path(MODULE.__file__)), str(path),
             "--scenario-root", str(self.scenarios), "--output", str(output)],
            capture_output=True, text=True, check=False,
        )
        self.assertEqual(2, result.returncode)
        self.assertIn("unsupported observations", result.stderr)
        self.assertNotIn("Traceback", result.stderr)
        self.assertEqual("preserved snapshot\n", output.read_text())


if __name__ == "__main__":
    unittest.main()

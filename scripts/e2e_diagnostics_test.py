import copy
import json
from pathlib import Path
import tempfile
import unittest

from e2e_diagnostics import summarize


class DiagnosticsTests(unittest.TestCase):
    def test_malformed_records_do_not_discard_valid_summaries(self):
        valid = {
            "schema_version": 1, "scenario": "valid/scenario.yaml",
            "failure": {"phase": "execution", "cause": "subprocess_deadline"},
            "commands": [{"step": "sync", "command": "sync-all", "outcome": "failed",
                          "duration_ms": 60000, "attempts": [{"duration_ms": 60000, "timeout_ms": 60000}]}],
        }
        malformed = [None, [], {"schema_version": 1, "commands": [{"step": "s"}]}]
        for field, values in {
            "failure": [True, False, [], {}, {"phase": "http"}, {"phase": [], "cause": "unknown"}],
            "scenario": [None, 123],
            "schema_version": [True, 2],
            "commands": [None, [True], [{"step": "s"}]],
        }.items():
            for value in values:
                malformed.append({**valid, field: value})
        for field, values in {
            "duration_ms": [None, "slow", -1, True, 1.5],
            "attempts": [None, [None], [{}], [{"duration_ms": "slow", "timeout_ms": 1}]],
            "outcome": [None],
            "attempt_limit": ["six"],
        }.items():
            for value in values:
                record = copy.deepcopy(valid)
                record["commands"][0][field] = value
                malformed.append(record)

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for index, record in enumerate([valid, *malformed]):
                path = root / str(index) / "scenario-diagnostics.json"
                path.parent.mkdir()
                path.write_text(json.dumps(record))
            summary = summarize(root)
            self.assertIn(f"Captured 1 scenario records; {len(malformed)} unreadable", summary)
            self.assertIn("subprocess_deadline", summary)
            self.assertIn("valid/scenario.yaml", summary)
            self.assertIn("60000 ms", summary)
            self.assertIn("Slowest commands", summary)

    def test_summary_includes_failure_and_successful_timings(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            record = {
                "schema_version": 1, "scenario": "all|<test>",
                "failure": {"step": "sync", "command": "sync-all", "phase": "execution",
                            "cause": "subprocess_deadline"},
                "commands": [
                    {"step": "init", "command": "reset", "duration_ms": 1000,
                     "outcome": "passed", "attempts": []},
                    {"step": "sync", "command": "sync-all", "duration_ms": 60000,
                     "outcome": "failed", "retry_stop": "retry_policy",
                     "attempts": [{"duration_ms": 60000, "timeout_ms": 60000}]},
                ],
            }
            (root / "scenario-diagnostics.json").write_text(json.dumps(record))
            summary = summarize(root)
            for expected in ["subprocess_deadline", "60000 / 60000 ms", "retry_policy",
                             "passed", "1000 ms", "all&#124;&lt;test&gt;"]:
                self.assertIn(expected, summary)

    def test_missing_and_invalid_records_are_explicit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.assertIn("Captured 0 scenario records", summarize(root))
            (root / "scenario-diagnostics.json").write_text("not json")
            self.assertIn("1 unreadable or unsupported", summarize(root))
            self.assertIn("Missing records are not evidence of success", summarize(root))


if __name__ == "__main__":
    unittest.main()

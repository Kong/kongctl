from __future__ import annotations

import importlib.util
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


if __name__ == "__main__":
    unittest.main()

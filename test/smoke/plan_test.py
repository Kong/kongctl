#!/usr/bin/env python3
"""Regression cases for transition-plan safety checks, without Konnect access."""

import copy
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

TEMP_ROOT = sys.argv.pop(1)
CHECKER = pathlib.Path(__file__).resolve().parents[2] / "scripts/smoke/assert-plan.py"


class PlanChecks(unittest.TestCase):
    def setUp(self):
        identities = [
            ("CREATE", "ai_gateway_model_provider"),
            ("CREATE", "ai_gateway_policy"),
            ("UPDATE", "ai_gateway_model"),
            ("DELETE", "ai_gateway_model_provider"),
            ("DELETE", "ai_gateway_policy"),
        ]
        self.plan = {
            "metadata": {"mode": "sync"},
            "changes": [{"id": str(i), "action": action, "resource_type": kind,
                         "namespace": "smoke"} for i, (action, kind) in enumerate(identities)],
            "summary": {"total_changes": 5},
            "execution_order": ["0", "1", "2", "3", "4"],
            "execution_groups": [["0", "1"], ["2"], ["3", "4"]],
        }

    def check_plan(self, plan, expected_error=None, resource="ai_gateway", stage="replacement"):
        with tempfile.TemporaryDirectory(dir=TEMP_ROOT) as directory:
            path = pathlib.Path(directory) / "plan.json"
            path.write_text(json.dumps(plan))
            result = subprocess.run(
                [sys.executable, str(CHECKER), str(path), resource, stage, "smoke", "false"],
                capture_output=True, text=True, check=False,
            )
        if expected_error is None:
            self.assertEqual(result.returncode, 0, result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(expected_error, result.stderr)

    def test_valid_sequential_and_concurrent_plans(self):
        self.check_plan(self.plan)
        del self.plan["execution_groups"]
        self.check_plan(self.plan)

    def test_dependencies_cannot_share_a_concurrency_group(self):
        self.plan["execution_groups"] = [["0", "1", "2"], ["3", "4"]]
        self.check_plan(self.plan, "unsafe concurrent execution")

    def test_namespace_escape(self):
        self.plan["changes"][0]["namespace"] = "another-project"
        self.check_plan(self.plan, "escapes the smoke namespace")

    def test_missing_and_duplicate_execution_ids(self):
        for order in (["0", "1", "2", "3"], ["0", "1", "2", "3", "3"]):
            with self.subTest(order=order):
                plan = copy.deepcopy(self.plan)
                plan["execution_order"] = order
                self.check_plan(plan, "every change exactly once")

    def test_unexpected_root_mutation(self):
        self.plan["changes"][2]["resource_type"] = "ai_gateway"
        self.check_plan(self.plan, "unexpectedly changes the root")

    def test_missing_policy_delete(self):
        self.plan["changes"].pop()
        self.plan["summary"]["total_changes"] = 4
        self.check_plan(self.plan, "unexpected actions")

    def test_event_gateway_cascade_requires_separate_groups(self):
        plan = {
            "metadata": {"mode": "sync"}, "summary": {"total_changes": 2},
            "changes": [
                {"id": "vc", "action": "DELETE", "resource_type": "event_gateway_virtual_cluster",
                 "namespace": "smoke"},
                {"id": "bc", "action": "DELETE", "resource_type": "event_gateway_backend_cluster",
                 "namespace": "smoke"},
            ],
            "execution_order": ["vc", "bc"], "execution_groups": [["vc"], ["bc"]],
        }
        self.check_plan(plan, resource="event_gateway", stage="prune")
        plan["execution_groups"] = [["vc", "bc"]]
        self.check_plan(plan, "unsafe concurrent execution", resource="event_gateway", stage="prune")
        plan["execution_order"] = ["bc", "vc"]
        self.check_plan(plan, "unsafe dependency order", resource="event_gateway", stage="prune")


if __name__ == "__main__":
    unittest.main()

import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("routing", Path(__file__).with_name("e2e-replay-routing.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class RoutingTest(unittest.TestCase):
    def test_workflows_guard_replay_and_force_live_status(self):
        workflows = MODULE.REPLAY.ROOT / ".github/workflows"
        suite = (workflows / "e2e.yaml").read_text()
        status = (workflows / "e2e-required-status.yaml").read_text()
        replay_job = suite.split("  e2e-replay:", 1)[1].split("  e2e:", 1)[0]
        self.assertIn("github.event_name == 'pull_request'", replay_job)
        self.assertIn("needs.e2e-build.outputs.replay_mode == 'pr'", replay_job)
        self.assertIn("ref: ${{ github.sha }}", replay_job)
        self.assertNotIn("needs.e2e-needed.outputs.checkout_ref", replay_job)
        self.assertNotIn("needs.e2e-needed.outputs.checkout_repository", replay_job)
        self.assertIn("python3 .e2e-verifier-tools/scripts/e2e-replay-routing.py verify", suite)
        self.assertIn('--root "$GITHUB_WORKSPACE"', suite)
        self.assertIn("currentPR.labels.some(label => label.name === 'e2e:force-live')", suite)
        self.assertIn("github.event.label.name == 'e2e:force-live'", status)
        self.assertIn("- labeled", status)
        self.assertIn("- unlabeled", status)

    def test_main_is_all_live_and_pr_is_an_exact_partition(self):
        live = MODULE.make_plan(MODULE.REPLAY.ROOT, "live")
        pr = MODULE.make_plan(MODULE.REPLAY.ROOT, "pr")
        self.assertEqual([], live["replay"])
        self.assertEqual(live["live"], sorted(pr["live"] + pr["replay"]))
        self.assertFalse(set(pr["live"]) & set(pr["replay"]))
        self.assertIn("control-plane/get/scenario.yaml", pr["replay"])

    def test_stale_cassette_fails_pr_but_does_not_block_live(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            scenario = root / "test/e2e/scenarios/control-plane/get"
            scenario.mkdir(parents=True)
            (scenario / "scenario.yaml").write_text("baseInputsPath: testdata\n")
            policy = root / "test/e2e/replay-scenarios.json"
            policy.write_text(json.dumps({"schema_version": 1, "scenarios": ["control-plane/get"]}))
            self.assertEqual([], MODULE.make_plan(root, "live")["replay"])
            with self.assertRaises(FileNotFoundError):
                MODULE.make_plan(root, "pr")

    def test_replay_results_are_complete_isolated_and_bound_to_run(self):
        plan = {"replay": ["control-plane/get/scenario.yaml"]}
        report = {"commit": "abc", "run_id": "123", "run_attempt": 1, "scenarios": [{
            "scenario": "control-plane/get", "mode": "replay", "status": "pass",
            "network_isolated": True, "source_kind": "recorded", "interactions": 11,
        }]}
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with self.assertRaisesRegex(ValueError, "missing"):
                MODULE.verify_results(plan, root, "abc", "123")
            path = root / "replay-results.json"
            path.write_text(json.dumps(report))
            MODULE.verify_results(plan, root, "abc", "123")
            for field, value in [("network_isolated", False), ("status", "skip"), ("source_kind", "bootstrap")]:
                invalid = copy.deepcopy(report)
                invalid["scenarios"][0][field] = value
                path.write_text(json.dumps(invalid))
                with self.subTest(field=field), self.assertRaises(ValueError):
                    MODULE.verify_results(plan, root, "abc", "123")
            for summaries in [[], report["scenarios"] * 2]:
                path.write_text(json.dumps({**report, "scenarios": summaries}))
                with self.assertRaisesRegex(ValueError, "coverage"):
                    MODULE.verify_results(plan, root, "abc", "123")
            path.write_text(json.dumps(report))
            with self.assertRaisesRegex(ValueError, "another execution"):
                MODULE.verify_results(plan, root, "other", "123")
            with self.assertRaisesRegex(ValueError, "all-live"):
                MODULE.verify_results({"replay": []}, root, "abc", "123")


if __name__ == "__main__":
    unittest.main()

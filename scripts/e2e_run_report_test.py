"""Regression coverage for incomplete runs, attempt selection, timings and redaction."""
import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from e2e_run_report import aggregate, collect, http_events, render, write


class RunReportTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.metadata = {"run": {"id": 12, "html_url": "https://github.com/Kong/kongctl/actions/runs/12",
                                 "head_sha": "abc", "created_at": "2026-09-19T00:00:00Z"},
                         "jobs": [{"id": 1, "name": "Run scenarios — org", "run_attempt": 1,
                                   "started_at": "2026-09-19T00:01:00Z",
                                   "completed_at": "2026-09-19T00:02:00Z", "conclusion": "success"}],
                         "artifacts": []}
        self.needs = {"e2e-needed": {"result": "success", "outputs": {"required": "true", "run_shards": "true",
                      "orgs_json": '[{"org_name":"org"}]'}}, "e2e-verify": {"result": "success"}}
        self.shard = {"run_id": 12, "org_name": "org", "run_attempt": 1,
                      "started_at": 1789776070, "finished_at": 1789776100,
                      "execution_duration_seconds": 30, "selected_scenario_count": 1,
                      "results": {"passed": 1, "failed": 0, "skipped": 0, "beta_failed": 0},
                      "scenario_durations": [{"scenario": "example", "result": "pass", "duration_seconds": 30}],
                      "events": [], "evidence_warnings": []}
        write(self.root / "shard/e2e-report-shard.json", self.shard)
        write(self.root / "e2e-routing.json", {"live": ["example"], "replay": []})

    def report(self):
        return aggregate(self.root, self.metadata, self.needs)

    def test_wall_time_not_sum_and_queue_separate(self):
        second = dict(self.shard, org_name="org2", started_at=self.shard["started_at"] + 5,
                      finished_at=self.shard["finished_at"] + 5)
        write(self.root / "second/e2e-report-shard.json", second)
        write(self.root / "e2e-routing.json", {"live": ["example", "other"], "replay": []})
        original_rglob = Path.rglob
        for reverse in (False, True):
            with self.subTest(reverse=reverse):
                def ordered_rglob(path, pattern):
                    return iter(sorted(original_rglob(path, pattern), key=str, reverse=reverse))

                with patch.object(Path, "rglob", ordered_rglob):
                    report = self.report()
                self.assertEqual("PASSED", report["state"])
                self.assertEqual(35, report["live_window_seconds"])
                self.assertEqual(60, report["initial_queue_seconds"])
                shards = {shard["org_name"]: shard for shard in report["shards"]}
                self.assertEqual(60, shards["org"]["job_seconds"])
                self.assertIsNone(shards["org2"]["job_seconds"])
                self.assertIn("Recorded replay", render(report))

    def test_missing_new_attempt_never_reuses_old_success(self):
        self.metadata["jobs"][0]["run_attempt"] = 2
        report = self.report()
        self.assertEqual("INCOMPLETE", report["state"])
        self.assertEqual(0, report["live_counts"]["passed"])
        self.assertTrue(report["missing_shards"])
        self.assertIsNone(report["live_window_seconds"])

    def test_latest_report_replaces_old_without_double_counting(self):
        second = copy.deepcopy(self.shard)
        second.update(run_attempt=2, events=[{"scenario": "example", "class": "timeout", "operation": "Reset list",
                                            "duration_ms": 15000, "outcome": "recovered", "evidence": "observation.json"}])
        write(self.root / "retry/e2e-report-shard.json", second)
        report = self.report()
        self.assertEqual(1, report["live_counts"]["passed"])
        self.assertEqual(1, len(report["shards"][0]["events"]))
        self.assertIn("15.000s", render(report))

    def test_missing_and_failed_jobs(self):
        (self.root / "shard/e2e-report-shard.json").unlink()
        self.assertEqual("INCOMPLETE", self.report()["state"])
        self.needs["e2e-verify"]["result"] = "failure"
        self.assertEqual("FAILED", self.report()["state"])
        self.needs["e2e-verify"]["result"] = "cancelled"
        self.assertEqual("CANCELLED / INCOMPLETE", self.report()["state"])

    def test_replay_missing_then_present(self):
        write(self.root / "e2e-routing.json", {"live": ["example"], "replay": ["replayed"]})
        self.assertEqual("INCOMPLETE", self.report()["state"])
        write(self.root / "replay-results.json", {"run_id": "12", "run_attempt": 1,
                                                  "scenarios": [{"scenario": "replayed", "status": "pass"}]})
        self.assertEqual("PASSED", self.report()["state"])
        self.assertEqual(1, self.report()["replay_counts"]["passed"])
        self.metadata["jobs"].append({"id": 2, "name": "Replay approved scenarios", "run_attempt": 2})
        self.assertEqual("INCOMPLETE", self.report()["state"])

    def test_advisories_are_not_clean_success(self):
        self.shard["results"].update(passed=0, beta_failed=1)
        write(self.root / "shard/e2e-report-shard.json", self.shard)
        self.assertEqual("PASSED WITH ADVISORIES", self.report()["state"])

    def test_http_events_drop_secrets_and_avoid_retry_double_count(self):
        log = self.root / "kongctl.log"
        log.write_text('msg="log_type=http_error" log_type=http_response\n'
                       'log_type=http_error method=GET duration=15.005s error="Get https://x?token=SECRET: timeout"\n'
                       'log_type=http_retry event=retry_attempt error="SECRET timeout"\n'
                       'log_type=http_retry event=retry_attempt status_code=503 method=POST\n')
        events = http_events(log)
        self.assertEqual(2, len(events))
        self.assertEqual(15005, events[0]["duration_ms"])
        self.assertNotIn("SECRET", json.dumps(events))
        self.assertNotIn("https://x", json.dumps(events))

    def test_structured_reset_events_deduplicate_preserved_artifacts(self):
        scenario = self.root / "scenario"
        write(scenario / "scenario-diagnostics.json", {"scenario": "test/e2e/scenarios/example/scenario.yaml",
                                                       "commands": []})
        observation = {"type": "reset_summary", "details": [{"endpoint": "apis", "error": "SECRET",
            "events": [{"timestamp": "2026-09-19T00:00:01Z", "operation": "list", "attempt": 1,
                        "duration_ms": 15000, "class": "timeout", "outcome": "recovered"}]}]}
        write(scenario / "commands/reset/observation.json", observation)
        write(scenario / "commands/reset/attempts/000/observation.json", observation)
        (self.root / "scenario-results.txt").write_text("exit_code=0\npassed_count=1\n")
        with patch("e2e_run_report.collect_metrics", return_value=copy.deepcopy(self.shard)):
            data = collect(self.root, {})
        self.assertEqual(1, len(data["events"]))
        self.assertEqual("example", data["events"][0]["scenario"])
        self.assertEqual("recovered", data["events"][0]["outcome"])
        self.assertNotIn("SECRET", json.dumps(data))

    def test_table_escapes_artifact_text(self):
        self.shard["org_name"] = "<script>|[click](evil)"
        write(self.root / "shard/e2e-report-shard.json", self.shard)
        markdown = render(self.report())
        self.assertNotIn("<script>", markdown)
        self.assertNotIn("[click](evil)", markdown)

    def test_malformed_artifact_preserves_other_evidence(self):
        (self.root / "e2e-routing.json").write_text("{broken")
        (self.root / "replay-results.json").write_text("null")
        report = self.report()
        self.assertEqual("INCOMPLETE", report["state"])
        self.assertEqual(1, report["live_counts"]["passed"])
        self.assertIn("Unreadable routing plan", report["warnings"])
        self.assertIn("Unreadable replay results", report["warnings"])

    def test_missing_finish_timestamp_keeps_runtime_unknown(self):
        self.shard["finished_at"] = None
        write(self.root / "shard/e2e-report-shard.json", self.shard)
        self.assertIsNone(self.report()["live_window_seconds"])

    def test_not_required_and_trusted_run_states(self):
        self.needs["e2e-needed"]["outputs"]["required"] = "false"
        self.assertEqual("NOT REQUIRED", self.report()["state"])
        self.needs["e2e-needed"]["outputs"].update(required="true", run_shards="false",
                                                trusted_e2e_required="true", status_sha="reviewed-head")
        self.assertEqual("AWAITING TRUSTED RUN", self.report()["state"])
        markdown = render(self.report())
        self.assertIn("fork PR code cannot receive E2E secrets automatically", markdown)
        self.assertIn("Reviewed SHA required: `reviewed-head`", markdown)

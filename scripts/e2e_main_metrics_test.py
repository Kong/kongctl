import unittest

from e2e_main_metrics import measure


class MainMetricsTests(unittest.TestCase):
    def test_overlap_is_intersection_of_main_use_and_pr_queue(self):
        runs = [
            {"id": 1, "display_title": "main · scenarios · overnight · sha · ticket",
             "status": "completed", "updated_at": "2026-11-20T08:00:00Z"},
            {"id": 2, "display_title": "readiness:12/sha/com/pr/123", "status": "completed",
             "created_at": "2026-11-20T07:20:00Z"},
            {"id": 3, "display_title": "PR #13 · trusted scenarios · sha", "status": "completed",
             "created_at": "2026-11-20T08:10:00Z"},
        ]
        jobs = {run_id: [{"name": "Determine scenario requirement", "started_at": start}]
                for run_id, start in [(1, "2026-11-20T07:00:00Z"),
                                      (2, "2026-11-20T08:05:00Z"), (3, "2026-11-20T08:11:00Z")]}
        report = measure(runs, jobs)
        self.assertEqual(report["overlapping_pr_runs"], 1)
        self.assertEqual(report["queue_overlap_seconds"], 40 * 60)
        self.assertEqual(report["overlaps"][0]["pr_run"], 2)

    def test_missing_admission_and_reruns_are_not_fabricated_measurements(self):
        runs = [{"id": 1, "run_attempt": 2}, {"id": 2}]
        self.assertEqual(measure(runs, {2: []})["pr_runs"], 0)


if __name__ == "__main__":
    unittest.main()

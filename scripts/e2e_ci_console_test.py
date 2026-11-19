import unittest

import e2e_ci_console as console


class TransformLinesTest(unittest.TestCase):
    def test_folds_passing_scenario_and_drops_heartbeats(self):
        lines = [
            "=== RUN   Test_Scenarios",
            "=== RUN   Test_Scenarios/test/e2e/scenarios/adopt/team-create-adopt-dump/scenario.yaml",
            "[0/43] elapsed 0s · current: adopt/team-create-adopt-dump",
            "[0/43] elapsed 30s · current: adopt/team-create-adopt-dump",
            "    scenario_test.go:10: doing a thing",
            "--- PASS: Test_Scenarios/test/e2e/scenarios/adopt/team-create-adopt-dump/scenario.yaml (12.34s)",
            "--- PASS: Test_Scenarios (12.50s)",
            "PASS",
        ]
        out = list(console.transform_lines(lines))
        self.assertEqual(out, [
            "=== RUN   Test_Scenarios",
            "::group::adopt/team-create-adopt-dump",
            "=== RUN   Test_Scenarios/test/e2e/scenarios/adopt/team-create-adopt-dump/scenario.yaml",
            "    scenario_test.go:10: doing a thing",
            "::endgroup::",
            "--- PASS: Test_Scenarios/test/e2e/scenarios/adopt/team-create-adopt-dump/scenario.yaml (12.34s)",
            "--- PASS: Test_Scenarios (12.50s)",
            "PASS",
        ])

    def test_failed_scenario_emits_annotation_with_failure_detail_and_stays_unfolded(self):
        lines = [
            "=== RUN   Test_Scenarios/test/e2e/scenarios/identity/declarative-basic-auth/scenario.yaml",
            "    scenarios_test.go:147: scenario failed: command verify-lifecycle external "
            "execution failed: exit status 1",
            "--- FAIL: Test_Scenarios/test/e2e/scenarios/identity/declarative-basic-auth/scenario.yaml (5.00s)",
        ]
        out = list(console.transform_lines(lines))
        self.assertEqual(out, [
            "::group::identity/declarative-basic-auth",
            "=== RUN   Test_Scenarios/test/e2e/scenarios/identity/declarative-basic-auth/scenario.yaml",
            "    scenarios_test.go:147: scenario failed: command verify-lifecycle external "
            "execution failed: exit status 1",
            "::endgroup::",
            "::error::identity/declarative-basic-auth: scenarios_test.go:147: scenario failed: "
            "command verify-lifecycle external execution failed: exit status 1",
            "--- FAIL: Test_Scenarios/test/e2e/scenarios/identity/declarative-basic-auth/"
            "scenario.yaml (5.00s)",
        ])

    def test_second_scenario_opens_group_even_if_first_left_unclosed(self):
        lines = [
            "=== RUN   Test_Scenarios/a/scenario.yaml",
            "panic: boom",
            "=== RUN   Test_Scenarios/b/scenario.yaml",
            "--- PASS: Test_Scenarios/b/scenario.yaml (1.00s)",
        ]
        out = list(console.transform_lines(lines))
        self.assertIn("::error::panic: boom", out)
        self.assertEqual(out.count("::group::a"), 1)
        self.assertEqual(out.count("::group::b"), 1)

    def test_unterminated_group_closes_at_end_of_stream(self):
        lines = ["=== RUN   Test_Scenarios/a/scenario.yaml", "still going"]
        out = list(console.transform_lines(lines))
        self.assertEqual(out[-1], "::endgroup::")

    def test_no_non_heartbeat_lines_are_dropped(self):
        lines = [
            "=== RUN   Test_Scenarios/a/scenario.yaml",
            "[0/1] elapsed 0s · current: a",
            "detail one",
            "detail two",
            "--- PASS: Test_Scenarios/a/scenario.yaml (0.10s)",
        ]
        out = list(console.transform_lines(lines))
        self.assertIn("detail one", out)
        self.assertIn("detail two", out)
        self.assertNotIn("[0/1] elapsed 0s · current: a", out)

    def test_scenario_display_name_strips_known_prefixes_and_suffix(self):
        self.assertEqual(
            console.scenario_display_name("test/e2e/scenarios/adopt/team-create-adopt-dump/scenario.yaml"),
            "adopt/team-create-adopt-dump",
        )
        self.assertEqual(
            console.scenario_display_name("scenarios/apis/basic/scenario.yaml"),
            "apis/basic",
        )

    def test_escape_annotation_encodes_percent_and_newlines(self):
        self.assertEqual(console.escape_annotation("100% done\r\n"), "100%25 done%0D%0A")


if __name__ == "__main__":
    unittest.main()

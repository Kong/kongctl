package main

import (
	"strings"
	"testing"
)

const (
	scenarioOneTest   = "Test_Scenarios/scenarios/identity/one/scenario.yaml"
	scenarioTwoTest   = "Test_Scenarios/scenarios/identity/two/scenario.yaml"
	scenarioThreeTest = "Test_Scenarios/scenarios/identity/three/scenario.yaml"
)

// decodeFixture runs the given newline-delimited test2json events through the
// real decoder into an aggregator and returns every update produced, in order.
func decodeFixture(t *testing.T, agg *aggregator, lines string) []update {
	t.Helper()
	var all []update
	err := decodeEvents(strings.NewReader(lines), func(ev testEvent) {
		all = append(all, agg.apply(ev)...)
	})
	if err != nil {
		t.Fatalf("decodeEvents: %v", err)
	}
	return all
}

func TestScenarioNameStripsPrefixesAndSuffix(t *testing.T) {
	cases := []struct {
		test string
		want string
		ok   bool
	}{
		{"Test_Scenarios/scenarios/identity/groups/scenario.yaml", "identity/groups", true},
		{"Test_Scenarios/test/e2e/scenarios/apis/basic/scenario.yaml", "apis/basic", true},
		{"Test_Scenarios", "", false},
		{scenarioOneTest + "/attempt-1", "", false},
		{"TestOther/foo", "", false},
	}
	for _, tc := range cases {
		got, ok := scenarioName(tc.test)
		if ok != tc.ok || got != tc.want {
			t.Errorf("scenarioName(%q) = (%q, %v), want (%q, %v)", tc.test, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAggregatorSentinelTotal(t *testing.T) {
	agg := newAggregator()
	lines := events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "2\n"},
	)
	updates := decodeFixture(t, agg, lines)
	if !agg.totalKnown || agg.total != 2 {
		t.Fatalf("expected total known = 2, got known=%v total=%d", agg.totalKnown, agg.total)
	}
	var sawTotal bool
	for _, u := range updates {
		if u.kind == updateTotalKnown && u.total == 2 {
			sawTotal = true
		}
	}
	if !sawTotal {
		t.Fatalf("expected an updateTotalKnown update, got %+v", updates)
	}
}

func TestAggregatorPassFailSkip(t *testing.T) {
	agg := newAggregator()
	lines := events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "3\n"},

		testEvent{Action: "run", Test: scenarioOneTest},
		testEvent{Action: "output", Test: scenarioOneTest, Output: "--- PASS: " + scenarioOneTest + " (1.50s)\n"},
		testEvent{Action: "pass", Test: scenarioOneTest, Elapsed: 1.5},

		testEvent{Action: "run", Test: scenarioTwoTest},
		testEvent{Action: "output", Test: scenarioTwoTest, Output: "    run.go:58: beta disabled\n"},
		testEvent{Action: "output", Test: scenarioTwoTest, Output: "--- SKIP: " + scenarioTwoTest + " (0.00s)\n"},
		testEvent{Action: "skip", Test: scenarioTwoTest, Elapsed: 0},

		testEvent{Action: "run", Test: scenarioThreeTest},
		testEvent{Action: "output", Test: scenarioThreeTest, Output: "    run.go:120: step 4: expected exit code 0, got 1\n"},
		testEvent{Action: "output", Test: scenarioThreeTest, Output: "--- FAIL: " + scenarioThreeTest + " (23.39s)\n"},
		testEvent{Action: "fail", Test: scenarioThreeTest, Elapsed: 23.39},

		testEvent{Action: "output", Test: topLevelTest, Output: "--- FAIL: " + topLevelTest + " (24.89s)\n"},
		testEvent{Action: "fail", Test: topLevelTest, Elapsed: 24.89},
	)
	updates := decodeFixture(t, agg, lines)

	if agg.passed != 1 || agg.skipped != 1 || agg.failed != 1 || agg.completed != 3 {
		t.Fatalf("unexpected counts: %+v", agg.snapshotCounts())
	}
	if len(agg.failedList) != 1 || agg.failedList[0].name != "identity/three" {
		t.Fatalf("unexpected failedList: %+v", agg.failedList)
	}
	if !agg.runDone || !agg.elapsedKnown {
		t.Fatalf("expected run to be marked done with known elapsed")
	}

	var skipUpdate, failUpdate, runFinished update
	for _, u := range updates {
		switch {
		case u.kind == updateScenarioFinished && u.result.outcome == outcomeSkip:
			skipUpdate = u
		case u.kind == updateScenarioFinished && u.result.outcome == outcomeFail:
			failUpdate = u
		case u.kind == updateRunFinished:
			runFinished = u
		}
	}
	if skipUpdate.result.detail != "run.go:58: beta disabled" {
		t.Fatalf("expected skip reason %q, got %q", "run.go:58: beta disabled", skipUpdate.result.detail)
	}
	wantFailDetail := "run.go:120: step 4: expected exit code 0, got 1"
	if failUpdate.result.detail != wantFailDetail {
		t.Fatalf("expected failure detail %q, got %q", wantFailDetail, failUpdate.result.detail)
	}
	if runFinished.result.outcome != outcomeFail {
		t.Fatalf("expected top-level run outcome fail, got %v", runFinished.result.outcome)
	}

	summary := agg.summary("/tmp/artifacts")
	if len(summary.slowest) != 2 {
		t.Fatalf("expected slowest to keep at most 2, got %d", len(summary.slowest))
	}
	if summary.slowest[0].name != "identity/three" {
		t.Fatalf("expected slowest[0] to be the 23.39s scenario, got %+v", summary.slowest[0])
	}
}

func TestAggregatorBuildFail(t *testing.T) {
	agg := newAggregator()
	lines := events(
		t,
		testEvent{Action: "build-output", Output: "# t2jcheck\n"},
		testEvent{Action: "build-output", Output: "./x_test.go:5:2: undefined: foo\n"},
		testEvent{Action: "build-fail"},
	)
	updates := decodeFixture(t, agg, lines)
	if !agg.runDone {
		t.Fatalf("expected build-fail to mark the run done")
	}
	var found bool
	for _, u := range updates {
		if u.kind == updateRunFinished && u.abnormal {
			found = true
			if !strings.Contains(u.result.detail, "undefined: foo") {
				t.Fatalf("expected build failure detail to include compiler output, got %q", u.result.detail)
			}
		}
	}
	if !found {
		t.Fatalf("expected an abnormal updateRunFinished, got %+v", updates)
	}
}

func TestAggregatorZeroScenariosSkipsTopLevel(t *testing.T) {
	agg := newAggregator()
	lines := events(
		t,
		testEvent{Action: "run", Test: topLevelTest},
		testEvent{Action: "output", Test: topLevelTest, Output: sentinelPrefix + "0\n"},
		testEvent{Action: "output", Test: topLevelTest, Output: "    scenarios_test.go:112: no scenarios found\n"},
		testEvent{Action: "output", Test: topLevelTest, Output: "--- SKIP: " + topLevelTest + " (0.00s)\n"},
		testEvent{Action: "skip", Test: topLevelTest, Elapsed: 0},
	)
	updates := decodeFixture(t, agg, lines)
	if agg.total != 0 || !agg.totalKnown {
		t.Fatalf("expected total known = 0, got known=%v total=%d", agg.totalKnown, agg.total)
	}
	if agg.completed != 0 {
		t.Fatalf("expected no scenarios to have run, got completed=%d", agg.completed)
	}

	var runFinished update
	for _, u := range updates {
		if u.kind == updateRunFinished {
			runFinished = u
		}
	}
	wantDetail := "scenarios_test.go:112: no scenarios found"
	if runFinished.result.outcome != outcomeSkip || runFinished.result.detail != wantDetail {
		t.Fatalf("expected top-level skip with reason, got %+v", runFinished.result)
	}
}

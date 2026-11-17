package main

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

// topLevelTest is the name go test assigns to Test_Scenarios itself, as opposed
// to one of its scenario subtests (e.g. "Test_Scenarios/scenarios/identity/groups/scenario.yaml").
const topLevelTest = "Test_Scenarios"

// testEvent mirrors the JSON shape cmd/test2json emits for `go test -json`.
type testEvent struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// decodeEvents reads newline-delimited test2json events from r, invoking fn for
// each one in order, until EOF or a decode error.
func decodeEvents(r io.Reader, fn func(testEvent)) error {
	dec := json.NewDecoder(r)
	for {
		var ev testEvent
		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		fn(ev)
	}
}

// scenarioName derives the compact scenario name (e.g. "identity/groups") from a
// test2json Test field, mirroring progressScenarioName/normalizeScenarioPath in
// the sibling test/e2e package. It reports ok=false for the top-level
// Test_Scenarios test and anything else that isn't a scenario subtest.
func scenarioName(test string) (string, bool) {
	const prefix = topLevelTest + "/"
	path, ok := strings.CutPrefix(test, prefix)
	if !ok {
		return "", false
	}
	path = strings.TrimPrefix(path, "scenarios/")
	path = strings.TrimPrefix(path, "test/e2e/scenarios/")
	path = strings.TrimSuffix(path, "/scenario.yaml")
	return path, true
}

// durationFromSeconds converts a test2json Elapsed value (fractional seconds) to
// a time.Duration.
func durationFromSeconds(seconds float64) time.Duration {
	if seconds < 0 {
		seconds = 0
	}
	return time.Duration(seconds * float64(time.Second))
}

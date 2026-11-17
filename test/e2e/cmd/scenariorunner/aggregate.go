package main

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// sentinelPrefix marks the stdout line scenarios_test.go prints with the number
// of selected scenarios, before any scenario subtest has started.
const sentinelPrefix = "##KONGCTL_E2E_TOTAL="

type outcome int

const (
	outcomePass outcome = iota
	outcomeSkip
	outcomeFail
)

func (o outcome) label() string {
	switch o {
	case outcomePass:
		return "PASS"
	case outcomeSkip:
		return "SKIP"
	case outcomeFail:
		return "FAIL"
	default:
		return "?"
	}
}

// scenarioResult describes one finished scenario subtest.
type scenarioResult struct {
	name    string
	outcome outcome
	elapsed time.Duration
	// detail holds the skip reason or failure diagnostics, derived from the
	// subtest's buffered -v output.
	detail string
}

type updateKind int

const (
	updateTotalKnown updateKind = iota
	updateRunStarted
	updateScenarioStarted
	updateScenarioFinished
	updateRunFinished
)

// counts is a snapshot of aggregate progress, attached to updates so the
// renderer never needs to reach back into the aggregator.
type counts struct {
	total, completed, passed, skipped, failed int
}

// update describes one semantic change in the aggregator's state produced by
// a single test2json event.
type update struct {
	kind    updateKind
	total   int
	name    string
	result  scenarioResult
	elapsed time.Duration // total run elapsed, for updateRunFinished
	counts  counts
	// abnormal marks an updateRunFinished caused by a build failure rather than
	// Test_Scenarios completing normally.
	abnormal bool
}

// aggregator turns a stream of test2json events into semantic updates and
// tracks the running totals/failed/slowest lists needed for the final summary.
type aggregator struct {
	total      int
	totalKnown bool
	completed  int
	passed     int
	skipped    int
	failed     int

	failedList []scenarioResult
	slowest    []scenarioResult

	runStarted   bool
	runDone      bool
	totalElapsed time.Duration
	elapsedKnown bool

	outputBuf map[string][]string
}

func newAggregator() *aggregator {
	return &aggregator{outputBuf: make(map[string][]string)}
}

func (a *aggregator) snapshotCounts() counts {
	return counts{total: a.total, completed: a.completed, passed: a.passed, skipped: a.skipped, failed: a.failed}
}

// apply processes one test2json event, returning zero or more updates.
func (a *aggregator) apply(ev testEvent) []update {
	switch ev.Action {
	case "output", "build-output":
		return a.applyOutput(ev)
	case "run":
		return a.applyRun(ev)
	case "pass", "fail", "skip":
		return a.applyDone(ev)
	case "build-fail":
		a.runDone = true
		return []update{{kind: updateRunFinished, abnormal: true, counts: a.snapshotCounts(), result: scenarioResult{
			outcome: outcomeFail,
			detail:  strings.Join(a.outputBuf[""], ""),
		}}}
	default:
		return nil
	}
}

func (a *aggregator) applyOutput(ev testEvent) []update {
	if (ev.Test == topLevelTest || ev.Test == "") && !a.totalKnown {
		if line, ok := strings.CutPrefix(strings.TrimRight(ev.Output, "\n"), sentinelPrefix); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
				a.total = n
				a.totalKnown = true
				return []update{{kind: updateTotalKnown, total: n, counts: a.snapshotCounts()}}
			}
		}
	}
	a.outputBuf[ev.Test] = append(a.outputBuf[ev.Test], ev.Output)
	return nil
}

func (a *aggregator) applyRun(ev testEvent) []update {
	if name, ok := scenarioName(ev.Test); ok {
		return []update{{kind: updateScenarioStarted, name: name, counts: a.snapshotCounts()}}
	}
	if ev.Test == topLevelTest {
		a.runStarted = true
		return []update{{kind: updateRunStarted, counts: a.snapshotCounts()}}
	}
	return nil
}

func (a *aggregator) applyDone(ev testEvent) []update {
	if name, ok := scenarioName(ev.Test); ok {
		res := scenarioResult{
			name:    name,
			elapsed: durationFromSeconds(ev.Elapsed),
			detail:  extractDetail(ev.Action, a.outputBuf[ev.Test]),
		}
		switch ev.Action {
		case "pass":
			res.outcome = outcomePass
			a.passed++
		case "skip":
			res.outcome = outcomeSkip
			a.skipped++
		case "fail":
			res.outcome = outcomeFail
			a.failed++
			a.failedList = append(a.failedList, res)
		}
		a.completed++
		delete(a.outputBuf, ev.Test)
		a.recordSlowest(res)
		return []update{{kind: updateScenarioFinished, result: res, counts: a.snapshotCounts()}}
	}
	if ev.Test == topLevelTest {
		a.runDone = true
		res := scenarioResult{elapsed: durationFromSeconds(ev.Elapsed)}
		switch ev.Action {
		case "pass":
			res.outcome = outcomePass
		case "skip":
			res.outcome = outcomeSkip
			res.detail = extractDetail(ev.Action, a.outputBuf[topLevelTest])
		case "fail":
			res.outcome = outcomeFail
			res.detail = extractDetail(ev.Action, a.outputBuf[topLevelTest])
		}
		delete(a.outputBuf, topLevelTest)
		a.totalElapsed = res.elapsed
		a.elapsedKnown = true
		return []update{{kind: updateRunFinished, elapsed: res.elapsed, result: res, counts: a.snapshotCounts()}}
	}
	return nil
}

// summary builds the final report data from the aggregator's accumulated
// state, for Renderer.Finish.
func (a *aggregator) summary(artifactsDir string) summary {
	return summary{
		counts:       a.snapshotCounts(),
		totalElapsed: a.totalElapsed,
		elapsedKnown: a.elapsedKnown,
		failedList:   a.failedList,
		slowest:      a.slowest,
		artifactsDir: artifactsDir,
	}
}

// recordSlowest keeps the two slowest finished scenarios, sorted descending by
// elapsed duration.
func (a *aggregator) recordSlowest(res scenarioResult) {
	const keep = 2
	a.slowest = append(a.slowest, res)
	slices.SortFunc(a.slowest, func(x, y scenarioResult) int {
		return cmp.Compare(y.elapsed, x.elapsed)
	})
	if len(a.slowest) > keep {
		a.slowest = a.slowest[:keep]
	}
}

// extractDetail returns the lines of buffered -v output worth surfacing for a
// finished subtest: the skip reason for a skip, or the failure body for a fail.
// Lines matching go test's own "=== RUN"/"--- PASS|FAIL|SKIP" framing are
// dropped since the renderer already reports outcome and duration itself.
func extractDetail(action string, buffered []string) string {
	if action != "fail" && action != "skip" {
		return ""
	}
	var lines []string
	for _, raw := range buffered {
		for line := range strings.Lines(raw) {
			line = strings.TrimRight(line, "\n")
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "=== RUN") || strings.HasPrefix(trimmed, "--- PASS:") ||
				strings.HasPrefix(trimmed, "--- FAIL:") || strings.HasPrefix(trimmed, "--- SKIP:") {
				continue
			}
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

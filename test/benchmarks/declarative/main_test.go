//go:build e2e

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/declarative/loader"
)

func TestSelectCasesAliases(t *testing.T) {
	t.Parallel()

	cases, err := selectCases("medium-single")
	if err != nil {
		t.Fatalf("selectCases() error = %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}
	if cases[0].Name != "medium-single-file" {
		t.Fatalf("case name = %q, want medium-single-file", cases[0].Name)
	}

	cases, err = selectCases("small")
	if err != nil {
		t.Fatalf("selectCases() error = %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("len(cases) = %d, want 2", len(cases))
	}
}

func TestParseHTTPLog(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kongctl.log")
	log := stringsJoinLines(
		`time=now level=debug log_type=http_request method=GET route=/v2/apis`,
		`time=now level=debug log_type=http_request method=POST route=/v2/apis`,
		`time=now level=debug log_type=http_response method=GET route=/v2/apis status_code=200 duration=10ms`,
		`time=now level=debug log_type=http_error method=POST route=/v2/apis duration=2s`,
	)
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}

	metrics, err := parseHTTPLog(path)
	if err != nil {
		t.Fatalf("parseHTTPLog() error = %v", err)
	}
	if metrics.Requests != 2 {
		t.Fatalf("requests = %d, want 2", metrics.Requests)
	}
	if metrics.Responses != 1 {
		t.Fatalf("responses = %d, want 1", metrics.Responses)
	}
	if metrics.Errors != 1 {
		t.Fatalf("errors = %d, want 1", metrics.Errors)
	}
	if metrics.Timing.Combined.Count != 2 {
		t.Fatalf("combined timing count = %d, want 2", metrics.Timing.Combined.Count)
	}
	if metrics.Timing.Combined.SumMS != 2010 {
		t.Fatalf("combined timing sum = %f, want 2010", metrics.Timing.Combined.SumMS)
	}
}

func TestRenderTerminalSummary(t *testing.T) {
	t.Parallel()

	suite := suiteResult{
		SchemaVersion: schemaVersion,
		RunID:         "run-1",
		GitCommit:     "abc123",
		BaseURL:       "https://example.test",
		Summary: suiteSummary{
			CaseCount:       1,
			PhaseCount:      1,
			TotalRequests:   6,
			TotalResponses:  6,
			TotalHTTPErrors: 0,
		},
		Cases: []caseResult{
			{
				Name: "small-single-file",
				Resources: resourceCounts{
					APIs:         1,
					APIDocuments: 2,
				},
				Phases: []phaseResult{
					{
						Name:       "apply_create",
						DurationMS: 702,
						HTTPMetrics: httpMetrics{
							Requests:  6,
							Responses: 6,
						},
					},
				},
			},
		},
	}

	summary := renderTerminalSummary(suite)
	for _, want := range []string{
		"Declarative benchmark summary",
		"Suite:",
		"CASE",
		"small-single-file",
		"apply_create",
		"702ms",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("terminal summary missing %q:\n%s", want, summary)
		}
	}
}

func TestBuildHistoryReportDetectsRequestRegression(t *testing.T) {
	t.Parallel()

	historyDir := filepath.Join(t.TempDir(), "history")
	previous := testSuiteResult("previous", 10, 1000, 0)
	if err := writeJSON(filepath.Join(historyDir, "runs", "previous", "results.json"), previous); err != nil {
		t.Fatal(err)
	}

	current := testSuiteResult("current", 12, 1050, 0)
	report, err := buildHistoryReport(config{
		HistoryDir:            historyDir,
		MinHistorySamples:     1,
		RequestCountThreshold: 0.05,
		DurationThreshold:     0.50,
	}, current)
	if err != nil {
		t.Fatalf("buildHistoryReport() error = %v", err)
	}
	if !report.HasRegressions {
		t.Fatalf("HasRegressions = false, want true")
	}
	if len(report.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(report.Rows))
	}
	if !report.Rows[0].RequestRegression {
		t.Fatalf("RequestRegression = false, want true")
	}
}

func TestBuildHistoryReportRequiresMinimumHistory(t *testing.T) {
	t.Parallel()

	historyDir := filepath.Join(t.TempDir(), "history")
	previous := testSuiteResult("previous", 10, 1000, 0)
	if err := writeJSON(filepath.Join(historyDir, "runs", "previous", "results.json"), previous); err != nil {
		t.Fatal(err)
	}

	current := testSuiteResult("current", 12, 1050, 0)
	report, err := buildHistoryReport(config{
		HistoryDir:            historyDir,
		MinHistorySamples:     2,
		RequestCountThreshold: 0.05,
		DurationThreshold:     0.50,
	}, current)
	if err != nil {
		t.Fatalf("buildHistoryReport() error = %v", err)
	}
	if report.HasRegressions {
		t.Fatalf("HasRegressions = true, want false")
	}
	if report.InsufficientHistoryRows != 1 {
		t.Fatalf("InsufficientHistoryRows = %d, want 1", report.InsufficientHistoryRows)
	}
}

func TestBuildHistoryReportMissingHistoryDirNotConfigured(t *testing.T) {
	t.Parallel()

	report, err := buildHistoryReport(config{
		HistoryDir:            filepath.Join(t.TempDir(), "missing"),
		MinHistorySamples:     1,
		RequestCountThreshold: 0.05,
		DurationThreshold:     0.50,
	}, testSuiteResult("current", 10, 1000, 0))
	if err != nil {
		t.Fatalf("buildHistoryReport() error = %v", err)
	}
	if report.HistoryConfigured {
		t.Fatalf("HistoryConfigured = true, want false")
	}
}

func TestCompareBaselineAggregatesRepeatedSamples(t *testing.T) {
	t.Parallel()

	baseline := testSuiteResult("baseline", 10, 1000, 0)
	baseline.Cases = append(baseline.Cases, testCaseResult(12, 1200, 0, 2))
	current := testSuiteResult("current", 13, 1400, 0)
	current.Cases = append(current.Cases, testCaseResult(15, 1600, 0, 2))

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeJSON(baselinePath, baseline); err != nil {
		t.Fatal(err)
	}

	comparison, err := compareBaseline(config{
		BaselinePath:          baselinePath,
		RequestCountThreshold: 0.05,
		DurationThreshold:     0.50,
	}, current)
	if err != nil {
		t.Fatalf("compareBaseline() error = %v", err)
	}
	if comparison.ComparedPhases != 1 {
		t.Fatalf("ComparedPhases = %d, want 1", comparison.ComparedPhases)
	}
	if len(comparison.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(comparison.Rows))
	}
	row := comparison.Rows[0]
	if row.BaselineRequests != 11 || row.CurrentRequests != 14 {
		t.Fatalf(
			"requests baseline/current = %d/%d, want 11/14",
			row.BaselineRequests,
			row.CurrentRequests,
		)
	}
	if row.BaselineDurationMS != 1100 || row.CurrentDurationMS != 1500 {
		t.Fatalf(
			"duration baseline/current = %d/%d, want 1100/1500",
			row.BaselineDurationMS,
			row.CurrentDurationMS,
		)
	}
}

func TestNormalizeEnvironmentBenchmarkAuthOverridesE2E(t *testing.T) {
	t.Setenv("KONGCTL_BENCHMARK_KONNECT_PAT", "benchmark-pat")
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "e2e-pat")
	t.Setenv("KONGCTL_BENCHMARK_KONNECT_BASE_URL", "https://benchmark.example.test")
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", "https://e2e.example.test")
	t.Setenv("KONGCTL_BENCHMARK_ARTIFACTS_DIR", t.TempDir())

	if _, err := normalizeEnvironment(); err != nil {
		t.Fatalf("normalizeEnvironment() error = %v", err)
	}
	if got := os.Getenv("KONGCTL_E2E_KONNECT_PAT"); got != "benchmark-pat" {
		t.Fatalf("KONGCTL_E2E_KONNECT_PAT = %q, want benchmark-pat", got)
	}
	if got := os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL"); got != "https://benchmark.example.test" {
		t.Fatalf("KONGCTL_E2E_KONNECT_BASE_URL = %q, want benchmark URL", got)
	}
}

func TestGenerateFixturesLoad(t *testing.T) {
	t.Parallel()

	for _, layout := range layouts {
		t.Run(layout, func(t *testing.T) {
			t.Parallel()

			workload := workloadSpec{Size: "test", APICount: 2, DocumentsPerAPI: 3, DocumentBytes: 128}
			benchmarkCase := benchmarkCase{
				Name:     "test-" + layout,
				Layout:   layout,
				Workload: workload,
			}
			fixture, counts, err := generateFixture(t.TempDir(), benchmarkCase)
			if err != nil {
				t.Fatalf("generateFixture() error = %v", err)
			}
			if counts.APIs != 2 || counts.APIDocuments != 6 {
				t.Fatalf("counts = %+v, want 2 APIs and 6 API documents", counts)
			}

			sources := make([]loader.Source, 0, len(fixture.Files))
			for _, file := range fixture.Files {
				sources = append(sources, loader.Source{Path: file, Type: loader.SourceTypeFile})
			}
			resourceSet, err := loader.New().LoadFromSources(sources, false)
			if err != nil {
				t.Fatalf("LoadFromSources() error = %v", err)
			}
			if len(resourceSet.APIs) != 2 {
				t.Fatalf("len(APIs) = %d, want 2", len(resourceSet.APIs))
			}

			documentCount := len(resourceSet.APIDocuments)
			for _, api := range resourceSet.APIs {
				documentCount += len(api.Documents)
			}
			if documentCount != 6 {
				t.Fatalf("document count = %d, want 6", documentCount)
			}
		})
	}
}

func stringsJoinLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func testSuiteResult(runID string, requests int, durationMS int64, errors int) suiteResult {
	startedAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	return suiteResult{
		SchemaVersion: schemaVersion,
		RunID:         runID,
		StartedAt:     startedAt,
		FinishedAt:    startedAt.Add(time.Duration(durationMS) * time.Millisecond),
		Cases:         []caseResult{testCaseResult(requests, durationMS, errors, 1)},
	}
}

func testCaseResult(requests int, durationMS int64, errors, repetition int) caseResult {
	return caseResult{
		Name:       "small-single-file",
		Size:       "small",
		Layout:     "single-file",
		Repetition: repetition,
		Resources: resourceCounts{
			APIs:         1,
			APIDocuments: 2,
		},
		Phases: []phaseResult{
			{
				Name:       "apply_create",
				DurationMS: durationMS,
				HTTPMetrics: httpMetrics{
					Requests:  requests,
					Responses: requests,
					Errors:    errors,
				},
			},
		},
	}
}

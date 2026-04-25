//go:build e2e

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	historySampleLimit       = 20
	minDurationRegressionMS  = 500
	historyReportSchema      = "kongctl.declarative-benchmark-history.v1"
	regressionSignalError    = "error"
	regressionSignalRequest  = "requests"
	regressionSignalDuration = "duration"
)

type historyReport struct {
	SchemaVersion           string       `json:"schema_version"`
	GeneratedAt             time.Time    `json:"generated_at"`
	RunID                   string       `json:"run_id"`
	RunURL                  string       `json:"run_url,omitempty"`
	GitCommit               string       `json:"git_commit,omitempty"`
	HistoryDir              string       `json:"history_dir,omitempty"`
	HistoryConfigured       bool         `json:"history_configured"`
	MinHistorySamples       int          `json:"min_history_samples"`
	HistoricalRunCount      int          `json:"historical_run_count"`
	HistoricalSampleCount   int          `json:"historical_sample_count"`
	RequestThreshold        float64      `json:"request_threshold"`
	DurationThreshold       float64      `json:"duration_threshold"`
	ComparedRows            int          `json:"compared_rows"`
	InsufficientHistoryRows int          `json:"insufficient_history_rows"`
	HasRegressions          bool         `json:"has_regressions"`
	Rows                    []historyRow `json:"rows"`
}

type historyRow struct {
	CaseName                   string   `json:"case_name"`
	PhaseName                  string   `json:"phase_name"`
	CurrentSamples             int      `json:"current_samples"`
	HistoricalSamples          int      `json:"historical_samples"`
	CurrentRequestsMedian      float64  `json:"current_requests_median"`
	HistoricalRequestsMedian   float64  `json:"historical_requests_median,omitempty"`
	RequestDelta               float64  `json:"request_delta"`
	RequestDeltaPercent        float64  `json:"request_delta_percent"`
	RequestRegression          bool     `json:"request_regression"`
	CurrentDurationMedianMS    float64  `json:"current_duration_median_ms"`
	HistoricalDurationMedianMS float64  `json:"historical_duration_median_ms,omitempty"`
	DurationDeltaMS            float64  `json:"duration_delta_ms"`
	DurationDeltaPercent       float64  `json:"duration_delta_percent"`
	DurationMADMS              float64  `json:"duration_mad_ms,omitempty"`
	DurationThresholdMS        float64  `json:"duration_threshold_ms,omitempty"`
	DurationRegression         bool     `json:"duration_regression"`
	CurrentErrors              int      `json:"current_errors"`
	CurrentFailedPhases        int      `json:"current_failed_phases"`
	ErrorRegression            bool     `json:"error_regression"`
	RegressionSignals          []string `json:"regression_signals,omitempty"`
}

type phaseSample struct {
	RunID      string
	StartedAt  time.Time
	CaseName   string
	PhaseName  string
	Repetition int
	DurationMS int64
	Requests   int
	Responses  int
	Errors     int
	Failed     bool
}

type phaseKey struct {
	CaseName  string
	PhaseName string
}

func writeHistoryOutputs(runDir string, suite suiteResult, cfg config) error {
	report, err := buildHistoryReport(cfg, suite)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(runDir, "history-report.json"), report); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(runDir, "regressions.json"), report); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "dashboard.md"), []byte(renderDashboard(report, suite)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, "regressions.md"), []byte(renderRegressions(report, suite)), 0o644)
}

func buildHistoryReport(cfg config, current suiteResult) (historyReport, error) {
	historyDir := strings.TrimSpace(cfg.HistoryDir)
	historicalSamples, historyConfigured, err := loadHistorySamples(historyDir, current.RunID)
	if err != nil {
		return historyReport{}, err
	}

	report := historyReport{
		SchemaVersion:         historyReportSchema,
		GeneratedAt:           time.Now().UTC(),
		RunID:                 current.RunID,
		RunURL:                current.RunURL,
		GitCommit:             current.GitCommit,
		HistoryDir:            historyDir,
		HistoryConfigured:     historyConfigured,
		MinHistorySamples:     cfg.MinHistorySamples,
		RequestThreshold:      cfg.RequestCountThreshold,
		DurationThreshold:     cfg.DurationThreshold,
		HistoricalSampleCount: len(historicalSamples),
		Rows:                  []historyRow{},
	}

	currentGroups := groupPhaseSamples(samplesFromSuite(current))
	historyGroups := groupPhaseSamples(historicalSamples)
	historicalRuns := map[string]bool{}
	for _, sample := range historicalSamples {
		historicalRuns[sample.RunID] = true
	}
	report.HistoricalRunCount = len(historicalRuns)

	for key, currentSamples := range currentGroups {
		history := recentSamples(historyGroups[key], historySampleLimit)
		row := compareHistoryRow(cfg, key, currentSamples, history)
		report.Rows = append(report.Rows, row)
		if row.HistoricalSamples >= cfg.MinHistorySamples {
			report.ComparedRows++
		} else {
			report.InsufficientHistoryRows++
		}
		if hasRegression(row) {
			report.HasRegressions = true
		}
	}
	sortHistoryRows(report.Rows)

	return report, nil
}

func loadHistorySamples(historyDir, currentRunID string) ([]phaseSample, bool, error) {
	if strings.TrimSpace(historyDir) == "" {
		return nil, false, nil
	}
	info, err := os.Stat(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, false, fmt.Errorf("history dir %s is not a directory", historyDir)
	}

	root := historyDir
	if runsDir := filepath.Join(historyDir, "runs"); isDir(runsDir) {
		root = runsDir
	}

	samples := []phaseSample{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "results.json" {
			return nil
		}

		var suite suiteResult
		if err := readJSON(path, &suite); err != nil {
			return fmt.Errorf("read history result %s: %w", path, err)
		}
		if suite.RunID == currentRunID {
			return nil
		}
		samples = append(samples, samplesFromSuite(suite)...)
		return nil
	})
	if err != nil {
		return nil, true, err
	}
	return samples, true, nil
}

func samplesFromSuite(suite suiteResult) []phaseSample {
	samples := []phaseSample{}
	for _, benchmarkCase := range suite.Cases {
		for _, phase := range benchmarkCase.Phases {
			samples = append(samples, phaseSample{
				RunID:      suite.RunID,
				StartedAt:  suite.StartedAt,
				CaseName:   benchmarkCase.Name,
				PhaseName:  phase.Name,
				Repetition: caseRepetition(benchmarkCase),
				DurationMS: phase.DurationMS,
				Requests:   phase.HTTPMetrics.Requests,
				Responses:  phase.HTTPMetrics.Responses,
				Errors:     phase.HTTPMetrics.Errors,
				Failed:     phase.ExitCode != 0 || phase.TimedOut || strings.TrimSpace(phase.Error) != "",
			})
		}
	}
	return samples
}

func groupPhaseSamples(samples []phaseSample) map[phaseKey][]phaseSample {
	groups := map[phaseKey][]phaseSample{}
	for _, sample := range samples {
		key := phaseKey{CaseName: sample.CaseName, PhaseName: sample.PhaseName}
		groups[key] = append(groups[key], sample)
	}
	return groups
}

func recentSamples(samples []phaseSample, limit int) []phaseSample {
	if len(samples) == 0 {
		return nil
	}
	samples = slices.Clone(samples)
	slices.SortFunc(samples, func(left, right phaseSample) int {
		if cmp := left.StartedAt.Compare(right.StartedAt); cmp != 0 {
			return cmp
		}
		if left.RunID != right.RunID {
			return strings.Compare(left.RunID, right.RunID)
		}
		return left.Repetition - right.Repetition
	})
	if limit > 0 && len(samples) > limit {
		return samples[len(samples)-limit:]
	}
	return samples
}

func compareHistoryRow(cfg config, key phaseKey, currentSamples, historicalSamples []phaseSample) historyRow {
	currentRequests := sampleRequests(currentSamples)
	currentDurations := sampleDurations(currentSamples)
	historicalRequests := sampleRequests(historicalSamples)
	historicalDurations := sampleDurations(historicalSamples)

	row := historyRow{
		CaseName:                key.CaseName,
		PhaseName:               key.PhaseName,
		CurrentSamples:          len(currentSamples),
		HistoricalSamples:       len(historicalSamples),
		CurrentRequestsMedian:   medianFloat64(currentRequests),
		CurrentDurationMedianMS: medianFloat64(currentDurations),
		CurrentErrors:           sumSampleErrors(currentSamples),
		CurrentFailedPhases:     sumSampleFailures(currentSamples),
	}
	row.ErrorRegression = row.CurrentErrors > 0 || row.CurrentFailedPhases > 0
	if row.ErrorRegression {
		row.RegressionSignals = append(row.RegressionSignals, regressionSignalError)
	}

	if row.HistoricalSamples < cfg.MinHistorySamples {
		return row
	}

	row.HistoricalRequestsMedian = medianFloat64(historicalRequests)
	row.HistoricalDurationMedianMS = medianFloat64(historicalDurations)
	row.RequestDelta = row.CurrentRequestsMedian - row.HistoricalRequestsMedian
	row.RequestDeltaPercent = percentDeltaFloat(row.HistoricalRequestsMedian, row.CurrentRequestsMedian)
	row.RequestRegression = row.RequestDelta >= 1 && row.RequestDeltaPercent > cfg.RequestCountThreshold
	if row.RequestRegression {
		row.RegressionSignals = append(row.RegressionSignals, regressionSignalRequest)
	}

	row.DurationDeltaMS = row.CurrentDurationMedianMS - row.HistoricalDurationMedianMS
	row.DurationDeltaPercent = percentDeltaFloat(row.HistoricalDurationMedianMS, row.CurrentDurationMedianMS)
	row.DurationMADMS = medianAbsoluteDeviation(historicalDurations, row.HistoricalDurationMedianMS)
	row.DurationThresholdMS = maxFloat(
		minDurationRegressionMS,
		row.HistoricalDurationMedianMS*cfg.DurationThreshold,
		row.DurationMADMS*3,
	)
	row.DurationRegression = row.DurationDeltaMS > row.DurationThresholdMS
	if row.DurationRegression {
		row.RegressionSignals = append(row.RegressionSignals, regressionSignalDuration)
	}

	return row
}

func sampleRequests(samples []phaseSample) []float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, float64(sample.Requests))
	}
	return values
}

func sampleDurations(samples []phaseSample) []float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, float64(sample.DurationMS))
	}
	return values
}

func sumSampleErrors(samples []phaseSample) int {
	total := 0
	for _, sample := range samples {
		total += sample.Errors
	}
	return total
}

func sumSampleFailures(samples []phaseSample) int {
	total := 0
	for _, sample := range samples {
		if sample.Failed {
			total++
		}
	}
	return total
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	values = slices.Clone(values)
	slices.Sort(values)
	midpoint := len(values) / 2
	if len(values)%2 == 1 {
		return values[midpoint]
	}
	return (values[midpoint-1] + values[midpoint]) / 2
}

func medianAbsoluteDeviation(values []float64, center float64) float64 {
	if len(values) == 0 {
		return 0
	}
	deviations := make([]float64, 0, len(values))
	for _, value := range values {
		deviations = append(deviations, math.Abs(value-center))
	}
	return medianFloat64(deviations)
}

func percentDeltaFloat(baseline, current float64) float64 {
	if baseline == 0 {
		if current == 0 {
			return 0
		}
		return 1
	}
	return (current - baseline) / baseline
}

func maxFloat(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return slices.Max(values)
}

func sortHistoryRows(rows []historyRow) {
	slices.SortFunc(rows, func(left, right historyRow) int {
		if left.CaseName != right.CaseName {
			return strings.Compare(left.CaseName, right.CaseName)
		}
		return strings.Compare(left.PhaseName, right.PhaseName)
	})
}

func hasRegression(row historyRow) bool {
	return row.ErrorRegression || row.RequestRegression || row.DurationRegression
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func renderDashboard(report historyReport, suite suiteResult) string {
	var b strings.Builder
	b.WriteString("# Declarative Benchmark Dashboard\n\n")
	b.WriteString("<!-- Generated by the Declarative Benchmark workflow. -->\n\n")
	fmt.Fprintf(&b, "- Latest run: %s\n", markdownRunLink(report.RunID, report.RunURL))
	if report.GitCommit != "" {
		fmt.Fprintf(&b, "- Git commit: `%s`\n", report.GitCommit)
	}
	fmt.Fprintf(&b, "- Generated: `%s`\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "- Suite duration: `%s`\n", suite.FinishedAt.Sub(suite.StartedAt).Round(time.Millisecond))
	fmt.Fprintf(&b, "- Case executions: `%d`\n", suite.Summary.CaseCount)
	fmt.Fprintf(&b, "- Phases: `%d`\n", suite.Summary.PhaseCount)
	fmt.Fprintf(&b, "- HTTP requests: `%d`\n", suite.Summary.TotalRequests)
	fmt.Fprintf(&b, "- HTTP errors: `%d`\n", suite.Summary.TotalHTTPErrors)
	fmt.Fprintf(&b, "- Regression status: **%s**\n\n", dashboardStatus(report))

	fmt.Fprintf(
		&b,
		"History: `%d` runs / `%d` samples. Statistical checks require `%d` historical samples per case phase.\n\n",
		report.HistoricalRunCount,
		report.HistoricalSampleCount,
		report.MinHistorySamples,
	)
	b.WriteString(
		"Request regressions compare median request counts against recent history. " +
			"Duration regressions compare median wall-clock time against recent history with a MAD-based guardrail.\n\n",
	)

	b.WriteString(
		"| Case | Phase | Samples | Requests p50 | History requests p50 | Duration p50 | " +
			"History duration p50 | History samples | Status |\n",
	)
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, row := range report.Rows {
		fmt.Fprintf(
			&b,
			"| `%s` | `%s` | %d | %s | %s | %s | %s | %d | %s |\n",
			row.CaseName,
			row.PhaseName,
			row.CurrentSamples,
			formatCountMedian(row.CurrentRequestsMedian),
			formatOptionalCountMedian(row.HistoricalSamples, row.HistoricalRequestsMedian),
			formatMillisecondsFloat(row.CurrentDurationMedianMS),
			formatOptionalMilliseconds(row.HistoricalSamples, row.HistoricalDurationMedianMS),
			row.HistoricalSamples,
			rowStatus(row, report.MinHistorySamples),
		)
	}

	return b.String()
}

func renderRegressions(report historyReport, suite suiteResult) string {
	var b strings.Builder
	b.WriteString("# Declarative Benchmark Regression Report\n\n")
	fmt.Fprintf(&b, "- Run: %s\n", markdownRunLink(report.RunID, report.RunURL))
	if report.GitCommit != "" {
		fmt.Fprintf(&b, "- Git commit: `%s`\n", report.GitCommit)
	}
	fmt.Fprintf(&b, "- Suite duration: `%s`\n", suite.FinishedAt.Sub(suite.StartedAt).Round(time.Millisecond))
	fmt.Fprintf(&b, "- HTTP requests: `%d`\n", suite.Summary.TotalRequests)
	fmt.Fprintf(&b, "- HTTP errors: `%d`\n", suite.Summary.TotalHTTPErrors)
	fmt.Fprintf(&b, "- History samples required: `%d`\n\n", report.MinHistorySamples)

	if !report.HasRegressions {
		b.WriteString("No regressions detected in the latest benchmark run.\n")
		return b.String()
	}

	b.WriteString("Regressions detected in the latest benchmark run.\n\n")
	b.WriteString(
		"| Case | Phase | Signals | Request Δ | Duration Δ | Current errors | Failed phases |\n",
	)
	b.WriteString("| --- | --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, row := range report.Rows {
		if !hasRegression(row) {
			continue
		}
		fmt.Fprintf(
			&b,
			"| `%s` | `%s` | %s | %s | %s | %d | %d |\n",
			row.CaseName,
			row.PhaseName,
			strings.Join(row.RegressionSignals, ", "),
			formatDeltaPercent(row.RequestDelta, row.RequestDeltaPercent),
			formatDeltaPercent(row.DurationDeltaMS, row.DurationDeltaPercent),
			row.CurrentErrors,
			row.CurrentFailedPhases,
		)
	}
	b.WriteString(
		"\nInspect workflow artifacts for raw `kongctl` logs, generated fixtures, and per-command HTTP metrics.\n",
	)
	return b.String()
}

func dashboardStatus(report historyReport) string {
	if report.HasRegressions {
		return "regression detected"
	}
	if report.InsufficientHistoryRows > 0 {
		return "collecting history"
	}
	return "ok"
}

func rowStatus(row historyRow, minHistorySamples int) string {
	if hasRegression(row) {
		return strings.Join(row.RegressionSignals, ", ")
	}
	if row.HistoricalSamples < minHistorySamples {
		return "collecting history"
	}
	return "ok"
}

func markdownRunLink(runID, runURL string) string {
	if strings.TrimSpace(runURL) == "" {
		return "`" + runID + "`"
	}
	return fmt.Sprintf("[`%s`](%s)", runID, runURL)
}

func formatCountMedian(value float64) string {
	if math.Trunc(value) == value {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func formatOptionalCountMedian(samples int, value float64) string {
	if samples == 0 {
		return "n/a"
	}
	return formatCountMedian(value)
}

func formatMillisecondsFloat(value float64) string {
	return (time.Duration(math.Round(value)) * time.Millisecond).String()
}

func formatOptionalMilliseconds(samples int, value float64) string {
	if samples == 0 {
		return "n/a"
	}
	return formatMillisecondsFloat(value)
}

func formatDeltaPercent(delta, percent float64) string {
	if delta == 0 && percent == 0 {
		return "0"
	}
	return fmt.Sprintf("%+.0f (%+.1f%%)", delta, percent*100)
}

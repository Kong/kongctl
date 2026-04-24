//go:build e2e

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

const (
	schemaVersion  = "kongctl.declarative-benchmark.v1"
	defaultBaseURL = "https://us.api.konghq.com"
)

type config struct {
	CaseFilter            string
	BaselinePath          string
	FailOnRegression      bool
	RequestCountThreshold float64
	DurationThreshold     float64
	CommandTimeout        time.Duration
	Repeat                int
	HistoryDir            string
	RunURL                string
	MinHistorySamples     int
}

type workloadSpec struct {
	Size            string `json:"size"`
	APICount        int    `json:"api_count"`
	DocumentsPerAPI int    `json:"documents_per_api"`
	DocumentBytes   int    `json:"document_bytes"`
}

type benchmarkCase struct {
	Name     string       `json:"name"`
	Layout   string       `json:"layout"`
	Workload workloadSpec `json:"workload"`
}

type suiteResult struct {
	SchemaVersion string            `json:"schema_version"`
	RunID         string            `json:"run_id"`
	RunURL        string            `json:"run_url,omitempty"`
	GitCommit     string            `json:"git_commit,omitempty"`
	BaseURL       string            `json:"base_url"`
	StartedAt     time.Time         `json:"started_at"`
	FinishedAt    time.Time         `json:"finished_at"`
	DurationMS    int64             `json:"duration_ms"`
	Cases         []caseResult      `json:"cases"`
	Summary       suiteSummary      `json:"summary"`
	Comparison    *comparisonResult `json:"comparison,omitempty"`
}

type suiteSummary struct {
	CaseCount           int   `json:"case_count"`
	PhaseCount          int   `json:"phase_count"`
	FailedPhases        int   `json:"failed_phases"`
	TotalAPIs           int   `json:"total_apis"`
	TotalDocuments      int   `json:"total_api_documents"`
	TotalRequests       int   `json:"total_http_requests"`
	TotalResponses      int   `json:"total_http_responses"`
	TotalHTTPErrors     int   `json:"total_http_errors"`
	TotalDurationMS     int64 `json:"total_duration_ms"`
	ComparedPhases      int   `json:"compared_phases,omitempty"`
	RequestRegressions  int   `json:"request_regressions,omitempty"`
	DurationRegressions int   `json:"duration_regressions,omitempty"`
}

type caseResult struct {
	Name       string         `json:"name"`
	Size       string         `json:"size"`
	Layout     string         `json:"layout"`
	Repetition int            `json:"repetition,omitempty"`
	Fixture    fixtureResult  `json:"fixture"`
	Resources  resourceCounts `json:"resources"`
	Phases     []phaseResult  `json:"phases"`
}

type resourceCounts struct {
	APIs          int `json:"apis"`
	APIDocuments  int `json:"api_documents"`
	DocumentBytes int `json:"api_document_bytes"`
}

type fixtureResult struct {
	RootDir string   `json:"root_dir"`
	Files   []string `json:"files"`
	Args    []string `json:"args"`
}

type phaseResult struct {
	Name        string      `json:"name"`
	CommandDir  string      `json:"command_dir,omitempty"`
	ExitCode    int         `json:"exit_code"`
	TimedOut    bool        `json:"timed_out"`
	DurationMS  int64       `json:"duration_ms"`
	Error       string      `json:"error,omitempty"`
	HTTPMetrics httpMetrics `json:"http_metrics"`
}

type httpMetrics struct {
	Requests             int              `json:"requests"`
	Responses            int              `json:"responses"`
	Errors               int              `json:"errors"`
	RequestCountsByRoute []methodRouteRow `json:"request_counts_by_route"`
	ResponseStatusCounts []statusRow      `json:"response_status_counts"`
	Timing               httpTiming       `json:"timing"`
}

type methodRouteRow struct {
	Method string `json:"method"`
	Route  string `json:"route"`
	Count  int    `json:"count"`
}

type statusRow struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type httpTiming struct {
	Responses durationStats `json:"responses"`
	Errors    durationStats `json:"errors"`
	Combined  durationStats `json:"combined"`
}

type durationStats struct {
	Count int     `json:"count"`
	SumMS float64 `json:"sum_ms"`
	MinMS float64 `json:"min_ms"`
	MaxMS float64 `json:"max_ms"`
	AvgMS float64 `json:"avg_ms"`
}

type comparisonResult struct {
	BaselinePath        string          `json:"baseline_path"`
	RequestThreshold    float64         `json:"request_threshold"`
	DurationThreshold   float64         `json:"duration_threshold"`
	ComparedPhases      int             `json:"compared_phases"`
	MissingBaselineData []string        `json:"missing_baseline_data,omitempty"`
	Rows                []comparisonRow `json:"rows"`
	RequestRegressions  int             `json:"request_regressions"`
	DurationRegressions int             `json:"duration_regressions"`
}

type comparisonRow struct {
	CaseName             string  `json:"case_name"`
	PhaseName            string  `json:"phase_name"`
	BaselineRequests     int     `json:"baseline_requests"`
	CurrentRequests      int     `json:"current_requests"`
	RequestDelta         int     `json:"request_delta"`
	RequestDeltaPercent  float64 `json:"request_delta_percent"`
	RequestRegression    bool    `json:"request_regression"`
	BaselineDurationMS   int64   `json:"baseline_duration_ms"`
	CurrentDurationMS    int64   `json:"current_duration_ms"`
	DurationDeltaMS      int64   `json:"duration_delta_ms"`
	DurationDeltaPercent float64 `json:"duration_delta_percent"`
	DurationRegression   bool    `json:"duration_regression"`
}

var workloads = []workloadSpec{
	{Size: "small", APICount: 1, DocumentsPerAPI: 2, DocumentBytes: 1024},
	{Size: "medium", APICount: 5, DocumentsPerAPI: 5, DocumentBytes: 2048},
	{Size: "large", APICount: 20, DocumentsPerAPI: 8, DocumentBytes: 4096},
	{Size: "xl", APICount: 50, DocumentsPerAPI: 10, DocumentBytes: 8192},
}

var layouts = []string{"single-file", "multi-file"}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := parseConfig()
	runDir, err := normalizeEnvironment()
	if err != nil {
		return err
	}

	cases, err := selectCases(cfg.CaseFilter)
	if err != nil {
		return err
	}

	started := time.Now().UTC()
	suite := suiteResult{
		SchemaVersion: schemaVersion,
		RunID:         benchmarkRunID(started),
		RunURL:        cfg.RunURL,
		GitCommit:     gitCommit(),
		BaseURL:       os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL"),
		StartedAt:     started,
		Cases:         []caseResult{},
	}

	runErr := executeSuite(context.Background(), cfg, cases, &suite)
	suite.FinishedAt = time.Now().UTC()
	suite.DurationMS = suite.FinishedAt.Sub(suite.StartedAt).Milliseconds()
	suite.Summary = summarizeSuite(suite.Cases)

	if strings.TrimSpace(cfg.BaselinePath) != "" {
		comparison, err := compareBaseline(cfg, suite)
		if err != nil {
			runErr = errors.Join(runErr, err)
		} else {
			suite.Comparison = comparison
			suite.Summary.ComparedPhases = comparison.ComparedPhases
			suite.Summary.RequestRegressions = comparison.RequestRegressions
			suite.Summary.DurationRegressions = comparison.DurationRegressions
			if cfg.FailOnRegression && comparison.RequestRegressions > 0 {
				runErr = errors.Join(runErr, fmt.Errorf("%d request-count regressions detected", comparison.RequestRegressions))
			}
		}
	}

	if err := writeSuiteOutputs(runDir, suite, cfg); err != nil {
		runErr = errors.Join(runErr, err)
	}

	fmt.Printf("Declarative benchmark artifacts: %s\n", runDir)
	fmt.Printf("Declarative benchmark results: %s\n", filepath.Join(runDir, "results.json"))
	fmt.Printf("Declarative benchmark summary: %s\n", filepath.Join(runDir, "summary.md"))
	fmt.Printf("Declarative benchmark terminal summary: %s\n", filepath.Join(runDir, "summary.txt"))

	return runErr
}

func parseConfig() config {
	defaultCase := envOrDefault("KONGCTL_BENCHMARK_CASE", "all")
	defaultTimeout := durationEnv("KONGCTL_BENCHMARK_COMMAND_TIMEOUT", 30*time.Minute)

	cfg := config{}
	flag.StringVar(&cfg.CaseFilter, "case", defaultCase, "benchmark case selector: all, a size, or a case name")
	flag.IntVar(
		&cfg.Repeat,
		"repeat",
		intEnv("KONGCTL_BENCHMARK_REPEAT", 1),
		"number of times to execute each selected case",
	)
	flag.StringVar(
		&cfg.BaselinePath,
		"baseline",
		os.Getenv("KONGCTL_BENCHMARK_BASELINE"),
		"optional baseline results.json",
	)
	flag.StringVar(
		&cfg.HistoryDir,
		"history-dir",
		os.Getenv("KONGCTL_BENCHMARK_HISTORY_DIR"),
		"optional benchmark-results history directory used for dashboard and regression reports",
	)
	flag.StringVar(
		&cfg.RunURL,
		"run-url",
		os.Getenv("KONGCTL_BENCHMARK_RUN_URL"),
		"optional URL for the benchmark run included in generated reports",
	)
	flag.IntVar(
		&cfg.MinHistorySamples,
		"min-history-samples",
		intEnv("KONGCTL_BENCHMARK_MIN_HISTORY_SAMPLES", 3),
		"minimum historical samples required before statistical regressions are reported",
	)
	flag.BoolVar(
		&cfg.FailOnRegression,
		"fail-on-regression",
		boolEnv("KONGCTL_BENCHMARK_FAIL_ON_REGRESSION", false),
		"exit non-zero when request-count regressions exceed thresholds",
	)
	flag.Float64Var(
		&cfg.RequestCountThreshold,
		"request-count-threshold",
		floatEnv("KONGCTL_BENCHMARK_REQUEST_COUNT_THRESHOLD", 0.05),
		"allowed request-count increase ratio when comparing to a baseline",
	)
	flag.Float64Var(
		&cfg.DurationThreshold,
		"duration-threshold",
		floatEnv("KONGCTL_BENCHMARK_DURATION_THRESHOLD", 0.50),
		"tracked wall-clock duration increase ratio when comparing to a baseline",
	)
	flag.DurationVar(&cfg.CommandTimeout, "command-timeout", defaultTimeout, "timeout for each measured kongctl command")
	flag.Parse()
	if cfg.Repeat < 1 {
		cfg.Repeat = 1
	}
	if cfg.MinHistorySamples < 1 {
		cfg.MinHistorySamples = 1
	}
	return cfg
}

func normalizeEnvironment() (string, error) {
	setEnvFromBenchmarkOverride("KONGCTL_E2E_KONNECT_PAT", "KONGCTL_BENCHMARK_KONNECT_PAT")
	setEnvFromBenchmarkOverride("KONGCTL_E2E_KONNECT_BASE_URL", "KONGCTL_BENCHMARK_KONNECT_BASE_URL")
	setEnvFromBenchmark("KONGCTL_E2E_ARTIFACTS_DIR", "KONGCTL_BENCHMARK_ARTIFACTS_DIR")
	setEnvFromBenchmark("KONGCTL_E2E_LOG_LEVEL", "KONGCTL_BENCHMARK_LOG_LEVEL")
	setEnvFromBenchmark("KONGCTL_E2E_CONSOLE_LOG_LEVEL", "KONGCTL_BENCHMARK_CONSOLE_LOG_LEVEL")

	setDefaultEnv("KONGCTL_E2E_KONNECT_BASE_URL", defaultBaseURL)
	setDefaultEnv("KONGCTL_E2E_LOG_LEVEL", "debug")
	setDefaultEnv("KONGCTL_E2E_CONSOLE_LOG_LEVEL", "warn")
	setDefaultEnv("KONGCTL_E2E_RESET", "1")

	if strings.TrimSpace(os.Getenv("KONGCTL_E2E_ARTIFACTS_DIR")) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		runDir := filepath.Join(cwd, ".benchmark-artifacts", time.Now().UTC().Format("20060102-150405"))
		if err := os.Setenv("KONGCTL_E2E_ARTIFACTS_DIR", runDir); err != nil {
			return "", err
		}
	}

	runDir := os.Getenv("KONGCTL_E2E_ARTIFACTS_DIR")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}
	runDir, err := filepath.Abs(runDir)
	if err != nil {
		return "", err
	}
	if err := os.Setenv("KONGCTL_E2E_ARTIFACTS_DIR", runDir); err != nil {
		return "", err
	}
	if strings.TrimSpace(os.Getenv("KONGCTL_E2E_KONNECT_PAT")) == "" {
		return "", fmt.Errorf("KONGCTL_BENCHMARK_KONNECT_PAT or KONGCTL_E2E_KONNECT_PAT is required")
	}
	return runDir, nil
}

func executeSuite(ctx context.Context, cfg config, cases []benchmarkCase, suite *suiteResult) error {
	for _, benchmarkCase := range cases {
		for repetition := 1; repetition <= cfg.Repeat; repetition++ {
			caseResult, err := executeCase(ctx, cfg, benchmarkCase, repetition)
			suite.Cases = append(suite.Cases, caseResult)
			if err != nil {
				return fmt.Errorf("case %s repetition %d failed: %w", benchmarkCase.Name, repetition, err)
			}
		}
	}
	return nil
}

func executeCase(ctx context.Context, cfg config, benchmarkCase benchmarkCase, repetition int) (caseResult, error) {
	artifactName := "declarative-" + benchmarkCase.Name
	resetStage := "before-" + benchmarkCase.Name
	if cfg.Repeat > 1 {
		artifactName = fmt.Sprintf("%s-r%03d", artifactName, repetition)
		resetStage = fmt.Sprintf("%s-r%03d", resetStage, repetition)
	}
	cli, err := harness.NewCLIForArtifacts(artifactName, "benchmarks")
	if err != nil {
		return caseResult{}, err
	}
	cli.SetLogLevel(benchmarkCommandLogLevel())
	cli.Timeout = cfg.CommandTimeout

	fixture, counts, err := generateFixture(cli.TestDir, benchmarkCase)
	result := caseResult{
		Name:       benchmarkCase.Name,
		Size:       benchmarkCase.Workload.Size,
		Layout:     benchmarkCase.Layout,
		Repetition: repetition,
		Fixture:    fixture,
		Resources:  counts,
		Phases:     []phaseResult{},
	}
	if err != nil {
		return result, err
	}

	if err := harness.ResetOrgWithCapture(resetStage); err != nil {
		return result, err
	}

	create, err := executePhase(ctx, cli, fixture.Args, "apply_create")
	result.Phases = append(result.Phases, create)
	if err != nil {
		return result, err
	}

	noop, err := executePhase(ctx, cli, fixture.Args, "apply_noop")
	result.Phases = append(result.Phases, noop)
	if err != nil {
		return result, err
	}

	return result, nil
}

func executePhase(ctx context.Context, cli *harness.CLI, fixtureArgs []string, phase string) (phaseResult, error) {
	args := append([]string{"apply"}, fixtureArgs...)
	args = append(args, "--auto-approve")
	cli.OverrideNextCommandSlug(phase)
	res, err := cli.Run(ctx, args...)

	phaseResult := phaseResult{
		Name:       phase,
		CommandDir: cli.LastCommandDir,
		ExitCode:   res.ExitCode,
		TimedOut:   res.TimedOut,
		DurationMS: res.Duration.Milliseconds(),
	}
	if err != nil {
		phaseResult.Error = err.Error()
	}
	if cli.LastCommandDir != "" {
		logPath := filepath.Join(cli.LastCommandDir, "kongctl.log")
		metrics, metricsErr := parseHTTPLog(logPath)
		if metricsErr == nil {
			phaseResult.HTTPMetrics = metrics
			_ = writeJSON(filepath.Join(cli.LastCommandDir, "http-metrics.json"), metrics)
		} else if !errors.Is(metricsErr, os.ErrNotExist) {
			phaseResult.Error = appendErrorMessage(phaseResult.Error, metricsErr.Error())
		}
	}
	return phaseResult, err
}

func selectCases(filter string) ([]benchmarkCase, error) {
	allCases := allBenchmarkCases()
	clean := strings.ToLower(strings.TrimSpace(filter))
	if clean == "" || clean == "all" {
		return allCases, nil
	}

	selected := []benchmarkCase{}
	seen := map[string]bool{}
	for _, token := range strings.Split(clean, ",") {
		token = normalizeCaseToken(token)
		if token == "" {
			continue
		}
		matched := false
		for _, candidate := range allCases {
			if caseTokenMatches(token, candidate) {
				if !seen[candidate.Name] {
					selected = append(selected, candidate)
					seen[candidate.Name] = true
				}
				matched = true
			}
		}
		if !matched {
			return nil, fmt.Errorf("unknown benchmark case selector %q", token)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no benchmark cases selected")
	}
	return selected, nil
}

func allBenchmarkCases() []benchmarkCase {
	cases := []benchmarkCase{}
	for _, workload := range workloads {
		for _, layout := range layouts {
			cases = append(cases, benchmarkCase{
				Name:     workload.Size + "-" + layout,
				Layout:   layout,
				Workload: workload,
			})
		}
	}
	return cases
}

func caseTokenMatches(token string, candidate benchmarkCase) bool {
	if token == candidate.Name ||
		token == candidate.Workload.Size ||
		token == candidate.Layout ||
		token == strings.TrimSuffix(candidate.Name, "-file") {
		return true
	}
	return false
}

func normalizeCaseToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.ReplaceAll(token, "_", "-")
	token = strings.ReplaceAll(token, " ", "-")
	return token
}

func generateFixture(testDir string, benchmarkCase benchmarkCase) (fixtureResult, resourceCounts, error) {
	inputDir := filepath.Join(testDir, "inputs", benchmarkCase.Name)
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		return fixtureResult{}, resourceCounts{}, err
	}

	switch benchmarkCase.Layout {
	case "single-file":
		return generateSingleFileFixture(inputDir, benchmarkCase)
	case "multi-file":
		return generateMultiFileFixture(inputDir, benchmarkCase)
	default:
		return fixtureResult{}, resourceCounts{}, fmt.Errorf("unsupported fixture layout %q", benchmarkCase.Layout)
	}
}

func generateSingleFileFixture(inputDir string, benchmarkCase benchmarkCase) (fixtureResult, resourceCounts, error) {
	path := filepath.Join(inputDir, "config.yaml")
	counts := benchmarkCaseCounts(benchmarkCase)
	var yaml strings.Builder
	writeAPIs(&yaml, benchmarkCase, true)
	if err := os.WriteFile(path, []byte(yaml.String()), 0o644); err != nil {
		return fixtureResult{}, resourceCounts{}, err
	}
	return fixtureResult{
		RootDir: inputDir,
		Files:   []string{path},
		Args:    []string{"-f", path},
	}, counts, nil
}

func generateMultiFileFixture(inputDir string, benchmarkCase benchmarkCase) (fixtureResult, resourceCounts, error) {
	files := []string{}
	counts := benchmarkCaseCounts(benchmarkCase)

	apisPath := filepath.Join(inputDir, "apis.yaml")
	var apis strings.Builder
	writeAPIs(&apis, benchmarkCase, false)
	if err := os.WriteFile(apisPath, []byte(apis.String()), 0o644); err != nil {
		return fixtureResult{}, resourceCounts{}, err
	}
	files = append(files, apisPath)

	for apiIndex := range benchmarkCase.Workload.APICount {
		path := filepath.Join(inputDir, fmt.Sprintf("api-%03d-documents.yaml", apiIndex+1))
		var docs strings.Builder
		writeAPIDocuments(&docs, benchmarkCase, apiIndex)
		if err := os.WriteFile(path, []byte(docs.String()), 0o644); err != nil {
			return fixtureResult{}, resourceCounts{}, err
		}
		files = append(files, path)
	}

	args := []string{}
	for _, path := range files {
		args = append(args, "-f", path)
	}
	return fixtureResult{RootDir: inputDir, Files: files, Args: args}, counts, nil
}

func benchmarkCaseCounts(benchmarkCase benchmarkCase) resourceCounts {
	return resourceCounts{
		APIs:         benchmarkCase.Workload.APICount,
		APIDocuments: benchmarkCase.Workload.APICount * benchmarkCase.Workload.DocumentsPerAPI,
		DocumentBytes: benchmarkCase.Workload.APICount *
			benchmarkCase.Workload.DocumentsPerAPI *
			benchmarkCase.Workload.DocumentBytes,
	}
}

func writeAPIs(yaml *strings.Builder, benchmarkCase benchmarkCase, includeDocuments bool) {
	yaml.WriteString("apis:\n")
	for apiIndex := range benchmarkCase.Workload.APICount {
		apiRef := apiRef(benchmarkCase, apiIndex)
		fmt.Fprintf(yaml, "  - ref: %s\n", apiRef)
		fmt.Fprintf(yaml, "    name: %q\n", apiName(benchmarkCase, apiIndex))
		fmt.Fprintf(yaml, "    description: %q\n", "Declarative benchmark API resource")
		fmt.Fprintf(yaml, "    version: %q\n", "1.0.0")
		fmt.Fprintf(yaml, "    slug: %q\n", apiRef)
		yaml.WriteString("    labels:\n")
		yaml.WriteString("      benchmark: gh-730\n")
		fmt.Fprintf(yaml, "      benchmark_size: %s\n", benchmarkCase.Workload.Size)
		fmt.Fprintf(yaml, "      benchmark_layout: %s\n", benchmarkCase.Layout)
		yaml.WriteString("    kongctl:\n")
		yaml.WriteString("      namespace: gh-730-benchmark\n")
		if includeDocuments {
			yaml.WriteString("    documents:\n")
			for docIndex := range benchmarkCase.Workload.DocumentsPerAPI {
				writeNestedDocument(yaml, benchmarkCase, apiIndex, docIndex)
			}
		}
	}
}

func writeAPIDocuments(yaml *strings.Builder, benchmarkCase benchmarkCase, apiIndex int) {
	yaml.WriteString("api_documents:\n")
	for docIndex := range benchmarkCase.Workload.DocumentsPerAPI {
		writeRootDocument(yaml, benchmarkCase, apiIndex, docIndex)
	}
}

func writeNestedDocument(yaml *strings.Builder, benchmarkCase benchmarkCase, apiIndex, docIndex int) {
	docRef := documentRef(benchmarkCase, apiIndex, docIndex)
	fmt.Fprintf(yaml, "      - ref: %s\n", docRef)
	fmt.Fprintf(yaml, "        title: %q\n", documentTitle(benchmarkCase, apiIndex, docIndex))
	fmt.Fprintf(yaml, "        slug: %q\n", docRef)
	yaml.WriteString("        status: published\n")
	writeLiteral(yaml, "        ", "content", documentContent(benchmarkCase, apiIndex, docIndex))
}

func writeRootDocument(yaml *strings.Builder, benchmarkCase benchmarkCase, apiIndex, docIndex int) {
	docRef := documentRef(benchmarkCase, apiIndex, docIndex)
	fmt.Fprintf(yaml, "  - ref: %s\n", docRef)
	fmt.Fprintf(yaml, "    api: %s\n", apiRef(benchmarkCase, apiIndex))
	fmt.Fprintf(yaml, "    title: %q\n", documentTitle(benchmarkCase, apiIndex, docIndex))
	fmt.Fprintf(yaml, "    slug: %q\n", docRef)
	yaml.WriteString("    status: published\n")
	writeLiteral(yaml, "    ", "content", documentContent(benchmarkCase, apiIndex, docIndex))
}

func writeLiteral(yaml *strings.Builder, indent, key, value string) {
	fmt.Fprintf(yaml, "%s%s: |\n", indent, key)
	for _, line := range strings.Split(strings.TrimRight(value, "\n"), "\n") {
		fmt.Fprintf(yaml, "%s  %s\n", indent, line)
	}
}

func apiRef(benchmarkCase benchmarkCase, apiIndex int) string {
	return fmt.Sprintf("gh-730-%s-api-%03d", benchmarkCase.Workload.Size, apiIndex+1)
}

func apiName(benchmarkCase benchmarkCase, apiIndex int) string {
	return apiRef(benchmarkCase, apiIndex)
}

func documentRef(benchmarkCase benchmarkCase, apiIndex, docIndex int) string {
	return fmt.Sprintf("gh-730-%s-api-%03d-doc-%03d", benchmarkCase.Workload.Size, apiIndex+1, docIndex+1)
}

func documentTitle(benchmarkCase benchmarkCase, apiIndex, docIndex int) string {
	return documentRef(benchmarkCase, apiIndex, docIndex)
}

func documentContent(benchmarkCase benchmarkCase, apiIndex, docIndex int) string {
	header := fmt.Sprintf(
		"# %s\n\nAPI index: %03d\nDocument index: %03d\nBenchmark layout: %s\n\n",
		documentTitle(benchmarkCase, apiIndex, docIndex),
		apiIndex+1,
		docIndex+1,
		benchmarkCase.Layout,
	)
	paragraph := "This deterministic API document body is generated for declarative performance benchmarking. " +
		"It intentionally repeats realistic Markdown prose so api_documents exercise larger request payloads.\n\n"

	var b strings.Builder
	b.WriteString(header)
	for b.Len() < benchmarkCase.Workload.DocumentBytes {
		b.WriteString(paragraph)
	}
	return b.String()
}

var logFieldRe = regexp.MustCompile(`([A-Za-z0-9_]+)=("[^"]*"|[^ ]+)`)

func parseHTTPLog(path string) (httpMetrics, error) {
	file, err := os.Open(path)
	if err != nil {
		return httpMetrics{}, err
	}
	defer file.Close()

	routeCounts := map[string]int{}
	statusCounts := map[string]int{}
	metrics := httpMetrics{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := parseLogFields(scanner.Text())
		switch fields["log_type"] {
		case "http_request":
			metrics.Requests++
			method := strings.ToUpper(fields["method"])
			route := fields["route"]
			if method != "" && route != "" {
				routeCounts[method+"\x00"+route]++
			}
		case "http_response":
			metrics.Responses++
			if status := fields["status_code"]; status != "" {
				statusCounts[status]++
			}
			addDuration(&metrics.Timing.Responses, fields["duration"])
		case "http_error":
			metrics.Errors++
			addDuration(&metrics.Timing.Errors, fields["duration"])
		}
	}
	if err := scanner.Err(); err != nil {
		return httpMetrics{}, err
	}
	finalizeDurationStats(&metrics.Timing.Responses)
	finalizeDurationStats(&metrics.Timing.Errors)
	metrics.Timing.Combined = combineDurationStats(metrics.Timing.Responses, metrics.Timing.Errors)
	metrics.RequestCountsByRoute = sortedMethodRouteRows(routeCounts)
	metrics.ResponseStatusCounts = sortedStatusRows(statusCounts)
	return metrics, nil
}

func parseLogFields(line string) map[string]string {
	fields := map[string]string{}
	matches := logFieldRe.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		value := match[2]
		if strings.HasPrefix(value, `"`) {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			} else {
				value = strings.Trim(value, `"`)
			}
		}
		fields[match[1]] = value
	}
	return fields
}

func addDuration(stats *durationStats, raw string) {
	ms, ok := parseDurationMS(raw)
	if !ok {
		return
	}
	stats.Count++
	stats.SumMS += ms
	if stats.Count == 1 || ms < stats.MinMS {
		stats.MinMS = ms
	}
	if stats.Count == 1 || ms > stats.MaxMS {
		stats.MaxMS = ms
	}
}

func finalizeDurationStats(stats *durationStats) {
	if stats.Count > 0 {
		stats.AvgMS = stats.SumMS / float64(stats.Count)
	}
}

func combineDurationStats(left, right durationStats) durationStats {
	combined := durationStats{
		Count: left.Count + right.Count,
		SumMS: left.SumMS + right.SumMS,
	}
	switch {
	case left.Count > 0 && right.Count > 0:
		combined.MinMS = min(left.MinMS, right.MinMS)
		combined.MaxMS = max(left.MaxMS, right.MaxMS)
	case left.Count > 0:
		combined.MinMS = left.MinMS
		combined.MaxMS = left.MaxMS
	case right.Count > 0:
		combined.MinMS = right.MinMS
		combined.MaxMS = right.MaxMS
	}
	finalizeDurationStats(&combined)
	return combined
}

func parseDurationMS(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "µs", "us")
	raw = strings.ReplaceAll(raw, "μs", "us")
	if raw == "" {
		return 0, false
	}

	units := []struct {
		Suffix     string
		Multiplier float64
	}{
		{Suffix: "ns", Multiplier: 1.0 / 1_000_000},
		{Suffix: "us", Multiplier: 1.0 / 1_000},
		{Suffix: "ms", Multiplier: 1},
		{Suffix: "s", Multiplier: 1000},
		{Suffix: "m", Multiplier: 60_000},
		{Suffix: "h", Multiplier: 3_600_000},
	}
	for _, unit := range units {
		if strings.HasSuffix(raw, unit.Suffix) {
			value, err := strconv.ParseFloat(strings.TrimSuffix(raw, unit.Suffix), 64)
			if err != nil {
				return 0, false
			}
			return value * unit.Multiplier, true
		}
	}
	return 0, false
}

func sortedMethodRouteRows(counts map[string]int) []methodRouteRow {
	rows := make([]methodRouteRow, 0, len(counts))
	for key, count := range counts {
		method, route, _ := strings.Cut(key, "\x00")
		rows = append(rows, methodRouteRow{Method: method, Route: route, Count: count})
	}
	slices.SortFunc(rows, func(left, right methodRouteRow) int {
		if left.Count != right.Count {
			return right.Count - left.Count
		}
		if left.Method != right.Method {
			return strings.Compare(left.Method, right.Method)
		}
		return strings.Compare(left.Route, right.Route)
	})
	return rows
}

func sortedStatusRows(counts map[string]int) []statusRow {
	rows := make([]statusRow, 0, len(counts))
	for status, count := range counts {
		rows = append(rows, statusRow{Status: status, Count: count})
	}
	slices.SortFunc(rows, func(left, right statusRow) int {
		if left.Count != right.Count {
			return right.Count - left.Count
		}
		return strings.Compare(left.Status, right.Status)
	})
	return rows
}

func summarizeSuite(cases []caseResult) suiteSummary {
	summary := suiteSummary{CaseCount: len(cases)}
	for _, benchmarkCase := range cases {
		summary.TotalAPIs += benchmarkCase.Resources.APIs
		summary.TotalDocuments += benchmarkCase.Resources.APIDocuments
		for _, phase := range benchmarkCase.Phases {
			summary.PhaseCount++
			if phase.ExitCode != 0 || phase.Error != "" {
				summary.FailedPhases++
			}
			summary.TotalRequests += phase.HTTPMetrics.Requests
			summary.TotalResponses += phase.HTTPMetrics.Responses
			summary.TotalHTTPErrors += phase.HTTPMetrics.Errors
			summary.TotalDurationMS += phase.DurationMS
		}
	}
	return summary
}

func compareBaseline(cfg config, current suiteResult) (*comparisonResult, error) {
	var baseline suiteResult
	if err := readJSON(cfg.BaselinePath, &baseline); err != nil {
		return nil, fmt.Errorf("read baseline: %w", err)
	}

	baselinePhases := aggregateSuitePhases(baseline)
	currentPhases := aggregateSuitePhases(current)

	comparison := &comparisonResult{
		BaselinePath:      cfg.BaselinePath,
		RequestThreshold:  cfg.RequestCountThreshold,
		DurationThreshold: cfg.DurationThreshold,
		Rows:              []comparisonRow{},
	}
	keys := slices.Sorted(maps.Keys(currentPhases))
	for _, key := range keys {
		currentPhase := currentPhases[key]
		baselinePhase, ok := baselinePhases[key]
		if !ok {
			comparison.MissingBaselineData = append(comparison.MissingBaselineData, comparisonKeyLabel(key))
			continue
		}
		row := comparisonRow{
			CaseName:           currentPhase.CaseName,
			PhaseName:          currentPhase.PhaseName,
			BaselineRequests:   baselinePhase.Phase.HTTPMetrics.Requests,
			CurrentRequests:    currentPhase.Phase.HTTPMetrics.Requests,
			RequestDelta:       currentPhase.Phase.HTTPMetrics.Requests - baselinePhase.Phase.HTTPMetrics.Requests,
			BaselineDurationMS: baselinePhase.Phase.DurationMS,
			CurrentDurationMS:  currentPhase.Phase.DurationMS,
			DurationDeltaMS:    currentPhase.Phase.DurationMS - baselinePhase.Phase.DurationMS,
		}
		row.RequestDeltaPercent = percentDelta(row.BaselineRequests, row.CurrentRequests)
		row.DurationDeltaPercent = percentDelta64(row.BaselineDurationMS, row.CurrentDurationMS)
		row.RequestRegression = row.RequestDelta > 0 && row.RequestDeltaPercent > cfg.RequestCountThreshold
		row.DurationRegression = row.DurationDeltaMS > 0 && row.DurationDeltaPercent > cfg.DurationThreshold
		if row.RequestRegression {
			comparison.RequestRegressions++
		}
		if row.DurationRegression {
			comparison.DurationRegressions++
		}
		comparison.ComparedPhases++
		comparison.Rows = append(comparison.Rows, row)
	}

	return comparison, nil
}

type phaseAggregate struct {
	CaseName  string
	PhaseName string
	Phase     phaseResult
}

func aggregateSuitePhases(suite suiteResult) map[string]phaseAggregate {
	phaseSamples := map[string][]phaseAggregate{}
	for _, benchmarkCase := range suite.Cases {
		for _, phase := range benchmarkCase.Phases {
			key := phaseComparisonKey(benchmarkCase.Name, phase.Name)
			phaseSamples[key] = append(phaseSamples[key], phaseAggregate{
				CaseName:  benchmarkCase.Name,
				PhaseName: phase.Name,
				Phase:     phase,
			})
		}
	}

	aggregated := make(map[string]phaseAggregate, len(phaseSamples))
	for key, phases := range phaseSamples {
		aggregated[key] = aggregatePhaseResults(phases)
	}
	return aggregated
}

func aggregatePhaseResults(phases []phaseAggregate) phaseAggregate {
	if len(phases) == 0 {
		return phaseAggregate{}
	}
	requests := make([]int, 0, len(phases))
	durations := make([]int64, 0, len(phases))
	for _, phase := range phases {
		requests = append(requests, phase.Phase.HTTPMetrics.Requests)
		durations = append(durations, phase.Phase.DurationMS)
	}

	aggregated := phases[0]
	aggregated.Phase.HTTPMetrics.Requests = medianInt(requests)
	aggregated.Phase.DurationMS = medianInt64(durations)
	return aggregated
}

func medianInt(values []int) int {
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

func medianInt64(values []int64) int64 {
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

func phaseComparisonKey(caseName, phaseName string) string {
	return caseName + "\x00" + phaseName
}

func comparisonKeyLabel(key string) string {
	caseName, phaseName, _ := strings.Cut(key, "\x00")
	return caseName + "/" + phaseName
}

func percentDelta(baseline, current int) float64 {
	if baseline == 0 {
		if current == 0 {
			return 0
		}
		return 1
	}
	return float64(current-baseline) / float64(baseline)
}

func percentDelta64(baseline, current int64) float64 {
	if baseline == 0 {
		if current == 0 {
			return 0
		}
		return 1
	}
	return float64(current-baseline) / float64(baseline)
}

func writeSuiteOutputs(runDir string, suite suiteResult, cfg config) error {
	if err := writeJSON(filepath.Join(runDir, "results.json"), suite); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(renderSummary(suite)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.txt"), []byte(renderTerminalSummary(suite)), 0o644); err != nil {
		return err
	}
	if err := writeHistoryOutputs(runDir, suite, cfg); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, ".benchmark_artifacts_dir"), []byte(runDir+"\n"), 0o644)
}

func renderSummary(suite suiteResult) string {
	var b strings.Builder
	b.WriteString("# Declarative Benchmark Results\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", suite.RunID)
	if suite.GitCommit != "" {
		fmt.Fprintf(&b, "- Git commit: `%s`\n", suite.GitCommit)
	}
	fmt.Fprintf(&b, "- Base URL: `%s`\n", suite.BaseURL)
	fmt.Fprintf(&b, "- Duration: `%s`\n", suite.FinishedAt.Sub(suite.StartedAt).Round(time.Millisecond))
	fmt.Fprintf(&b, "- Cases: `%d`\n", suite.Summary.CaseCount)
	fmt.Fprintf(&b, "- Phases: `%d`\n", suite.Summary.PhaseCount)
	fmt.Fprintf(&b, "- HTTP requests: `%d`\n", suite.Summary.TotalRequests)
	fmt.Fprintf(&b, "- HTTP errors: `%d`\n\n", suite.Summary.TotalHTTPErrors)
	b.WriteString(
		"Suite duration includes fixture generation and destructive org reset. " +
			"Phase rows measure only `kongctl apply` commands.\n\n",
	)

	b.WriteString("| Case | Phase | Rep | APIs | API documents | Duration | Requests | Responses | Errors |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, benchmarkCase := range suite.Cases {
		for _, phase := range benchmarkCase.Phases {
			fmt.Fprintf(
				&b,
				"| `%s` | `%s` | %d | %d | %d | %s | %d | %d | %d |\n",
				benchmarkCase.Name,
				phase.Name,
				caseRepetition(benchmarkCase),
				benchmarkCase.Resources.APIs,
				benchmarkCase.Resources.APIDocuments,
				time.Duration(phase.DurationMS)*time.Millisecond,
				phase.HTTPMetrics.Requests,
				phase.HTTPMetrics.Responses,
				phase.HTTPMetrics.Errors,
			)
		}
	}

	if suite.Comparison != nil {
		b.WriteString("\n## Baseline Comparison\n\n")
		fmt.Fprintf(&b, "- Baseline: `%s`\n", suite.Comparison.BaselinePath)
		fmt.Fprintf(&b, "- Compared phases: `%d`\n", suite.Comparison.ComparedPhases)
		fmt.Fprintf(&b, "- Request regressions: `%d`\n", suite.Comparison.RequestRegressions)
		fmt.Fprintf(&b, "- Duration regressions: `%d`\n\n", suite.Comparison.DurationRegressions)
		b.WriteString("| Case | Phase | Request Δ | Duration Δ |\n")
		b.WriteString("| --- | --- | ---: | ---: |\n")
		for _, row := range suite.Comparison.Rows {
			fmt.Fprintf(
				&b,
				"| `%s` | `%s` | %+d (%.1f%%) | %+s (%.1f%%) |\n",
				row.CaseName,
				row.PhaseName,
				row.RequestDelta,
				row.RequestDeltaPercent*100,
				time.Duration(row.DurationDeltaMS)*time.Millisecond,
				row.DurationDeltaPercent*100,
			)
		}
	}

	return b.String()
}

func renderTerminalSummary(suite suiteResult) string {
	var b strings.Builder
	b.WriteString("Declarative benchmark summary\n")
	fmt.Fprintf(&b, "Run: %s", suite.RunID)
	if suite.GitCommit != "" {
		fmt.Fprintf(&b, "  Commit: %s", suite.GitCommit)
	}
	fmt.Fprintf(&b, "\nBase URL: %s\n", suite.BaseURL)
	fmt.Fprintf(
		&b,
		"Suite: %s  Cases: %d  Phases: %d  Requests: %d  Responses: %d  Errors: %d\n",
		suite.FinishedAt.Sub(suite.StartedAt).Round(time.Millisecond),
		suite.Summary.CaseCount,
		suite.Summary.PhaseCount,
		suite.Summary.TotalRequests,
		suite.Summary.TotalResponses,
		suite.Summary.TotalHTTPErrors,
	)
	if suite.Summary.FailedPhases > 0 {
		fmt.Fprintf(&b, "Failed phases: %d\n", suite.Summary.FailedPhases)
	}
	b.WriteString("Note: suite duration includes fixture generation and destructive org reset.\n")
	b.WriteString("      phase duration measures only the kongctl apply command.\n\n")

	writeTerminalPhaseRow(&b, "CASE", "PHASE", "REP", "APIS", "DOCS", "DURATION", "REQ", "RESP", "ERR")
	writeTerminalPhaseRow(
		&b,
		strings.Repeat("-", 24),
		strings.Repeat("-", 13),
		strings.Repeat("-", 4),
		strings.Repeat("-", 5),
		strings.Repeat("-", 5),
		strings.Repeat("-", 10),
		strings.Repeat("-", 6),
		strings.Repeat("-", 6),
		strings.Repeat("-", 6),
	)
	for _, benchmarkCase := range suite.Cases {
		for _, phase := range benchmarkCase.Phases {
			writeTerminalPhaseRow(
				&b,
				benchmarkCase.Name,
				phase.Name,
				strconv.Itoa(caseRepetition(benchmarkCase)),
				strconv.Itoa(benchmarkCase.Resources.APIs),
				strconv.Itoa(benchmarkCase.Resources.APIDocuments),
				formatMilliseconds(phase.DurationMS),
				strconv.Itoa(phase.HTTPMetrics.Requests),
				strconv.Itoa(phase.HTTPMetrics.Responses),
				strconv.Itoa(phase.HTTPMetrics.Errors),
			)
		}
	}

	if suite.Comparison != nil {
		b.WriteString("\nBaseline comparison\n")
		fmt.Fprintf(&b, "Baseline: %s\n", suite.Comparison.BaselinePath)
		fmt.Fprintf(
			&b,
			"Compared phases: %d  Request regressions: %d  Duration regressions: %d\n\n",
			suite.Comparison.ComparedPhases,
			suite.Comparison.RequestRegressions,
			suite.Comparison.DurationRegressions,
		)
		writeTerminalComparisonRow(&b, "CASE", "PHASE", "REQ_DELTA", "REQ_DELTA_%", "DUR_DELTA", "DUR_DELTA_%")
		writeTerminalComparisonRow(
			&b,
			strings.Repeat("-", 24),
			strings.Repeat("-", 13),
			strings.Repeat("-", 10),
			strings.Repeat("-", 12),
			strings.Repeat("-", 8),
			strings.Repeat("-", 10),
		)
		for _, row := range suite.Comparison.Rows {
			writeTerminalComparisonRow(
				&b,
				row.CaseName,
				row.PhaseName,
				fmt.Sprintf("%+d", row.RequestDelta),
				fmt.Sprintf("%+.1f%%", row.RequestDeltaPercent*100),
				formatMilliseconds(row.DurationDeltaMS),
				fmt.Sprintf("%+.1f%%", row.DurationDeltaPercent*100),
			)
		}
	}

	return b.String()
}

func writeTerminalPhaseRow(
	b *strings.Builder,
	caseName, phase, repetition, apis, docs, duration, requests, responses, errors string,
) {
	fmt.Fprintf(
		b,
		"%-24s %-13s %4s %5s %5s %10s %6s %6s %6s\n",
		caseName,
		phase,
		repetition,
		apis,
		docs,
		duration,
		requests,
		responses,
		errors,
	)
}

func writeTerminalComparisonRow(
	b *strings.Builder,
	caseName, phase, requestDelta, requestDeltaPercent, durationDelta, durationDeltaPercent string,
) {
	fmt.Fprintf(
		b,
		"%-24s %-13s %10s %12s %10s %12s\n",
		caseName,
		phase,
		requestDelta,
		requestDeltaPercent,
		durationDelta,
		durationDeltaPercent,
	)
}

func formatMilliseconds(ms int64) string {
	return (time.Duration(ms) * time.Millisecond).String()
}

func caseRepetition(benchmarkCase caseResult) int {
	if benchmarkCase.Repetition < 1 {
		return 1
	}
	return benchmarkCase.Repetition
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func appendErrorMessage(existing, next string) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)
	if existing == "" {
		return next
	}
	if next == "" {
		return existing
	}
	return existing + "; " + next
}

func benchmarkRunID(started time.Time) string {
	if v := strings.TrimSpace(os.Getenv("KONGCTL_BENCHMARK_RUN_ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("GITHUB_RUN_ID")); v != "" {
		return v
	}
	return started.Format("20060102-150405")
}

func gitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func setEnvFromBenchmark(target, source string) {
	if strings.TrimSpace(os.Getenv(target)) != "" {
		return
	}
	setEnvFromBenchmarkOverride(target, source)
}

func setEnvFromBenchmarkOverride(target, source string) {
	if value := strings.TrimSpace(os.Getenv(source)); value != "" {
		_ = os.Setenv(target, value)
	}
}

func setDefaultEnv(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) == "" {
		_ = os.Setenv(key, value)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on", "y":
		return true
	case "0", "false", "no", "off", "n":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}

func floatEnv(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func benchmarkCommandLogLevel() string {
	if value := strings.TrimSpace(os.Getenv("KONGCTL_BENCHMARK_LOG_LEVEL")); value != "" {
		return value
	}
	return "debug"
}

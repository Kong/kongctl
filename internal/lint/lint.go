package lint

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/daveshanley/vacuum/motor"
	"github.com/daveshanley/vacuum/rulesets"
	"sigs.k8s.io/yaml"
)

// Severity represents the severity level of a linting violation.
type Severity int

const (
	// SeverityHint is the lowest severity level.
	SeverityHint Severity = iota
	// SeverityInfo represents informational violations.
	SeverityInfo
	// SeverityWarn represents warning violations.
	SeverityWarn
	// SeverityError represents error violations (highest severity).
	SeverityError
)

var severityStrings = [...]string{
	"hint",
	"info",
	"warn",
	"error",
}

// String returns the string representation of a Severity level.
func (s Severity) String() string {
	idx := int(s)
	if idx >= 0 && idx < len(severityStrings) {
		return severityStrings[idx]
	}
	return "unknown"
}

// Result represents a single linting violation found in a file.
type Result struct {
	Message  string `json:"message"        yaml:"message"`
	Path     string `json:"path"           yaml:"path"`
	Severity string `json:"severity"       yaml:"severity"`
	RuleID   string `json:"rule_id"        yaml:"rule_id"`
	Line     int    `json:"line"           yaml:"line"`
	Column   int    `json:"column"         yaml:"column"`
	File     string `json:"file,omitempty" yaml:"file,omitempty"`
}

// Output contains the aggregated linting results across all files.
type Output struct {
	TotalCount int      `json:"total_count" yaml:"total_count"`
	FailCount  int      `json:"fail_count"  yaml:"fail_count"`
	Results    []Result `json:"results"     yaml:"results"`
}

// ParseSeverity converts a severity string to its Severity enum value.
// Returns SeverityWarn if the string is not recognized.
func ParseSeverity(s string) Severity {
	for i, str := range severityStrings {
		if s == str {
			return Severity(i)
		}
	}
	return SeverityWarn
}

// getRuleSet reads the ruleset bytes and returns a RuleSet object,
// merging with default rulesets if the custom ruleset extends them.
func getRuleSet(ruleSetBytes []byte) (*rulesets.RuleSet, error) {
	customRuleSet, err := rulesets.CreateRuleSetFromData(ruleSetBytes)
	if err != nil {
		return nil, fmt.Errorf("error creating ruleset: %w", err)
	}

	extends := customRuleSet.GetExtendsValue()
	if len(extends) > 0 {
		defaultRuleSet := rulesets.BuildDefaultRuleSets()
		return defaultRuleSet.GenerateRuleSetFromSuppliedRuleSet(customRuleSet), nil
	}

	return customRuleSet, nil
}

// isOpenAPISpec checks whether the given bytes represent an OpenAPI
// specification by looking for the "openapi" top-level key.
func isOpenAPISpec(fileBytes []byte) bool {
	var contents map[string]interface{}
	if err := yaml.Unmarshal(fileBytes, &contents); err != nil {
		return false
	}
	return contents["openapi"] != nil
}

// Bytes lints raw file bytes against a pre-parsed ruleset.
func Bytes(
	fileBytes []byte,
	ruleSet *rulesets.RuleSet,
	failSeverity string,
	onlyFailures bool,
	fileName string,
) *Output {
	ruleSetResults := motor.ApplyRulesToRuleSet(&motor.RuleSetExecution{
		RuleSet:           ruleSet,
		Spec:              fileBytes,
		SkipDocumentCheck: !isOpenAPISpec(fileBytes),
		AllowLookup:       true,
		SilenceLogs:       true,
	})

	var (
		failingCount int
		totalCount   int
		results      []Result
	)

	failSev := ParseSeverity(failSeverity)

	for _, r := range ruleSetResults.Results {
		sev := r.RuleSeverity
		if sev == "" && r.Rule != nil {
			sev = r.Rule.Severity
		}

		sevLevel := ParseSeverity(sev)

		if onlyFailures && sevLevel < failSev {
			continue
		}
		if sevLevel >= failSev {
			failingCount++
		}
		totalCount++

		line := 0
		col := 0
		if r.StartNode != nil {
			line = r.StartNode.Line
			col = r.StartNode.Column
		}

		ruleID := r.RuleId
		if ruleID == "" && r.Rule != nil {
			ruleID = r.Rule.Id
		}

		path := r.Path
		if path == "" && r.Rule != nil {
			if given, ok := r.Rule.Given.(string); ok {
				path = given
			}
		}

		results = append(results, Result{
			Message:  r.Message,
			Path:     path,
			Severity: sev,
			RuleID:   ruleID,
			Line:     line,
			Column:   col,
			File:     fileName,
		})
	}

	// Sort results by file, then line, then column for consistent output
	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		if results[i].Line != results[j].Line {
			return results[i].Line < results[j].Line
		}
		return results[i].Column < results[j].Column
	})

	return &Output{
		TotalCount: totalCount,
		FailCount:  failingCount,
		Results:    results,
	}
}

// Content lints raw content bytes against a ruleset provided as bytes.
// This is useful for linting stdin or other non-file sources.
func Content(
	content []byte,
	ruleSetBytes []byte,
	failSeverity string,
	onlyFailures bool,
	sourceName string,
) (*Output, error) {
	ruleSet, err := getRuleSet(ruleSetBytes)
	if err != nil {
		return nil, err
	}
	return Bytes(content, ruleSet, failSeverity, onlyFailures, sourceName), nil
}

// File reads a file from disk and lints it against the provided ruleset.
func File(
	filePath string,
	ruleSetBytes []byte,
	failSeverity string,
	onlyFailures bool,
) (*Output, error) {
	ruleSet, err := getRuleSet(ruleSetBytes)
	if err != nil {
		return nil, err
	}

	fileBytes, err := readFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file %q: %w", filePath, err)
	}

	return Bytes(fileBytes, ruleSet, failSeverity, onlyFailures, filePath), nil
}

// Files lints multiple files against the same ruleset, returning
// aggregated results.
func Files(
	filePaths []string,
	ruleSetBytes []byte,
	failSeverity string,
	onlyFailures bool,
) (*Output, error) {
	ruleSet, err := getRuleSet(ruleSetBytes)
	if err != nil {
		return nil, err
	}

	aggregated := &Output{
		Results: []Result{},
	}

	for _, fp := range filePaths {
		fileBytes, err := readFile(fp)
		if err != nil {
			return nil, fmt.Errorf("failed to read input file %q: %w", fp, err)
		}

		result := Bytes(fileBytes, ruleSet, failSeverity, onlyFailures, fp)
		aggregated.TotalCount += result.TotalCount
		aggregated.FailCount += result.FailCount
		aggregated.Results = append(aggregated.Results, result.Results...)
	}

	// Sort the aggregated results globally for deterministic output
	sort.Slice(aggregated.Results, func(i, j int) bool {
		if aggregated.Results[i].File != aggregated.Results[j].File {
			return aggregated.Results[i].File < aggregated.Results[j].File
		}
		if aggregated.Results[i].Line != aggregated.Results[j].Line {
			return aggregated.Results[i].Line < aggregated.Results[j].Line
		}
		return aggregated.Results[i].Column < aggregated.Results[j].Column
	})

	return aggregated, nil
}

// FormatPlain writes the linting output in plain text format to the writer.
func FormatPlain(w io.Writer, output *Output) error {
	if output.TotalCount == 0 {
		return nil
	}

	if _, err := fmt.Fprintf(w, "Linting Violations: %d\n", output.TotalCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Failures: %d\n\n", output.FailCount); err != nil {
		return err
	}

	for _, v := range output.Results {
		var prefix string
		if v.File != "" {
			prefix = fmt.Sprintf("%s:%d:%d: ", v.File, v.Line, v.Column)
		} else {
			prefix = fmt.Sprintf("%d:%d: ", v.Line, v.Column)
		}
		if _, err := fmt.Fprintf(w, "%s[%s] %s\n",
			prefix, v.Severity, v.Message,
		); err != nil {
			return err
		}
	}

	return nil
}

// FormatJSON writes the linting output in JSON format to the writer.
func FormatJSON(w io.Writer, output *Output) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// FormatYAML writes the linting output in YAML format to the writer.
func FormatYAML(w io.Writer, output *Output) error {
	data, err := yaml.Marshal(output)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// FormatOutput writes linting output in the specified format.
func FormatOutput(w io.Writer, output *Output, format string) error {
	switch format {
	case "json":
		return FormatJSON(w, output)
	case "yaml":
		return FormatYAML(w, output)
	case "text", "":
		return FormatPlain(w, output)
	default:
		return fmt.Errorf("unsupported output format: %q", format)
	}
}

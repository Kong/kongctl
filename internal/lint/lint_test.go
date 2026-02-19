package lint

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected Severity
	}{
		{"hint", SeverityHint},
		{"info", SeverityInfo},
		{"warn", SeverityWarn},
		{"error", SeverityError},
		{"unknown", SeverityWarn},
		{"", SeverityWarn},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseSeverity(tt.input))
		})
	}
}

func TestSeverityString(t *testing.T) {
	assert.Equal(t, "hint", SeverityHint.String())
	assert.Equal(t, "info", SeverityInfo.String())
	assert.Equal(t, "warn", SeverityWarn.String())
	assert.Equal(t, "error", SeverityError.String())
	assert.Equal(t, "unknown", Severity(99).String())
}

func TestBytes_NoViolations(t *testing.T) {
	input := []byte(`
name: test-service
host: example.com
port: 8080
tags:
  - name: test-tag
    description: "A test tag"
`)

	ruleset := []byte(`
rules:
  must-have-name:
    description: "All documents must have a name"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`)
	rs, err := getRuleSet(ruleset)
	require.NoError(t, err)

	output := Bytes(input, rs, "error", false, "test.yaml")
	assert.Equal(t, 0, output.FailCount)
}

func TestBytes_WithViolations(t *testing.T) {
	input := []byte(`
host: example.com
port: 8080
`)

	ruleset := []byte(`
rules:
  must-have-name:
    description: "All documents must have a name"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`)
	rs, err := getRuleSet(ruleset)
	require.NoError(t, err)

	output := Bytes(input, rs, "error", false, "test.yaml")
	assert.Greater(t, output.TotalCount, 0)
	assert.Greater(t, output.FailCount, 0)
	assert.NotEmpty(t, output.Results)

	// Check file is set on results
	for _, r := range output.Results {
		assert.Equal(t, "test.yaml", r.File)
	}
}

func TestBytes_OnlyFailures(t *testing.T) {
	input := []byte(`
host: example.com
`)

	ruleset := []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
  should-have-port:
    description: "port is recommended"
    given: "$"
    severity: info
    then:
      field: port
      function: truthy
`)
	rs, err := getRuleSet(ruleset)
	require.NoError(t, err)

	// Without onlyFailures, should include both
	outputAll := Bytes(input, rs, "error", false, "test.yaml")

	// With onlyFailures and fail-severity=error, should exclude info
	outputFailOnly := Bytes(input, rs, "error", true, "test.yaml")

	assert.GreaterOrEqual(t, outputAll.TotalCount, outputFailOnly.TotalCount)
}

func TestFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create input file
	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`host: example.com`), 0o600)
	require.NoError(t, err)

	// Create ruleset
	ruleset := []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`)

	output, err := File(inputFile, ruleset, "error", false)
	require.NoError(t, err)
	assert.NotNil(t, output)
	assert.Greater(t, output.TotalCount, 0)
}

func TestFile_NotFound(t *testing.T) {
	ruleset := []byte(`
rules:
  test-rule:
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`)

	_, err := File("/nonexistent/file.yaml", ruleset, "error", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read input file")
}

func TestFile_InvalidRuleset(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(inputFile, []byte(`name: test`), 0o600)
	require.NoError(t, err)

	_, err = File(inputFile, []byte(`not valid yaml: [`), "error", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error creating ruleset")
}

func TestFiles_Multiple(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two input files
	file1 := filepath.Join(tmpDir, "config1.yaml")
	file2 := filepath.Join(tmpDir, "config2.yaml")
	err := os.WriteFile(file1, []byte(`host: a.example.com`), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte(`host: b.example.com`), 0o600)
	require.NoError(t, err)

	ruleset := []byte(`
rules:
  must-have-name:
    description: "name is required"
    given: "$"
    severity: error
    then:
      field: name
      function: truthy
`)

	output, err := Files([]string{file1, file2}, ruleset, "error", false)
	require.NoError(t, err)
	assert.NotNil(t, output)
	// Both files should have violations
	assert.Greater(t, output.TotalCount, 1)
}

func TestFormatPlain(t *testing.T) {
	output := &Output{
		TotalCount: 2,
		FailCount:  1,
		Results: []Result{
			{
				Message:  "name is required",
				Severity: "error",
				Line:     5,
				Column:   3,
				File:     "config.yaml",
			},
			{
				Message:  "port is recommended",
				Severity: "info",
				Line:     10,
				Column:   1,
				File:     "config.yaml",
			},
		},
	}

	var buf bytes.Buffer
	err := FormatPlain(&buf, output)
	require.NoError(t, err)

	text := buf.String()
	assert.Contains(t, text, "Linting Violations: 2")
	assert.Contains(t, text, "Failures: 1")
	assert.Contains(t, text, "config.yaml:5:3: [error] name is required")
	assert.Contains(t, text, "config.yaml:10:1: [info] port is recommended")
}

func TestFormatPlain_NoViolations(t *testing.T) {
	output := &Output{
		TotalCount: 0,
		FailCount:  0,
		Results:    []Result{},
	}

	var buf bytes.Buffer
	err := FormatPlain(&buf, output)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestFormatJSON(t *testing.T) {
	output := &Output{
		TotalCount: 1,
		FailCount:  1,
		Results: []Result{
			{
				Message:  "name is required",
				Severity: "error",
				Line:     5,
				Column:   3,
				File:     "test.yaml",
			},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(&buf, output)
	require.NoError(t, err)

	text := buf.String()
	assert.Contains(t, text, `"total_count": 1`)
	assert.Contains(t, text, `"fail_count": 1`)
	assert.Contains(t, text, `"message": "name is required"`)
	assert.Contains(t, text, `"severity": "error"`)
}

func TestFormatYAML(t *testing.T) {
	output := &Output{
		TotalCount: 1,
		FailCount:  1,
		Results: []Result{
			{
				Message:  "name is required",
				Severity: "error",
				Line:     5,
				Column:   3,
			},
		},
	}

	var buf bytes.Buffer
	err := FormatYAML(&buf, output)
	require.NoError(t, err)

	text := buf.String()
	assert.Contains(t, text, "total_count: 1")
	assert.Contains(t, text, "fail_count: 1")
	assert.Contains(t, text, "message: name is required")
}

func TestFormatOutput_InvalidFormat(t *testing.T) {
	output := &Output{}
	var buf bytes.Buffer
	err := FormatOutput(&buf, output, "xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported output format")
}

func TestFormatOutput_AllFormats(t *testing.T) {
	output := &Output{
		TotalCount: 1,
		FailCount:  0,
		Results: []Result{
			{Message: "test", Severity: "info", Line: 1, Column: 1},
		},
	}

	for _, format := range []string{"text", "json", "yaml", ""} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			err := FormatOutput(&buf, output, format)
			assert.NoError(t, err)
		})
	}
}

func TestGetRuleSet_WithExtends(t *testing.T) {
	// A ruleset that extends the recommended set
	ruleset := []byte(`
extends: [[spectral:oas, recommended]]
rules:
  custom-rule:
    description: "Custom rule"
    given: "$"
    severity: warn
    then:
      field: info
      function: truthy
`)

	rs, err := getRuleSet(ruleset)
	require.NoError(t, err)
	assert.NotNil(t, rs)
}

func TestGetRuleSet_Standalone(t *testing.T) {
	ruleset := []byte(`
rules:
  custom-rule:
    description: "Custom rule"
    given: "$"
    severity: warn
    then:
      field: name
      function: truthy
`)

	rs, err := getRuleSet(ruleset)
	require.NoError(t, err)
	assert.NotNil(t, rs)
}

func TestIsOpenAPISpec(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "OpenAPI spec",
			input:    `openapi: "3.0.0"`,
			expected: true,
		},
		{
			name:     "not OpenAPI",
			input:    `name: my-service`,
			expected: false,
		},
		{
			name:     "invalid yaml",
			input:    `not valid: [yaml`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isOpenAPISpec([]byte(tt.input)))
		})
	}
}

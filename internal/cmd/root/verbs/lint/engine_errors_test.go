package lint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLintEngineFailure(t *testing.T) {
	dir := t.TempDir()
	inputs := filepath.Join(dir, "inputs")
	require.NoError(t, os.Mkdir(inputs, 0o700))
	input := "apis:\n  - force_delete: false\n"
	file := filepath.Join(inputs, "api.yaml")
	require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
	rules := filepath.Join(dir, "rules.yaml")
	require.NoError(t, os.WriteFile(rules, []byte(`rules:
  require-force-delete:
    given: '$.apis['
    severity: hint
    then:
      field: force_delete
      function: truthy
`), 0o600))

	for _, source := range []string{"file", "directory", "stdin"} {
		for _, format := range []string{"json", "yaml", "text"} {
			for _, severity := range []string{"hint", "info", "warn", "error"} {
				for _, failuresOnly := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/filter=%t", source, format, severity, failuresOnly), func(t *testing.T) {
						root := newTestRootWithLint(t)
						root.SetContext(t.Context())
						var stdout, stderr bytes.Buffer
						root.SetOut(&stdout)
						root.SetErr(&stderr)
						root.SetIn(strings.NewReader(input))
						path, diagnosticSource := file, file
						switch source {
						case "directory":
							path = inputs
						case "stdin":
							path, diagnosticSource = "-", "<stdin>"
						}
						root.SetArgs([]string{
							"lint", "-f", path, "-r", rules, "-o", format,
							"--fail-severity", severity, fmt.Sprintf("--display-only-failures=%t", failuresOnly),
						})
						err := root.Execute()
						require.Error(t, err)
						require.Contains(t, err.Error(), diagnosticSource)
						require.Contains(t, err.Error(), "require-force-delete")
						require.Contains(t, err.Error(), "$.apis[")
						require.Empty(t, stdout.String(), "engine failures must not emit successful lint output")
					})
				}
			}
		}
	}
}

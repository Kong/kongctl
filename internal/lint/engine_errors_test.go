package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/daveshanley/vacuum/motor"
	"github.com/stretchr/testify/require"
)

func TestRuleExecutionErrors(t *testing.T) {
	input := []byte("apis:\n  - force_delete: false\n")
	file := filepath.Join(t.TempDir(), "apis.yaml")
	require.NoError(t, os.WriteFile(file, input, 0o600))
	for _, tc := range []struct {
		name       string
		selector   string
		brokenRule bool
		violations int
	}{
		{name: "invalid", selector: "$.apis[", brokenRule: true},
		{name: "no matches", selector: "$.portals[*]"},
		{name: "violation", selector: "$.apis[*]", violations: 1},
	} {
		rules := fmt.Appendf(nil, `rules:
  require-force-delete:
    given: '%s'
    severity: error
    then:
      field: force_delete
      function: truthy
`, tc.selector)
		rs, err := getRuleSet(rules)
		require.NoError(t, err)
		for _, entry := range []struct {
			name string
			run  func() (*Output, error)
		}{
			{"bytes", func() (*Output, error) { return Bytes(input, rs, "error", false, file) }},
			{"content", func() (*Output, error) { return Content(input, rules, "error", false, file) }},
			{"file", func() (*Output, error) { return File(file, rules, "error", false) }},
			{"files", func() (*Output, error) { return Files([]string{file}, rules, "error", false) }},
		} {
			t.Run(tc.name+"/"+entry.name, func(t *testing.T) {
				out, err := entry.run()
				if tc.brokenRule {
					require.Error(t, err)
					require.Nil(t, out)
					require.Contains(t, err.Error(), file)
					var lookup *motor.RuleLookupError
					require.ErrorAs(t, err, &lookup)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, out)
				require.Equal(t, tc.violations, out.TotalCount)
				require.Equal(t, tc.violations, out.FailCount)
			})
		}
	}
}

func TestAllRuleExecutionErrorsReported(t *testing.T) {
	rules := []byte(`rules:
  broken-api:
    given: '$.apis['
    then:
      function: truthy
  broken-portal:
    given: '$.portals['
    then:
      function: truthy
`)
	out, err := Content([]byte("apis: []\n"), rules, "error", true, "<stdin>")
	require.Error(t, err)
	require.Nil(t, out)
	for _, detail := range []string{"<stdin>", "broken-api", "$.apis[", "broken-portal", "$.portals["} {
		require.Contains(t, err.Error(), detail)
	}
}

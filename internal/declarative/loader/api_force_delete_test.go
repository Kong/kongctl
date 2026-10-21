package loader

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIForceDelete(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		rs, err := New().parseYAML(strings.NewReader(`
apis:
  - ref: bookings
    name: Bookings
    force_delete: `+value+`
    versions:
      - ref: beta2
        version: beta.2
`), "test.yaml", ".")
		require.NoError(t, err)
		require.NotNil(t, rs.APIs[0].ForceDelete)
		require.Equal(t, value == "true", *rs.APIs[0].ForceDelete)
		require.Len(t, rs.APIVersions, 1)
	}
	for _, input := range []string{
		"apis:\n  - ref: api\n    name: API\n    kongctl:\n      allow_operation_removal: true\n",
		"apis:\n  - ref: api\n    name: API\n    kongctl:\n      force_delete: true\n",
		"portals:\n  - ref: portal\n    name: Portal\n    force_delete: true\n",
		"_defaults:\n  kongctl:\n    force_delete: true\n",
		`apis:
  - ref: api
    name: API
    versions:
      - ref: v1
        version: v1
        force_delete: true
`,
		"apis:\n  - ref: api\n    name: API\n    force_delete: yes-please\n",
	} {
		_, err := New().parseYAML(strings.NewReader(input), "test.yaml", ".")
		require.Error(t, err)
	}
}

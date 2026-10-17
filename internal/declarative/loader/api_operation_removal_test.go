package loader

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIOperationRemovalMetadata(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		rs, err := New().parseYAML(strings.NewReader(`
apis:
  - ref: bookings
    name: Bookings
    kongctl:
      allow_operation_removal: `+value+`
    versions:
      - ref: beta2
        version: beta.2
`), "test.yaml", ".")
		require.NoError(t, err)
		require.NotNil(t, rs.APIs[0].Kongctl.AllowOperationRemoval)
		require.Equal(t, value == "true", *rs.APIs[0].Kongctl.AllowOperationRemoval)
		require.Len(t, rs.APIVersions, 1)
	}
	for _, input := range []string{
		"portals:\n  - ref: portal\n    name: Portal\n    kongctl:\n      allow_operation_removal: true\n",
		"_defaults:\n  kongctl:\n    allow_operation_removal: true\n",
		`apis:
  - ref: api
    name: API
    versions:
      - ref: v1
        version: v1
        kongctl:
          allow_operation_removal: true
`,
		"apis:\n  - ref: api\n    name: API\n    kongctl:\n      allow_operation_removal: yes-please\n",
	} {
		_, err := New().parseYAML(strings.NewReader(input), "test.yaml", ".")
		require.Error(t, err)
	}
}

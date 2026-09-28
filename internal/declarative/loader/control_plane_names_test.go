package loader

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestControlPlaneNames(t *testing.T) {
	for _, tt := range []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "managed name is required",
			input: `control_planes:
  - ref: local-cp
`,
			wantErr: "name is required for control plane local-cp",
		},
		{
			name: "managed whitespace name is invalid",
			input: `control_planes:
  - ref: local-cp
    name: "   "
`,
			wantErr: "name is required for control plane local-cp",
		},
		{
			name: "managed names remain unique",
			input: `control_planes:
  - ref: first
    name: shared-name
  - ref: second
    name: shared-name
`,
			wantErr: "duplicate control_plane name 'shared-name'",
		},
		{
			name: "external blocks remain validated",
			input: `control_planes:
  - ref: external
    _external: {}
`,
			wantErr: "_external block must have either 'id' or 'selector'",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New().LoadFile(writeLoaderTestFile(t, tt.input))
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestControlPlaneNamesRemainIndependentOfRefs(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, `control_planes:
  - ref: managed-local-ref
    name: managed-konnect-name
  - ref: external-by-id
    _external:
      id: "11111111-1111-1111-1111-111111111111"
  - ref: external-by-id-selector
    _external:
      selector:
        matchFields:
          id: "22222222-2222-2222-2222-222222222222"
  - ref: external-by-name-selector
    _external:
      selector:
        matchFields:
          name: external-konnect-name
  - ref: external-with-explicit-name
    name: managed-konnect-name
    _external:
      selector:
        matchFields:
          name: managed-konnect-name
`))
	require.NoError(t, err)
	require.Len(t, rs.ControlPlanes, 5)
	require.Equal(t, "managed-konnect-name", rs.ControlPlanes[0].Name)
	for _, cp := range rs.ControlPlanes[1:4] {
		require.True(t, cp.IsExternal())
		require.Empty(t, cp.Name)
	}
	require.Equal(t, "managed-konnect-name", rs.ControlPlanes[4].Name)
}

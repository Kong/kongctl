package mesh

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// selectorCmd builds a command carrying the control plane selection flags, with
// the named ones marked as given on the command line.
func selectorCmd(t *testing.T, given ...string) *cobra.Command {
	t.Helper()

	cmdObj := &cobra.Command{Use: "mesh-selector-test"}
	meshcommon.AddControlPlaneFlags(cmdObj.Flags())
	for _, flag := range given {
		require.NoError(t, cmdObj.Flags().Set(flag, "value-for-"+flag))
	}
	return cmdObj
}

func TestExplicitControlPlaneSelector(t *testing.T) {
	cases := []struct {
		name  string
		given []string
		want  string
	}{
		{name: "nothing given", given: nil, want: ""},
		{name: "id", given: []string{meshcommon.ControlPlaneIDFlagName}, want: meshcommon.ControlPlaneIDFlagName},
		{
			name:  "name",
			given: []string{meshcommon.ControlPlaneNameFlagName},
			want:  meshcommon.ControlPlaneNameFlagName,
		},
		{name: "url", given: []string{meshcommon.ControlPlaneURLFlagName}, want: meshcommon.ControlPlaneURLFlagName},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper := &cmd.MockHelper{}
			helper.EXPECT().GetCmd().Return(selectorCmd(t, tc.given...))

			got, err := explicitControlPlaneSelector(helper)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// Two selectors name two different control planes, and this resolver serves
// writes and deletes, so the conflict is reported rather than resolved.
func TestExplicitControlPlaneSelectorRejectsConflicts(t *testing.T) {
	cases := [][]string{
		{meshcommon.ControlPlaneIDFlagName, meshcommon.ControlPlaneNameFlagName},
		{meshcommon.ControlPlaneURLFlagName, meshcommon.ControlPlaneIDFlagName},
		{meshcommon.ControlPlaneURLFlagName, meshcommon.ControlPlaneNameFlagName},
		{
			meshcommon.ControlPlaneURLFlagName,
			meshcommon.ControlPlaneIDFlagName,
			meshcommon.ControlPlaneNameFlagName,
		},
	}

	for _, given := range cases {
		helper := &cmd.MockHelper{}
		helper.EXPECT().GetCmd().Return(selectorCmd(t, given...))

		_, err := explicitControlPlaneSelector(helper)
		require.Error(t, err, "expected %v to conflict", given)
		require.Contains(t, err.Error(), "provide only one")
		for _, flag := range given {
			require.Contains(t, err.Error(), "--"+flag)
		}
	}
}

// A nil helper or command must not panic: some call paths build the helper
// before a command is attached.
func TestExplicitControlPlaneSelectorWithoutCommand(t *testing.T) {
	got, err := explicitControlPlaneSelector(nil)
	require.NoError(t, err)
	require.Equal(t, "", got)

	helper := &cmd.MockHelper{}
	helper.EXPECT().GetCmd().Return(nil)
	got, err = explicitControlPlaneSelector(helper)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

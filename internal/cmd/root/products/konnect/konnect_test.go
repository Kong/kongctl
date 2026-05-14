package konnect

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNewKonnectCmdCreateDoesNotExposeAuditLogs(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Create)
	require.NoError(t, err)
	require.False(t, hasSubcommandNamed(cmd, "audit-logs"))
}

func TestNewKonnectCmdListenExposesAuditLogs(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Listen)
	require.NoError(t, err)
	require.True(t, hasSubcommandNamed(cmd, "audit-logs"))
}

func TestNewKonnectCmdGetExposesAuditLogs(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Get)
	require.NoError(t, err)
	require.True(t, hasSubcommandNamed(cmd, "audit-logs"))
}

func TestNewKonnectCmdDeclarativeVerbsUseVerbSpecificExamples(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		verb             verbs.VerbValue
		wantShorthand    string
		wantExplicitForm string
	}{
		{
			name:             "plan",
			verb:             verbs.Plan,
			wantShorthand:    "kongctl plan -f api.yaml",
			wantExplicitForm: "kongctl plan konnect -f api.yaml",
		},
		{
			name:             "sync",
			verb:             verbs.Sync,
			wantShorthand:    "kongctl sync -f api.yaml",
			wantExplicitForm: "kongctl sync konnect -f api.yaml",
		},
		{
			name:             "diff",
			verb:             verbs.Diff,
			wantShorthand:    "kongctl diff -f api.yaml",
			wantExplicitForm: "kongctl diff konnect -f api.yaml",
		},
		{
			name:             "export",
			verb:             verbs.Export,
			wantShorthand:    "kongctl export -o ./exported-config",
			wantExplicitForm: "kongctl export konnect -o ./exported-config",
		},
		{
			name:             "apply",
			verb:             verbs.Apply,
			wantShorthand:    "kongctl apply -f api.yaml",
			wantExplicitForm: "kongctl apply konnect -f api.yaml",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd, err := NewKonnectCmd(tt.verb)
			require.NoError(t, err)
			require.Contains(t, cmd.Example, tt.wantShorthand)
			require.Contains(t, cmd.Example, tt.wantExplicitForm)
			require.NotContains(t, cmd.Example, "kongctl get konnect gateway control-planes")
		})
	}
}

func TestNewKonnectCmdGetExposesAnalytics(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Get)
	require.NoError(t, err)
	require.True(t, hasSubcommandNamed(cmd, "analytics"))
}

func TestNewKonnectCmdAdoptExposesAnalytics(t *testing.T) {
	t.Parallel()

	cmd, err := NewKonnectCmd(verbs.Adopt)
	require.NoError(t, err)
	require.True(t, hasSubcommandNamed(cmd, "analytics"))
	require.False(t, hasSubcommandNamed(cmd, "dashboard"))
}

func hasSubcommandNamed(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return true
		}
	}

	return false
}

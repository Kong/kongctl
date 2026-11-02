package identity

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestOrgChartImportRequiresExplicitIntentScopeAndPrivateReport(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		command, err := newOrgChartImportCommand(nil, nil)
		require.NoError(t, err)
		require.NoError(t, command.Flags().Set("directory-name", "default"))
		require.ErrorContains(t, command.Args(command, nil), "choose exactly one")
		mode := "execute"
		if dryRun {
			mode = "dry-run"
		}
		require.NoError(t, command.Flags().Set(mode, "false"))
		require.ErrorContains(t, command.Args(command, nil), "choose exactly one")
		require.NoError(t, command.Flags().Set(mode, "true"))
		require.ErrorContains(t, command.Args(command, nil), "file must not be empty")
		require.NoError(t, command.Flags().Set("file", "org.csv"))
		require.ErrorContains(t, command.Args(command, nil), "report-output-file")
		require.NoError(t, command.Flags().Set("report-output-file", "private-report.json"))
		require.NoError(t, command.Args(command, nil))
		require.NoError(t, command.ValidateRequiredFlags())
		require.NoError(t, command.ValidateFlagGroups())
		require.Error(t, command.Args(command, []string{"extra"}))
		require.NoError(t, command.Flags().Set("directory-id", fixtureCommandID))
		require.Error(t, command.Args(command, nil))
	}
	command, err := newOrgChartImportCommand(nil, nil)
	require.NoError(t, err)
	require.NoError(t, command.Flags().Set("directory-id", fixtureCommandID))
	require.NoError(t, command.Flags().Set("dry-run", "true"))
	require.NoError(t, command.Flags().Set("execute", "true"))
	require.ErrorContains(t, command.Args(command, nil), "choose exactly one")
	root, err := NewIdentityCmd(verbs.Create, nil, nil)
	require.NoError(t, err)
	for _, name := range []string{"org-chart-import", "org-chart-imports"} {
		found, _, err := root.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, "org-chart-import", found.Name())
	}
}

func TestOrgChartImportTextIsCountsOnly(t *testing.T) {
	value := &helpers.IdentityOrgChartImportSummary{
		Status: "reported_errors", DryRun: true, Rows: 5,
		Counts: helpers.IdentityOrgChartImportCounts{
			Users: &helpers.IdentityOrgChartImportUsers{
				Created: new(int64(1)), Updated: new(int64(2)),
				Unchanged: new(int64(3)), Inactive: new(int64(4)), Adopted: new(int64(5)),
			},
			Teams:       &helpers.IdentityOrgChartImportTeams{Created: new(int64(6)), Deleted: new(int64(7))},
			Memberships: &helpers.IdentityOrgChartImportMemberships{Added: new(int64(8)), Removed: new(int64(9))},
		}, WarningCount: 2, ErrorCount: 1,
	}
	var buffer bytes.Buffer
	require.NoError(t, printOrgChartImportSummary(&buffer, value))
	require.Contains(t, buffer.String(), "users.adopted")
	require.Contains(t, buffer.String(), "reported_errors")
	require.Contains(t, buffer.String(), "warning_count")
	require.NotContains(t, buffer.String(), "{")
}

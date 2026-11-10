package identity

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestOrgChartImportRequiresExplicitIntentScopeAndPrivateReport(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		command, err := newOrgChartImportCommand(verbs.Import, nil, nil)
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
		require.ErrorContains(t, command.Args(command, nil), "--file <path>")
		require.NoError(t, command.Flags().Set("file", "org.csv"))
		require.ErrorContains(t, command.Args(command, nil), "report-output-dir")
		require.NoError(t, command.Flags().Set("report-output-dir", "private-reports"))
		require.NoError(t, command.Args(command, nil))
		require.NoError(t, command.ValidateRequiredFlags())
		require.NoError(t, command.ValidateFlagGroups())
		require.Error(t, command.Args(command, []string{"extra"}))
		require.NoError(t, command.Flags().Set("directory-id", fixtureCommandID))
		require.Error(t, command.Args(command, nil))
	}
	command, err := newOrgChartImportCommand(verbs.Import, nil, nil)
	require.NoError(t, err)
	require.NoError(t, command.Flags().Set("directory-id", fixtureCommandID))
	require.NoError(t, command.Flags().Set("dry-run", "true"))
	require.NoError(t, command.Flags().Set("execute", "true"))
	require.ErrorContains(t, command.Args(command, nil), "choose exactly one")
	root, err := NewIdentityCmd(verbs.Import, nil, nil)
	require.NoError(t, err)
	for _, name := range []string{"org-chart", "org-charts"} {
		found, _, err := root.Find([]string{name})
		require.NoError(t, err)
		require.Equal(t, "org-chart", found.Name())
	}
}

func TestOrgChartImportListsAllMissingFlags(t *testing.T) {
	command, err := newOrgChartImportCommand(verbs.Import, nil, nil)
	require.NoError(t, err)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs(nil)
	err = command.Execute()
	require.EqualError(t, err, "required flags (missing or invalid):\n"+
		"  --file <path>\n"+
		"  --report-output-dir <directory>\n"+
		"  --dry-run or --execute (choose exactly one)")

	require.NoError(t, command.Flags().Set("file", "org.csv"))
	err = command.Args(command, nil)
	require.ErrorContains(t, err, "--report-output-dir <directory>")
	require.ErrorContains(t, err, "--dry-run or --execute")
	require.NotContains(t, err.Error(), "--file <path>")

	require.NoError(t, command.Flags().Set("report-output-dir", "reports"))
	require.NoError(t, command.Flags().Set("dry-run", "true"))
	require.NoError(t, command.Args(command, nil))
	require.NoError(t, command.Flags().Set("execute", "true"))
	require.EqualError(t, command.Args(command, nil), "required flags (missing or invalid):\n"+
		"  --dry-run or --execute (choose exactly one)")
}

func TestOrgChartImportRejectsReportFileFlag(t *testing.T) {
	command, err := newOrgChartImportCommand(verbs.Import, nil, nil)
	require.NoError(t, err)
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"--report-output-file", "report.json", "--dry-run"})
	require.ErrorContains(t, command.Execute(), "unknown flag: --report-output-file")
}

func TestOrgChartReportDirectoryGeneratesPathsForBothModes(t *testing.T) {
	command, err := newOrgChartImportCommand(verbs.Import, nil, nil)
	require.NoError(t, err)
	directory := filepath.Join(t.TempDir(), "reports")
	require.NoError(t, command.Flags().Set("report-output-dir", directory))
	for _, dryRun := range []bool{true, false} {
		path, err := orgChartReportPath(command, dryRun)
		require.NoError(t, err)
		require.Equal(t, directory, filepath.Dir(path))
		mode := "execute"
		if dryRun {
			mode = "preview"
		}
		require.Regexp(t, `^org-chart-`+mode+`-\d{8}T\d{6}\.\d{9}Z\.json$`, filepath.Base(path))
	}
	info, err := os.Stat(directory)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	require.Zero(t, info.Mode().Perm()&0o077)
	require.NoError(t, command.Flags().Set("file", "org.csv"))
	require.NoError(t, command.Flags().Set("dry-run", "true"))
	require.NoError(t, command.Args(command, nil))
	require.NoError(t, command.ValidateRequiredFlags())
	require.NoError(t, command.ValidateFlagGroups())
	require.Nil(t, command.Flags().Lookup("report-output-file"))
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

func TestOrgChartImportVerbUsesTheResourceNoun(t *testing.T) {
	var callbackVerb verbs.VerbValue
	command, err := newOrgChartImportCommand(verbs.Import, func(verb verbs.VerbValue, _ *cobra.Command) {
		callbackVerb = verb
	}, nil)
	require.NoError(t, err)
	require.Equal(t, verbs.Import, callbackVerb)
	require.Equal(t, "org-chart", command.Name())
	require.Contains(t, command.Aliases, "org-charts")
	require.ErrorContains(t, command.Args(command, nil), "choose exactly one")
	require.NoError(t, command.Flags().Set("dry-run", "true"))
	require.NoError(t, command.Flags().Set("file", "org.csv"))
	require.NoError(t, command.Flags().Set("report-output-dir", "private-reports"))
	require.NoError(t, command.Args(command, nil))
	require.NoError(t, command.ValidateRequiredFlags())
	require.NoError(t, command.ValidateFlagGroups())
	require.NoError(t, command.Flags().Set("execute", "true"))
	require.ErrorContains(t, command.Args(command, nil), "choose exactly one")
	_, err = newOrgChartImportCommand(verbs.Get, nil, nil)
	require.Error(t, err)
	root, err := NewIdentityCmd(verbs.Import, nil, nil)
	require.NoError(t, err)
	require.Len(t, root.Commands(), 1)
	for _, name := range []string{"org-chart", "org-charts"} {
		found, remaining, err := root.Find([]string{name})
		require.NoError(t, err)
		require.Empty(t, remaining)
		require.Equal(t, "org-chart", found.Name())
	}
}

func TestOrgChartImportHelpUsesPlainLanguage(t *testing.T) {
	command, err := newOrgChartImportCommand(verbs.Import, nil, nil)
	require.NoError(t, err)
	var output bytes.Buffer
	command.SetOut(&output)
	require.NoError(t, command.Help())
	help := output.String()
	require.Contains(t, help, "CSV file containing all users and their managers")
	require.Contains(t, help, "Required flags: --file, --report-output-dir")
	for _, field := range []string{"external_id", "email", "given_name", "family_name", "manager_external_id"} {
		require.Regexp(t, `(?m)^  `+field+`\s+Yes\s+`, help)
	}
	require.Regexp(t, `(?m)^  cost_center\s+No\s+`, help)
	require.Contains(t, help, "leave its value blank")
	require.Contains(t, help, "https://developer.konghq.com/ai-cost-management/org-chart/")
	require.Contains(t, help, "Directory for uniquely named private JSON reports (required; created if needed)")
	require.Contains(t, help, "- Default: [ default ]")
	require.NotContains(t, help, "canonical")
	require.NotContains(t, help, "0600")
	require.NotContains(t, help, "tech preview")
	for line := range strings.SplitSeq(command.Long, "\n") {
		require.LessOrEqual(t, len(line), 80, line)
	}
	require.ErrorContains(t, command.ValidateRequiredFlags(), "file")
	require.NoError(t, command.Flags().Set("file", "org-chart.csv"))
	require.ErrorContains(t, command.ValidateRequiredFlags(), "report-output-dir")
}

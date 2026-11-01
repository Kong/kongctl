package identity

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newOrgChartImportCommand(addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "org-chart-import", Aliases: []string{"org-chart-imports"},
		Short: "Upload a complete organization-chart CSV and privately capture its report",
		Long: "Validate or execute the native full-file organization-chart import. " +
			"Choose --dry-run to preview, or --execute to apply the complete file. " +
			"Execution can adopt existing users, change team relationships and deactivate absent imported users. " +
			"A required new 0600 report file preserves detailed diagnostics; normal output omits employee values. " +
			"Imports follow the API's matching/provisioning rules and do not stamp KongCTL ownership metadata.",
		PreRunE: parentPreRun,
	}
	command.Flags().String("directory-id", "", "Target directory UUID")
	command.Flags().String("directory-name", "", "Exact target directory name")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	command.Flags().StringP("file", "f", "", "Complete canonical UTF-8 CSV file")
	command.Flags().String("report-output-file", "", "New private report file (0600; never overwritten)")
	for _, flag := range []string{"file", "report-output-file"} {
		if err := command.MarkFlagRequired(flag); err != nil {
			return nil, err
		}
	}
	command.Flags().Bool("dry-run", false, "Validate/report without changing the hierarchy")
	command.Flags().Bool("execute", false, "Apply the complete file, including import-owned departures")
	command.MarkFlagsOneRequired("dry-run", "execute")
	command.MarkFlagsMutuallyExclusive("dry-run", "execute")
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.NoArgs(cmd, args); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		execute, _ := cmd.Flags().GetBool("execute")
		if dryRun == execute {
			return fmt.Errorf("choose exactly one of --dry-run or --execute")
		}
		for _, flag := range []string{"file", "report-output-file"} {
			value, _ := cmd.Flags().GetString(flag)
			if value == "" {
				return fmt.Errorf("%s must not be empty", flag)
			}
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verbs.Create, command)
	}
	command.RunE = runOrgChartImport
	return command, nil
}

func runOrgChartImport(command *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	format, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	path, _ := command.Flags().GetString("file")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open org-chart CSV: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("org-chart CSV must be a regular file")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read org-chart CSV: %w", err)
	}
	if _, err := helpers.ValidateIdentityOrgChartCSV(contents); err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}
	ctx := helper.GetContext()
	directory, err := groupDirectoryIDFromFlags(ctx, command, sdk.GetIdentityDirectoriesAPI(),
		int64(cfg.GetInt(common.RequestPageSizeConfigPath)))
	if err != nil {
		return err
	}
	dryRun, _ := command.Flags().GetBool("dry-run")
	output, _ := command.Flags().GetString("report-output-file")
	summary, importErr := helpers.ImportAndCaptureIdentityOrgChart(
		ctx, sdk.GetIdentityOrgChartImportAPI(), directory, contents, dryRun, output,
	)
	if summary == nil {
		return importErr
	}
	if format == cmdCommon.TEXT {
		if err := printOrgChartImportSummary(helper.GetStreams().Out, summary); err != nil {
			return err
		}
	} else {
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(summary)
	}
	return importErr
}

func printOrgChartImportSummary(out io.Writer, value *helpers.IdentityOrgChartImportSummary) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, err := fmt.Fprintf(w, "status\t%s\ndry_run\t%t\nrows\t%d\n", value.Status, value.DryRun, value.Rows)
	if err != nil {
		return err
	}
	users, teams, memberships := value.Counts.Users, value.Counts.Teams, value.Counts.Memberships
	for _, row := range []struct {
		name  string
		count *int64
	}{
		{"users.created", users.Created},
		{"users.updated", users.Updated},
		{"users.unchanged", users.Unchanged},
		{"users.inactive", users.Inactive},
		{"users.adopted", users.Adopted},
		{"teams.created", teams.Created},
		{"teams.deleted", teams.Deleted},
		{"memberships.added", memberships.Added},
		{"memberships.removed", memberships.Removed},
	} {
		if row.count != nil {
			if _, err := fmt.Fprintf(w, "%s\t%d\n", row.name, *row.count); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "warning_count\t%d\nerror_count\t%d\n", value.WarningCount, value.ErrorCount); err != nil {
		return err
	}
	for _, warning := range value.Warnings {
		row := "-"
		if warning.Row != nil {
			row = fmt.Sprint(*warning.Row)
		}
		if _, err := fmt.Fprintf(w, "warning\trow %s\t%s\n", row, warning.Type); err != nil {
			return err
		}
	}
	for _, failure := range value.Errors {
		row := "-"
		if failure.Row != nil {
			row = fmt.Sprint(*failure.Row)
		}
		if _, err := fmt.Fprintf(w, "error\trow %s\t%s\n", row, failure.Field); err != nil {
			return err
		}
	}
	return w.Flush()
}

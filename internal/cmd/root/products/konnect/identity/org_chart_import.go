package identity

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newOrgChartImportCommand(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	if verb != verbs.Import && verb != verbs.Create {
		return nil, fmt.Errorf("unsupported organization-chart import verb %q", verb)
	}
	command := &cobra.Command{
		Use: "org-chart", Aliases: []string{"org-charts"},
		Short: "Upload a complete organization-chart CSV and privately capture its report",
		Long: normalizers.LongDesc(`Import users and manager relationships from an organization-chart CSV file.

Required flags: --file, --report-output-dir, and exactly one of --dry-run
or --execute. Start with --dry-run to preview the import.

CSV columns (one row per user, with a header row):

  COLUMN               REQUIRED  DESCRIPTION
  external_id          Yes       Unique person ID from your HR system;
                                 matches the user across imports.
  email                Yes       User's email address.
  given_name           Yes       User's first name.
  family_name          Yes       User's last name.
  manager_external_id  Yes       Manager's external_id; builds the teams.
  cost_center          No        User's cost center.

The manager_external_id column is required, but leave its value blank for
the one user at the top of the organization. Column names are matched
without regard to case; extra columns are ignored.

Execution applies the complete roster. It can adopt existing users, change
team relationships and deactivate previously imported users absent from
the file. Imports follow the API's matching and provisioning rules; they
do not add KongCTL ownership metadata.

Each run saves a private JSON report in --report-output-dir. The directory
is created if needed. Filenames include preview or execute and a timestamp.
Existing reports are preserved; output includes the actual report path.
Normal output shows counts and diagnostic categories, omitting employee
values. Reuse the same directory for preview and execution.

CSV format and import behavior:
https://developer.konghq.com/ai-cost-management/org-chart/`),
		Example: normalizers.Examples(`
  # Preview the complete roster without changing users or teams
  kongctl import id org-chart \
    --file org-chart.csv \
    --dry-run \
    --report-output-dir ./import-reports

  # Apply the roster after reviewing the preview report
  kongctl import id org-chart \
    --file org-chart.csv \
    --execute \
    --report-output-dir ./import-reports`),
		PreRunE: parentPreRun,
	}
	if verb == verbs.Create {
		command.Use = "org-chart-import"
		command.Aliases = []string{"org-chart-imports"}
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().StringP("file", "f", "", "CSV file containing all users and their managers (required)")
	command.Flags().String("report-output-dir", "",
		"Directory for uniquely named private JSON reports (required; created if needed)")
	for _, flag := range []string{"file", "report-output-dir"} {
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
		var missing []string
		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			missing = append(missing, "  --file <path>")
		}
		reportDir, _ := cmd.Flags().GetString("report-output-dir")
		if reportDir == "" {
			missing = append(missing, "  --report-output-dir <directory>")
		}
		if dryRun == execute {
			missing = append(missing, "  --dry-run or --execute (choose exactly one)")
		}
		if len(missing) != 0 {
			return fmt.Errorf("required flags (missing or invalid):\n%s", strings.Join(missing, "\n"))
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
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
	output, err := orgChartReportPath(command, dryRun)
	if err != nil {
		return err
	}
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

func orgChartReportPath(command *cobra.Command, dryRun bool) (string, error) {
	directory, _ := command.Flags().GetString("report-output-dir")
	if directory == "" {
		return "", fmt.Errorf("report-output-dir must not be empty")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create report output directory: %w", err)
	}
	mode := "execute"
	if dryRun {
		mode = "preview"
	}
	name := "org-chart-" + mode + "-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".json"
	return filepath.Join(directory, name), nil
}

func printOrgChartImportSummary(out io.Writer, value *helpers.IdentityOrgChartImportSummary) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(w, "report_output_file\t%s\n", value.ReportOutputFile); err != nil {
		return err
	}
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

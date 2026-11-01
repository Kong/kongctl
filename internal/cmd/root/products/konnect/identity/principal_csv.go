package identity

import (
	"fmt"
	"io"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newPrincipalCSVInspectCommand(addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal-csv", Short: "Validate a principal CSV locally and show structural counts",
		Long: "Validate principal CSV headers, rows and supported nested declarations without contacting Konnect. " +
			"Use the CSV through a YAML !csv principal collection for plan/apply/diff/sync/delete. " +
			"Inspection does not verify remote ownership, references or UUID assertions.",
		Args: cobra.NoArgs, PreRunE: parentPreRun,
	}
	command.Flags().StringP("file", "f", "", "Local principal CSV file")
	command.Flags().String("metadata-mode", "", "Public metadata policy: merge or sync (explicitly required)")
	for _, flag := range []string{"file", "metadata-mode"} {
		if err := command.MarkFlagRequired(flag); err != nil {
			return nil, err
		}
	}
	if addParentFlags != nil {
		addParentFlags(verbs.Inspect, command)
	}
	command.RunE = func(command *cobra.Command, args []string) error {
		path, _ := command.Flags().GetString("file")
		mode, _ := command.Flags().GetString("metadata-mode")
		value, err := loader.InspectPrincipalCSV(path, mode)
		if err != nil {
			return err
		}
		helper := cmdpkg.BuildHelper(command, args)
		format, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			return printPrincipalCSVSummary(helper.GetStreams().Out, value)
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(value)
		return nil
	}
	return command, nil
}

func printPrincipalCSVSummary(out io.Writer, value *loader.PrincipalCSVSummary) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, err := fmt.Fprintf(w,
		"rows\t%d\nidentities\t%d\napi_keys\t%d\nbasic_auths\t%d\nmetadata_mode\t%s\nremote_state_checked\tfalse\n",
		value.Rows, value.Identities, value.APIKeys, value.BasicAuths, value.MetadataMode)
	if err != nil {
		return err
	}
	return w.Flush()
}

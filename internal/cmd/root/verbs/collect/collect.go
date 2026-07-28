package collect

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/cmd/root/verbs/collect/supportdata"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

const (
	Verb = verbs.Collect
)

var (
	collectUse = Verb.String()

	collectShort = i18n.T("root.verbs.collect.collectShort", "Collect diagnostic information")

	collectLong = normalizers.LongDesc(i18n.T("root.verbs.collect.collectLong",
		`Use collect to gather diagnostic information from Kong deployments.

This command collects logs, configuration, system information, and other
diagnostic data for troubleshooting and support purposes.`))

	collectExamples = normalizers.Examples(i18n.T("root.verbs.collect.collectExamples",
		fmt.Sprintf(`
        # Collect support data from Konnect-managed data planes
        %[1]s collect support-data konnect --control-plane my-cp

        # Collect support data from on-prem Kubernetes deployment
        %[1]s collect support-data on-prem --runtime kubernetes --namespace kong

        # Collect with sanitization (removes sensitive data)
        %[1]s collect support-data on-prem --sanitize

        # Collect with custom output directory
        %[1]s collect support-data on-prem --output-dir ./support-data
        `, meta.CLIName)))
)

// NewCollectCmd creates the parent collect command.
//
// This function follows kongctl's command factory pattern where each
// verb is created by a New*Cmd function that returns a (*cobra.Command, error).
func NewCollectCmd() (*cobra.Command, error) {
	collectCommand := &cobra.Command{
		Use:     collectUse,
		Short:   collectShort,
		Long:    collectLong,
		Example: collectExamples,
		Aliases: []string{"c"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	collectCommand.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
	}

	// Add support-data subcommand
	collectCommand.AddCommand(supportdata.NewSupportDataCmd())

	return collectCommand, nil
}

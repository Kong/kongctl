package supportdata

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

var (
	supportDataUse = "support-data"

	supportDataShort = i18n.T("root.verbs.collect.supportdata.supportDataShort",
		"Collect support data from Kong deployments")

	supportDataLong = normalizers.LongDesc(i18n.T("root.verbs.collect.supportdata.supportDataLong",
		`Collect logs, configuration, and system information from Kong
deployments for troubleshooting and support purposes.

Use the appropriate subcommand based on your deployment type:
- konnect:  For Konnect-managed data planes
- on-prem:  For self-managed Kong Gateway (Docker, Kubernetes, VM)

The command produces a ZIP archive containing:
- Kong configuration (via Admin API)
- Container/pod logs
- System information
- Kong process details`))

	supportDataExamples = normalizers.Examples(i18n.T("root.verbs.collect.supportdata.supportDataExamples",
		fmt.Sprintf(`
        # Collect from Konnect-managed data planes
        %[1]s collect support-data konnect --control-plane my-cp

        # Collect from on-prem Kubernetes deployment
        %[1]s collect support-data on-prem --runtime kubernetes --namespace kong

        # Collect from on-prem with sanitization
        %[1]s collect support-data on-prem --sanitize
        `, meta.CLIName)))
)

// NewSupportDataCmd creates the support-data parent command.
func NewSupportDataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     supportDataUse,
		Aliases: []string{"support", "diag", "diagnostics"},
		Short:   supportDataShort,
		Long:    supportDataLong,
		Example: supportDataExamples,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Add target subcommands
	cmd.AddCommand(NewOnPremCmd())
	cmd.AddCommand(NewKonnectCmd())

	return cmd
}

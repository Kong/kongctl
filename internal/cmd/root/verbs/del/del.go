package del

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	onprem "github.com/kong/kongctl/internal/cmd/root/products/on-prem"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Delete
)

var (
	deleteuse = Verb.String()

	deleteShort = i18n.T("root.verbs.delete.deleteShort", "Delete objects")

	deleteLong = normalizers.LongDesc(i18n.T("root.verbs.delete.deleteLong",
		`Use delete to delete a new object.

Further sub-commands are required to determine which remote system is contacted (if necessary). 
The command will delete an object and report a result depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	deleteExamples = normalizers.Examples(i18n.T("root.verbs.delete.deleteExamples",
		fmt.Sprintf(`
		# Delete a Konnect Kong Gateway control plane (Konnect-first)
		%[1]s delete gateway control-plane <id>
		# Delete a Konnect Kong Gateway control plane (explicit)
		%[1]s delete konnect gateway control-plane <id>
		# Delete a Konnect portal by ID (Konnect-first)
		%[1]s delete portal 12345678-1234-1234-1234-123456789012
		# Delete a Konnect portal by name
		%[1]s delete portal my-portal
		`, meta.CLIName)))
)

func NewDeleteCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     deleteuse,
		Short:   deleteShort,
		Long:    deleteLong,
		Example: deleteExamples,
		Aliases: []string{"d", "D", "del", "rm", "DEL", "RM"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	// Add on-prem product command
	streams := &iostreams.IOStreams{}
	cmd.AddCommand(onprem.NewOnPremCmd(streams))

	// Add gateway command directly for Konnect-first pattern
	gatewayCmd, err := NewDirectGatewayCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(gatewayCmd)

	// Add portal command directly for Konnect-first pattern
	portalCmd, err := NewDirectPortalCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(portalCmd)

	return cmd, nil
}

package list

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
	Verb = verbs.List
)

var (
	listUse = Verb.String()

	listShort = i18n.T("root.verbs.list.listShort", "Retrieve object lists")

	listLong = normalizers.LongDesc(i18n.T("root.verbs.list.listLong",
		`Use list to retrieve a list of objects.

Further sub-commands are required to determine which remote system is contacted (if necessary). 
The command will return a list depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	listExamples = normalizers.Examples(i18n.T("root.verbs.list.listExamples",
		fmt.Sprintf(`
		# Retrieve Konnect portals
		%[1]s list portals
		# Retrieve Konnect APIs
		%[1]s list apis
		# Retrieve Konnect auth strategies
		%[1]s list auth-strategies
		# Retrieve Konnect control planes (Konnect-first)
		%[1]s list gateway control-planes
		# Retrieve Konnect control planes (explicit)
		%[1]s list konnect gateway control-planes
		`, meta.CLIName)))
)

func NewListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     listUse,
		Short:   listShort,
		Long:    listLong,
		Example: listExamples,
		Aliases: []string{"ls", "l"},
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

	// Add portal command directly for Konnect-first pattern
	portalCmd, err := NewDirectPortalCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(portalCmd)

	// Add API command directly for Konnect-first pattern
	apiCmd, err := NewDirectAPICmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(apiCmd)

	// Add auth strategy command directly for Konnect-first pattern
	authStrategyCmd, err := NewDirectAuthStrategyCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(authStrategyCmd)

	// Add gateway command directly for Konnect-first pattern
	gatewayCmd, err := NewDirectGatewayCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(gatewayCmd)

	return cmd, nil
}

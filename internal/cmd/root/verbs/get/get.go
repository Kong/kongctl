package get

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	profileCmd "github.com/kong/kongctl/internal/cmd/root/profile"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Get
)

var (
	getUse = Verb.String()

	getShort = i18n.T("root.verbs.get.getShort", "Retrieve objects")

	getLong = normalizers.LongDesc(i18n.T("root.verbs.get.getLong",
		`Use get to retrieve an object or list of objects.

Further sub-commands are required to determine which remote system is contacted (if necessary). 
The command will return an object or a list depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	getExamples = normalizers.Examples(i18n.T("root.verbs.get.getExamples",
		fmt.Sprintf(`
		# Retrieve Konnect portals
		%[1]s get portals
		# Retrieve Konnect APIs
		%[1]s get apis
		# Retrieve Konnect auth strategies
		%[1]s get auth-strategies
		# Retrieve Konnect control planes (Konnect-first)
		%[1]s get gateway control-planes
		# Retrieve Konnect control planes (explicit)
		%[1]s get konnect gateway control-planes
		`, meta.CLIName)))
)

func NewGetCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     getUse,
		Short:   getShort,
		Long:    getLong,
		Example: getExamples,
		Aliases: []string{"g", "G"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	cmd.AddCommand(profileCmd.NewProfileCmd())

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

	// Add me command directly for Konnect-first pattern
	meCmd, err := NewDirectMeCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(meCmd)

	return cmd, nil
}

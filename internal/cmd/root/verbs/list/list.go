package list

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
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
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return bindKonnectFlags(c, args)
		},
	}

	// Add Konnect-specific flags as persistent flags so they appear in help
	cmd.PersistentFlags().String(common.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			common.BaseURLConfigPath, common.BaseURLDefault))

	cmd.PersistentFlags().String(common.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			common.BaseURLFlagName, common.RegionConfigPath),
	)

	cmd.PersistentFlags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	cmd.PersistentFlags().Int(
		common.RequestPageSizeFlagName,
		common.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page for get and list operations.
- Config path: [ %s ]`,
			common.RequestPageSizeConfigPath))

	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

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

	cmd.AddCommand(newThemesCmd())

	organizationCmd, err := NewDirectOrganizationCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(organizationCmd)

	return cmd, nil
}

// bindKonnectFlags binds Konnect-specific flags to configuration
func bindKonnectFlags(c *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if f := c.Flags().Lookup(common.BaseURLFlagName); f != nil {
		if err := cfg.BindFlag(common.BaseURLConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RegionFlagName); f != nil {
		if err := cfg.BindFlag(common.RegionConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.PATFlagName); f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RequestPageSizeFlagName); f != nil {
		if err := cfg.BindFlag(common.RequestPageSizeConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

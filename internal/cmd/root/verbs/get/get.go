package get

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
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
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return bindKonnectFlags(c, args)
		},
	}

	// Add Konnect-specific flags as persistent flags so they appear in help
	cmd.PersistentFlags().String(common.BaseURLFlagName, common.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			common.BaseURLConfigPath))

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

	cmd.PersistentFlags().BoolP(
		cmdCommon.InteractiveFlagName,
		cmdCommon.InteractiveFlagShort,
		false,
		i18n.T("root.verbs.get.flags.interactive", "Launch the interactive resource browser."),
	)

	cmd.RunE = func(c *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(c, args)
		if _, err := helper.GetOutputFormat(); err != nil {
			return err
		}
		interactive, err := helper.IsInteractive()
		if err != nil {
			return err
		}
		if interactive {
			return navigator.Run(helper, navigator.Options{})
		}
		return c.Help()
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

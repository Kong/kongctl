package del

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
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
		`Use delete to delete objects.

When used with -f, deletes all resources defined in the declarative configuration
files from Konnect. This is equivalent to running:
  kongctl plan --mode delete -f <files> | kongctl sync --plan -

Without -f, further sub-commands are required to determine which resource to delete.
Output can be formatted in multiple ways to aid in further processing.`))

	deleteExamples = normalizers.Examples(i18n.T("root.verbs.delete.deleteExamples",
		fmt.Sprintf(`
		# Delete resources defined in declarative configuration
		%[1]s delete -f config.yaml
		%[1]s delete -f ./configs/ --recursive
		%[1]s delete -f config.yaml --dry-run
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
	var force, autoApprove bool

	// Create the declarative delete command to get its implementation
	declDeleteCmd, err := declarative.NewDeclarativeCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     deleteuse,
		Short:   deleteShort,
		Long:    deleteLong,
		Example: deleteExamples,
		Aliases: []string{"d", "D", "del", "rm", "DEL", "RM"},
		// When -f is provided, run declarative delete; otherwise show help
		RunE: func(c *cobra.Command, args []string) error {
			filenames, _ := c.Flags().GetStringSlice("filename")
			planFile, _ := c.Flags().GetString("plan")
			if len(filenames) > 0 || planFile != "" {
				return declDeleteCmd.RunE(c, args)
			}
			return c.Help()
		},
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			ctx = context.WithValue(ctx, products.Product, konnect.Product)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
			c.SetContext(ctx)

			if err := bindKonnectFlags(c, args); err != nil {
				return err
			}
			cmdpkg.SetDeleteForce(c, force)
			cmdpkg.SetDeleteAutoApprove(c, autoApprove)
			return nil
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

	cmd.PersistentFlags().BoolVar(&force, "force", false,
		"Force deletion even when related resources exist (not configurable)")
	cmd.PersistentFlags().BoolVar(&autoApprove, "approve", false,
		"Skip confirmation prompts for delete operations (not configurable)")

	// Add declarative flags from the declarative delete command
	cmd.Flags().AddFlagSet(declDeleteCmd.Flags())

	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

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

	return nil
}

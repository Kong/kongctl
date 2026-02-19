package konnect

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/api"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/auditlogs"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/authstrategy"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/eventgateway"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/me"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/organization"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/portal"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/regions"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Product = products.ProductValue("konnect")
)

var (
	konnectUse   = Product.String()
	konnectShort = i18n.T("root.products.konnect.konnectShort", "Manage Konnect resources")
	konnectLong  = normalizers.LongDesc(i18n.T("root.products.konnect.konnectLong",
		`The konnect command allows you to manage Kong Konnect resources.`))
	konnectExamples = normalizers.Examples(i18n.T("root.products.konnect.konnectExamples",
		fmt.Sprintf(`# Retrieve the Konnect Kong Gateway control planes from the current organization
		 %[1]s get konnect gateway control-planes`, meta.CLIName)))
)

func addFlags(verb verbs.VerbValue, cmd *cobra.Command) {
	if verb != verbs.Login && verb != verbs.Logout {
		cmd.Flags().String(common.BaseURLFlagName, "",
			fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
				common.BaseURLConfigPath, common.BaseURLDefault))
		cmd.Flags().String(common.RegionFlagName, "",
			fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
				common.BaseURLFlagName, common.RegionConfigPath),
		)
	}

	if verb != verbs.Login && verb != verbs.Logout {
		cmd.Flags().String(common.PATFlagName, "",
			fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
				common.PATConfigPath))
	}

	if verb == verbs.Get || verb == verbs.List {
		cmd.Flags().Int(
			common.RequestPageSizeFlagName,
			common.DefaultRequestPageSize,
			fmt.Sprintf(`Max number of results to include per response page for get and list operations.
- Config path: [ %s ]`,
				common.RequestPageSizeConfigPath))
	}
}

func bindFlags(c *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(common.BaseURLFlagName)
	if f != nil {
		err = cfg.BindFlag(common.BaseURLConfigPath, f)
		if err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(common.RegionFlagName)
	if f != nil {
		err = cfg.BindFlag(common.RegionConfigPath, f)
		if err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(common.PATFlagName)
	if f != nil { // might not be present depending on verb
		err = cfg.BindFlag(common.PATConfigPath, f)
		if err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(common.RequestPageSizeFlagName)
	if f != nil { // might not be present depending on verb
		err = cfg.BindFlag(common.RequestPageSizeConfigPath, f)
		if err != nil {
			return err
		}
	}

	return nil
}

func preRunE(c *cobra.Command, args []string) error {
	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, products.Product, Product)
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
	c.SetContext(ctx)
	return bindFlags(c, args)
}

func NewKonnectCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     konnectUse,
		Short:   konnectShort,
		Long:    konnectLong,
		Example: konnectExamples,
		Aliases: []string{"k", "K"},
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, products.Product, Product)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
			c.SetContext(ctx)
			return bindFlags(c, args)
		},
	}

	// Handle Login verb
	if verb == verbs.Login {
		return newLoginKonnectCmd(verb, cmd, addFlags, preRunE).Command, nil
	}

	// Handle Logout verb
	if verb == verbs.Logout {
		return newLogoutKonnectCmd(verb, cmd, addFlags, preRunE).Command, nil
	}

	if verb == verbs.Listen {
		alc, err := auditlogs.NewAuditLogsCmd(verb, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(alc)
		addFlags(verb, cmd)
		return cmd, nil
	}

	// Handle declarative configuration verbs
	if verb == verbs.Plan || verb == verbs.Sync || verb == verbs.Diff || verb == verbs.Export || verb == verbs.Apply {
		c, e := declarative.NewDeclarativeCmd(verb)
		if e != nil {
			return nil, e
		}
		// Replace the konnect command with the declarative command
		cmd.Use = c.Use
		cmd.Short = c.Short
		cmd.Long = c.Long
		cmd.RunE = c.RunE
		// Copy flags from declarative command
		cmd.Flags().AddFlagSet(c.Flags())
		addFlags(verb, cmd)
		return cmd, nil
	}

	// Handle Adopt verb with specific subcommands
	if verb == verbs.Adopt {
		portalCmd, err := adopt.NewPortalCmd(verb, &cobra.Command{}, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(portalCmd)

		controlPlaneCmd, err := adopt.NewControlPlaneCmd(verb, &cobra.Command{}, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(controlPlaneCmd)

		apiCmd, err := adopt.NewAPICmd(verb, &cobra.Command{}, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(apiCmd)

		authStrategyCmd, err := adopt.NewAuthStrategyCmd(verb, &cobra.Command{}, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(authStrategyCmd)

		eventGatewayCmd, err := adopt.NewEventGatewayControlPlaneCmd(verb, &cobra.Command{}, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(eventGatewayCmd)

		orgCmd, err := organization.NewOrganizationCmd(verb, addFlags, preRunE)
		if err != nil {
			return nil, err
		}
		cmd.AddCommand(orgCmd)

		addFlags(verb, cmd)
		return cmd, nil
	}

	// For all other verbs, build the standard command tree

	c, e := gateway.NewGatewayCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	// Add portal command
	pc, e := portal.NewPortalCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(pc)

	// Add API command
	ac, e := api.NewAPICmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(ac)

	// Add auth strategy command
	asc, e := authstrategy.NewAuthStrategyCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(asc)

	// Add me command (read-only)
	if verb == verbs.Get {
		mc, e := me.NewMeCmd(verb, addFlags, preRunE)
		if e != nil {
			return nil, e
		}
		cmd.AddCommand(mc)
	}

	// Add organization command
	oc, e := organization.NewOrganizationCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(oc)

	rc, e := regions.NewRegionsCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(rc)

	// Add EventGateway command
	egcpc, e := eventgateway.NewEventGatewayCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(egcpc)

	if verb == verbs.Get {
		cmd.RunE = func(c *cobra.Command, args []string) error {
			helper := cmdpkg.BuildHelper(c, args)
			if _, err := helper.GetOutputFormat(); err != nil {
				return err
			}
			return c.Help()
		}
	}

	return cmd, e
}

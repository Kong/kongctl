package konnect

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/api"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/authstrategy"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/portal"
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
	cmd.Flags().String(common.BaseURLFlagName, common.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			common.BaseURLConfigPath))

	if verb != verbs.Login {
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
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(common.BaseURLFlagName)
	err = cfg.BindFlag(common.BaseURLConfigPath, f)
	if err != nil {
		return err
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
	ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
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
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
			c.SetContext(ctx)
			return bindFlags(c, args)
		},
	}

	if verb == verbs.Login {
		return newLoginKonnectCmd(verb, cmd, addFlags, preRunE).Command, nil
	}

	// Handle declarative configuration verbs
	switch verb {
	case verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Apply:
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
	case verbs.Add, verbs.Get, verbs.Create, verbs.Dump, verbs.Update,
		verbs.Delete, verbs.Help, verbs.List, verbs.Login:
		// These verbs don't use declarative configuration, continue below
	}

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

	return cmd, e
}

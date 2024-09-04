package konnect

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway"
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

func bindFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	f := c.Flags().Lookup(common.PATFlagName)
	err = cfg.BindFlag(common.PATConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.BaseURLFlagName)
	err = cfg.BindFlag(common.BaseURLConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.AuthPathFlagName)
	err = cfg.BindFlag(common.AuthPathConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.RefreshPathFlagName)
	err = cfg.BindFlag(common.RefreshPathConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.TokenPathFlagName)
	err = cfg.BindFlag(common.TokenURLPathConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.MachineClientIDFlagName)
	err = cfg.BindFlag(common.MachineClientIDConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.RequestPageSizeFlagName)
	if f != nil {
		err = cfg.BindFlag(common.RequestPageSizeConfigPath, f)
		if err != nil {
			return err
		}
	}

	return nil
}

func NewKonnectCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     konnectUse,
		Short:   konnectShort,
		Long:    konnectLong,
		Example: konnectExamples,
		Aliases: []string{"k", "K"},
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(),
				products.Product, Product))
			c.SetContext(context.WithValue(c.Context(),
				helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory)))
			return bindFlags(c, args)
		},
	}

	cmd.PersistentFlags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	cmd.PersistentFlags().String(common.BaseURLFlagName, common.BaseURLDefault,
		fmt.Sprintf(`Base URL used to initiate Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.BaseURLConfigPath))
	e := cmd.PersistentFlags().MarkHidden(common.BaseURLFlagName)
	if e != nil {
		return nil, e
	}

	cmd.PersistentFlags().String(common.AuthPathFlagName, common.AuthPathDefault,
		fmt.Sprintf(`URL path used to initiate Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.AuthPathConfigPath))
	e = cmd.PersistentFlags().MarkHidden(common.AuthPathFlagName)
	if e != nil {
		return nil, e
	}

	cmd.PersistentFlags().String(common.RefreshPathFlagName, common.RefreshPathDefault,
		fmt.Sprintf(`URL path used to refresh the Konnect auth token.
- Config path: [ %s ]
-`, // (default ...)
			common.RefreshPathConfigPath))
	e = cmd.PersistentFlags().MarkHidden(common.RefreshPathFlagName)
	if e != nil {
		return nil, e
	}

	cmd.PersistentFlags().String(common.MachineClientIDFlagName, common.MachineClientIDDefault,
		fmt.Sprintf(`Machine Client ID used to identify the application for Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.MachineClientIDConfigPath))
	e = cmd.PersistentFlags().MarkHidden(common.MachineClientIDFlagName)
	if e != nil {
		return nil, e
	}

	cmd.PersistentFlags().String(common.TokenPathFlagName, common.TokenPathDefault,
		fmt.Sprintf(`URL path used to poll for the Konnect Authorization response token.
- Config path: [ %s ]
-`, // (default ...)
			common.TokenURLPathConfigPath))
	e = cmd.PersistentFlags().MarkHidden(common.TokenPathFlagName)
	if e != nil {
		return nil, e
	}

	if verb == verbs.Get || verb == verbs.List {
		cmd.PersistentFlags().Int(
			common.RequestPageSizeFlagName,
			common.DefaultRequestPageSize,
			fmt.Sprintf(
				"Max number of results to include per response page for get and list operations.\n (config path = '%s')",
				common.RequestPageSizeConfigPath))
	}

	if verb == verbs.Login {
		return newLoginKonnectCmd(cmd).Command, nil
	}

	c, e := gateway.NewGatewayCmd(verb)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	return cmd, e
}

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
	c.SetContext(context.WithValue(c.Context(),
		products.Product, Product))
	c.SetContext(context.WithValue(c.Context(),
		helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory)))
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
			c.SetContext(context.WithValue(c.Context(),
				products.Product, Product))
			c.SetContext(context.WithValue(c.Context(),
				helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory)))
			return bindFlags(c, args)
		},
	}

	if verb == verbs.Login {
		return newLoginKonnectCmd(verb, cmd, addFlags, preRunE).Command, nil
	}

	c, e := gateway.NewGatewayCmd(verb, addFlags, preRunE)
	if e != nil {
		return nil, e
	}
	cmd.AddCommand(c)

	return cmd, e
}

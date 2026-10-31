package identity

import (
	"context"
	"fmt"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func NewDirectIdentityCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	group, err := NewIdentityCmd(verb, nil, func(cmd *cobra.Command, args []string) error {
		ctx := context.WithValue(cmd.Context(), products.Product, products.ProductKonnect)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactoryForVerb(verb))
		cmd.SetContext(ctx)
		if verb == verbs.Patch {
			cfg, err := cmdpkg.BuildHelper(cmd, args).GetConfig()
			if err != nil {
				return err
			}
			bindings := []struct{ flag, path string }{
				{common.BaseURLFlagName, common.BaseURLConfigPath},
				{common.RegionFlagName, common.RegionConfigPath},
				{common.PATFlagName, common.PATConfigPath},
			}
			for _, binding := range bindings {
				if err := verbs.BindFlag(cfg, cmd.Flags(), binding.flag, binding.path); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if verb == verbs.Patch {
		group.PersistentFlags().String(common.BaseURLFlagName, "", "Konnect API base URL")
		group.PersistentFlags().String(common.RegionFlagName, "", "Konnect region")
		group.PersistentFlags().String(common.PATFlagName, "", "Konnect personal access token")
	}
	return group, nil
}

func NewIdentityCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	group := &cobra.Command{Use: "identity", Short: "Manage Kong Identity resources"}
	cmdpkg.ConfigureRequiresSubcommand(group)
	if err := maturity.AnnotateCommand(group, maturity.Metadata{
		Level:   maturity.LevelTechPreview,
		Message: "Requires the Kong Identity SDK preview and regional management API access.",
	}); err != nil {
		return nil, err
	}
	if verb == verbs.Adopt {
		child, err := adopt.NewIdentityDirectoryCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		principal, err := adopt.NewIdentityPrincipalCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		authServer, err := adopt.NewIdentityAuthServerCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		client, err := adopt.NewIdentityAuthServerClientCmd(verb, addParentFlags, parentPreRun)
		if err != nil {
			return nil, err
		}
		group.AddCommand(child, principal, authServer, client)
		return group, nil
	}
	if verb != verbs.Get && verb != verbs.List {
		if verb == verbs.Create || verb == verbs.Patch || verb == verbs.Delete {
			group.AddCommand(newAuthServerSecretMutationCommand(verb, addParentFlags, parentPreRun))
		}
		return group, nil
	}
	child := &cobra.Command{
		Use: "directory [id|name]", Aliases: []string{"directories"},
		Short: "List or get Kong Identity directories in the configured region",
		Args:  cobra.MaximumNArgs(1), PreRunE: parentPreRun,
	}
	if addParentFlags != nil {
		addParentFlags(verb, child)
	}
	child.RunE = func(command *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(command, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}
		logger, err := helper.GetLogger()
		if err != nil {
			return err
		}
		sdk, err := helper.GetKonnectSDK(cfg, logger)
		if err != nil {
			return err
		}
		format, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}
		api := sdk.GetIdentityDirectoriesAPI()
		var data any
		if len(args) == 1 {
			value, err := helpers.ResolveIdentityDirectory(helper.GetContext(), api, args[0],
				int64(cfg.GetInt(common.RequestPageSizeConfigPath)))
			if err != nil {
				return err
			}
			data = value
			if format == cmdCommon.TEXT {
				_, err := fmt.Fprintf(helper.GetStreams().Out, "id: %s\nname: %s\ndescription: %s\n",
					value.ID, value.Name, value.Description)
				return err
			}
		} else {
			values, err := helpers.ListIdentityDirectories(helper.GetContext(), api,
				int64(cfg.GetInt(common.RequestPageSizeConfigPath)))
			if err != nil {
				return err
			}
			data = values
			if format == cmdCommon.TEXT {
				writer := tabwriter.NewWriter(helper.GetStreams().Out, 0, 4, 2, ' ', 0)
				fmt.Fprintln(writer, "ID\tNAME\tDESCRIPTION")
				for _, value := range values {
					fmt.Fprintf(writer, "%s\t%s\t%s\n", value.ID, value.Name, value.Description)
				}
				return writer.Flush()
			}
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(data)
		return nil
	}
	group.AddCommand(child, newAuthServerCommand(verb, addParentFlags, parentPreRun))
	group.AddCommand(newPrincipalCommand(verb, addParentFlags, parentPreRun))
	principalIdentity, err := newPrincipalIdentityCommand(verb, addParentFlags, parentPreRun)
	if err != nil {
		return nil, err
	}
	group.AddCommand(principalIdentity)
	group.AddCommand(newAuthServerChildCommand(verb, false, addParentFlags, parentPreRun))
	group.AddCommand(newAuthServerChildCommand(verb, true, addParentFlags, parentPreRun))
	group.AddCommand(newAuthServerClientCommand(verb, false, addParentFlags, parentPreRun))
	group.AddCommand(newAuthServerClientCommand(verb, true, addParentFlags, parentPreRun))
	return group, nil
}

package identity

import (
	"fmt"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newAuthServerCommand(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "auth-server [id|name]", Aliases: []string{"auth-servers", "auth_servers"},
		Short: "List or get Kong Identity authorization servers", Args: cobra.MaximumNArgs(1), PreRunE: parentPreRun,
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(cmd, args)
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
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		var data any
		if len(args) == 1 {
			value, err := helpers.ResolveIdentityAuthServer(
				helper.GetContext(), sdk.GetIdentityAuthServersAPI(), args[0], pageSize,
			)
			if err != nil {
				return err
			}
			data = value
			if format == cmdCommon.TEXT {
				_, err := fmt.Fprintf(
					helper.GetStreams().Out,
					"id: %s\nname: %s\naudience: %s\nissuer: %s\n", value.ID, value.Name, value.Audience, value.Issuer,
				)
				return err
			}
		} else {
			values, err := helpers.ListIdentityAuthServers(helper.GetContext(), sdk.GetIdentityAuthServersAPI(), pageSize)
			if err != nil {
				return err
			}
			data = values
			if format == cmdCommon.TEXT {
				writer := tabwriter.NewWriter(helper.GetStreams().Out, 0, 4, 2, ' ', 0)
				if _, err := fmt.Fprintln(writer, "ID\tNAME\tAUDIENCE"); err != nil {
					return err
				}
				for _, value := range values {
					if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", value.ID, value.Name, value.Audience); err != nil {
						return err
					}
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
	return command
}

package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newAuthServerChildCommand(verb verbs.VerbValue, claims bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "auth-server-scope [id|name]", Aliases: []string{"auth-server-scopes", "auth_server_scopes"},
		Short: "List or get scopes under an explicit authorization server",
		Args:  cobra.MaximumNArgs(1), PreRunE: parentPreRun,
	}
	if claims {
		command.Use = "auth-server-claim [id|name]"
		command.Aliases = []string{"auth-server-claims", "auth_server_claims"}
		command.Short = "List or get claims under an explicit authorization server"
		command.Long = "Claim names can repeat. Use the claim UUID when a name has multiple matches."
	}
	command.Flags().String("auth-server-id", "", "Parent authorization server UUID")
	command.Flags().String("auth-server-name", "", "Exact parent authorization server name")
	command.MarkFlagsOneRequired("auth-server-id", "auth-server-name")
	command.MarkFlagsMutuallyExclusive("auth-server-id", "auth-server-name")
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
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		serverID, err := authServerIDFromFlags(helper.GetContext(), cmd, sdk.GetIdentityAuthServersAPI(), pageSize)
		if err != nil {
			return err
		}
		format, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}
		var data any
		var rows [][3]string
		if claims {
			api := sdk.GetIdentityAuthServerClaimsAPI()
			if len(args) == 1 {
				data, err = helpers.ResolveIdentityAuthServerClaim(
					helper.GetContext(), api, serverID, args[0], pageSize,
				)
			} else {
				values, listErr := helpers.ListIdentityAuthServerClaims(helper.GetContext(), api, serverID, pageSize)
				err, data = listErr, values
				for _, value := range values {
					rows = append(rows, [3]string{value.ID, value.Name, authChildBool(value.Enabled)})
				}
			}
		} else {
			api := sdk.GetIdentityAuthServerScopesAPI()
			if len(args) == 1 {
				data, err = helpers.ResolveIdentityAuthServerScope(
					helper.GetContext(), api, serverID, args[0], pageSize,
				)
			} else {
				values, listErr := helpers.ListIdentityAuthServerScopes(helper.GetContext(), api, serverID, pageSize)
				err, data = listErr, values
				for _, value := range values {
					rows = append(rows, [3]string{value.ID, value.Name, authChildBool(value.Enabled)})
				}
			}
		}
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			if len(args) == 1 {
				encoded, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(helper.GetStreams().Out, string(encoded))
				return err
			}
			writer := tabwriter.NewWriter(helper.GetStreams().Out, 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(writer, "ID\tNAME\tENABLED"); err != nil {
				return err
			}
			for _, row := range rows {
				if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", row[0], row[1], row[2]); err != nil {
					return err
				}
			}
			return writer.Flush()
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

func authChildBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprint(*value)
}

func authServerIDFromFlags(ctx context.Context, cmd *cobra.Command, api helpers.IdentityAuthServersAPI,
	pageSize int64,
) (string, error) {
	id, err := cmd.Flags().GetString("auth-server-id")
	if err != nil {
		return "", err
	}
	name, err := cmd.Flags().GetString("auth-server-name")
	if err != nil {
		return "", err
	}
	if (id == "") == (name == "") {
		return "", fmt.Errorf("provide exactly one of --auth-server-id or --auth-server-name")
	}
	if id != "" {
		if !util.IsValidUUID(id) {
			return "", fmt.Errorf("auth-server-id must be a UUID")
		}
		return id, nil
	}
	value, err := helpers.ResolveIdentityAuthServer(ctx, api, name, pageSize)
	if err != nil {
		return "", err
	}
	return value.ID, nil
}

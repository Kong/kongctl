package identity

import (
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

func newAuthServerClientCommand(verb verbs.VerbValue, secrets bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "auth-server-client [client-id]", Aliases: []string{"auth-server-clients", "auth_server_clients"},
		Short:   "List or get OAuth clients under an explicit authorization server",
		Long:    "Client IDs are opaque strings. Use --client-name for an exact name lookup.",
		PreRunE: parentPreRun,
	}
	command.Flags().String("client-name", "", "Exact OAuth client name")
	if secrets {
		command.Use = "auth-server-client-secret [secret-id]"
		command.Aliases = []string{"auth-server-client-secrets", "auth_server_client_secrets"}
		command.Short = "List or get OAuth client secret metadata"
		command.Long = "Read secret IDs, enabled state and timestamps under an explicit OAuth client. " +
			"Secret values are never displayed."
		command.Flags().String("client-id", "", "Parent OAuth client ID (an opaque string)")
		command.MarkFlagsOneRequired("client-id", "client-name")
		command.MarkFlagsMutuallyExclusive("client-id", "client-name")
	}
	command.Flags().String("auth-server-id", "", "Parent authorization server UUID")
	command.Flags().String("auth-server-name", "", "Exact parent authorization server name")
	command.MarkFlagsOneRequired("auth-server-id", "auth-server-name")
	command.MarkFlagsMutuallyExclusive("auth-server-id", "auth-server-name")
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("client-name")
		if cmd.Flags().Changed("client-name") && name == "" {
			return fmt.Errorf("client-name must not be empty")
		}
		if secrets {
			id, _ := cmd.Flags().GetString("client-id")
			if (id == "") == (name == "") {
				return fmt.Errorf("provide exactly one of --client-id or --client-name")
			}
			if len(args) == 1 && !util.IsValidUUID(args[0]) {
				return fmt.Errorf("secret-id must be a UUID")
			}
		} else if name != "" && len(args) != 0 {
			return fmt.Errorf("provide a client ID argument or --client-name, not both")
		} else if len(args) == 1 && args[0] == "" {
			return fmt.Errorf("client ID must not be empty")
		}
		return nil
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
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		ctx := helper.GetContext()
		serverID, err := authServerIDFromFlags(ctx, cmd, sdk.GetIdentityAuthServersAPI(), pageSize)
		if err != nil {
			return err
		}
		api := sdk.GetIdentityAuthServerClientsAPI()
		name, _ := cmd.Flags().GetString("client-name")
		var data any
		var rows [][2]string
		detail := len(args) == 1 || (!secrets && name != "")
		if secrets {
			clientID, _ := cmd.Flags().GetString("client-id")
			if name != "" {
				value, err := helpers.ResolveIdentityAuthServerClient(ctx, api, serverID, name, true, pageSize)
				if err != nil {
					return err
				}
				clientID = value.ID
			}
			if detail {
				data, err = helpers.GetIdentityClientSecret(ctx, api, serverID, clientID, args[0])
			} else {
				values, listErr := helpers.ListIdentityClientSecrets(ctx, api, serverID, clientID, pageSize)
				data, err = values, listErr
				for _, value := range values {
					rows = append(rows, [2]string{value.ID, authChildBool(value.Enabled)})
				}
			}
		} else if detail {
			identifier := name
			if name == "" {
				identifier = args[0]
			}
			data, err = helpers.ResolveIdentityAuthServerClient(ctx, api, serverID, identifier, name != "", pageSize)
		} else {
			values, listErr := helpers.ListIdentityAuthServerClients(ctx, api, serverID, pageSize)
			data, err = values, listErr
			for _, value := range values {
				rows = append(rows, [2]string{value.ID, value.Name})
			}
		}
		if err != nil {
			return err
		}
		format, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			if detail {
				encoded, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(helper.GetStreams().Out, string(encoded))
				return err
			}
			writer := tabwriter.NewWriter(helper.GetStreams().Out, 0, 4, 2, ' ', 0)
			header := "ID\tNAME"
			if secrets {
				header = "ID\tENABLED"
			}
			if _, err := fmt.Fprintln(writer, header); err != nil {
				return err
			}
			for _, row := range rows {
				if _, err := fmt.Fprintf(writer, "%s\t%s\n", row[0], row[1]); err != nil {
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

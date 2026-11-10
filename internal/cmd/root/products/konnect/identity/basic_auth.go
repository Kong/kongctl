package identity

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newBasicAuthReadCommand(verb verbs.VerbValue, passwords bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal-basic-auth [basic-auth-id]", Aliases: []string{"principal-basic-auths", "principal_basic_auths"},
		Short: "List or get basic-auth username records under an explicit principal",
		Long: `Inspect username records separately from their password children. Positional
selectors are UUIDs; use --username for an exact username lookup, including
UUID-shaped usernames.`,
		PreRunE: parentPreRun,
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String("principal-id", "", "Parent principal UUID")
	command.Flags().String("username", "", "Exact basic-auth username")
	if err := command.MarkFlagRequired("principal-id"); err != nil {
		return nil, err
	}
	if passwords {
		command.Use = "principal-basic-auth-password [password-id]"
		command.Aliases = []string{"principal-basic-auth-passwords", "principal_basic_auth_passwords"}
		command.Short = "List or get public basic-auth password metadata"
		command.Long = `Inspect password IDs, expiration, management metadata and timestamps. Password
values are never displayed. Select the username record with --basic-auth-id or
--username.`
		command.Flags().String("basic-auth-id", "", "Parent basic-auth record UUID")
		command.MarkFlagsOneRequired("basic-auth-id", "username")
		command.MarkFlagsMutuallyExclusive("basic-auth-id", "username")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		if !util.IsValidUUID(principal) {
			return fmt.Errorf("principal-id must be a UUID")
		}
		username, _ := cmd.Flags().GetString("username")
		if cmd.Flags().Changed("username") && username == "" {
			return fmt.Errorf("username must not be empty")
		}
		if len(args) == 1 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("basic-auth and password ID arguments must be UUIDs")
		}
		if passwords {
			id, _ := cmd.Flags().GetString("basic-auth-id")
			if (id == "") == (username == "") {
				return fmt.Errorf("provide exactly one of --basic-auth-id or --username")
			}
			if id != "" && !util.IsValidUUID(id) {
				return fmt.Errorf("basic-auth-id must be a UUID")
			}
		} else if len(args) != 0 && username != "" {
			return fmt.Errorf("provide a basic-auth ID argument or --username, not both")
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
		ctx := helper.GetContext()
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		directory, err := groupDirectoryIDFromFlags(ctx, cmd, sdk.GetIdentityDirectoriesAPI(), pageSize)
		if err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		username, _ := cmd.Flags().GetString("username")
		api := sdk.GetIdentityBasicAuthAPI()
		var data any
		var rows [][2]string
		if passwords {
			basicAuthID, _ := cmd.Flags().GetString("basic-auth-id")
			if username != "" {
				value, err := helpers.ResolveIdentityBasicAuthUsername(
					ctx,
					api,
					directory,
					principal,
					username,
					pageSize,
				)
				if err != nil {
					return err
				}
				basicAuthID = value.ID
			}
			if len(args) == 1 {
				value, readErr := helpers.GetIdentityPassword(ctx, api, directory, principal, basicAuthID, args[0])
				data, err = value, readErr
				if value != nil {
					rows = append(rows, [2]string{value.ID, basicAuthPasswordExpiration(value.ExpiresAt)})
				}
			} else {
				values, readErr := helpers.ListIdentityPasswords(ctx, api, directory, principal, basicAuthID, pageSize)
				data, err = values, readErr
				for _, value := range values {
					rows = append(rows, [2]string{value.ID, basicAuthPasswordExpiration(value.ExpiresAt)})
				}
			}
		} else if len(args) == 1 || username != "" {
			var value *helpers.IdentityBasicAuth
			if username != "" {
				value, err = helpers.ResolveIdentityBasicAuthUsername(ctx, api, directory, principal, username, pageSize)
			} else {
				value, err = helpers.GetIdentityBasicAuth(ctx, api, directory, principal, args[0])
			}
			data = value
			if value != nil {
				rows = append(rows, [2]string{value.ID, value.Username})
			}
		} else {
			values, readErr := helpers.ListIdentityBasicAuths(ctx, api, directory, principal, pageSize)
			data, err = values, readErr
			for _, value := range values {
				rows = append(rows, [2]string{value.ID, value.Username})
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
			return printBasicAuthReadRows(helper.GetStreams().Out, rows, passwords)
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(data)
		return nil
	}
	return command, nil
}

func basicAuthPasswordExpiration(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func printBasicAuthReadRows(out io.Writer, rows [][2]string, passwords bool) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	header := "ID\tUSERNAME"
	if passwords {
		header = "ID\tEXPIRES_AT"
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

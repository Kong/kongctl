package identity

import (
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/optionalnullable"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newPasswordMutationCommand(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal-basic-auth-password <password-id>", Aliases: []string{"principal-basic-auth-passwords"},
		Short: fmt.Sprintf("%s a basic-auth password", verb), PreRunE: parentPreRun,
		Long: "Manage password material and expiration separately from the username record. " +
			"Choose the parent with --basic-auth-id or exact --username. " +
			"Values never appear in normal output. These explicit operations preserve management metadata.",
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.Flags().String("principal-id", "", "Parent principal UUID")
	command.Flags().String("basic-auth-id", "", "Parent basic-auth record UUID")
	command.Flags().String("username", "", "Exact parent basic-auth username")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	command.MarkFlagsOneRequired("basic-auth-id", "username")
	command.MarkFlagsMutuallyExclusive("basic-auth-id", "username")
	if err := command.MarkFlagRequired("principal-id"); err != nil {
		return nil, err
	}
	if verb == verbs.Create || verb == verbs.Patch {
		command.Flags().String("secret-env", "", "Environment variable containing password material")
		command.Flags().String("secret-file", "", "File containing password material")
		command.MarkFlagsMutuallyExclusive("secret-env", "secret-file")
		command.Flags().String("expires-at", "", "Expiration timestamp in RFC3339 format")
		command.Flags().Bool("no-expiration", false, "Explicitly clear expiration")
		command.MarkFlagsMutuallyExclusive("expires-at", "no-expiration")
	}
	//exhaustive:ignore
	switch verb {
	case verbs.Create:
		command.Use = "principal-basic-auth-password"
		command.Short = "Create a password and capture generated or imported material in a new private file"
		command.Long += " Omit secret source flags to generate a password. " +
			"The API permits two total password records; expired records still occupy a slot."
		command.Flags().String("secret-output-file", "", "New private capture file (0600; never overwritten)")
		if err := command.MarkFlagRequired("secret-output-file"); err != nil {
			return nil, err
		}
	case verbs.Patch:
		command.Short = "Change password expiration or replace explicitly supplied password material"
		command.Long += " Expiration-only changes use PATCH. Supplying --secret-env or --secret-file " +
			"replaces material in place through PUT, preserving omitted expiration and existing management metadata. " +
			"For rotation with a second record, create another password before explicitly retiring the old one."
	case verbs.Delete:
		command.Use = "principal-basic-auth-password [password-id]"
		command.Flags().Bool("all", false, "Delete every password under the explicitly selected username record")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		if !util.IsValidUUID(principal) {
			return fmt.Errorf("principal-id must be a UUID")
		}
		basic, _ := cmd.Flags().GetString("basic-auth-id")
		username, _ := cmd.Flags().GetString("username")
		if (basic == "") == (username == "") {
			return fmt.Errorf("provide exactly one of --basic-auth-id or --username")
		}
		if basic != "" && !util.IsValidUUID(basic) {
			return fmt.Errorf("basic-auth-id must be a UUID")
		}
		if cmd.Flags().Changed("username") && username == "" {
			return fmt.Errorf("username must not be empty")
		}
		if verb == verbs.Create {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
		} else {
			all := false
			if verb == verbs.Delete {
				all, _ = cmd.Flags().GetBool("all")
			}
			if all {
				if len(args) != 0 {
					return fmt.Errorf("provide a password ID or --all, not both")
				}
			} else {
				if err := cobra.ExactArgs(1)(cmd, args); err != nil {
					return err
				}
				if !util.IsValidUUID(args[0]) {
					return fmt.Errorf("password-id must be a UUID")
				}
			}
		}
		if verb == verbs.Create || verb == verbs.Patch {
			if _, _, err := passwordExpirationFromFlags(cmd); err != nil {
				return err
			}
			variable, _ := cmd.Flags().GetString("secret-env")
			file, _ := cmd.Flags().GetString("secret-file")
			if variable != "" && file != "" {
				return fmt.Errorf("provide one password source, not both --secret-env and --secret-file")
			}
			if cmd.Flags().Changed("secret-env") && variable == "" || cmd.Flags().Changed("secret-file") && file == "" {
				return fmt.Errorf("secret source must not be empty")
			}
			if verb == verbs.Patch && variable == "" && file == "" &&
				!cmd.Flags().Changed("expires-at") && !cmd.Flags().Changed("no-expiration") {
				return fmt.Errorf("provide --expires-at, --no-expiration or an explicit secret source")
			}
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		return runPasswordMutation(cmd, args, verb)
	}
	return command, nil
}

func passwordExpirationFromFlags(command *cobra.Command) (*time.Time, bool, error) {
	value, _ := command.Flags().GetString("expires-at")
	clearExpiration, _ := command.Flags().GetBool("no-expiration")
	if command.Flags().Changed("expires-at") && command.Flags().Changed("no-expiration") {
		return nil, false, fmt.Errorf("provide --expires-at or --no-expiration, not both")
	}
	if command.Flags().Changed("no-expiration") {
		if !clearExpiration {
			return nil, false, fmt.Errorf("--no-expiration must be true when provided")
		}
		return nil, true, nil
	}
	if !command.Flags().Changed("expires-at") {
		return nil, false, nil
	}
	expires, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, false, fmt.Errorf("expires-at must be a nonempty RFC3339 timestamp")
	}
	return &expires, true, nil
}

func runPasswordMutation(command *cobra.Command, args []string, verb verbs.VerbValue) error {
	helper := cmdpkg.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	format, err := helper.GetOutputFormat()
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
	directory, err := groupDirectoryIDFromFlags(ctx, command, sdk.GetIdentityDirectoriesAPI(), pageSize)
	if err != nil {
		return err
	}
	principal, _ := command.Flags().GetString("principal-id")
	basicID, _ := command.Flags().GetString("basic-auth-id")
	username, _ := command.Flags().GetString("username")
	basicAPI := sdk.GetIdentityBasicAuthAPI()
	var parent *helpers.IdentityBasicAuth
	if username != "" {
		parent, err = helpers.ResolveIdentityBasicAuthUsername(ctx, basicAPI, directory, principal, username, pageSize)
	} else {
		parent, err = helpers.GetIdentityBasicAuth(ctx, basicAPI, directory, principal, basicID)
	}
	if err != nil {
		return err
	}
	api := sdk.GetIdentityPasswordWritesAPI()
	var data any
	var password *helpers.IdentityPassword
	//exhaustive:ignore
	switch verb {
	case verbs.Create, verbs.Patch:
		material, err := identitySecretSource(command)
		if err != nil {
			return err
		}
		expiresAt, supplied, err := passwordExpirationFromFlags(command)
		if err != nil {
			return err
		}
		if verb == verbs.Create {
			path, _ := command.Flags().GetString("secret-output-file")
			body := kkComps.CreatePasswordBody{Secret: material}
			if supplied {
				body.ExpiresAt = optionalnullable.From(expiresAt)
			}
			password, err = helpers.CreateAndCaptureIdentityPassword(
				ctx,
				api,
				directory,
				principal,
				parent.ID,
				body,
				path,
			)
		} else if material != nil {
			password, err = helpers.ReplaceIdentityPassword(ctx, api, directory, principal, parent.ID, args[0],
				*material, expiresAt, supplied)
		} else {
			password, err = helpers.UpdateIdentityPasswordExpiration(
				ctx, api, directory, principal, parent.ID, args[0], expiresAt,
			)
		}
		if err != nil {
			return err
		}
		data = password
	case verbs.Delete:
		all, _ := command.Flags().GetBool("all")
		if all {
			values, err := helpers.ListIdentityPasswords(ctx, api, directory, principal, parent.ID, pageSize)
			if err != nil {
				return err
			}
			if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("all passwords for username %q", parent.Username)); err != nil {
				return err
			}
			if err := helpers.DeleteAllIdentityPasswords(ctx, api, directory, principal, parent.ID); err != nil {
				return err
			}
			data = map[string]any{
				"basic_auth_id":           parent.ID,
				"status":                  "deleted",
				"observed_password_count": len(values),
			}
		} else {
			if _, err := helpers.GetIdentityPassword(ctx, api, directory, principal, parent.ID, args[0]); err != nil {
				return err
			}
			if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("basic-auth password %q", args[0])); err != nil {
				return err
			}
			if err := helpers.DeleteIdentityPassword(ctx, api, directory, principal, parent.ID, args[0]); err != nil {
				return err
			}
			data = map[string]string{"id": args[0], "status": "deleted"}
		}
	default:
		return fmt.Errorf("unsupported password operation %q", verb)
	}
	if format == cmdCommon.TEXT && password != nil {
		return printBasicAuthReadRows(helper.GetStreams().Out,
			[][2]string{{password.ID, basicAuthPasswordExpiration(password.ExpiresAt)}}, true)
	}
	printer, err := cli.Format(format.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(data)
	return nil
}

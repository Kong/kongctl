package identity

import (
	"fmt"
	"text/tabwriter"
	"time"

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

func newIdentityAPIKeyCommand(verb verbs.VerbValue, group bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	parent := "principal"
	if group {
		parent = "group"
	}
	command := &cobra.Command{
		Use: parent + "-api-key [key-id]", Aliases: []string{parent + "-api-keys", parent + "_api_keys"},
		Short: fmt.Sprintf("%s %s API-key metadata", verb, parent), PreRunE: parentPreRun,
		Long: `Select a parent UUID; the directory defaults to default. Names are not unique
key identifiers. Material is captured only at creation in a new private file.
Rotation creates a new key. Explicit mutations preserve management metadata and
do not stamp ownership.`,
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String(parent+"-id", "", "Parent "+parent+" UUID")
	if err := command.MarkFlagRequired(parent + "-id"); err != nil {
		return nil, err
	}
	if verb == verbs.Create || verb == verbs.Patch {
		command.Flags().String("expires-at", "", "Expiration timestamp in RFC3339 format")
		command.Flags().Bool("no-expiration", false, "Explicitly clear expiration")
		command.MarkFlagsMutuallyExclusive("expires-at", "no-expiration")
		if !group || verb == verbs.Create {
			command.Flags().String("name", "", "Optional display name; not unique")
		}
	}
	if verb == verbs.Create {
		command.Use = parent + "-api-key"
		command.Flags().String("secret-env", "", "Environment variable containing imported key material")
		command.Flags().String("secret-file", "", "File containing imported key material")
		command.MarkFlagsMutuallyExclusive("secret-env", "secret-file")
		command.Flags().String("secret-output-file", "", "New private capture file (never overwritten)")
		if err := command.MarkFlagRequired("secret-output-file"); err != nil {
			return nil, err
		}
		command.Long += "\n\n" + `Omit secret source flags to generate a key. Imported group keys cannot set
names.`
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		parentID, _ := cmd.Flags().GetString(parent + "-id")
		if !util.IsValidUUID(parentID) {
			return fmt.Errorf("%s-id must be a UUID", parent)
		}
		if verb == verbs.Create {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			variable, _ := cmd.Flags().GetString("secret-env")
			file, _ := cmd.Flags().GetString("secret-file")
			if variable != "" && file != "" {
				return fmt.Errorf("provide --secret-env or --secret-file, not both")
			}
			if cmd.Flags().Changed("secret-env") && variable == "" || cmd.Flags().Changed("secret-file") && file == "" {
				return fmt.Errorf("secret source must not be empty")
			}
			if group && cmd.Flags().Changed("name") && (variable != "" || file != "") {
				return fmt.Errorf("imported group API-key names are not supported by the deployed API")
			}
		} else {
			validate := cobra.MaximumNArgs(1)
			if verb == verbs.Patch || verb == verbs.Delete {
				validate = cobra.ExactArgs(1)
			}
			if err := validate(cmd, args); err != nil {
				return err
			}
			if len(args) == 1 && !util.IsValidUUID(args[0]) {
				return fmt.Errorf("API-key ID must be a UUID")
			}
		}
		if verb == verbs.Create || verb == verbs.Patch {
			_, supplied, err := passwordExpirationFromFlags(cmd)
			if err != nil {
				return err
			}
			if verb == verbs.Patch && !supplied && !cmd.Flags().Changed("name") {
				return fmt.Errorf("provide --expires-at, --no-expiration or a supported --name change")
			}
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error { return runIdentityAPIKey(cmd, args, verb, group) }
	return command, nil
}

func runIdentityAPIKey(command *cobra.Command, args []string, verb verbs.VerbValue, group bool) error {
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
	parent := "principal"
	if group {
		parent = "group"
	}
	parentID, _ := command.Flags().GetString(parent + "-id")
	var data any
	var value *helpers.IdentityAPIKey
	var values []helpers.IdentityAPIKey
	if verb == verbs.Get || verb == verbs.List {
		if group {
			if len(args) == 1 {
				value, err = helpers.GetIdentityGroupAPIKey(ctx, sdk.GetIdentityGroupKeysAPI(), directory, parentID, args[0])
			} else {
				values, err = helpers.ListIdentityGroupAPIKeys(ctx, sdk.GetIdentityGroupKeysAPI(), directory, parentID, pageSize)
			}
		} else {
			if len(args) == 1 {
				value, err = helpers.GetPrincipalAPIKey(ctx, sdk.GetIdentityPrincipalKeysReader(), directory, parentID, args[0])
			} else {
				values, err = helpers.ListPrincipalAPIKeys(ctx, sdk.GetIdentityPrincipalKeysReader(), directory, parentID, pageSize)
			}
		}
	} else {
		var client *helpers.IdentityAPIKeyClient
		if group {
			client, err = helpers.NewGroupAPIKeyClient(sdk.GetIdentityGroupKeysAPI(), directory, parentID)
		} else {
			client, err = helpers.NewPrincipalAPIKeyClient(sdk.GetIdentityPrincipalKeysAPI(), directory, parentID)
		}
		if err != nil {
			return err
		}
		//exhaustive:ignore
		switch verb {
		case verbs.Create, verbs.Patch:
			expiresAt, supplied, parseErr := passwordExpirationFromFlags(command)
			if parseErr != nil {
				return parseErr
			}
			var expiration optionalnullable.OptionalNullable[time.Time]
			if supplied {
				expiration = optionalnullable.From(expiresAt)
			}
			var name *string
			if command.Flags().Changed("name") {
				raw, _ := command.Flags().GetString("name")
				name = &raw
			}
			if verb == verbs.Create {
				material, sourceErr := identitySecretSource(command)
				if sourceErr != nil {
					return sourceErr
				}
				path, _ := command.Flags().GetString("secret-output-file")
				value, err = client.CreateAndCapture(ctx,
					helpers.IdentityAPIKeyCreate{Name: name, Secret: material, ExpiresAt: expiration}, path)
			} else {
				value, err = client.Update(ctx, args[0], helpers.IdentityAPIKeyUpdate{Name: name, ExpiresAt: expiration})
			}
		case verbs.Delete:
			if _, err := client.Get(ctx, args[0]); err != nil {
				return err
			}
			if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("%s API key %q", parent, args[0])); err != nil {
				return err
			}
			err = client.Delete(ctx, args[0])
			data = map[string]string{"id": args[0], "status": "deleted"}
		}
	}
	if err != nil {
		return err
	}
	if value != nil {
		data = value
	} else if data == nil {
		data = values
	}
	if format == cmdCommon.TEXT {
		if verb == verbs.Delete {
			_, err := fmt.Fprintf(helper.GetStreams().Out, "deleted API key: %s\n", args[0])
			return err
		}
		if value != nil {
			name := "-"
			if value.Name != nil {
				name = *value.Name
			}
			_, err := fmt.Fprintf(helper.GetStreams().Out, "id: %s\nname: %s\nexpires_at: %s\n",
				value.ID, name, principalKeyExpiration(*value))
			return err
		}
		writer := tabwriter.NewWriter(helper.GetStreams().Out, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(writer, "ID\tNAME\tEXPIRES_AT"); err != nil {
			return err
		}
		for _, value := range values {
			name := "-"
			if value.Name != nil {
				name = *value.Name
			}
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", value.ID, name, principalKeyExpiration(value)); err != nil {
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

func principalKeyExpiration(value helpers.PrincipalAPIKey) string {
	if value.ExpiresAt == nil {
		return "-"
	}
	return value.ExpiresAt.UTC().Format(time.RFC3339)
}

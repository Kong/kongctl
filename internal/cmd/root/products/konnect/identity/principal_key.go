package identity

import (
	"fmt"
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

func newIdentityAPIKeyCommand(verb verbs.VerbValue, group bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	if verb != verbs.Get && verb != verbs.List {
		return nil, fmt.Errorf("unsupported API-key read verb %q", verb)
	}
	parent := "principal"
	if group {
		parent = "group"
	}
	command := &cobra.Command{
		Use: parent + "-api-key [key-id]", Aliases: []string{parent + "-api-keys", parent + "_api_keys"},
		Short: fmt.Sprintf("%s %s API-key metadata", verb, parent), PreRunE: parentPreRun,
		Long: `Read API-key metadata under an explicit parent UUID. The directory defaults
to default. Names are not unique key identifiers; secret values are excluded.`,
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String(parent+"-id", "", "Parent "+parent+" UUID")
	if err := command.MarkFlagRequired(parent + "-id"); err != nil {
		return nil, err
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		id, _ := cmd.Flags().GetString(parent + "-id")
		if !util.IsValidUUID(id) {
			return fmt.Errorf("%s-id must be a UUID", parent)
		}
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if len(args) == 1 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("API-key ID must be a UUID")
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

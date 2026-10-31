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

func newPrincipalIdentityCommand(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal-identity [identity-id]", Aliases: []string{"principal-identities", "principal_identities"},
		Short: "List or get external identities under an explicit directory principal", PreRunE: parentPreRun,
		Long: "Inspect OIDC, OAuth client, control-plane consumer and custom identity associations. " +
			"Select a directory and principal UUID explicitly; display names are not unique.",
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.Flags().String("principal-id", "", "Parent principal UUID")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	if err := command.MarkFlagRequired("principal-id"); err != nil {
		return nil, err
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		principal, err := cmd.Flags().GetString("principal-id")
		if err != nil {
			return err
		}
		if !util.IsValidUUID(principal) {
			return fmt.Errorf("principal-id must be a UUID")
		}
		if len(args) == 1 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("principal identity ID must be a UUID")
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
		directory, err := cmd.Flags().GetString("directory-id")
		if err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("directory-name")
		if err != nil {
			return err
		}
		principal, err := cmd.Flags().GetString("principal-id")
		if err != nil {
			return err
		}
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		if name != "" {
			value, err := helpers.ResolveIdentityDirectory(
				helper.GetContext(),
				sdk.GetIdentityDirectoriesAPI(),
				name,
				pageSize,
			)
			if err != nil {
				return err
			}
			directory = value.ID
		}
		api := sdk.GetIdentityPrincipalIdentitiesAPI()
		var data any
		var values []helpers.PrincipalIdentity
		if len(args) == 1 {
			data, err = helpers.GetPrincipalIdentity(helper.GetContext(), api, directory, principal, args[0], pageSize)
		} else {
			values, err = helpers.ListPrincipalIdentities(helper.GetContext(), api, directory, principal, pageSize)
			data = values
		}
		if err != nil {
			return err
		}
		format, err := helper.GetOutputFormat()
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
			if _, err := fmt.Fprintln(writer, "ID\tTYPE"); err != nil {
				return err
			}
			for _, value := range values {
				if _, err := fmt.Fprintf(writer, "%s\t%s\n", value.ID, value.Type); err != nil {
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
	return command, nil
}

package identity

import (
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newPrincipalCommand(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "principal [id]", Aliases: []string{"principals"},
		Short: "List or get principals in an explicitly selected Kong Identity directory",
		Long:  "List principals or get one by UUID. Display names are not unique and cannot select a principal.",
		Args:  cobra.MaximumNArgs(1), PreRunE: parentPreRun,
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
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
		directoryID, err := cmd.Flags().GetString("directory-id")
		if err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("directory-name")
		if err != nil {
			return err
		}
		pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
		if name != "" {
			directory, err := helpers.ResolveIdentityDirectory(
				helper.GetContext(), sdk.GetIdentityDirectoriesAPI(), name, pageSize,
			)
			if err != nil {
				return err
			}
			directoryID = directory.ID
		}
		if !util.IsValidUUID(directoryID) {
			return fmt.Errorf("directory-id must be a UUID")
		}
		printer, err := cli.Format(format.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		if len(args) == 1 {
			if !util.IsValidUUID(args[0]) {
				return fmt.Errorf("principal identifier must be a UUID; display names are not unique")
			}
			value, err := helpers.GetIdentityPrincipal(
				helper.GetContext(),
				sdk.GetIdentityPrincipalsAPI(),
				directoryID,
				args[0],
			)
			if err != nil {
				return err
			}
			return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(),
				principalDisplay(*value), value, "")
		}
		values, err := helpers.ListIdentityPrincipals(
			helper.GetContext(), sdk.GetIdentityPrincipalsAPI(), directoryID, pageSize,
		)
		if err != nil {
			return err
		}
		records := make([]principalDisplayRecord, 0, len(values))
		for _, value := range values {
			records = append(records, principalDisplay(value))
		}
		return tableview.RenderForFormat(helper, false, format, printer, helper.GetStreams(), records, values, "")
	}
	return command
}

type principalDisplayRecord struct{ ID, DisplayName, Type, Active, ProvisionedBy string }

func principalDisplay(value helpers.IdentityPrincipal) principalDisplayRecord {
	result := principalDisplayRecord{ID: value.ID, Type: "unclassified", Active: "unknown"}
	if value.DisplayName != nil {
		result.DisplayName = *value.DisplayName
	}
	if value.Type != nil {
		result.Type = string(*value.Type)
	}
	if value.Active != nil {
		result.Active = fmt.Sprint(*value.Active)
	}
	if value.ProvisionedBy != nil {
		result.ProvisionedBy = string(*value.ProvisionedBy)
	}
	return result
}

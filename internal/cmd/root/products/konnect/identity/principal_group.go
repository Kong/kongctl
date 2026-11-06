package identity

import (
	"fmt"
	"io"
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

func newPrincipalGroupReadCommand(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal-group [group-id|group-name]", Aliases: []string{"principal-groups", "principal_groups"},
		Short: "Inspect a principal's direct and inherited group memberships", PreRunE: parentPreRun,
		Long: `List the groups a principal belongs to, including inherited organization-chart
teams. The optional group UUID or exact name selects one association from the
complete collection. membership_id identifies a direct entry; inherited entries
have no membership_id. Inspection does not grant authority to remove imported or
inherited memberships.`,
	}
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String("principal-id", "", "Parent principal UUID; display names are not unique")
	if err := command.MarkFlagRequired("principal-id"); err != nil {
		return nil, err
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		principal, err := cmd.Flags().GetString("principal-id")
		if err != nil {
			return err
		}
		if !util.IsValidUUID(principal) {
			return fmt.Errorf("principal-id must be a UUID")
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
		directory, err := groupDirectoryIDFromFlags(helper.GetContext(), cmd, sdk.GetIdentityDirectoriesAPI(), pageSize)
		if err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		api := sdk.GetIdentityPrincipalGroupsReader()
		var data any
		var values []helpers.IdentityPrincipalGroup
		if len(args) == 1 {
			value, lookupErr := helpers.ResolveIdentityPrincipalGroup(
				helper.GetContext(), api, directory, principal, args[0], pageSize,
			)
			if lookupErr != nil {
				return lookupErr
			}
			data, values = value, []helpers.IdentityPrincipalGroup{*value}
		} else {
			values, err = helpers.ListIdentityPrincipalGroups(helper.GetContext(), api, directory, principal, pageSize)
			if err != nil {
				return err
			}
			data = values
		}
		format, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}
		if format == cmdCommon.TEXT {
			return printPrincipalGroupRows(helper.GetStreams().Out, values, len(args) == 1)
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

func printPrincipalGroupRows(out io.Writer, values []helpers.IdentityPrincipalGroup, detail bool) error {
	if detail && len(values) != 1 {
		return fmt.Errorf("principal group detail requires one association")
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if !detail {
		header := "GROUP_ID\tNAME\tTYPE\tDIRECT\tMEMBERSHIP_ID\tPROVISIONED_BY\tTEAM_MANAGER_ID"
		if _, err := fmt.Fprintln(w, header); err != nil {
			return err
		}
	}
	for _, value := range values {
		kind, direct, membership, provisioned, manager, managerName, email := "-", "-", "-", "-", "-", "-", "-"
		if value.Type != nil {
			kind = string(*value.Type)
		}
		if value.Direct != nil {
			direct = fmt.Sprint(*value.Direct)
		}
		if value.MembershipID != nil {
			membership = *value.MembershipID
		}
		if value.ProvisionedBy != nil {
			provisioned = string(*value.ProvisionedBy)
		}
		if value.TeamManager != nil {
			manager, managerName = value.TeamManager.ID, value.TeamManager.DisplayName
			if value.TeamManager.Email != nil {
				email = *value.TeamManager.Email
			}
		}
		if detail {
			for _, field := range []struct{ name, value string }{
				{"id", value.ID},
				{"name", value.Name},
				{"type", kind},
				{"direct", direct},
				{"membership_id", membership},
				{"provisioned_by", provisioned},
				{"team_manager.id", manager},
				{"team_manager.display_name", managerName},
				{"team_manager.email", email},
			} {
				if _, err := fmt.Fprintf(w, "%s:\t%s\n", field.name, field.value); err != nil {
					return err
				}
			}
		} else if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			value.ID, value.Name, kind, direct, membership, provisioned, manager); err != nil {
			return err
		}
	}
	return w.Flush()
}

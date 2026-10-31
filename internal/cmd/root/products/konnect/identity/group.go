package identity

import (
	"context"
	"encoding/json"
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

func newGroupReadCommand(verb verbs.VerbValue, memberships bool,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: "group [id|name]", Aliases: []string{"groups"}, PreRunE: parentPreRun,
		Short: "List or get Kong Identity groups in an explicit directory",
		Long: "Inspect ordinary groups and organization-chart teams. " +
			"Select one by UUID or exact name within the selected directory.",
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	if memberships {
		command.Use = "group-membership [membership-id]"
		command.Aliases = []string{"group-memberships", "group_memberships"}
		command.Short = "List or get direct membership entries in an explicit group"
		command.Long = "Inspect direct membership entries under a selected group. " +
			"The optional UUID selects the membership entry, not its member principal or group. " +
			"This collection does not expand transitive membership."
		command.Flags().String("group-id", "", "Parent group UUID")
		command.Flags().String("group-name", "", "Exact parent group name within the selected directory")
		command.MarkFlagsOneRequired("group-id", "group-name")
		command.MarkFlagsMutuallyExclusive("group-id", "group-name")
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		if _, err := groupParentSelector(cmd, "directory"); err != nil {
			return err
		}
		if memberships {
			if _, err := groupParentSelector(cmd, "group"); err != nil {
				return err
			}
			if len(args) == 1 && !util.IsValidUUID(args[0]) {
				return fmt.Errorf("group membership entry ID must be a UUID")
			}
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
		var data any
		rows := make([]groupReadRow, 0)
		if memberships {
			selector, selectorErr := groupParentSelector(cmd, "group")
			if selectorErr != nil {
				return selectorErr
			}
			group, resolveErr := helpers.ResolveIdentityGroup(
				helper.GetContext(), sdk.GetIdentityGroupsReader(), directory, selector, pageSize,
			)
			if resolveErr != nil {
				return resolveErr
			}
			api := sdk.GetIdentityGroupMembershipsReader()
			if len(args) == 1 {
				data, err = helpers.GetIdentityGroupMembership(
					helper.GetContext(), api, directory, group.ID, args[0], pageSize,
				)
			} else {
				values, listErr := helpers.ListIdentityGroupMemberships(
					helper.GetContext(), api, directory, group.ID, pageSize,
				)
				data, err = values, listErr
				for _, value := range values {
					name := ""
					if value.Member.DisplayName != nil {
						name = *value.Member.DisplayName
					}
					rows = append(rows, groupReadRow{value.ID, name, string(value.Member.Type), value.Member.ID})
				}
			}
		} else if len(args) == 1 {
			data, err = helpers.ResolveIdentityGroup(
				helper.GetContext(), sdk.GetIdentityGroupsReader(), directory, args[0], pageSize,
			)
		} else {
			values, listErr := helpers.ListIdentityGroups(
				helper.GetContext(), sdk.GetIdentityGroupsReader(), directory, pageSize,
			)
			data, err = values, listErr
			for _, value := range values {
				kind := "unknown"
				if value.Type != nil {
					kind = string(*value.Type)
				}
				rows = append(rows, groupReadRow{ID: value.ID, Name: value.Name, Type: kind})
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
			if len(args) == 1 {
				encoded, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(helper.GetStreams().Out, string(encoded))
				return err
			}
			return printGroupReadRows(helper.GetStreams().Out, rows, memberships)
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

type groupReadRow struct{ ID, Name, Type, MemberID string }

func printGroupReadRows(out io.Writer, rows []groupReadRow, memberships bool) error {
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	header := "ID\tNAME\tTYPE"
	if memberships {
		header = "ID\tMEMBER_ID\tMEMBER_TYPE\tDISPLAY_NAME"
	}
	if _, err := fmt.Fprintln(writer, header); err != nil {
		return err
	}
	for _, value := range rows {
		var err error
		if memberships {
			_, err = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", value.ID, value.MemberID, value.Type, value.Name)
		} else {
			_, err = fmt.Fprintf(writer, "%s\t%s\t%s\n", value.ID, value.Name, value.Type)
		}
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}

func groupParentSelector(cmd *cobra.Command, parent string) (string, error) {
	id, err := cmd.Flags().GetString(parent + "-id")
	if err != nil {
		return "", err
	}
	name, err := cmd.Flags().GetString(parent + "-name")
	if err != nil {
		return "", err
	}
	if (id == "") == (name == "") {
		return "", fmt.Errorf("provide exactly one of --%s-id or --%s-name", parent, parent)
	}
	if id != "" {
		if !util.IsValidUUID(id) {
			return "", fmt.Errorf("%s-id must be a UUID", parent)
		}
		return id, nil
	}
	return name, nil
}

func groupDirectoryIDFromFlags(ctx context.Context, cmd *cobra.Command, api helpers.IdentityDirectoriesAPI,
	pageSize int64,
) (string, error) {
	selector, err := groupParentSelector(cmd, "directory")
	if err != nil {
		return "", err
	}
	if id, _ := cmd.Flags().GetString("directory-id"); id != "" {
		return id, nil
	}
	value, err := helpers.ResolveIdentityDirectory(ctx, api, selector, pageSize)
	if err != nil {
		return "", err
	}
	return value.ID, nil
}

package adopt

import (
	"context"
	"fmt"
	"maps"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/build"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

func NewIdentityGroupCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "group <id|name>", Aliases: []string{"groups"},
		Short: "Adopt an ordinary Kong Identity group using managed_by ownership",
		Args:  cobra.ExactArgs(1), PreRunE: parentPreRun,
	}
	common.AddIdentityDirectoryFlags(command)
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		setup, err := adoptCommon.SetupAdoptRun(cmd, args)
		if err != nil {
			return err
		}
		directoryID, name, err := common.IdentityDirectorySelector(cmd)
		if err != nil {
			return err
		}
		if name != "" {
			directory, err := helpers.ResolveIdentityDirectory(
				setup.Helper.GetContext(),
				setup.SDK.GetIdentityDirectoriesAPI(),
				name,
				100,
			)
			if err != nil {
				return err
			}
			directoryID = directory.ID
		}
		result, err := adoptIdentityGroup(setup.Helper.GetContext(), setup.SDK.GetIdentityGroupsAPI(), directoryID,
			args[0], setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity group")
	}
	return command, nil
}

func adoptIdentityGroup(ctx context.Context, api helpers.IdentityGroupsAPI, directoryID, identifier, namespace string,
	overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	ctx = adoptCommon.EnsureContext(ctx)
	group, err := helpers.ResolveIdentityGroup(ctx, api, directoryID, identifier, 100)
	if err != nil {
		return nil, err
	}
	if current := group.ManagedBy[resources.ChildNamespaceKey]; current != "" && !overwrite {
		return nil, fmt.Errorf(
			"group %q already has namespace %q; use --overwrite-namespace to adopt it",
			group.Name,
			current,
		)
	}
	request, err := helpers.IdentityGroupReplacement(*group)
	if err != nil {
		return nil, err
	}
	request.ManagedBy = resources.BuildChildManagedBy(
		namespace,
		resources.IsChildProtected(group.ManagedBy),
		build.Version(ctx),
	)
	response, err := api.ReplaceGroup(
		ctx,
		kkOps.ReplaceGroupRequest{DirectoryID: directoryID, GroupID: group.ID, ReplaceGroupBody: request},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to adopt identity group: %w", err)
	}
	if response == nil || response.KongGroup == nil || response.KongGroup.ID != group.ID ||
		!maps.Equal(response.KongGroup.ManagedBy, request.ManagedBy) {
		return nil, fmt.Errorf("group API did not retain adopted ownership")
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeIdentityGroup), ID: group.ID,
		Name: group.Name, Namespace: namespace,
	}, nil
}

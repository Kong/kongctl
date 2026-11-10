package adopt

import (
	"context"
	"fmt"
	"maps"
	"reflect"

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
		Use: "group [id|name]", Aliases: []string{"groups"},
		Short:   "Adopt an ordinary Kong Identity group using managed_by ownership",
		PreRunE: parentPreRun,
	}
	addChildSelectionFlags(command)
	command.Args = func(cmd *cobra.Command, args []string) error {
		_, err := readChildSelection(cmd, args, "", "")
		return err
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
		return runIdentityGroupAdoptions(cmd, args, setup, directoryID)
	}
	return command, nil
}

func adoptIdentityGroup(ctx context.Context, api helpers.IdentityGroupsAPI,
	directoryID, identifier, namespace string, overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	write, err := prepareAdoptIdentityGroup(ctx, api, directoryID, identifier, namespace, overwrite)
	if err != nil {
		return nil, err
	}
	return write()
}

func prepareAdoptIdentityGroup(ctx context.Context, api helpers.IdentityGroupsAPI,
	directoryID, identifier, namespace string, overwrite bool, expectedName ...string,
) (preparedChildAdoption, error) {
	ctx = adoptCommon.EnsureContext(ctx)
	group, err := helpers.ResolveIdentityGroup(ctx, api, directoryID, identifier, 100)
	if err != nil {
		return nil, err
	}
	if len(expectedName) != 0 && expectedName[0] != group.Name {
		return nil, fmt.Errorf("child name changed after selection; retry adoption")
	}
	metadata, err := adoptionManagedBy(group.ManagedBy, namespace, overwrite, build.Version(ctx))
	if err != nil {
		return nil, err
	}
	request, err := helpers.IdentityGroupReplacement(*group)
	if err != nil {
		return nil, err
	}
	request.ManagedBy = metadata
	return func() (*adoptCommon.AdoptResult, error) {
		if result, dryRun := adoptCommon.PreviewAdoption(
			ctx, string(resources.ResourceTypeIdentityGroup), group.ID, group.Name, namespace,
			map[string]string{"directory_id": directoryID}, "managed_by", group.ManagedBy, request.ManagedBy,
		); dryRun {
			return result, nil
		}
		fresh, err := helpers.ResolveIdentityGroup(ctx, api, directoryID, group.ID, 100)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(fresh, group) {
			return nil, fmt.Errorf("child changed after adoption validation; retry adoption")
		}
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
	}, nil
}

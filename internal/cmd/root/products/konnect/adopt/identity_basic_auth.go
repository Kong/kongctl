package adopt

import (
	"context"
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/build"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

func NewIdentityBasicAuthCmd(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal-basic-auth [basic-auth-id]", Aliases: []string{"principal-basic-auths"},
		Short: "Adopt a basic-auth username record using managed_by ownership", PreRunE: parentPreRun,
	}
	adoptCommon.AddDeclarativeExportFlags(command)
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String("principal-id", "", "Parent principal UUID")
	command.Flags().String("username", "", "Exact basic-auth username, including UUID-shaped usernames")
	if err := command.MarkFlagRequired("principal-id"); err != nil {
		return nil, err
	}
	addChildSelectionFlags(command)
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		if !util.IsValidUUID(principal) {
			return fmt.Errorf("principal-id must be a UUID")
		}
		if _, err := readChildSelection(cmd, args, "", "username"); err != nil {
			return err
		}

		if len(args) != 0 && !util.IsValidUUID(args[0]) {
			return fmt.Errorf("basic-auth ID must be a UUID")
		}
		return nil
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		setup, cleanup, err := adoptCommon.SetupExportRun(cmd, args)
		if err != nil {
			return err
		}
		defer cleanup()
		ctx := setup.Helper.GetContext()
		directory, name, err := common.IdentityDirectorySelector(cmd)
		if err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		if name != "" {
			value, err := helpers.ResolveIdentityDirectory(ctx, setup.SDK.GetIdentityDirectoriesAPI(), name, 100)
			if err != nil {
				return err
			}
			directory = value.ID
		}
		return runIdentityBasicAuthAdoptions(cmd, args, setup, directory, principal)
	}
	return command, nil
}

func adoptIdentityBasicAuth(ctx context.Context, api helpers.IdentityBasicAuthAPI,
	directory, principal, id, username, namespace string, overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	write, err := prepareAdoptIdentityBasicAuth(ctx, api, directory, principal, id, username, namespace, overwrite)
	if err != nil {
		return nil, err
	}
	return write()
}

func prepareAdoptIdentityBasicAuth(ctx context.Context, api helpers.IdentityBasicAuthAPI,
	directory, principal, id, username, namespace string, overwrite bool, expectedName ...string,
) (preparedChildAdoption, error) {
	ctx = adoptCommon.EnsureContext(ctx)
	if id != "" {
		value, err := helpers.GetIdentityBasicAuth(ctx, api, directory, principal, id)
		if err != nil {
			return nil, err
		}
		username = value.Username
	}
	value, err := helpers.ResolveIdentityBasicAuthUsername(ctx, api, directory, principal, username, 100)
	if err != nil {
		return nil, err
	}
	if id != "" && value.ID != id {
		return nil, fmt.Errorf("basic-auth identity changed during adoption")
	}
	if len(expectedName) != 0 && expectedName[0] != value.Username {
		return nil, fmt.Errorf("child name changed after selection; retry adoption")
	}
	metadata, err := adoptionManagedBy(value.ManagedBy, namespace, overwrite, build.Version(ctx))
	if err != nil {
		return nil, err
	}
	request := helpers.IdentityBasicAuthReplacement(*value)
	request.ManagedBy = metadata
	if err := adoptCommon.PrepareDeclarativeChild(
		ctx, resources.ResourceTypeIdentityBasicAuth, value.ID, value.ID, namespace,
		resources.IsChildProtected(metadata), value,
		adoptCommon.AdoptionParent{Kind: resources.ResourceTypeIdentityPrincipal, ID: principal, DirectoryID: directory},
	); err != nil {
		return nil, err
	}
	return func() (*adoptCommon.AdoptResult, error) {
		if result, dryRun := adoptCommon.PreviewAdoption(
			ctx, string(resources.ResourceTypeIdentityBasicAuth), value.ID, value.Username, namespace,
			map[string]string{"directory_id": directory, "principal_id": principal}, "managed_by", value.ManagedBy,
			request.ManagedBy,
		); dryRun {
			return adoptCommon.FinishDeclarativeChild(ctx, result)
		}
		fresh, err := helpers.GetIdentityBasicAuth(ctx, api, directory, principal, value.ID)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(fresh, value) {
			return nil, fmt.Errorf("child changed after adoption validation; retry adoption")
		}
		if _, err := helpers.ReplaceIdentityBasicAuth(ctx, api, directory, principal, value.ID, request); err != nil {
			return nil, err
		}
		return adoptCommon.FinishDeclarativeChild(ctx, &adoptCommon.AdoptResult{
			ResourceType: string(
				resources.ResourceTypeIdentityBasicAuth,
			), ID: value.ID, Name: value.Username, Namespace: namespace,
		})
	}, nil
}

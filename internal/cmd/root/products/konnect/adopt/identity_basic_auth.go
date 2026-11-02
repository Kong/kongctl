package adopt

import (
	"context"
	"fmt"

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
	common.AddIdentityDirectoryFlags(command)
	command.Flags().String("principal-id", "", "Parent principal UUID")
	command.Flags().String("username", "", "Exact basic-auth username, including UUID-shaped usernames")
	if err := command.MarkFlagRequired("principal-id"); err != nil {
		return nil, err
	}
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		username, _ := cmd.Flags().GetString("username")
		if !util.IsValidUUID(principal) {
			return fmt.Errorf("principal-id must be a UUID")
		}
		if (len(args) == 0) == (username == "") {
			return fmt.Errorf("provide exactly one basic-auth ID argument or --username")
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
		setup, err := adoptCommon.SetupAdoptRun(cmd, args)
		if err != nil {
			return err
		}
		ctx := setup.Helper.GetContext()
		directory, name, err := common.IdentityDirectorySelector(cmd)
		if err != nil {
			return err
		}
		principal, _ := cmd.Flags().GetString("principal-id")
		username, _ := cmd.Flags().GetString("username")
		if name != "" {
			value, err := helpers.ResolveIdentityDirectory(ctx, setup.SDK.GetIdentityDirectoriesAPI(), name, 100)
			if err != nil {
				return err
			}
			directory = value.ID
		}
		id := ""
		if len(args) != 0 {
			id = args[0]
		}
		result, err := adoptIdentityBasicAuth(ctx, setup.SDK.GetIdentityBasicAuthAPI(), directory, principal,
			id, username, setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity basic-auth credential")
	}
	return command, nil
}

func adoptIdentityBasicAuth(ctx context.Context, api helpers.IdentityBasicAuthAPI, directory, principal,
	id, username, namespace string, overwrite bool,
) (*adoptCommon.AdoptResult, error) {
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
	if current := value.ManagedBy[resources.ChildNamespaceKey]; current != "" && !overwrite {
		return nil, fmt.Errorf("basic-auth already has namespace %q; use --overwrite-namespace to adopt it", current)
	}
	request := helpers.IdentityBasicAuthReplacement(*value)
	request.ManagedBy = resources.BuildChildManagedBy(
		namespace,
		resources.IsChildProtected(value.ManagedBy),
		build.Version(ctx),
	)
	if _, err := helpers.ReplaceIdentityBasicAuth(ctx, api, directory, principal, value.ID, request); err != nil {
		return nil, err
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(
			resources.ResourceTypeIdentityBasicAuth,
		), ID: value.ID, Name: value.Username, Namespace: namespace,
	}, nil
}

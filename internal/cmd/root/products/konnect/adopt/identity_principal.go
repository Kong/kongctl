package adopt

import (
	"context"
	"fmt"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/build"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

func NewIdentityPrincipalCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "principal <id>", Aliases: []string{"principals"},
		Short: "Adopt a principal by UUID with an explicit directory and local ref",
		Args:  cobra.ExactArgs(1), PreRunE: parentPreRun,
	}
	command.Flags().String("directory-id", "", "Parent directory UUID")
	command.Flags().String("directory-name", "", "Exact parent directory name")
	command.Flags().String("ref", "", "Declarative ref to persist on this principal")
	command.MarkFlagsOneRequired("directory-id", "directory-name")
	command.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	if err := command.MarkFlagRequired("ref"); err != nil {
		return nil, err
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		setup, err := adoptCommon.SetupAdoptRun(cmd, args)
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
		ref, err := cmd.Flags().GetString("ref")
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
		result, err := adoptIdentityPrincipal(
			setup.Helper.GetContext(),
			setup.SDK.GetIdentityPrincipalsAPI(),
			directoryID,
			args[0],
			ref,
			setup.AdoptFlags.Namespace,
			setup.AdoptFlags.OverwriteNamespace,
		)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity principal")
	}
	return command, nil
}

func adoptIdentityPrincipal(
	ctx context.Context,
	api helpers.IdentityPrincipalsAPI,
	directoryID, id, ref, namespace string,
	overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(id) {
		return nil, fmt.Errorf("directory and principal identifiers must be UUIDs")
	}
	if err := resources.ValidateRef(ref); err != nil {
		return nil, err
	}
	ctx = adoptCommon.EnsureContext(ctx)
	current, err := helpers.GetIdentityPrincipal(ctx, api, directoryID, id)
	if err != nil {
		return nil, err
	}
	if owner := current.ManagedBy[resources.ChildNamespaceKey]; owner != "" && !overwrite {
		return nil, fmt.Errorf("principal already has namespace %q; use --overwrite-namespace to adopt it", owner)
	}
	values, err := helpers.ListIdentityPrincipals(ctx, api, directoryID, 100)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.ID != id && value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			value.ManagedBy[resources.ChildNamespaceKey] == namespace &&
			value.ManagedBy[resources.ManagedByRefKey] == ref {
			return nil, fmt.Errorf("another principal already has namespace %q and ref %q", namespace, ref)
		}
	}
	request := helpers.IdentityPrincipalReplacement(*current)
	request.ManagedBy = resources.BuildPrincipalManagedBy(
		namespace,
		ref,
		resources.IsChildProtected(current.ManagedBy),
		build.Version(ctx),
	)
	response, err := api.ReplacePrincipal(
		ctx,
		kkOps.ReplacePrincipalRequest{DirectoryID: directoryID, PrincipalID: id, ReplacePrincipalBody: request},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to adopt principal: %w", err)
	}
	if response == nil || response.KongPrincipal == nil ||
		response.KongPrincipal.ManagedBy[resources.ManagedByRefKey] != ref ||
		response.KongPrincipal.ManagedBy[resources.ChildNamespaceKey] != namespace {
		return nil, fmt.Errorf("principal API did not retain adoption identity")
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeIdentityPrincipal),
		ID:           id,
		Name:         ref,
		Namespace:    namespace,
	}, nil
}

package adopt

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/build"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

func NewIdentityPrincipalIdentityCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use: "principal-identity <id>", Aliases: []string{"principal-identities"},
		Short:   "Adopt a principal identity with explicit parent UUIDs and a local ref",
		Args:    cobra.ExactArgs(1),
		PreRunE: parentPreRun,
	}
	cmd.Flags().String("directory-id", "", "Parent directory UUID")
	cmd.Flags().String("directory-name", "", "Exact parent directory name")
	cmd.Flags().String("principal-id", "", "Parent principal UUID")
	cmd.Flags().String("ref", "", "Declarative ref to persist on this identity")
	cmd.MarkFlagsOneRequired("directory-id", "directory-name")
	cmd.MarkFlagsMutuallyExclusive("directory-id", "directory-name")
	for _, flag := range []string{"ref", "principal-id"} {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			return nil, err
		}
	}
	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		setup, err := adoptCommon.SetupAdoptRun(cmd, args)
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
		ref, err := cmd.Flags().GetString("ref")
		if err != nil {
			return err
		}
		if name != "" {
			d, err := helpers.ResolveIdentityDirectory(
				setup.Helper.GetContext(),
				setup.SDK.GetIdentityDirectoriesAPI(),
				name,
				100,
			)
			if err != nil {
				return err
			}
			directory = d.ID
		}
		result, err := adoptPrincipalIdentity(setup.Helper.GetContext(), setup.SDK.GetIdentityPrincipalIdentitiesAPI(),
			directory, principal, args[0], ref, setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity principal identity")
	}
	return cmd, nil
}

func adoptPrincipalIdentity(ctx context.Context, api helpers.IdentityPrincipalIdentitiesAPI,
	directoryID, principalID, id, ref, namespace string, overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) || !util.IsValidUUID(id) {
		return nil, fmt.Errorf("directory, principal and identity identifiers must be UUIDs")
	}
	if err := resources.ValidateRef(ref); err != nil {
		return nil, err
	}
	ctx = adoptCommon.EnsureContext(ctx)
	current, err := helpers.GetPrincipalIdentity(ctx, api, directoryID, principalID, id, 100)
	if err != nil {
		return nil, err
	}
	if current.WorkspaceID != nil {
		return nil, fmt.Errorf("adoption of identities with workspace_id is deferred pending API question Q25")
	}
	if owner := current.ManagedBy[resources.ChildNamespaceKey]; owner != "" && !overwrite {
		return nil, fmt.Errorf("identity already has namespace %q; use --overwrite-namespace to adopt it", owner)
	}
	all, err := helpers.ListPrincipalIdentities(ctx, api, directoryID, principalID, 100)
	if err != nil {
		return nil, err
	}
	for _, v := range all {
		if v.ID != id && v.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			v.ManagedBy[resources.ChildNamespaceKey] == namespace && v.ManagedBy[resources.ManagedByRefKey] == ref {
			return nil, fmt.Errorf("another identity already has namespace %q and ref %q", namespace, ref)
		}
	}
	// Reproject with the complete generated map; helpers retain public fields.
	current.ManagedBy = resources.BuildPrincipalManagedBy(
		namespace,
		ref,
		resources.IsChildProtected(current.ManagedBy),
		build.Version(ctx),
	)
	body, err := helpers.PrincipalIdentityReplacement(*current)
	if err != nil {
		return nil, err
	}
	if _, err := helpers.ReplacePrincipalIdentity(ctx, api, directoryID, principalID, id, body); err != nil {
		return nil, err
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeIdentityPrincipalIdentity),
		ID:           id,
		Name:         ref,
		Namespace:    namespace,
	}, nil
}

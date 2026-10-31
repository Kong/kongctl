package adopt

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/build"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

func NewIdentityDirectoryCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command), parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "directory <id|name>", Aliases: []string{"directories"},
		Short: "Adopt a Kong Identity directory using managed_by ownership metadata",
		Args:  cobra.ExactArgs(1), PreRunE: parentPreRun,
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		setup, err := adoptCommon.SetupAdoptRun(cmd, args)
		if err != nil {
			return err
		}
		result, err := adoptIdentityDirectory(setup.Helper.GetContext(), setup.SDK.GetIdentityDirectoriesAPI(),
			args[0], setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity directory")
	}
	return command, nil
}

func adoptIdentityDirectory(ctx context.Context, api helpers.IdentityDirectoriesAPI, identifier, namespace string,
	overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	ctx = adoptCommon.EnsureContext(ctx)
	directory, err := helpers.ResolveIdentityDirectory(ctx, api, strings.TrimSpace(identifier), 100)
	if err != nil {
		return nil, err
	}
	if current := directory.ManagedBy[resources.ChildNamespaceKey]; current != "" && !overwrite {
		return nil, fmt.Errorf("directory %q already has namespace %q; use --overwrite-namespace to adopt it",
			directory.Name, current)
	}
	request, err := helpers.IdentityDirectoryReplacement(*directory)
	if err != nil {
		return nil, err
	}
	request.ManagedBy = resources.BuildChildManagedBy(
		namespace, resources.IsChildProtected(directory.ManagedBy), build.Version(ctx),
	)
	response, err := api.ReplaceDirectory(ctx, directory.ID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to adopt Kong Identity directory: %w", err)
	}
	if response == nil || response.KongDirectory == nil {
		return nil, fmt.Errorf("missing adopted Kong Identity directory response")
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeIdentityDirectory),
		ID:           directory.ID, Name: directory.Name, Namespace: namespace,
	}, nil
}

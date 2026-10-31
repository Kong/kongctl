package adopt

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

func NewIdentityAuthServerCmd(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "auth-server <id|name>", Aliases: []string{"auth-servers", "auth_servers"},
		Short: "Adopt a Kong Identity authorization server using labels", Args: cobra.ExactArgs(1), PreRunE: parentPreRun,
	}
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		setup, err := adoptCommon.SetupAdoptRun(cmd, args)
		if err != nil {
			return err
		}
		result, err := adoptIdentityAuthServer(
			setup.Helper.GetContext(), setup.SDK.GetIdentityAuthServersAPI(), args[0],
			setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace,
		)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity auth server")
	}
	return command, nil
}

func adoptIdentityAuthServer(ctx context.Context, api helpers.IdentityAuthServersAPI, identifier, namespace string,
	overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	ctx = adoptCommon.EnsureContext(ctx)
	current, err := helpers.ResolveIdentityAuthServer(ctx, api, identifier, 100)
	if err != nil {
		return nil, err
	}
	if owner := current.Labels[labels.NamespaceKey]; owner != "" && !overwrite {
		return nil, fmt.Errorf("auth server already has namespace %q; use --overwrite-namespace to adopt it", owner)
	}
	protected := "false"
	if labels.IsProtectedResource(current.Labels) {
		protected = "true"
	}
	request := kkComps.UpdateAuthServer{
		Labels: map[string]*string{labels.NamespaceKey: &namespace, labels.ProtectedKey: &protected},
	}
	response, err := api.UpdateAuthServer(ctx, current.ID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to adopt auth server: %w", err)
	}
	if response == nil || response.AuthServer == nil || response.AuthServer.Labels[labels.NamespaceKey] != namespace {
		return nil, fmt.Errorf("auth-server API did not retain adoption labels")
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeIdentityAuthServer),
		ID:           current.ID, Name: current.Name, Namespace: namespace,
	}, nil
}

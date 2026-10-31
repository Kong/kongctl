package adopt

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

func NewIdentityAuthServerClientCmd(verb verbs.VerbValue, addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	command := &cobra.Command{
		Use: "auth-server-client [client-id]", Aliases: []string{"auth-server-clients", "auth_server_clients"},
		Short: "Adopt an OAuth client using labels under an explicit authorization server", PreRunE: parentPreRun,
	}
	command.Flags().String("auth-server-id", "", "Parent authorization server UUID")
	command.Flags().String("auth-server-name", "", "Exact parent authorization server name")
	command.Flags().String("client-name", "", "Exact OAuth client name instead of a client ID argument")
	command.MarkFlagsOneRequired("auth-server-id", "auth-server-name")
	command.MarkFlagsMutuallyExclusive("auth-server-id", "auth-server-name")
	command.Args = func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("client-name")
		if err != nil {
			return err
		}
		if (len(args) == 0) == (name == "") {
			return fmt.Errorf("provide a client ID argument or --client-name")
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
		id, err := cmd.Flags().GetString("auth-server-id")
		if err != nil {
			return err
		}
		name, err := cmd.Flags().GetString("auth-server-name")
		if err != nil {
			return err
		}
		if name != "" {
			server, err := helpers.ResolveIdentityAuthServer(
				setup.Helper.GetContext(),
				setup.SDK.GetIdentityAuthServersAPI(),
				name,
				100,
			)
			if err != nil {
				return err
			}
			id = server.ID
		}
		selector, err := cmd.Flags().GetString("client-name")
		if err != nil {
			return err
		}
		byName := selector != ""
		if !byName {
			selector = args[0]
		}
		result, err := adoptIdentityAuthServerClient(
			setup.Helper.GetContext(),
			setup.SDK.GetIdentityAuthServerClientsAPI(),
			id,
			selector,
			byName,
			setup.AdoptFlags.Namespace,
			setup.AdoptFlags.OverwriteNamespace,
		)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity auth-server client")
	}
	return command, nil
}

func adoptIdentityAuthServerClient(ctx context.Context, api helpers.IdentityAuthServerClientsAPI,
	parent, identifier string, byName bool, namespace string, overwrite bool,
) (*adoptCommon.AdoptResult, error) {
	if !util.IsValidUUID(parent) || namespace == "" {
		return nil, fmt.Errorf("valid auth-server UUID and namespace are required")
	}
	ctx = adoptCommon.EnsureContext(ctx)
	current, err := helpers.ResolveIdentityAuthServerClient(ctx, api, parent, identifier, byName, 100)
	if err != nil {
		return nil, err
	}
	if owner := current.Labels[labels.NamespaceKey]; owner != "" && !overwrite {
		return nil, fmt.Errorf("client already has namespace %q; use --overwrite-namespace to adopt it", owner)
	}
	if current.TokenEndpointAuthMethod == nil {
		return nil, fmt.Errorf("client authentication method is unavailable; cannot preserve it during adoption")
	}
	protected := current.Labels[labels.ProtectedKey]
	if protected == "" {
		protected = "false"
	}
	if protected != "true" && protected != "false" {
		return nil, fmt.Errorf("client has invalid protection labels")
	}
	response, err := api.UpdateAuthServerClient(ctx, kkOps.UpdateAuthServerClientRequest{
		AuthServerID: parent, ClientID: current.ID,
		UpdateClient: kkComps.UpdateClient{
			TokenEndpointAuthMethod: current.TokenEndpointAuthMethod,
			Labels: map[string]*string{
				labels.NamespaceKey: &namespace,
				labels.ProtectedKey: &protected,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to adopt auth-server client: %w", err)
	}
	if response == nil || response.AuthServerClient == nil || response.AuthServerClient.ID != current.ID ||
		response.AuthServerClient.Labels[labels.NamespaceKey] != namespace ||
		response.AuthServerClient.TokenEndpointAuthMethod == nil ||
		*response.AuthServerClient.TokenEndpointAuthMethod != *current.TokenEndpointAuthMethod {
		return nil, fmt.Errorf("client API did not retain adoption labels or authentication method")
	}
	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeIdentityAuthServerClient),
		ID:           current.ID, Name: current.Name, Namespace: namespace,
	}, nil
}

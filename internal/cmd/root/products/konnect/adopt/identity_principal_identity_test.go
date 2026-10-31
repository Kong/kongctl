package adopt

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestPrincipalIdentityAdoptionPreservesAllFourVariants(t *testing.T) {
	const directory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	const principal = "731559d3-39bd-48c7-b33c-eb192847eb21"
	const id = "731559d3-39bd-48c7-b33c-eb192847eb22"
	labels := map[string]string{"team": "identity"}
	values := []kkComps.KongPrincipalIdentity{
		kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
			ID: id, Key: "employee", Value: "123", Labels: labels,
		}),
		kkComps.CreateKongPrincipalIdentityOidc(kkComps.KongPrincipalIdentityOIDCResponse{
			ID: id, Issuer: "https://issuer.example.test",
			Claim: kkComps.KongPrincipalIdentityOIDCResponseClaim{Name: "sub", Value: "123"}, Labels: labels,
		}),
		kkComps.CreateKongPrincipalIdentityAuthServerClient(kkComps.KongPrincipalIdentityAuthServerClientResponse{
			ID: id, AuthServerID: directory, ClientID: "opaque-client", Labels: labels,
		}),
		kkComps.CreateKongPrincipalIdentityControlPlaneConsumer(kkComps.KongPrincipalIdentityCPConsumerResponse{
			ID: id, ControlPlaneID: directory, ConsumerID: principal, Labels: labels,
		}),
	}
	for _, value := range values {
		t.Run(string(value.Type), func(t *testing.T) {
			api := &fixture.PrincipalIdentitiesAPI{Parents: map[string][]kkComps.KongPrincipalIdentity{
				directory + "/" + principal: {value},
			}}
			result, err := adoptPrincipalIdentity(
				t.Context(),
				api,
				directory,
				principal,
				id,
				"employee",
				"identity-team",
				false,
			)
			require.NoError(t, err)
			require.Equal(t, id, result.ID)
			require.Equal(t, "employee", result.Name)
			require.Len(t, api.Replaces, 1)
			current, err := api.GetIdentity(t.Context(), api.Gets[0])
			require.NoError(t, err)
			require.Equal(t, value.Type, current.KongPrincipalIdentity.Type)
			request := api.Replaces[0].KongPrincipalIdentityBody
			var metadata map[string]string
			switch request.Type {
			case kkComps.KongPrincipalIdentityBodyTypeCustom:
				require.Equal(t, "123", request.KongPrincipalIdentityCustom.Value)
				metadata = request.KongPrincipalIdentityCustom.ManagedBy
			case kkComps.KongPrincipalIdentityBodyTypeOidc:
				require.Equal(t, "sub", request.KongPrincipalIdentityOIDC.Claim.Name)
				metadata = request.KongPrincipalIdentityOIDC.ManagedBy
			case kkComps.KongPrincipalIdentityBodyTypeAuthServerClient:
				require.Equal(t, "opaque-client", request.KongPrincipalIdentityAuthServerClient.ClientID)
				metadata = request.KongPrincipalIdentityAuthServerClient.ManagedBy
			case kkComps.KongPrincipalIdentityBodyTypeControlPlaneConsumer:
				require.Equal(t, principal, request.KongPrincipalIdentityCPConsumer.ConsumerID)
				metadata = request.KongPrincipalIdentityCPConsumer.ManagedBy
			}
			require.Len(t, metadata, 5)
			require.Equal(t, "employee", metadata[resources.ManagedByRefKey])
			require.Equal(t, "identity-team", metadata[resources.ChildNamespaceKey])
			_, err = adoptPrincipalIdentity(
				t.Context(),
				api,
				directory,
				principal,
				id,
				"employee",
				"identity-team",
				false,
			)
			require.ErrorContains(t, err, "overwrite-namespace")
			require.Len(t, api.Replaces, 1)
			api.Gets = nil
			_, err = adoptPrincipalIdentity(
				t.Context(),
				api,
				directory,
				"731559d3-39bd-48c7-b33c-eb192847eb23",
				id,
				"employee",
				"identity-team",
				true,
			)
			require.ErrorContains(t, err, "does not belong")
			require.Empty(t, api.Gets, "Q21: never use a detail response as proof of parent membership")
			require.Len(t, api.Replaces, 1)
		})
	}
}

func TestPrincipalIdentityAdoptionCommandRequiresScopedSelectors(t *testing.T) {
	cmd, err := NewIdentityPrincipalIdentityCmd(verbs.Adopt, nil, nil)
	require.NoError(t, err)
	require.Error(t, cmd.Args(cmd, nil))
	require.Error(t, cmd.Args(cmd, []string{"one", "two"}))
	require.NoError(t, cmd.Args(cmd, []string{"one"}))
	require.Error(t, cmd.ValidateRequiredFlags())
	for flag, value := range map[string]string{"principal-id": "parent", "ref": "employee", "directory-id": "directory"} {
		require.NoError(t, cmd.Flags().Set(flag, value))
	}
	require.NoError(t, cmd.ValidateRequiredFlags())
	require.NoError(t, cmd.ValidateFlagGroups())
	require.NoError(t, cmd.Flags().Set("directory-name", "default"))
	require.Error(t, cmd.ValidateFlagGroups())
}

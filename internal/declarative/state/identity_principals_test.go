package state

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

type changedPrincipalMetadataAPI struct{ *fixture.PrincipalAPI }

func (a changedPrincipalMetadataAPI) CreatePrincipal(ctx context.Context, directoryID string,
	request kkComps.CreatePrincipalBody, opts ...kkOps.Option,
) (*kkOps.CreatePrincipalResponse, error) {
	response, err := a.PrincipalAPI.CreatePrincipal(ctx, directoryID, request, opts...)
	if err == nil {
		response.KongPrincipal.Metadata["number"] = kkComps.CreateKongIdentityMetadataInteger(43)
	}
	return response, err
}

func (a changedPrincipalMetadataAPI) ReplacePrincipal(ctx context.Context, request kkOps.ReplacePrincipalRequest,
	opts ...kkOps.Option,
) (*kkOps.ReplacePrincipalResponse, error) {
	response, err := a.PrincipalAPI.ReplacePrincipal(ctx, request, opts...)
	if err == nil {
		response.KongPrincipal.Metadata["number"] = kkComps.CreateKongIdentityMetadataInteger(43)
	}
	return response, err
}

func TestPrincipalWritesReportChangedMetadata(t *testing.T) {
	for _, create := range []bool{true, false} {
		api := &changedPrincipalMetadataAPI{&fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{
			"directory-id": {{ID: "principal-id"}},
		}}}
		client := NewClient(ClientConfig{IdentityPrincipalsAPI: api})
		metadata := map[string]kkComps.KongIdentityMetadata{"number": kkComps.CreateKongIdentityMetadataInteger(42)}
		management := resources.BuildPrincipalManagedBy("identity-team", "alice", false, "test")
		var err error
		if create {
			_, err = client.CreateIdentityPrincipal(t.Context(), "directory-id", kkComps.CreatePrincipalBody{
				Metadata: metadata, ManagedBy: management,
			})
			require.Equal(t, 1, api.Creates)
		} else {
			err = client.ReplaceIdentityPrincipal(t.Context(), "directory-id", "principal-id", kkComps.ReplacePrincipalBody{
				Metadata: metadata, ManagedBy: management,
			})
			require.Len(t, api.Writes, 1)
		}
		require.ErrorContains(t, err, "did not retain requested metadata")
	}
}

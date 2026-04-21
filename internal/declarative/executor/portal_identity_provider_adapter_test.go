package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPortalIdentityProviderAPI struct {
	createReq kkComps.CreateIdentityProvider
	updateReq kkOps.UpdatePortalIdentityProviderRequest
}

func (s *stubPortalIdentityProviderAPI) ListPortalIdentityProviders(
	context.Context,
	kkOps.GetPortalIdentityProvidersRequest,
	...kkOps.Option,
) (*kkOps.GetPortalIdentityProvidersResponse, error) {
	return nil, nil
}

func (s *stubPortalIdentityProviderAPI) GetPortalIdentityProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetPortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubPortalIdentityProviderAPI) CreatePortalIdentityProvider(
	_ context.Context,
	_ string,
	request kkComps.CreateIdentityProvider,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalIdentityProviderResponse, error) {
	s.createReq = request
	id := "provider-1"
	return &kkOps.CreatePortalIdentityProviderResponse{
		IdentityProvider: &kkComps.IdentityProvider{ID: &id},
	}, nil
}

func (s *stubPortalIdentityProviderAPI) UpdatePortalIdentityProvider(
	_ context.Context,
	request kkOps.UpdatePortalIdentityProviderRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalIdentityProviderResponse, error) {
	s.updateReq = request
	return &kkOps.UpdatePortalIdentityProviderResponse{}, nil
}

func (s *stubPortalIdentityProviderAPI) DeletePortalIdentityProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeletePortalIdentityProviderResponse, error) {
	return nil, nil
}

var _ helpers.PortalIdentityProviderAPI = (*stubPortalIdentityProviderAPI)(nil)

func TestPortalIdentityProviderAdapterCreateUpdatesExplicitFalseEnabled(t *testing.T) {
	t.Parallel()

	api := &stubPortalIdentityProviderAPI{}
	client := state.NewClient(state.ClientConfig{PortalIdentityProviderAPI: api})
	adapter := NewPortalIdentityProviderAdapter(client)

	enabled := false
	req := kkComps.CreateIdentityProvider{
		Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
		Enabled: &enabled,
	}
	execCtx := NewExecutionContext(&planner.PlannedChange{
		Parent: &planner.ParentInfo{ID: "portal-1"},
	})

	id, err := adapter.Create(testContextWithLogger(), req, "default", execCtx)
	require.NoError(t, err)
	assert.Equal(t, "provider-1", id)

	require.NotNil(t, api.updateReq.UpdateIdentityProvider.Enabled)
	assert.False(t, *api.updateReq.UpdateIdentityProvider.Enabled)
	assert.Equal(t, "portal-1", api.updateReq.PortalID)
	assert.Equal(t, "provider-1", api.updateReq.ID)
}

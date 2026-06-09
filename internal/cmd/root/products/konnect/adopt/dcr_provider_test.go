package adopt

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
)

type dcrProviderAPIStub struct {
	t            *testing.T
	listPayloads *helpers.DCRProviderListPayload
	lastUpdate   kkComps.UpdateDcrProviderRequest
	updateID     string
	updateCalls  int
}

func (s *dcrProviderAPIStub) ListDcrProviders(
	context.Context,
	kkOps.ListDcrProvidersRequest,
	...kkOps.Option,
) (*kkOps.ListDcrProvidersResponse, error) {
	s.t.Fatalf("unexpected ListDcrProviders call")
	return nil, nil
}

func (s *dcrProviderAPIStub) ListDcrProviderPayloads(
	context.Context,
	kkOps.ListDcrProvidersRequest,
) (*helpers.DCRProviderListPayload, error) {
	return s.listPayloads, nil
}

func (s *dcrProviderAPIStub) CreateDcrProvider(
	context.Context,
	kkComps.CreateDcrProviderRequest,
) (*kkOps.CreateDcrProviderResponse, error) {
	s.t.Fatalf("unexpected CreateDcrProvider call")
	return nil, nil
}

func (s *dcrProviderAPIStub) UpdateDcrProvider(
	_ context.Context,
	id string,
	req kkComps.UpdateDcrProviderRequest,
) (*kkOps.UpdateDcrProviderResponse, error) {
	s.updateID = id
	s.updateCalls++
	s.lastUpdate = req
	return &kkOps.UpdateDcrProviderResponse{}, nil
}

func (s *dcrProviderAPIStub) DeleteDcrProvider(
	context.Context,
	string,
) (*kkOps.DeleteDcrProviderResponse, error) {
	s.t.Fatalf("unexpected DeleteDcrProvider call")
	return nil, nil
}

func TestAdoptDCRProviderAssignsNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Times(2)

	stub := &dcrProviderAPIStub{
		t: t,
		listPayloads: &helpers.DCRProviderListPayload{
			Data: []any{
				map[string]any{
					"id":     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
					"name":   "dcr-provider",
					"labels": map[string]string{"tier": "gold"},
				},
			},
			Total: 1,
		},
	}

	result, err := adoptDCRProvider(helper, stub, stubConfig{pageSize: 50}, "team-alpha", false, "dcr-provider")
	assert.NoError(t, err)
	assert.Equal(t, dcrProviderResourceType, result.ResourceType)
	assert.Equal(t, "22cd8a0b-72e7-4212-9099-0764f8e9c5ac", result.ID)
	assert.Equal(t, "team-alpha", result.Namespace)
	assert.Equal(t, "22cd8a0b-72e7-4212-9099-0764f8e9c5ac", stub.updateID)
	assert.Equal(t, 1, stub.updateCalls)
	assert.Equal(t, "gold", derefString(stub.lastUpdate.Labels["tier"]))
	assert.Equal(t, "team-alpha", derefString(stub.lastUpdate.Labels[labels.NamespaceKey]))

	helper.AssertExpectations(t)
}

func TestAdoptDCRProviderRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &dcrProviderAPIStub{
		t: t,
		listPayloads: &helpers.DCRProviderListPayload{
			Data: []any{
				map[string]any{
					"id":   "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
					"name": "dcr-provider",
					"labels": map[string]string{
						labels.NamespaceKey: "existing",
					},
				},
			},
			Total: 1,
		},
	}

	_, err := adoptDCRProvider(helper, stub, stubConfig{pageSize: 50}, "team-alpha", false, "dcr-provider")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, stub.updateCalls)

	helper.AssertExpectations(t)
}

func TestAdoptDCRProviderOverwritesExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Times(2)

	stub := &dcrProviderAPIStub{
		t: t,
		listPayloads: &helpers.DCRProviderListPayload{
			Data: []any{
				map[string]any{
					"id":   "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
					"name": "dcr-provider",
					"labels": map[string]string{
						"tier":              "gold",
						labels.NamespaceKey: "existing",
					},
				},
			},
			Total: 1,
		},
	}

	result, err := adoptDCRProvider(helper, stub, stubConfig{pageSize: 50}, "team-alpha", true, "dcr-provider")
	assert.NoError(t, err)
	assert.Equal(t, "team-alpha", result.Namespace)
	assert.Equal(t, 1, stub.updateCalls)
	assert.Equal(t, "gold", derefString(stub.lastUpdate.Labels["tier"]))
	assert.Equal(t, "team-alpha", derefString(stub.lastUpdate.Labels[labels.NamespaceKey]))

	helper.AssertExpectations(t)
}

var _ helpers.DCRProvidersAPI = (*dcrProviderAPIStub)(nil)

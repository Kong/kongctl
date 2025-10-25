package adopt

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	helpers "github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
)

type apiAPIStub struct {
	t             *testing.T
	fetchResponse *kkComps.APIResponseSchema
	listResponse  []kkComps.APIResponseSchema
	lastUpdate    kkComps.UpdateAPIRequest
	updateCalls   int
}

func (a *apiAPIStub) ListApis(
	context.Context,
	kkOps.ListApisRequest,
	...kkOps.Option,
) (*kkOps.ListApisResponse, error) {
	resp := &kkOps.ListApisResponse{
		ListAPIResponse: &kkComps.ListAPIResponse{
			Data: a.listResponse,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(a.listResponse))},
			},
		},
	}
	return resp, nil
}

func (a *apiAPIStub) FetchAPI(_ context.Context, id string, _ ...kkOps.Option) (*kkOps.FetchAPIResponse, error) {
	if id != a.fetchResponse.ID {
		a.t.Fatalf("unexpected API id: %s", id)
	}
	return &kkOps.FetchAPIResponse{APIResponseSchema: a.fetchResponse}, nil
}

func (a *apiAPIStub) CreateAPI(
	context.Context,
	kkComps.CreateAPIRequest,
	...kkOps.Option,
) (*kkOps.CreateAPIResponse, error) {
	a.t.Fatalf("unexpected CreateAPI call")
	return nil, nil
}

func (a *apiAPIStub) UpdateAPI(
	_ context.Context,
	id string,
	req kkComps.UpdateAPIRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAPIResponse, error) {
	if id != a.fetchResponse.ID {
		a.t.Fatalf("unexpected API id: %s", id)
	}
	a.updateCalls++
	a.lastUpdate = req

	labels := make(map[string]string)
	for k, v := range req.Labels {
		if v != nil {
			labels[k] = *v
		}
	}

	resp := &kkOps.UpdateAPIResponse{
		APIResponseSchema: &kkComps.APIResponseSchema{
			ID:     a.fetchResponse.ID,
			Name:   a.fetchResponse.Name,
			Labels: labels,
		},
	}
	return resp, nil
}

func (a *apiAPIStub) DeleteAPI(context.Context, string, ...kkOps.Option) (*kkOps.DeleteAPIResponse, error) {
	a.t.Fatalf("unexpected DeleteAPI call")
	return nil, nil
}

func TestAdoptAPIByNameAssignsNamespaceLabel(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	api := &apiAPIStub{
		t: t,
		fetchResponse: &kkComps.APIResponseSchema{
			ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:   stringPtr("payments"),
			Labels: map[string]string{"tier": "gold"},
		},
		listResponse: []kkComps.APIResponseSchema{
			{
				ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
				Name:   stringPtr("payments"),
				Labels: map[string]string{"tier": "gold"},
			},
		},
	}

	cfg := stubConfig{pageSize: 50}

	result, err := adoptAPI(helper, api, cfg, "team-alpha", "payments")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "api", result.ResourceType)
	assert.Equal(t, "22cd8a0b-72e7-4212-9099-0764f8e9c5ac", result.ID)
	assert.Equal(t, "payments", result.Name)
	assert.Equal(t, "team-alpha", result.Namespace)

	assert.Equal(t, 1, api.updateCalls)
	assert.Equal(t, "gold", derefString(api.lastUpdate.Labels["tier"]))
	assert.Equal(t, "team-alpha", derefString(api.lastUpdate.Labels[labels.NamespaceKey]))

	helper.AssertExpectations(t)
}

func TestAdoptAPIRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	api := &apiAPIStub{
		t: t,
		fetchResponse: &kkComps.APIResponseSchema{
			ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:   stringPtr("payments"),
			Labels: map[string]string{labels.NamespaceKey: "existing"},
		},
		listResponse: []kkComps.APIResponseSchema{
			{
				ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
				Name:   stringPtr("payments"),
				Labels: map[string]string{labels.NamespaceKey: "existing"},
			},
		},
	}

	cfg := stubConfig{pageSize: 50}

	_, err := adoptAPI(helper, api, cfg, "team-alpha", "payments")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, api.updateCalls)

	helper.AssertExpectations(t)
}

func TestResolveAPIDefaultsPageSize(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Twice()

	api := &apiAPIStub{
		t: t,
		fetchResponse: &kkComps.APIResponseSchema{
			ID:   "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name: stringPtr("billing"),
		},
		listResponse: []kkComps.APIResponseSchema{{
			ID:   "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name: stringPtr("billing"),
		}},
	}

	cfg := stubConfig{pageSize: 0}

	// Should resolve by name with fallback page size and succeed
	_, err := adoptAPI(helper, api, cfg, "platform", "billing")
	assert.NoError(t, err)
	assert.Equal(t, 1, api.updateCalls)

	helper.AssertExpectations(t)
}

var (
	_ helpers.APIAPI = (*apiAPIStub)(nil)
	_ config.Hook    = stubConfig{}
)

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

type authStrategyAPIStub struct {
	t             *testing.T
	fetchResponse *kkComps.CreateAppAuthStrategyResponse
	listResponse  []kkComps.AppAuthStrategy
	lastUpdate    kkComps.UpdateAppAuthStrategyRequest
	updateCalls   int
}

func (a *authStrategyAPIStub) ListAppAuthStrategies(
	context.Context,
	kkOps.ListAppAuthStrategiesRequest,
	...kkOps.Option,
) (*kkOps.ListAppAuthStrategiesResponse, error) {
	return &kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: a.listResponse,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(a.listResponse))},
			},
		},
	}, nil
}

func (a *authStrategyAPIStub) GetAppAuthStrategy(
	_ context.Context,
	id string,
) (*kkOps.GetAppAuthStrategyResponse, error) {
	if id != a.fetchResponse.AppAuthStrategyKeyAuthResponse.ID {
		a.t.Fatalf("unexpected auth strategy id: %s", id)
	}
	return &kkOps.GetAppAuthStrategyResponse{CreateAppAuthStrategyResponse: a.fetchResponse}, nil
}

func (a *authStrategyAPIStub) CreateAppAuthStrategy(
	context.Context,
	kkComps.CreateAppAuthStrategyRequest,
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	a.t.Fatalf("unexpected CreateAppAuthStrategy call")
	return nil, nil
}

func (a *authStrategyAPIStub) UpdateAppAuthStrategy(
	_ context.Context,
	id string,
	req kkComps.UpdateAppAuthStrategyRequest,
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	if id != a.fetchResponse.AppAuthStrategyKeyAuthResponse.ID {
		a.t.Fatalf("unexpected auth strategy id on update: %s", id)
	}
	a.updateCalls++
	a.lastUpdate = req
	return &kkOps.UpdateAppAuthStrategyResponse{CreateAppAuthStrategyResponse: a.fetchResponse}, nil
}

func (a *authStrategyAPIStub) DeleteAppAuthStrategy(
	context.Context,
	string,
) (*kkOps.DeleteAppAuthStrategyResponse, error) {
	a.t.Fatalf("unexpected DeleteAppAuthStrategy call")
	return nil, nil
}

func keyAuthStrategyResponse(id, name string, lbls map[string]string) *kkComps.CreateAppAuthStrategyResponse {
	return &kkComps.CreateAppAuthStrategyResponse{
		AppAuthStrategyKeyAuthResponse: &kkComps.AppAuthStrategyKeyAuthResponse{
			ID:           id,
			Name:         name,
			DisplayName:  name,
			StrategyType: kkComps.AppAuthStrategyKeyAuthResponseStrategyType("key_auth"),
			Configs: kkComps.AppAuthStrategyKeyAuthResponseConfigs{
				KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
			},
			Active: false,
			Labels: lbls,
		},
	}
}

func keyAuthStrategy(id, name string, lbls map[string]string) kkComps.AppAuthStrategy {
	resp := kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse{
		ID:           id,
		Name:         name,
		DisplayName:  name,
		StrategyType: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyStrategyType("key_auth"),
		Configs: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyConfigs{
			KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
		},
		Active: false,
		Labels: lbls,
	}
	return kkComps.CreateAppAuthStrategyKeyAuth(resp)
}

func TestAdoptAuthStrategyAssignsNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	stub := &authStrategyAPIStub{
		t:             t,
		fetchResponse: keyAuthStrategyResponse(id, "key-auth", map[string]string{"tier": "gold"}),
		listResponse: []kkComps.AppAuthStrategy{
			keyAuthStrategy(id, "key-auth", map[string]string{"tier": "gold"}),
		},
	}

	cfg := stubConfig{pageSize: 50}

	result, err := adoptAuthStrategy(helper, stub, cfg, "team-alpha", "key-auth")
	assert.NoError(t, err)
	assert.Equal(t, "auth_strategy", result.ResourceType)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, "team-alpha", result.Namespace)
	assert.Equal(t, 1, stub.updateCalls)
	assert.Equal(t, "gold", derefString(stub.lastUpdate.Labels["tier"]))
	assert.Equal(t, "team-alpha", derefString(stub.lastUpdate.Labels[labels.NamespaceKey]))

	helper.AssertExpectations(t)
}

func TestAdoptAuthStrategyRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	stub := &authStrategyAPIStub{
		t:             t,
		fetchResponse: keyAuthStrategyResponse(id, "key-auth", map[string]string{labels.NamespaceKey: "existing"}),
		listResponse: []kkComps.AppAuthStrategy{
			keyAuthStrategy(id, "key-auth", map[string]string{labels.NamespaceKey: "existing"}),
		},
	}

	_, err := adoptAuthStrategy(helper, stub, stubConfig{pageSize: 50}, "team-alpha", "key-auth")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, stub.updateCalls)

	helper.AssertExpectations(t)
}

func TestAdoptAuthStrategyDefaultsPageSize(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Times(2)

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	stub := &authStrategyAPIStub{
		t:             t,
		fetchResponse: keyAuthStrategyResponse(id, "key-auth", nil),
		listResponse: []kkComps.AppAuthStrategy{
			keyAuthStrategy(id, "key-auth", nil),
		},
	}

	_, err := adoptAuthStrategy(helper, stub, stubConfig{pageSize: 0}, "default", "key-auth")
	assert.NoError(t, err)
	assert.Equal(t, 1, stub.updateCalls)

	helper.AssertExpectations(t)
}

var (
	_ helpers.AppAuthStrategiesAPI = (*authStrategyAPIStub)(nil)
	_ config.Hook                  = stubConfig{}
)

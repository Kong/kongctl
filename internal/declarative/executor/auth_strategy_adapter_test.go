package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type stubAppAuthStrategiesAPI struct {
	t          *testing.T
	listData   []kkComps.AppAuthStrategy
	getByID    map[string]*kkComps.CreateAppAuthStrategyResponse
	listCalls  int
	getIDs     []string
	updateIDs  []string
	lastUpdate kkComps.UpdateAppAuthStrategyRequest
}

func (s *stubAppAuthStrategiesAPI) ListAppAuthStrategies(
	_ context.Context,
	_ kkOps.ListAppAuthStrategiesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAppAuthStrategiesResponse, error) {
	s.listCalls++

	return &kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: s.listData,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(s.listData))},
			},
		},
	}, nil
}

func (s *stubAppAuthStrategiesAPI) GetAppAuthStrategy(
	_ context.Context,
	id string,
) (*kkOps.GetAppAuthStrategyResponse, error) {
	s.getIDs = append(s.getIDs, id)

	return &kkOps.GetAppAuthStrategyResponse{
		CreateAppAuthStrategyResponse: s.getByID[id],
	}, nil
}

func (s *stubAppAuthStrategiesAPI) CreateAppAuthStrategy(
	context.Context,
	kkComps.CreateAppAuthStrategyRequest,
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	s.t.Fatalf("unexpected CreateAppAuthStrategy call")
	return nil, nil
}

func (s *stubAppAuthStrategiesAPI) UpdateAppAuthStrategy(
	_ context.Context,
	id string,
	req kkComps.UpdateAppAuthStrategyRequest,
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	s.updateIDs = append(s.updateIDs, id)
	s.lastUpdate = req

	return &kkOps.UpdateAppAuthStrategyResponse{}, nil
}

func (s *stubAppAuthStrategiesAPI) DeleteAppAuthStrategy(
	context.Context,
	string,
) (*kkOps.DeleteAppAuthStrategyResponse, error) {
	s.t.Fatalf("unexpected DeleteAppAuthStrategy call")
	return nil, nil
}

func TestAuthStrategyAdapterUpdateFallsBackToIDLookup(t *testing.T) {
	const strategyID = "1da45676-c973-4693-ab9f-c2986757ed07"
	managedLabels := map[string]string{labels.NamespaceKey: "default"}

	api := &stubAppAuthStrategiesAPI{
		t: t,
		listData: []kkComps.AppAuthStrategy{
			oidcAuthStrategy(strategyID, "oidc-e2e", managedLabels),
		},
		getByID: map[string]*kkComps.CreateAppAuthStrategyResponse{
			strategyID: oidcAuthStrategyResponse(strategyID, "oidc-e2e", managedLabels),
		},
	}

	client := state.NewClient(state.ClientConfig{AppAuthAPI: api})
	adapter := NewAuthStrategyAdapter(client)
	base := NewBaseExecutor[kkComps.CreateAppAuthStrategyRequest, kkComps.UpdateAppAuthStrategyRequest](
		adapter,
		client,
		false,
	)

	change := planner.PlannedChange{
		ID:           "1:u:application_auth_strategy:oidc-e2e",
		ResourceType: "application_auth_strategy",
		ResourceRef:  "oidc-e2e",
		ResourceID:   strategyID,
		Action:       planner.ActionUpdate,
		Fields: map[string]any{
			"configs": map[string]any{
				"openid-connect": map[string]any{
					"scopes": []string{"openid", "profile", "email"},
				},
			},
			planner.FieldStrategyType: "openid_connect",
		},
		Namespace: "default",
	}

	id, err := base.Update(testContextWithLogger(), change)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if id != strategyID {
		t.Fatalf("Update() returned id %q, want %q", id, strategyID)
	}
	if api.listCalls == 0 {
		t.Fatal("expected name lookup to list auth strategies before falling back to ID lookup")
	}
	if len(api.getIDs) != 1 || api.getIDs[0] != strategyID {
		t.Fatalf("expected ID lookup for %q, got %v", strategyID, api.getIDs)
	}
	if len(api.updateIDs) != 1 || api.updateIDs[0] != strategyID {
		t.Fatalf("expected update call for %q, got %v", strategyID, api.updateIDs)
	}
	if api.lastUpdate.Configs == nil {
		t.Fatal("expected sparse update request configs to be populated")
	}
}

func oidcAuthStrategy(id, name string, lbls map[string]string) kkComps.AppAuthStrategy {
	return kkComps.CreateAppAuthStrategyOpenidConnect(
		kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse{
			ID:          id,
			Name:        name,
			DisplayName: name,
			StrategyType: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyStrategyType(
				"openid_connect",
			),
			Configs: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyConfigs{
				OpenidConnect: kkComps.AppAuthStrategyConfigOpenIDConnect{
					Issuer:          "https://issuer.example.com",
					CredentialClaim: []string{"sub"},
					Scopes:          []string{"openid", "profile"},
					AuthMethods:     []string{"bearer"},
				},
			},
			Labels: lbls,
		},
	)
}

func oidcAuthStrategyResponse(id, name string, lbls map[string]string) *kkComps.CreateAppAuthStrategyResponse {
	return &kkComps.CreateAppAuthStrategyResponse{
		AppAuthStrategyOpenIDConnectResponse: &kkComps.AppAuthStrategyOpenIDConnectResponse{
			ID:           id,
			Name:         name,
			DisplayName:  name,
			StrategyType: kkComps.AppAuthStrategyOpenIDConnectResponseStrategyType("openid_connect"),
			Configs: kkComps.AppAuthStrategyOpenIDConnectResponseConfigs{
				OpenidConnect: kkComps.AppAuthStrategyConfigOpenIDConnect{
					Issuer:          "https://issuer.example.com",
					CredentialClaim: []string{"sub"},
					Scopes:          []string{"openid", "profile"},
					AuthMethods:     []string{"bearer"},
				},
			},
			Labels: lbls,
		},
	}
}

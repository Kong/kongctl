package planner

import (
	"context"
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/deck"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

type stubDeckAPIImplementationAPI struct{}

func (s *stubDeckAPIImplementationAPI) ListAPIImplementations(
	_ context.Context,
	_ kkOps.ListAPIImplementationsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	return &kkOps.ListAPIImplementationsResponse{
		ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{},
	}, nil
}

func (s *stubDeckAPIImplementationAPI) CreateAPIImplementation(
	_ context.Context,
	_ string,
	_ kkComps.APIImplementation,
	_ ...kkOps.Option,
) (*kkOps.CreateAPIImplementationResponse, error) {
	return nil, nil
}

func (s *stubDeckAPIImplementationAPI) DeleteAPIImplementation(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeleteAPIImplementationResponse, error) {
	return nil, nil
}

type stubDeckRunner struct {
	calls  []deck.RunOptions
	err    error
	result *deck.RunResult
}

func (s *stubDeckRunner) Run(_ context.Context, opts deck.RunOptions) (*deck.RunResult, error) {
	s.calls = append(s.calls, opts)
	if s.result != nil {
		return s.result, s.err
	}
	return &deck.RunResult{}, s.err
}

type stubGatewayServiceAPI struct {
	services []kkComps.ServiceOutput
}

func (s *stubGatewayServiceAPI) ListService(
	_ context.Context,
	_ kkOps.ListServiceRequest,
	_ ...kkOps.Option,
) (*kkOps.ListServiceResponse, error) {
	return &kkOps.ListServiceResponse{
		Object: &kkOps.ListServiceResponseBody{
			Data: s.services,
		},
	}, nil
}

func TestPlanDeckDependenciesAdded(t *testing.T) {
	cpID := "11111111-1111-1111-1111-111111111111"
	cpName := "cp"
	deckBaseDir := t.TempDir()

	api := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name: "api",
		},
		Ref: "api",
	}
	api.TryMatchKonnectResource(state.API{
		APIResponseSchema: kkComps.APIResponseSchema{
			ID:   "api-id",
			Name: "api",
		},
	})

	cp := resources.ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: cpName},
		Ref:                       "cp",
		Deck:                      &resources.DeckConfig{Files: []string{"gateway-service.yaml"}},
	}
	cp.TryMatchKonnectResource(state.ControlPlane{
		ControlPlane: kkComps.ControlPlane{
			ID:   cpID,
			Name: cpName,
		},
	})
	cp.SetDeckBaseDir(deckBaseDir)

	impl := resources.APIImplementationResource{
		Ref: "impl",
		API: "api",
		APIImplementation: kkComps.APIImplementation{
			Type: kkComps.APIImplementationTypeServiceReference,
			ServiceReference: &kkComps.ServiceReference{
				Service: &kkComps.APIImplementationService{
					ID:             "gw-service",
					ControlPlaneID: cpID,
				},
			},
		},
	}

	gw := resources.GatewayServiceResource{
		Ref:          "gw-service",
		ControlPlane: "cp",
		External: &resources.ExternalBlock{
			Selector: &resources.ExternalSelector{
				MatchFields: map[string]string{"name": "svc-name"},
			},
		},
	}

	rs := &resources.ResourceSet{
		APIs:               []resources.APIResource{api},
		APIImplementations: []resources.APIImplementationResource{impl},
		ControlPlanes:      []resources.ControlPlaneResource{cp},
		GatewayServices:    []resources.GatewayServiceResource{gw},
	}

	runner := &stubDeckRunner{
		result: &deck.RunResult{
			Stdout: `{"summary":{"creating":1,"updating":0,"deleting":0,"total":1},"errors":[]}`,
		},
	}

	stateClient := state.NewClient(state.ClientConfig{
		APIImplementationAPI: &stubDeckAPIImplementationAPI{},
		GatewayServiceAPI:    &stubGatewayServiceAPI{},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := NewPlanner(stateClient, logger)

	plan, err := p.GeneratePlan(context.Background(), rs, Options{
		Mode:      PlanModeApply,
		Generator: "test",
		Deck: DeckOptions{
			Runner:         runner,
			KonnectToken:   "token-123",
			KonnectAddress: "https://api.konghq.com",
		},
	})
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)
	expectedArgs := []string{"gateway", "diff", "--json-output", "--no-color", "gateway-service.yaml"}
	require.Equal(t, expectedArgs, runner.calls[0].Args)

	deps := plan.Summary.ByExternalTools[ResourceTypeDeck]
	require.Len(t, deps, 1)
	dep := deps[0]
	require.Equal(t, "cp", dep.ControlPlaneRef)
	require.Equal(t, cpID, dep.ControlPlaneID)
	require.Equal(t, cpName, dep.ControlPlaneName)
	require.Equal(t, deckBaseDir, dep.DeckBaseDir)
	require.Equal(t, []string{"gateway-service.yaml"}, dep.Files)
	require.Len(t, dep.GatewayServices, 1)
	require.Equal(t, "gw-service", dep.GatewayServices[0].Ref)
	require.Equal(t, "svc-name", dep.GatewayServices[0].Selector.MatchFields["name"])

	var deckChange *PlannedChange
	var apiChange *PlannedChange
	for i := range plan.Changes {
		change := &plan.Changes[i]
		switch change.ResourceType {
		case ResourceTypeDeck:
			deckChange = change
		case "api_implementation":
			apiChange = change
		}
	}

	require.NotNil(t, deckChange)
	require.Equal(t, ActionExternalTool, deckChange.Action)
	require.Len(t, deckChange.PostResolutionTargets, 1)
	target := deckChange.PostResolutionTargets[0]
	require.Equal(t, "gateway_service", target.ResourceType)
	require.Equal(t, "gw-service", target.ResourceRef)
	require.Equal(t, "cp", target.ControlPlaneRef)
	require.Equal(t, cpID, target.ControlPlaneID)
	require.Equal(t, cpName, target.ControlPlaneName)
	require.NotNil(t, target.Selector)
	require.Equal(t, "svc-name", target.Selector.MatchFields["name"])
	require.NotNil(t, apiChange)
	require.Contains(t, apiChange.DependsOn, deckChange.ID)
}

func TestResolveGatewayServiceIdentitiesDeckConfigNoMatch(t *testing.T) {
	cpID := "11111111-1111-1111-1111-111111111111"

	cp := resources.ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "cp"},
		Ref:                       "cp",
		Deck:                      &resources.DeckConfig{Files: []string{"gateway-service.yaml"}},
	}
	cp.TryMatchKonnectResource(state.ControlPlane{
		ControlPlane: kkComps.ControlPlane{
			ID:   cpID,
			Name: "cp",
		},
	})

	gw := resources.GatewayServiceResource{
		Ref:          "gw-service",
		ControlPlane: "cp",
		External: &resources.ExternalBlock{
			Selector: &resources.ExternalSelector{
				MatchFields: map[string]string{"name": "svc-name"},
			},
		},
	}

	services := []resources.GatewayServiceResource{gw}
	controlPlanes := []resources.ControlPlaneResource{cp}

	stateClient := state.NewClient(state.ClientConfig{
		GatewayServiceAPI: &stubGatewayServiceAPI{},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := NewPlanner(stateClient, logger)

	err := p.resolveGatewayServiceIdentities(context.Background(), services, controlPlanes)
	require.NoError(t, err)
}

func TestResolveGatewayServiceIdentitiesDeckConfigMatchesExisting(t *testing.T) {
	cpID := "11111111-1111-1111-1111-111111111111"
	serviceID := "svc-id"
	serviceName := "svc-name"

	cp := resources.ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "cp"},
		Ref:                       "cp",
		Deck:                      &resources.DeckConfig{Files: []string{"gateway-service.yaml"}},
	}
	cp.TryMatchKonnectResource(state.ControlPlane{
		ControlPlane: kkComps.ControlPlane{
			ID:   cpID,
			Name: "cp",
		},
	})

	gw := resources.GatewayServiceResource{
		Ref:          "gw-service",
		ControlPlane: "cp",
		External: &resources.ExternalBlock{
			Selector: &resources.ExternalSelector{
				MatchFields: map[string]string{"name": "svc-name"},
			},
		},
	}

	services := []resources.GatewayServiceResource{gw}
	controlPlanes := []resources.ControlPlaneResource{cp}

	stateClient := state.NewClient(state.ClientConfig{
		GatewayServiceAPI: &stubGatewayServiceAPI{
			services: []kkComps.ServiceOutput{{ID: &serviceID, Name: &serviceName}},
		},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := NewPlanner(stateClient, logger)

	err := p.resolveGatewayServiceIdentities(context.Background(), services, controlPlanes)
	require.NoError(t, err)
	require.Equal(t, serviceID, services[0].GetKonnectID())
}

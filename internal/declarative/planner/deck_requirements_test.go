package planner

import (
	"context"
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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

func TestPlanDeckDependenciesAdded(t *testing.T) {
	cpID := "11111111-1111-1111-1111-111111111111"

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
		ControlPlane: cpID,
		External: &resources.ExternalBlock{
			Selector: &resources.ExternalSelector{MatchFields: map[string]string{"name": "svc-name"}},
			Requires: &resources.ExternalRequires{
				Deck: []resources.DeckStep{{Args: []string{"gateway", "{{kongctl.mode}}"}}},
			},
		},
	}

	rs := &resources.ResourceSet{
		APIs:               []resources.APIResource{api},
		APIImplementations: []resources.APIImplementationResource{impl},
		GatewayServices:    []resources.GatewayServiceResource{gw},
	}

	stateClient := state.NewClient(state.ClientConfig{
		APIImplementationAPI: &stubDeckAPIImplementationAPI{},
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	p := NewPlanner(stateClient, logger)

	plan, err := p.GeneratePlan(context.Background(), rs, Options{
		Mode:      PlanModeApply,
		Generator: "test",
	})
	require.NoError(t, err)

	deps := plan.Summary.ByExternalTools[ResourceTypeDeck]
	require.Len(t, deps, 1)
	dep := deps[0]
	require.Equal(t, "gw-service", dep.GatewayServiceRef)
	require.NotNil(t, dep.Selector)
	require.Equal(t, "svc-name", dep.Selector.MatchFields["name"])
	require.Equal(t, cpID, dep.ControlPlaneID)

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
	require.NotNil(t, apiChange)
	require.Contains(t, apiChange.DependsOn, deckChange.ID)
}

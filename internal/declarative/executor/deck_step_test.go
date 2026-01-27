package executor

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/deck"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

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

func TestExecuteDeckStepUpdatesImplementation(t *testing.T) {
	cpID := "11111111-1111-1111-1111-111111111111"
	cpName := "cp-name"
	serviceID := "svc-id"
	selectorName := "svc-name"
	rootDir := t.TempDir()
	planBaseDir := filepath.Join(rootDir, "plans")
	deckBaseDir := filepath.Join("..", "configs", "prod")
	expectedWorkDir := filepath.Join(rootDir, "configs", "prod")

	runner := &stubDeckRunner{}
	stateClient := state.NewClient(state.ClientConfig{
		GatewayServiceAPI: &stubGatewayServiceAPI{
			services: []kkComps.ServiceOutput{
				{
					ID:   &serviceID,
					Name: &selectorName,
				},
			},
		},
	})

	exec := NewWithOptions(stateClient, nil, false, Options{
		DeckRunner:     runner,
		KonnectToken:   "token-123",
		KonnectBaseURL: "https://api.konghq.com",
		Mode:           planner.PlanModeApply,
		PlanBaseDir:    planBaseDir,
	})

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.Changes = []planner.PlannedChange{
		{
			ID:           "1",
			ResourceType: planner.ResourceTypeDeck,
			ResourceRef:  "cp-ref",
			Action:       planner.ActionExternalTool,
			Fields: map[string]any{
				"control_plane_ref":  cpID,
				"control_plane_id":   cpID,
				"control_plane_name": cpName,
				"deck_base_dir":      deckBaseDir,
				"files":              []string{"gateway-service.yaml"},
				"gateway_services": []map[string]any{
					{
						"ref": "gw-ref",
						"selector": map[string]any{
							"matchFields": map[string]string{
								"name": selectorName,
							},
						},
					},
				},
			},
		},
		{
			ID:           "2",
			ResourceType: "api_implementation",
			ResourceRef:  "impl",
			Action:       planner.ActionCreate,
			Fields: map[string]any{
				"service": map[string]any{
					"id":               "__REF__:gw-ref#id",
					"control_plane_id": "",
				},
			},
		},
	}

	ctx := context.WithValue(context.Background(), log.LoggerKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := exec.executeDeckStep(ctx, &plan.Changes[0], plan)
	require.NoError(t, err)

	require.Len(t, runner.calls, 1)
	require.Equal(t, "apply", runner.calls[0].Mode)
	require.Equal(t, cpName, runner.calls[0].KonnectControlPlaneName)
	require.Equal(t, expectedWorkDir, runner.calls[0].WorkDir)
	expectedArgs := []string{
		"gateway",
		"apply",
		"--json-output",
		"--no-color",
		"gateway-service.yaml",
	}
	require.Equal(t, expectedArgs, runner.calls[0].Args)

	serviceMap := plan.Changes[1].Fields["service"].(map[string]any)
	require.Equal(t, serviceID, serviceMap["id"])
	require.Equal(t, cpID, serviceMap["control_plane_id"])
	require.Equal(t, serviceID, exec.refToID["gateway_service"]["gw-ref"])
}

func TestExecutorDryRunSkipsDeckRunner(t *testing.T) {
	runner := &stubDeckRunner{}
	exec := NewWithOptions(nil, nil, true, Options{
		DeckRunner: runner,
		Mode:       planner.PlanModeApply,
	})

	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "1",
		ResourceType: planner.ResourceTypeDeck,
		ResourceRef:  "cp-ref",
		Action:       planner.ActionExternalTool,
		Fields: map[string]any{
			"control_plane_id":   "cp-id",
			"control_plane_name": "cp-name",
			"files":              []string{"gateway-service.yaml"},
			"gateway_services": []map[string]any{
				{
					"ref": "gw-ref",
					"selector": map[string]any{
						"matchFields": map[string]string{
							"name": "svc-name",
						},
					},
				},
			},
		},
	})
	plan.SetExecutionOrder([]string{"1"})

	result := exec.Execute(context.Background(), plan)
	require.Len(t, runner.calls, 0)
	require.Equal(t, 1, result.SkippedCount)
}

func TestExecuteDeckStepSkipsResolutionWithoutDependencies(t *testing.T) {
	runner := &stubDeckRunner{}
	exec := NewWithOptions(nil, nil, false, Options{
		DeckRunner:     runner,
		KonnectToken:   "token-123",
		KonnectBaseURL: "https://api.konghq.com",
		Mode:           planner.PlanModeSync,
	})

	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)
	plan.Changes = []planner.PlannedChange{
		{
			ID:           "1",
			ResourceType: planner.ResourceTypeDeck,
			ResourceRef:  "cp-ref",
			Action:       planner.ActionExternalTool,
			Fields: map[string]any{
				"control_plane_id":   "11111111-1111-1111-1111-111111111111",
				"control_plane_name": "cp-name",
				"files":              []string{"gateway-service.yaml"},
				"gateway_services": []map[string]any{
					{
						"ref": "gw-ref",
						"selector": map[string]any{
							"matchFields": map[string]string{
								"name": "svc-name",
							},
						},
					},
				},
			},
		},
	}

	ctx := context.WithValue(context.Background(), log.LoggerKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	err := exec.executeDeckStep(ctx, &plan.Changes[0], plan)
	require.NoError(t, err)
	require.Len(t, runner.calls, 1)
}

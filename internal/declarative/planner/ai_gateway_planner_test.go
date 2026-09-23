package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPlannerCreateUsesExplicitNameNotRef(t *testing.T) {
	deploymentType := kkComps.CreateAIGatewayRequestDeploymentTypeManaged
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "local-support-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				DeploymentType:     &deploymentType,
				Name:               "support-gateway",
				DisplayName:        "Support Gateway",
				MinRuntimeVersion:  new("2.1"),
				RuntimeAutoUpgrade: new(false),
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "local-support-gateway", change.ResourceRef)
	require.Equal(t, "support-gateway", change.Fields[FieldName])
	require.Equal(t, "Support Gateway", change.Fields[FieldDisplayName])
	require.Equal(t, kkComps.CreateAIGatewayRequestDeploymentTypeManaged, change.Fields[FieldDeploymentType])
	require.Equal(t, "2.1", change.Fields[FieldMinRuntimeVersion])
	require.Equal(t, false, change.Fields[FieldRuntimeAutoUpgrade])
}

func TestAIGatewayPlannerRuntimeSettings(t *testing.T) {
	current := state.AIGateway{AIGateway: kkComps.AIGateway{
		Name: "gateway", DisplayName: "Gateway",
		MinRuntimeVersion: new("2.0"), RuntimeAutoUpgrade: new(false),
	}}
	desired := resources.AIGatewayResource{
		BaseResource:           resources.BaseResource{Ref: "gateway"},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{Name: "gateway", DisplayName: "Gateway"},
	}
	p := &Planner{}
	changed, _, _ := p.shouldUpdateAIGateway(current, desired)
	require.False(t, changed, "omitted runtime settings must preserve the remote values")
	desired.MinRuntimeVersion = new("2.1")
	desired.RuntimeAutoUpgrade = new(true)
	changed, fields, diff := p.shouldUpdateAIGateway(current, desired)
	require.True(t, changed)
	require.Equal(t, "2.1", fields[FieldMinRuntimeVersion])
	require.Equal(t, true, fields[FieldRuntimeAutoUpgrade])
	require.Contains(t, diff, FieldMinRuntimeVersion)
	require.Contains(t, diff, FieldRuntimeAutoUpgrade)
	current.MinRuntimeVersion = new("2.1")
	current.RuntimeAutoUpgrade = new(true)
	changed, _, _ = p.shouldUpdateAIGateway(current, desired)
	require.False(t, changed)

	current.RuntimeAutoUpgrade = new(false)
	plan := &Plan{}
	p.planAIGatewayUpdate(current, desired, map[string]any{FieldDisplayName: "Renamed"}, nil, plan)
	require.Equal(t, false, plan.Changes[0].Fields[FieldRuntimeAutoUpgrade],
		"an unrelated update must not apply the SDK's runtime auto-upgrade default")
}

func TestAIGatewayRuntimeChangeOrdering(t *testing.T) {
	for _, downgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "downgrade"}[downgrade], func(t *testing.T) {
			plan := &Plan{Changes: []PlannedChange{
				{ID: "gateway", Action: ActionUpdate},
				{ID: "create", Action: ActionCreate},
				{ID: "update", Action: ActionUpdate},
				{ID: "delete", Action: ActionDelete},
			}}
			oldVersion, newVersion := "2.1", "2.10"
			if downgrade {
				oldVersion, newVersion = newVersion, oldVersion
			}
			orderAIGatewayRuntimeChange(plan, "gateway", 1, oldVersion, newVersion)
			if downgrade {
				require.Equal(t, []string{"create", "update", "delete"}, plan.Changes[0].DependsOn)
				for _, child := range plan.Changes[1:] {
					require.Empty(t, child.DependsOn)
				}
			} else {
				require.Empty(t, plan.Changes[0].DependsOn)
				require.Equal(t, []string{"gateway"}, plan.Changes[1].DependsOn)
				require.Equal(t, []string{"gateway"}, plan.Changes[2].DependsOn)
				require.Empty(t, plan.Changes[3].DependsOn)
			}
		})
	}
}

func TestAIGatewayPlannerMatchesByNameWhenDisplayNameChanges(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "support-gateway",
				DisplayName: "Old Support Gateway",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "local-support-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "New Support Gateway",
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionUpdate, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "local-support-gateway", change.ResourceRef)
	require.Equal(t, "gateway-id", change.ResourceID)
	require.Equal(t, "support-gateway", change.Fields[FieldName])
	require.Equal(t, "New Support Gateway", change.Fields[FieldDisplayName])
	require.Contains(t, change.ChangedFields, FieldDisplayName)
}

func TestAIGatewayPlannerCreatesForDifferentNameWithSameRefAndDisplayName(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "local-support-gateway",
				DisplayName: "Support Gateway",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "local-support-gateway"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "support-gateway", plan.Changes[0].Fields[FieldName])
}

func TestAIGatewayPlannerCreatesForDifferentNameWithSameRef(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "current-gateway-name",
				DisplayName: "Support Gateway",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "current-gateway-name"},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "desired-but-immutable-name",
				DisplayName: "Updated Support Gateway",
			},
		}},
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "desired-but-immutable-name", plan.Changes[0].Fields[FieldName])
	require.Equal(t, "Updated Support Gateway", plan.Changes[0].Fields[FieldDisplayName])
	require.NotContains(t, plan.Changes[0].ChangedFields, FieldName)
}

func TestAIGatewayPlannerNameIsAuthoritative(t *testing.T) {
	const existingID = "c3296957-12fb-4bdf-ae35-15b19a749592"
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync, PlanModeDelete} {
		for _, tc := range []struct{ ref, resolvedID string }{
			{ref: "local-ref"},
			{ref: "existing-name"},
			{ref: existingID},
			{ref: "cached-id", resolvedID: existingID},
		} {
			t.Run(string(mode)+"/"+tc.ref, func(t *testing.T) {
				client := state.NewClient(state.ClientConfig{
					AIGatewayAPI: &testAIGatewayAPI{
						gateways: []kkComps.AIGateway{{
							ID: existingID, Name: "existing-name", DisplayName: "Shared Display",
							Labels: map[string]string{labels.NamespaceKey: "default"},
						}},
					},
				})
				desired := resources.AIGatewayResource{
					BaseResource: resources.BaseResource{Ref: tc.ref},
					CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
						Name: "new-name", DisplayName: "Shared Display",
					},
				}
				// A cached ID must not override the declared name either.
				desired.SetKonnectID(tc.resolvedID)
				rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{desired}}
				rs.EnsureSyncScope().AddRoot(resources.ResourceTypeAIGateway)
				plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
				require.NoError(t, err)
				switch mode {
				case PlanModeApply:
					require.Len(t, plan.Changes, 1)
					require.Equal(t, ActionCreate, plan.Changes[0].Action)
				case PlanModeSync:
					require.Len(t, plan.Changes, 2)
					actions := map[ActionType]string{}
					for _, change := range plan.Changes {
						actions[change.Action] = change.Fields[FieldName].(string)
					}
					require.Equal(t, map[ActionType]string{ActionCreate: "new-name", ActionDelete: "existing-name"}, actions)
				case PlanModeDelete:
					require.Empty(t, plan.Changes)
					require.Len(t, plan.Warnings, 1)
					require.Contains(t, plan.Warnings[0].Message, "new-name")
					require.NotContains(t, plan.Warnings[0].Message, "Shared Display")
				}
			})
		}
	}
}

func TestAIGatewayPlannerDeleteMatchesName(t *testing.T) {
	for _, protected := range []bool{false, true} {
		t.Run(map[bool]string{false: "delete", true: "protected"}[protected], func(t *testing.T) {
			gatewayLabels := map[string]string{labels.NamespaceKey: "default"}
			if protected {
				gatewayLabels[labels.ProtectedKey] = labels.TrueValue
			}
			client := state.NewClient(state.ClientConfig{AIGatewayAPI: &testAIGatewayAPI{
				gateways: []kkComps.AIGateway{{
					ID: "gateway-id", Name: "existing-name", DisplayName: "Live Label", Labels: gatewayLabels,
				}},
			}})
			rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{{
				BaseResource: resources.BaseResource{Ref: "different-local-ref"},
				CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
					Name: "existing-name", DisplayName: "Desired Label",
				},
			}}}
			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
			if protected {
				require.ErrorContains(t, err, "existing-name")
				require.NotContains(t, err.Error(), "Desired Label")
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ActionDelete, plan.Changes[0].Action)
			require.Equal(t, "gateway-id", plan.Changes[0].ResourceID)
		})
	}
}

func TestResolveAIGatewayIdentitiesUsesNameWhenDisplayNameChanges(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{gateways: []kkComps.AIGateway{{
			ID: "gateway-id", Name: "support-gateway", DisplayName: "Old Display",
			Labels: map[string]string{labels.NamespaceKey: "default"},
		}}},
	})
	gateways := []resources.AIGatewayResource{{
		BaseResource: resources.BaseResource{Ref: "local-ref"},
		CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
			Name: "support-gateway", DisplayName: "New Display",
		},
	}}
	err := NewPlanner(client, slog.Default()).resolveAIGatewayIdentities(t.Context(), gateways)
	require.NoError(t, err)
	require.Equal(t, "gateway-id", gateways[0].GetKonnectID())
}

func TestAIGatewayPlannerDeleteUsesNameBeforeDisplayName(t *testing.T) {
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{{
				ID:          "gateway-id",
				Name:        "support-gateway",
				DisplayName: "Support Gateway With Spaces",
				Labels:      map[string]string{labels.NamespaceKey: "default"},
			}},
		},
	})
	rs := &resources.ResourceSet{}
	rs.EnsureSyncScope().AddRoot(resources.ResourceTypeAIGateway)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGateway, change.ResourceType)
	require.Equal(t, "support-gateway", change.ResourceRef)
	require.Contains(t, change.ID, "support-gateway")
}

func TestAIGatewayChildCreateLooksUpNewGatewayByName(t *testing.T) {
	agent := testAIGatewayAgentResource(t, nil)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{},
	})
	rs := testAIGatewayAgentResourceSet(agent)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)

	change := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayAgent, "booking-agent")
	require.NotNil(t, change.References)
	refInfo := change.References[FieldAIGatewayID]
	require.Equal(t, "support-gateway", refInfo.Ref)
	require.Equal(t, map[string]string{FieldName: "support-gateway"}, refInfo.LookupFields)
}

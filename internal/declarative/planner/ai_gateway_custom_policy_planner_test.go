package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayCustomPolicyManifestPlanOrder(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "examples", "declarative", "ai-gateway", "custom-policies.yaml")
	rs, err := loader.New().LoadFromSources([]loader.Source{{Path: path, Type: loader.SourceTypeFile}}, false)
	require.NoError(t, err)
	client := state.NewClient(state.ClientConfig{AIGatewayAPI: &testAIGatewayAPI{}})
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 3)
	changes := make(map[string]PlannedChange)
	for _, change := range plan.Changes {
		changes[change.ResourceType] = change
	}
	require.Contains(t, changes[ResourceTypeAIGatewayCustomPolicy].DependsOn, changes[ResourceTypeAIGateway].ID)
	require.Contains(t, changes[ResourceTypeAIGatewayPolicy].DependsOn, changes[ResourceTypeAIGatewayCustomPolicy].ID)
}

type customPolicyPlannerAPI struct {
	helpers.AIGatewayCustomPoliciesAPI
	policies []kkComps.AIGatewayCustomPolicy
}

func (a *customPolicyPlannerAPI) ListAiGatewayCustomPolicies(
	_ context.Context,
	_ kkOps.ListAiGatewayCustomPoliciesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewayCustomPoliciesResponse, error) {
	return &kkOps.ListAiGatewayCustomPoliciesResponse{
		ListAIGatewayCustomPoliciesResponse: &kkComps.ListAIGatewayCustomPoliciesResponse{Data: a.policies},
	}, nil
}

func (a *customPolicyPlannerAPI) GetAiGatewayCustomPolicy(
	_ context.Context,
	_, _ string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayCustomPolicyResponse, error) {
	return &kkOps.GetAiGatewayCustomPolicyResponse{AIGatewayCustomPolicy: &a.policies[0]}, nil
}

func TestAIGatewayCustomPolicyReconciliation(t *testing.T) {
	var current kkComps.AIGatewayCustomPolicy
	require.NoError(
		t,
		json.Unmarshal([]byte(`{"id":"custom-id","name":"plugin","type":"streaming","display_name":"Plugin",
"schema":"return {}","handler":"return {}",
"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`), &current),
	)
	desired, err := resources.AIGatewayCustomPolicyResourceFromResponse("gateway", current)
	require.NoError(t, err)
	for _, action := range []ActionType{ActionCreate, ActionUpdate, ActionDelete, "noop"} {
		t.Run(string(action), func(t *testing.T) {
			api := &customPolicyPlannerAPI{policies: []kkComps.AIGatewayCustomPolicy{current}}
			items := []resources.AIGatewayCustomPolicyResource{desired}
			if action == ActionCreate {
				api.policies = nil
			}
			if action == ActionUpdate {
				items[0].Handler = new("return { VERSION = '2.0' }")
			}
			if action == ActionDelete {
				items = nil
			}
			p := NewPlanner(state.NewClient(state.ClientConfig{AIGatewayCustomPoliciesAPI: api}), slog.Default())
			p.resources = &resources.ResourceSet{AIGatewayCustomPolicies: items}
			plan := &Plan{Metadata: PlanMetadata{Mode: PlanModeSync}}
			require.NoError(
				t,
				p.planAIGatewayCustomPolicyChanges(
					t.Context(),
					"default",
					"gateway",
					"Gateway",
					"gateway-id",
					"",
					items,
					plan,
				),
			)
			if action == "noop" {
				require.Empty(t, plan.Changes)
				return
			}
			require.Len(t, plan.Changes, 1)
			change := plan.Changes[0]
			require.Equal(t, action, change.Action)
			require.Equal(t, "gateway-id", change.Parent.ID)
			if action == ActionUpdate {
				require.Contains(t, change.ChangedFields, FieldHandler)
				require.Equal(t, "return {}", change.Fields[FieldSchema])
			}
		})
	}
}

func TestAIGatewayCustomPolicyDependencies(t *testing.T) {
	p := NewPlanner(nil, slog.Default())
	plan := &Plan{Changes: []PlannedChange{
		{
			ID:           "definition",
			Namespace:    "default",
			ResourceType: ResourceTypeAIGatewayCustomPolicy,
			Action:       ActionCreate,
			Fields:       map[string]any{FieldName: "plugin"},
			Parent:       &ParentInfo{Ref: "gateway"},
		},
		{
			ID:           "instance",
			Namespace:    "default",
			ResourceType: ResourceTypeAIGatewayPolicy,
			Action:       ActionCreate,
			Fields:       map[string]any{FieldType: "plugin"},
			Parent:       &ParentInfo{Ref: "gateway"},
		},
		{
			ID:           "other",
			Namespace:    "default",
			ResourceType: ResourceTypeAIGatewayPolicy,
			Action:       ActionCreate,
			Fields:       map[string]any{FieldType: "plugin"},
			Parent:       &ParentInfo{Ref: "other"},
		},
	}}
	require.NoError(
		t,
		p.resolveAIGatewayCustomPolicyDependencies(t.Context(), "default", "gateway", "gateway-id", plan),
	)
	require.Equal(t, []string{"definition"}, plan.Changes[1].DependsOn)
	require.Empty(t, plan.Changes[2].DependsOn)
	plan.Changes[0].Action = ActionDelete
	require.ErrorContains(
		t,
		p.resolveAIGatewayCustomPolicyDependencies(t.Context(), "default", "gateway", "gateway-id", plan),
		"while planned policy",
	)
}

func TestAIGatewayCustomPolicyDeletionSafety(t *testing.T) {
	policy := testAIGatewayPolicy()
	policy.Type = "plugin"
	p := NewPlanner(
		state.NewClient(
			state.ClientConfig{
				AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{policy}},
			},
		),
		slog.Default(),
	)
	plan := &Plan{
		Changes: []PlannedChange{
			{
				ID:           "definition-delete",
				Namespace:    "default",
				ResourceType: ResourceTypeAIGatewayCustomPolicy,
				Action:       ActionDelete,
				Fields:       map[string]any{FieldName: "plugin"},
				Parent:       &ParentInfo{Ref: "gateway"},
			},
		},
	}
	require.ErrorContains(
		t,
		p.resolveAIGatewayCustomPolicyDependencies(t.Context(), "default", "gateway", "gateway-id", plan),
		"still uses it",
	)
	plan.Changes = append(
		plan.Changes,
		PlannedChange{
			ID:           "instance-delete",
			Namespace:    "default",
			ResourceType: ResourceTypeAIGatewayPolicy,
			ResourceID:   policy.ID,
			Action:       ActionDelete,
			Parent:       &ParentInfo{Ref: "gateway"},
		},
	)
	require.NoError(
		t,
		p.resolveAIGatewayCustomPolicyDependencies(t.Context(), "default", "gateway", "gateway-id", plan),
	)
	require.Equal(t, []string{"instance-delete"}, plan.Changes[0].DependsOn)
}

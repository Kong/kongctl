package planner

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixtures "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayV13PolicyPlanning(t *testing.T) {
	for _, fixture := range fixtures.EventGatewayV13Policies(t) {
		t.Run(fixture.Body[FieldName].(string), func(t *testing.T) {
			p := newTestPlanner()
			body, err := json.Marshal(fixture.Body)
			require.NoError(t, err)
			var observed kkComps.EventGatewayPolicy
			require.NoError(t, json.Unmarshal(body, &observed))
			config := fixture.Body[FieldConfig].(map[string]any)
			var compare func() (bool, map[string]any, map[string]FieldChange)
			switch fixture.Phase {
			case "produce":
				var desired resources.EventGatewayProducePolicyResource
				require.NoError(t, json.Unmarshal(body, &desired))
				require.Equal(t, fixture.Body[FieldType], producePolicyType(desired))
				updated := producePolicyWithParentPolicyID(desired, "resolved-parent")
				require.Equal(t, "resolved-parent", producePolicyParentPolicyID(updated))
				require.Equal(t, "parent-id", producePolicyParentPolicyID(desired), "preparation must not mutate input")
				compare = func() (bool, map[string]any, map[string]FieldChange) {
					return p.shouldUpdateProducePolicy(state.EventGatewayVirtualClusterProducePolicyInfo{
						EventGatewayPolicy: observed, RawConfig: config,
					}, desired)
				}
			case "consume":
				var desired resources.EventGatewayConsumePolicyResource
				require.NoError(t, json.Unmarshal(body, &desired))
				require.Equal(t, fixture.Body[FieldType], consumePolicyType(desired))
				updated := consumePolicyWithParentPolicyID(desired, "resolved-parent")
				require.Equal(t, "resolved-parent", consumePolicyParentPolicyID(updated))
				require.Equal(t, "parent-id", consumePolicyParentPolicyID(desired))
				compare = func() (bool, map[string]any, map[string]FieldChange) {
					return p.shouldUpdateConsumePolicy(state.EventGatewayConsumePolicyInfo{
						EventGatewayPolicy: observed, RawConfig: config, NormalizedLabels: observed.Labels,
					}, desired)
				}
			case "cluster":
				var desired resources.EventGatewayClusterPolicyResource
				require.NoError(t, json.Unmarshal(body, &desired))
				compare = func() (bool, map[string]any, map[string]FieldChange) {
					return p.shouldUpdateClusterPolicy(state.EventGatewayClusterPolicyInfo{
						EventGatewayPolicy: observed, RawConfig: config, NormalizedLabels: observed.Labels,
					}, desired)
				}
			}
			needsUpdate, _, changes := compare()
			require.False(t, needsUpdate, "matching policy must converge")
			require.Empty(t, changes)
			observed.Condition = new("false")
			observed.Enabled = new(true)
			observed.Description = new("outdated description")
			config = map[string]any{}
			needsUpdate, fields, changes := compare()
			require.True(t, needsUpdate)
			require.Contains(t, changes, FieldConfig)
			require.Contains(t, changes, FieldCondition)
			require.Contains(t, changes, FieldEnabled)
			require.Contains(t, changes, FieldDescription)
			require.Equal(t, fixture.Body[FieldType], fields[FieldType])
			require.Equal(t, fixture.Body[FieldConfig], fields[FieldConfig])
			if fixture.Phase != "cluster" {
				require.NotContains(t, fields, FieldParentPolicyID)
			}
		})
	}
}

func TestEventGatewayClusterPolicyTypeChangeRecreates(t *testing.T) {
	for _, fixture := range fixtures.EventGatewayV13Policies(t) {
		if fixture.Phase != "cluster" {
			continue
		}
		body, err := json.Marshal(fixture.Body)
		require.NoError(t, err)
		var desired resources.EventGatewayClusterPolicyResource
		require.NoError(t, json.Unmarshal(body, &desired))
		p := newTestPlanner()
		p.client = state.NewClient(state.ClientConfig{
			EventGatewayClusterPolicyAPI: &stubExternalEventGatewayClusterPolicyAPI{
				policies: []kkComps.EventGatewayPolicy{{
					ID: "old-policy", Name: new(desired.GetMoniker()), Type: "acls",
				}},
			},
		})
		p.resources = &resources.ResourceSet{}
		plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
		require.NoError(t, p.planClusterPolicyChangesForExistingVirtualCluster(
			t.Context(), "default", "gateway-id", "gateway", "vc-id", "vc", "VC",
			[]resources.EventGatewayClusterPolicyResource{desired}, plan,
		))
		require.Len(t, plan.Changes, 2)
		require.Equal(t, ActionDelete, plan.Changes[0].Action)
		require.Equal(t, "old-policy", plan.Changes[0].ResourceID)
		require.Equal(t, ActionCreate, plan.Changes[1].Action)
		require.Contains(t, plan.Changes[1].DependsOn, plan.Changes[0].ID)
		require.Equal(t, "request_rule_validator", plan.Changes[1].Fields[FieldType])
	}
}

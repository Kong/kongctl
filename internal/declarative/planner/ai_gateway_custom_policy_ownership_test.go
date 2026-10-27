package planner

import (
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func ownedCustomPolicy(
	t *testing.T,
	name, namespace string,
) (resources.AIGatewayCustomPolicyResource, kkComps.AIGatewayCustomPolicy) {
	t.Helper()
	desired := resources.AIGatewayCustomPolicyResource{
		BaseResource: resources.BaseResource{Ref: name, Kongctl: &resources.KongctlMeta{Namespace: new(namespace)}},
		AIGateway:    "shared", Name: name, DisplayName: name, Type: "installed", Schema: "return {}",
	}

	payload, err := desired.MutablePayloadMap()
	require.NoError(t, err)
	payload[FieldID] = name + "-id"
	payload[FieldManagedBy] = resources.BuildChildManagedBy(
		nil,
		map[string]string{"other-tool": "retained"},
		namespace,
		false,
		"dev",
	)
	payload["created_at"], payload["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var observed kkComps.AIGatewayCustomPolicy
	require.NoError(t, json.Unmarshal(data, &observed))
	return desired, observed
}

func customPolicyResourceSet(policies ...resources.AIGatewayCustomPolicyResource) *resources.ResourceSet {
	rs := federatedResourceSet()
	rs.SyncScope = resources.NewSyncScope()
	rs.SyncScope.AddRoot(resources.ResourceTypeAIGateway)
	rs.AIGatewayCustomPolicies = policies
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "shared", resources.ResourceTypeAIGatewayCustomPolicy)
	for _, policy := range policies {
		rs.SyncScope.AddChildNamespace(
			resources.ResourceTypeAIGatewayCustomPolicy,
			"shared",
			resources.GetNamespace(policy.Kongctl),
		)
	}
	return rs
}

func TestFederatedCustomPoliciesScopeAndCollisions(t *testing.T) {
	a, observedA := ownedCustomPolicy(t, "custom-a", "team-a")
	b, observedB := ownedCustomPolicy(t, "custom-b", "team-b")
	_, unowned := ownedCustomPolicy(t, "legacy", "")
	unowned.AIGatewayCustomPolicyInstalled.Labels = map[string]string{
		"KONGCTL-namespace": "team-a",
		"KONGCTL-protected": "true",
	}
	api := &customPolicyPlannerAPI{policies: []kkComps.AIGatewayCustomPolicy{observedA, observedB, unowned}}
	client := state.NewClient(customPolicyEmptyUsersConfig(api))
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		rs := customPolicyResourceSet(a, b)
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
		require.NoError(t, err)
		require.Empty(t, plan.Changes)
		empty := customPolicyResourceSet()
		empty.SyncScope.AddChildNamespace(resources.ResourceTypeAIGatewayCustomPolicy, "shared", "team-a")
		// Scope deletion to team A while retaining foreign and unowned definitions.
		client = state.NewClient(
			customPolicyEmptyUsersConfig(api),
		)
		plan, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), empty, Options{Mode: mode})
		require.NoError(t, err)
		if mode == PlanModeApply {
			require.Empty(t, plan.Changes)
		} else {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "custom-a-id", plan.Changes[0].ResourceID)
			require.Equal(t, "team-a", plan.Changes[0].Namespace)
		}
	}
	for _, name := range []string{"custom-b", "legacy"} {
		desired, _ := ownedCustomPolicy(t, name, "team-a")
		plan, err := NewPlanner(
			client,
			slog.Default(),
		).GeneratePlan(t.Context(), customPolicyResourceSet(desired), Options{Mode: PlanModeApply})
		require.Error(t, err)
		require.Nil(t, plan)
	}
}

func TestCustomPolicyProtectionAndMetadataCapacity(t *testing.T) {
	desired, observed := ownedCustomPolicy(t, "custom-a", "team-a")
	observed.AIGatewayCustomPolicyInstalled.Labels = map[string]string{"public": "retained"}
	desired.Kongctl.Protected = new(true)
	api := &customPolicyPlannerAPI{policies: []kkComps.AIGatewayCustomPolicy{observed}}
	client := state.NewClient(customPolicyEmptyUsersConfig(api))
	plan, err := NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), customPolicyResourceSet(desired), Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.NotContains(t, plan.Changes[0].Fields, FieldLabels, "omitted labels are preserved at execution")
	metadata := plan.Changes[0].Fields[FieldManagedBy].(map[string]string)
	require.Equal(t, "true", metadata[resources.ChildProtectedKey])
	require.Equal(t, "retained", metadata["other-tool"])
	require.Equal(t, desired.Schema, plan.Changes[0].Fields[FieldSchema])
	desired.ManagedBy = map[string]string{"second-tool": "overflow"}
	_, err = NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(t.Context(), customPolicyResourceSet(desired), Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "5-entry")
}

func TestCustomPolicyAliasesShareOneCompleteReconciliation(t *testing.T) {
	a, observedA := ownedCustomPolicy(t, "policy-a", "team-a")
	b, observedB := ownedCustomPolicy(t, "policy-b", "team-b")
	b.AIGateway = "alias-b"
	rs := customPolicyResourceSet(a, b)
	alias := rs.AIGateways[0]
	alias.Ref = "alias-b"
	rs.AIGateways = append(rs.AIGateways, alias)
	api := &customPolicyPlannerAPI{policies: []kkComps.AIGatewayCustomPolicy{observedA, observedB}}
	client := state.NewClient(customPolicyEmptyUsersConfig(api))
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)

	a.Name = b.Name
	rs.AIGatewayCustomPolicies = []resources.AIGatewayCustomPolicyResource{a, b}
	_, err = NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.ErrorContains(t, err,
		`duplicate desired AI Gateway custom policy name "policy-b" on gateway "shared" `+
			`(refs "policy-a" and "policy-b")`)
}

func customPolicyEmptyUsersConfig(api *customPolicyPlannerAPI) state.ClientConfig {
	return state.ClientConfig{AIGatewayCustomPoliciesAPI: api, AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{}}
}

func TestCustomPolicyExplicitEmptyLabelsSurviveSavedPlan(t *testing.T) {
	desired, observed := ownedCustomPolicy(t, "shared-policy", "team-a")
	desired.Labels = map[string]string{}
	observed.AIGatewayCustomPolicyInstalled.Labels = map[string]string{"public": "remove"}
	api := &customPolicyPlannerAPI{policies: []kkComps.AIGatewayCustomPolicy{observed}}
	plan, err := NewPlanner(state.NewClient(customPolicyEmptyUsersConfig(api)), slog.Default()).GeneratePlan(
		t.Context(), customPolicyResourceSet(desired), Options{Mode: PlanModeApply},
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved Plan
	require.NoError(t, json.Unmarshal(encoded, &saved))
	require.Contains(t, saved.Changes[0].Fields, FieldLabels)
	require.Empty(t, saved.Changes[0].Fields[FieldLabels])
	require.Contains(t, saved.Changes[0].ChangedFields, FieldLabels)
}

func TestCustomPolicyReplacementOrdersForeignInstancesAcrossAliases(t *testing.T) {
	for _, owner := range []string{"team-b", ""} {
		for _, detach := range []bool{false, true} {
			if owner == "" && detach {
				continue
			}
			t.Run(owner+"/detach="+map[bool]string{false: "false", true: "true"}[detach], func(t *testing.T) {
				instance := testAIGatewayPolicy()
				instance.Name = "foreign-instance"
				instance.Type = "old-definition"
				instance.ManagedBy = nil
				if owner != "" {
					instance.ManagedBy = resources.BuildChildManagedBy(nil, nil, owner, false, "dev")
				}
				client := state.NewClient(state.ClientConfig{AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{
					policies: []kkComps.AIGatewayPolicy{instance},
				}})
				p := NewPlanner(client, slog.Default())
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.AddChange(PlannedChange{
					ID: "new-definition", Action: ActionCreate, Namespace: "team-c",
					ResourceType: ResourceTypeAIGatewayCustomPolicy, Fields: map[string]any{FieldName: "new-definition"},
					Parent: &ParentInfo{Ref: "definition-alias", ID: "gateway-id"},
				})
				plan.AddChange(PlannedChange{
					ID: "delete-old", Action: ActionDelete, Namespace: "team-a", ResourceID: "old-id",
					ResourceType: ResourceTypeAIGatewayCustomPolicy, Fields: map[string]any{FieldName: "old-definition"},
					Parent: &ParentInfo{Ref: "shared", ID: "gateway-id"},
				})
				if detach {
					plan.AddChange(PlannedChange{
						ID: "detach-instance", Action: ActionUpdate, Namespace: owner,
						ResourceType: ResourceTypeAIGatewayPolicy, ResourceID: instance.ID,
						Fields: map[string]any{FieldType: "new-definition"},
						Parent: &ParentInfo{Ref: "instance-alias", ID: "gateway-id"},
					})
				}
				err := p.resolveAIGatewayCustomPolicyDependencies(t.Context(), "team-a", "shared", "gateway-id", plan)
				if !detach {
					require.ErrorContains(t, err, "still uses it")
					return
				}
				require.NoError(t, err)
				encoded, err := json.Marshal(plan)
				require.NoError(t, err)
				var saved Plan
				require.NoError(t, json.Unmarshal(encoded, &saved))
				result, err := NewDependencyResolver().ResolveDependenciesWithGroups(saved.Changes)
				require.NoError(t, err)
				require.Less(t, indexOf(result.ExecutionOrder, "new-definition"), indexOf(result.ExecutionOrder, "detach-instance"))
				require.Less(t, indexOf(result.ExecutionOrder, "detach-instance"), indexOf(result.ExecutionOrder, "delete-old"))
			})
		}
	}
}

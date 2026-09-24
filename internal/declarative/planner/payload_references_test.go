package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag processing
)

func TestPayloadReferencesNewAndExistingGateway(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existing], func(t *testing.T) {
			policy := testAIGatewayPolicyResource(t)
			policy.Config = map[string]any{
				"headers": map[string]any{"X-Control-Plane-Id": "__REF__:support-gateway#id"},
				"dot.key": []any{map[string]any{"0": "__REF__:support-gateway#id"}},
			}
			remote := &testAIGatewayAPI{}
			if existing {
				remote.gateways = []kkComps.AIGateway{testAIGateway()}
			}
			client := state.NewClient(state.ClientConfig{
				AIGatewayAPI: remote, AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{},
			})
			rs := &resources.ResourceSet{
				AIGateways:        []resources.AIGatewayResource{testAIGatewayResource()},
				AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
			}
			plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			change := findAIGatewayModelTestChange(t, plan, ResourceTypeAIGatewayPolicy, policy.Ref)
			if existing {
				require.Empty(t, change.PayloadReferences)
				config := change.Fields[FieldConfig].(map[string]any)
				require.Equal(t, "gateway-id", config["headers"].(map[string]any)["X-Control-Plane-Id"])
			} else {
				require.Len(t, change.PayloadReferences, 2)
				require.Len(t, change.DependsOn, 1)
				require.Equal(t, "/config/dot.key/0/0", change.PayloadReferences[0].Path)
				require.Equal(t, ResourceTypeAIGateway, change.PayloadReferences[0].ResourceType)
				data, err := json.Marshal(plan)
				require.NoError(t, err)
				var saved Plan
				require.NoError(t, json.Unmarshal(data, &saved))
				require.NoError(t, ValidatePlanCompatibility(&saved))
				require.Equal(t, change.PayloadReferences, findAIGatewayModelTestChange(t, &saved,
					ResourceTypeAIGatewayPolicy, policy.Ref).PayloadReferences)
			}
		})
	}
}

func TestPayloadLookupsRequireExplicitContext(t *testing.T) {
	for _, tc := range []struct{ name, expression, want, err string }{
		{"id", "!lookup {resource_type: ai_gateway, id: known-id}", "known-id", ""},
		{"alias", "!external {resource_type: ai_gateway, id: known-id}", "known-id", ""},
		{"missing type", "!lookup {name: gateway}", "", "requires resource_type"},
		{"unsupported type", "!lookup {resource_type: nonexistent, name: gateway}", "", "does not support"},
		{"missing scope", "!lookup {resource_type: gateway_service, name: service}", "", "requires resolved"},
		{"scoped", "!lookup {resource_type: gateway_service, parent_ref: cp, name: service}", "service-id", ""},
		{"inline parent", `!lookup {resource_type: gateway_service, name: service,
            parent: !lookup {resource_type: control_plane, id: cp-id}}`, "service-id", ""},
		{
			"wrong parent type", `!lookup {resource_type: gateway_service, name: service,
            parent: !lookup {resource_type: ai_gateway, name: gateway}}`,
			"", "parent resource_type must be control_plane",
		},
		{"unscoped parent", `!lookup {resource_type: ai_gateway, name: gateway,
            parent: !lookup {resource_type: control_plane, id: cp-id}}`, "", "does not accept parent scope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte("value: "+tc.expression), &doc))
			node := doc.Content[0].Content[1]
			placeholder, err := tags.NewExternalTagResolver(node.Tag).Resolve(node)
			require.NoError(t, err)
			policy := testAIGatewayPolicyResource(t)
			policy.Config = map[string]any{"key.with.dots": []any{placeholder}}
			cp := resources.ControlPlaneResource{BaseResource: resources.BaseResource{Ref: "cp"}}
			cp.SetKonnectID("cp-id")
			rs := &resources.ResourceSet{
				ControlPlanes:     []resources.ControlPlaneResource{cp},
				AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
			}
			p := NewPlanner(nil, slog.Default())
			p.externalResolver = newExternalLookupResolver(p)
			p.externalResolver.adapters[resources.ResourceTypeGatewayService] = func(
				_ context.Context, req externalLookupRequest,
			) (string, error) {
				require.Equal(t, "cp-id", req.ParentID)
				return "service-id", nil
			}
			_, err = p.resolveKnownPayloadReferences(t.Context(), rs)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.ErrorContains(t, err, "/config/key.with.dots/0")
			} else {
				require.NoError(t, err)
				require.Equal(t, []any{tc.want}, rs.AIGatewayPolicies[0].Config["key.with.dots"])
			}
		})
	}
}

func TestPayloadLookupRemoteParentFailures(t *testing.T) {
	t.Setenv("REMOTE_PARENT_NAME", "sensitive-parent")
	for _, tc := range []struct {
		name              string
		parents, children int
		want              string
	}{
		{"missing parent", 0, 1, "no control_plane matched"},
		{"ambiguous parent", 2, 1, "matched 2 control_plane"},
		{"missing child", 1, 0, "no gateway_service matched"},
		{"ambiguous child", 1, 2, "matched 2 gateway_service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(`value: !lookup
  resource_type: gateway_service
  name: billing
  parent: !external
    resource_type: control_plane
    name: !env REMOTE_PARENT_NAME
`), &doc))
			value, err := tags.NewExternalTagResolver(tags.TagLookup).Resolve(doc.Content[0].Content[1])
			require.NoError(t, err)
			lookup, ok := tags.ParseExternalPlaceholder(value.(string))
			require.True(t, ok)
			p := NewPlanner(nil, slog.Default())
			p.externalResolver = newExternalLookupResolver(p)
			calls := []string{}
			p.externalResolver.adapters[resources.ResourceTypeControlPlane] = func(
				_ context.Context, req externalLookupRequest,
			) (string, error) {
				calls = append(calls, "parent")
				candidates := make([]state.ControlPlane, tc.parents)
				for i := range candidates {
					candidates[i] = state.ControlPlane{ControlPlane: kkComps.ControlPlane{Name: "sensitive-parent", ID: "cp-id"}}
				}
				return matchExternalCandidates(req, candidates, func(c state.ControlPlane) string { return c.ID })
			}
			p.externalResolver.adapters[resources.ResourceTypeGatewayService] = func(
				_ context.Context, req externalLookupRequest,
			) (string, error) {
				calls = append(calls, "child")
				require.Equal(t, "cp-id", req.ParentID)
				candidates := make([]state.GatewayService, tc.children)
				for i := range candidates {
					candidates[i] = state.GatewayService{Name: "billing", ID: "service-id"}
				}
				return matchExternalCandidates(req, candidates, func(c state.GatewayService) string { return c.ID })
			}
			_, err = p.resolvePayloadLookup(t.Context(), &resources.ResourceSet{}, lookup, "/config/service_id")
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, "/config/service_id")
			require.NotContains(t, err.Error(), "sensitive-parent")
			if tc.parents == 1 {
				require.Equal(t, []string{"parent", "child"}, calls)
			} else {
				require.Equal(t, []string{"parent"}, calls)
			}
		})
	}
}

func TestPayloadReferenceComparisonNoopAndUpdate(t *testing.T) {
	for _, update := range []bool{false, true} {
		policy := testAIGatewayPolicyResource(t)
		policy.Config = map[string]any{"headers": map[string]any{"X-Control-Plane-Id": "__REF__:support-gateway#id"}}
		current := testAIGatewayPolicy()
		current.Config = map[string]any{"headers": map[string]any{"X-Control-Plane-Id": "gateway-id"}}
		if update {
			policy.DisplayName = "Updated"
		}
		client := state.NewClient(state.ClientConfig{
			AIGatewayAPI:         &testAIGatewayAPI{gateways: []kkComps.AIGateway{testAIGateway()}},
			AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{current}},
		})
		rs := &resources.ResourceSet{
			AIGateways:        []resources.AIGatewayResource{testAIGatewayResource()},
			AIGatewayPolicies: []resources.AIGatewayPolicyResource{policy},
		}
		plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
		require.NoError(t, err)
		if update {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ActionUpdate, plan.Changes[0].Action)
		} else {
			require.Empty(t, plan.Changes)
		}
	}
}

func TestPayloadReferenceCycle(t *testing.T) {
	rs := &resources.ResourceSet{APIs: []resources.APIResource{
		{BaseResource: resources.BaseResource{Ref: "a"}},
		{BaseResource: resources.BaseResource{Ref: "b"}},
	}}
	p := NewPlanner(nil, slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	plan.Changes = []PlannedChange{
		{
			ID: "a", ResourceType: ResourceTypeAPI, ResourceRef: "a", Action: ActionCreate,
			Fields: map[string]any{FieldDescription: "__REF__:b#id"},
		},
		{
			ID: "b", ResourceType: ResourceTypeAPI, ResourceRef: "b", Action: ActionCreate,
			Fields: map[string]any{FieldDescription: "__REF__:a#id"},
		},
	}
	require.NoError(t, p.bindPayloadReferences(plan, rs))
	_, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
	require.ErrorContains(t, err, "cycl")
}

func TestPayloadReferenceToUnchangedChildInRootField(t *testing.T) {
	gateway := testAIGatewayResource()
	gateway.Description = new("__REF__:mask-sensitive-data#id")
	current := testAIGateway()
	current.Description = new("policy-id")
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI:         &testAIGatewayAPI{gateways: []kkComps.AIGateway{current}},
		AIGatewayPoliciesAPI: &testAIGatewayPolicyAPI{policies: []kkComps.AIGatewayPolicy{testAIGatewayPolicy()}},
	})
	rs := &resources.ResourceSet{
		AIGateways:        []resources.AIGatewayResource{gateway},
		AIGatewayPolicies: []resources.AIGatewayPolicyResource{testAIGatewayPolicyResource(t)},
	}
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestPayloadReferenceCycleIncludesDestinationPaths(t *testing.T) {
	first, second := testAIGatewayResource(), testAIGatewayResource()
	first.Description = new("__REF__:second#id")
	second.Ref, second.Name = "second", "second"
	second.Description = new("__REF__:support-gateway#id")
	rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{first, second}}
	client := state.NewClient(state.ClientConfig{AIGatewayAPI: &testAIGatewayAPI{}})
	_, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.ErrorContains(t, err, "cycl")
	require.ErrorContains(t, err, `ai_gateway "support-gateway" field /description`)
	require.ErrorContains(t, err, `ai_gateway "second" field /description`)
}

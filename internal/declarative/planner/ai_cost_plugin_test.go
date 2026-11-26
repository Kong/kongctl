package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const pluginGatewayID = "0198c5b6-3333-7000-8000-000000000003"

func pluginPlanner(plugins []resources.AICostPlugin, installed ...state.AICostObject) *Planner {
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{BaseResource: resources.BaseResource{Ref: "engineering"}}},
	}
	rs.EnsureCostManagement().AttributionPlugins = plugins
	p := &Planner{resources: rs, resourceCache: newPlanningResourceCache()}
	p.matchedIdentities = map[string]matchedResourceIdentity{
		"engineering": {
			resourceType: resources.ResourceTypeAIGateway,
			observation:  state.AIGateway{AIGateway: kkComps.AIGateway{ID: pluginGatewayID}},
		},
	}
	p.resourceCache.aiCost[resources.ResourceTypeAICostPlugin] = installed
	return p
}

func declaredPlugin(installVersion *string) resources.AICostPlugin {
	return resources.AICostPlugin{
		BaseResource: resources.BaseResource{Ref: "engineering-attribution"},
		AIGateway:    "engineering",
		AICostPluginConfig: resources.AICostPluginConfig{
			Integrations:   []string{"codex"},
			InstallVersion: installVersion,
		},
	}
}

func installation(managedBy map[string]any) state.AICostObject {
	return state.AICostObject{
		"gateway_id":     pluginGatewayID,
		"version":        "1.3.0",
		"latest_version": "1.4.0",
		"integrations":   []any{"codex"},
		"headers":        []any{},
		"prefixes":       []any{},
		"operator_tags":  map[string]any{},
		"group_patterns": []any{},
		"managed_by":     managedBy,
	}
}

func ownedBy(namespace, ref string) map[string]any {
	return map[string]any{
		resources.ManagedByToolKey:  resources.ManagedByTool,
		resources.ChildNamespaceKey: namespace,
		resources.ChildProtectedKey: "false",
		resources.ManagedByRefKey:   ref,
	}
}

func TestAICostPluginPlanning(t *testing.T) {
	latest := "1.4.0"
	installed := "1.3.0"
	tests := []struct {
		name      string
		mode      PlanMode
		pruned    bool
		plugins   []resources.AICostPlugin
		installed []state.AICostObject
		want      []PlannedChange
		wantErr   string
	}{
		{
			name:    "installs on the gateway's ID",
			plugins: []resources.AICostPlugin{declaredPlugin(nil)},
			want: []PlannedChange{{
				Action: ActionCreate, ResourceRef: "engineering-attribution",
				Fields: map[string]any{FieldAICostGatewayID: pluginGatewayID},
			}},
		},
		{
			name:      "leaves an unchanged installation alone",
			plugins:   []resources.AICostPlugin{declaredPlugin(&installed)},
			installed: []state.AICostObject{installation(ownedBy("default", "engineering-attribution"))},
		},
		{
			name:      "updates to the install version it names",
			plugins:   []resources.AICostPlugin{declaredPlugin(&latest)},
			installed: []state.AICostObject{installation(ownedBy("default", "engineering-attribution"))},
			want: []PlannedChange{{
				Action: ActionUpdate, ResourceRef: "engineering-attribution", ResourceID: pluginGatewayID,
				ChangedFields: map[string]FieldChange{FieldAICostInstallVersion: {Old: "1.3.0", New: "1.4.0"}},
			}},
		},
		{
			name:      "refuses an installation kongctl doesn't manage",
			plugins:   []resources.AICostPlugin{declaredPlugin(nil)},
			installed: []state.AICostObject{installation(map[string]any{})},
			wantErr:   "outside kongctl management",
		},
		{
			name:      "refuses an installation another declaration manages",
			plugins:   []resources.AICostPlugin{declaredPlugin(nil)},
			installed: []state.AICostObject{installation(ownedBy("platform", "shared"))},
			wantErr:   `managed as "shared" in namespace "platform"`,
		},
		{
			name:      "removes an owned installation that sync no longer declares",
			mode:      PlanModeSync,
			pruned:    true,
			installed: []state.AICostObject{installation(ownedBy("default", "engineering-attribution"))},
			want: []PlannedChange{{
				Action: ActionDelete, ResourceRef: "engineering-attribution", ResourceID: pluginGatewayID,
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pluginPlanner(tt.plugins, tt.installed...)
			if tt.pruned {
				scope := p.resources.EnsureSyncScope()
				scope.AICostNamespaces = map[resources.ResourceType]map[string]struct{}{
					resources.ResourceTypeAICostPlugin: {"default": {}},
				}
			}
			mode := tt.mode
			if mode == "" {
				mode = PlanModeApply
			}
			plan := &Plan{Metadata: PlanMetadata{Mode: mode}}

			err := p.planAICostPlugins(t.Context(), "default", plan)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Empty(t, plan.Changes)
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Changes, len(tt.want))
			for i, want := range tt.want {
				got := plan.Changes[i]
				require.Equal(t, want.Action, got.Action)
				require.Equal(t, want.ResourceRef, got.ResourceRef)
				require.Equal(t, want.ResourceID, got.ResourceID)
				for key, value := range want.Fields {
					require.Equal(t, value, got.Fields[key], key)
				}
				if want.ChangedFields != nil {
					require.Equal(t, want.ChangedFields, got.ChangedFields)
				}
			}
		})
	}
}

func TestAICostPluginReferencesAGatewayCreatedInThePlan(t *testing.T) {
	p := pluginPlanner([]resources.AICostPlugin{declaredPlugin(nil)})
	p.matchedIdentities = map[string]matchedResourceIdentity{}
	plan := &Plan{
		Metadata: PlanMetadata{Mode: PlanModeApply},
		Changes: []PlannedChange{
			{Action: ActionCreate, ResourceType: ResourceTypeAIGateway, ResourceRef: "engineering"},
		},
	}

	require.NoError(t, p.planAICostPlugins(t.Context(), "default", plan))

	require.Len(t, plan.Changes, 2)
	require.Equal(t, tags.RefPlaceholderPrefix+"engineering#id", plan.Changes[1].Fields[FieldAICostGatewayID])
}

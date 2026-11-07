package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func authServerDesired() resources.IdentityAuthServerResource {
	return resources.IdentityAuthServerResource{
		BaseResource: resources.BaseResource{
			Ref: "issuer-ref", Kongctl: &resources.KongctlMeta{Namespace: new("auth-team")},
		},
		Name: "issuer", Audience: "https://api.example.test",
	}
}

func authServerPlan(t *testing.T, api *fixture.AuthServerAPI,
	desired []resources.IdentityAuthServerResource, mode PlanMode, scoped bool,
) (*Plan, error) {
	t.Helper()
	rs := &resources.ResourceSet{
		DefaultNamespace: "auth-team", Identity: &resources.IdentityResource{AuthServers: desired},
		SyncScope: resources.NewSyncScope(),
	}
	if scoped {
		rs.SyncScope.AddRoot(resources.ResourceTypeIdentityAuthServer)
	}
	p := NewPlanner(state.NewClient(state.ClientConfig{IdentityAuthServersAPI: api}), slog.Default())
	return p.GeneratePlan(t.Context(), rs, Options{Mode: mode})
}

func TestAuthServerLifecycleAndOwnership(t *testing.T) {
	api := &fixture.AuthServerAPI{}
	d := authServerDesired()
	plan, err := authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	api.Servers = []kkComps.AuthServer{fixture.AuthServer("issuer", "auth-team", false)}
	plan, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	d.Description = new("")
	d.TrustedOrigins = []string{}
	d.Labels = map[string]string{}
	plan, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Contains(t, plan.Changes[0].Fields, FieldTrustedOrigins)
	for _, scoped := range []bool{false, true} {
		plan, err = authServerPlan(t, api, nil, PlanModeSync, scoped)
		require.NoError(t, err)
		if scoped {
			require.Len(t, plan.Changes, 1)
		} else {
			require.Empty(t, plan.Changes)
		}
	}
	api.Servers[0].Labels[labels.NamespaceKey] = "other"
	_, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.ErrorContains(t, err, "namespace")
	delete(api.Servers[0].Labels, labels.NamespaceKey)
	_, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.ErrorContains(t, err, "adopt")
}

func TestAuthServerProtectionExternalAndDuplicates(t *testing.T) {
	api := &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{fixture.AuthServer("issuer", "auth-team", true)}}
	_, err := authServerPlan(t, api, nil, PlanModeSync, true)
	require.ErrorContains(t, err, "protected")
	d := authServerDesired()
	plan, err := authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ProtectionChange{Old: true, New: false}, plan.Changes[0].Protection)
	d.Description = new("changed")
	_, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeApply, true)
	require.ErrorContains(t, err, "protected")
	d = resources.IdentityAuthServerResource{
		BaseResource: resources.BaseResource{Ref: "shared"}, External: &resources.ExternalBlock{ID: api.Servers[0].ID},
	}
	plan, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{d}, PlanModeSync, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	duplicate := api.Servers[0]
	duplicate.ID = "duplicate"
	api.Servers = append(api.Servers, duplicate)
	_, err = authServerPlan(t, api, []resources.IdentityAuthServerResource{authServerDesired()}, PlanModeApply, true)
	require.ErrorContains(t, err, "multiple auth servers")
}

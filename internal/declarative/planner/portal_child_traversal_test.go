package planner

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func portalTraversalFixture(
	t *testing.T, existing, external bool, id string, clients state.ClientConfig, logs *bytes.Buffer,
) (
	PortalPlanner, *Planner, *resources.PortalResource, *Config,
) {
	t.Helper()
	desired := resources.PortalResource{
		BaseResource: resources.BaseResource{
			Ref: "portal-ref", Kongctl: &resources.KongctlMeta{Namespace: new("child-space")},
		},
		CreatePortal: kkComps.CreatePortal{Name: "portal-name"},
	}
	namespace := "child-space"
	if external {
		desired.External = &resources.ExternalBlock{}
		desired.SetKonnectID(id)
		namespace = resources.NamespaceExternal
	}
	api := new(MockPortalAPI)
	var current []kkComps.ListPortalsResponsePortal
	if existing {
		current = append(current, newListPortal(id, desired.Name, map[string]string{labels.NamespaceKey: "child-space"}))
	}
	api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{Data: current},
	}, nil).Maybe()
	clients.PortalAPI = api
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p := NewPlanner(state.NewClient(clients), logger)
	p.resources = &resources.ResourceSet{Portals: []resources.PortalResource{desired}, SyncScope: resources.NewSyncScope()}
	p.resources.SyncScope.AddRoot(resources.ResourceTypePortal)
	return NewPortalPlanner(NewBasePlanner(p)), p, &p.resources.Portals[0], &Config{Namespace: namespace}
}

func TestPortalChildTraversalErrorPolicy(t *testing.T) {
	for _, tt := range []struct {
		name               string
		existing, external bool
		id                 string
		wantError          bool
	}{
		{name: "new managed parent continues"},
		{name: "unresolved external parent continues", external: true},
		{name: "existing managed parent stops", existing: true, id: "portal-id", wantError: true},
		{name: "resolved external parent stops", existing: true, external: true, id: "portal-id", wantError: true},
		{name: "existing policy does not depend on nonempty ID", existing: true, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			root, p, desired, cfg := portalTraversalFixture(t, tt.existing, tt.external, tt.id, state.ClientConfig{}, &logs)
			p.desiredPortalIPAllowLists = []resources.PortalIPAllowListResource{
				{Ref: "allow-list", Portal: desired.Ref, AllowedIPs: []string{"192.0.2.1"}},
			}
			p.desiredPortalCustomDomains = []resources.PortalCustomDomainResource{{
				Ref: "domain", Portal: desired.Ref,
				CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{Hostname: "portal.example.com"},
			}}
			plan := NewPlan("1", "test", PlanModeApply)
			err := root.PlanChanges(t.Context(), cfg, plan)
			if tt.wantError {
				require.ErrorContains(t, err, "failed to plan portal IP allow list changes:")
				require.False(t, plan.HasChange(ResourceTypePortalCustomDomain, "domain"))
				require.NotContains(t, logs.String(), "Failed to plan portal IP allow list for new portal")
			} else {
				require.NoError(t, err)
				require.True(t, plan.HasChange(ResourceTypePortalCustomDomain, "domain"))
				require.Contains(t, logs.String(), "Failed to plan portal IP allow list for new portal")
				for _, change := range plan.Changes {
					require.Equal(t, "child-space", change.Namespace)
				}
			}
		})
	}
}

type portalTraversalSnippetAPI struct {
	mockPortalSnippetAPI
	calls int
}

func (a *portalTraversalSnippetAPI) ListPortalSnippets(
	ctx context.Context, req kkOps.ListPortalSnippetsRequest, opts ...kkOps.Option,
) (*kkOps.ListPortalSnippetsResponse, error) {
	a.calls++
	return a.mockPortalSnippetAPI.ListPortalSnippets(ctx, req, opts...)
}

func TestPortalChildTraversalExternalScope(t *testing.T) {
	for _, tt := range []struct {
		name             string
		mode             PlanMode
		declared, scoped bool
		wantCalls        int
	}{
		{"apply skips empty collection despite scope", PlanModeApply, false, true, 0},
		{"apply visits declared collection", PlanModeApply, true, false, 1},
		{"sync visits scoped empty collection", PlanModeSync, false, true, 1},
		{"sync skips unscoped collection", PlanModeSync, false, false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			api := &portalTraversalSnippetAPI{}
			root, p, desired, cfg := portalTraversalFixture(t, true, true, "portal-id",
				state.ClientConfig{PortalSnippetAPI: api}, &logs)
			if tt.declared {
				p.desiredPortalSnippets = []resources.PortalSnippetResource{
					{Ref: "snippet", Portal: desired.Ref, Name: "snippet", Content: "content"},
				}
			}
			if tt.scoped {
				p.resources.SyncScope.AddChild(resources.ResourceTypePortal, desired.Ref, resources.ResourceTypePortalSnippet)
			}
			plan := NewPlan("1", "test", tt.mode)
			require.NoError(t, root.PlanChanges(t.Context(), cfg, plan))
			require.Equal(t, tt.wantCalls, api.calls)
			require.Equal(t, tt.declared, plan.HasChange(ResourceTypePortalSnippet, "snippet"))
		})
	}
}

type portalTraversalAssetsAPI struct {
	helpers.AssetsAPI
	data  string
	calls []string
}

func (a *portalTraversalAssetsAPI) GetPortalAssetLogo(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.GetPortalAssetLogoResponse, error) {
	a.calls = append(a.calls, "logo")
	return &kkOps.GetPortalAssetLogoResponse{PortalAssetResponse: &kkComps.PortalAssetResponse{Data: a.data}}, nil
}

func (a *portalTraversalAssetsAPI) GetPortalAssetFavicon(
	_ context.Context, _ string, _ ...kkOps.Option,
) (*kkOps.GetPortalAssetFaviconResponse, error) {
	a.calls = append(a.calls, "favicon")
	return &kkOps.GetPortalAssetFaviconResponse{PortalAssetResponse: &kkComps.PortalAssetResponse{Data: a.data}}, nil
}

func TestPortalChildTraversalNestedAssets(t *testing.T) {
	for _, tt := range []struct {
		name                                     string
		existing, external, same, alreadyPlanned bool
		wantRead, wantChange                     bool
	}{
		{name: "new parent writes without reads", wantChange: true},
		{name: "existing parent skips unchanged assets", existing: true, same: true, wantRead: true},
		{name: "existing parent updates changed assets", existing: true, wantRead: true, wantChange: true},
		{name: "external parent skips nested assets", existing: true, external: true},
		{name: "existing planned assets are not duplicated", existing: true, alreadyPlanned: true, wantChange: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			data := makeDataURL("image/png", []byte("desired"))
			api := &portalTraversalAssetsAPI{data: makeDataURL("image/png", []byte("current"))}
			if tt.same {
				api.data = data
			}
			id := ""
			if tt.existing {
				id = "portal-id"
			}
			root, _, desired, cfg := portalTraversalFixture(t, tt.existing, tt.external, id,
				state.ClientConfig{AssetsAPI: api}, &logs)
			desired.Assets = &resources.PortalAssetsResource{Logo: &data, Favicon: &data}
			plan := NewPlan("1", "test", PlanModeApply)
			if tt.alreadyPlanned {
				plan.AddChange(PlannedChange{
					ID: "logo", ResourceType: ResourceTypePortalAssetLogo, ResourceRef: desired.Ref + "-logo",
				})
				plan.AddChange(PlannedChange{
					ID: "favicon", ResourceType: ResourceTypePortalAssetFavicon, ResourceRef: desired.Ref + "-favicon",
				})
			}
			require.NoError(t, root.PlanChanges(t.Context(), cfg, plan))
			if tt.wantRead {
				require.Equal(t, []string{"logo", "favicon"}, api.calls)
			} else {
				require.Empty(t, api.calls)
			}
			var assetKinds []string
			for _, change := range plan.Changes {
				if change.ResourceType == ResourceTypePortalAssetLogo || change.ResourceType == ResourceTypePortalAssetFavicon {
					assetKinds = append(assetKinds, change.ResourceType)
				}
			}
			if tt.wantChange {
				require.Equal(t, []string{ResourceTypePortalAssetLogo, ResourceTypePortalAssetFavicon}, assetKinds)
			} else {
				require.Empty(t, assetKinds)
			}
		})
	}
}

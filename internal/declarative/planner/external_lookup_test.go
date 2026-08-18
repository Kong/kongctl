package planner

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required to build tagged test nodes
)

func TestExternalLookupResolverInlineAliasesShareCache(t *testing.T) {
	t.Parallel()

	portalAPI := &MockPortalAPI{}
	portalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{newListPortal("portal-id", "Shared Portal", nil)},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
	}, nil).Once()

	client := state.NewClient(state.ClientConfig{PortalAPI: portalAPI})
	planner := NewPlanner(client, slog.Default())
	resolver := newExternalLookupResolver(planner)
	planner.externalResolver = resolver
	rs := &resources.ResourceSet{APIPublications: []resources.APIPublicationResource{
		{Ref: "external", PortalID: externalPlaceholder(t, "!external")},
		{Ref: "lookup", PortalID: externalPlaceholder(t, "!lookup")},
	}}

	require.NoError(t, resolver.resolveInlineLookups(t.Context(), rs, resources.ResourceTypePortal))
	require.Equal(t, "portal-id", rs.APIPublications[0].PortalID)
	require.Equal(t, "portal-id", rs.APIPublications[1].PortalID)
	require.NotContains(t, planner.getResourceNamespaces(rs), resources.NamespaceExternal)
	portalAPI.AssertExpectations(t)
}

func TestExternalLookupResolverRejectsUnsupportedPlacement(t *testing.T) {
	t.Parallel()

	resolver := newExternalLookupResolver(NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default()))
	_, err := resolver.resolve(context.Background(), externalLookupRequest{
		ResourceType: resources.ResourceTypeAPI,
		MatchFields:  map[string]string{"name": "products"},
		Source:       "api_publication products field portal_id",
	})
	require.ErrorContains(t, err, "does not support external lookup")
}

func TestExternalLookupResolverRedactsNestedEnvSelectorErrors(t *testing.T) {
	t.Setenv("PORTAL_LOOKUP_NAME", "secret-portal-name")

	portalAPI := &MockPortalAPI{}
	portalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 0}},
		},
	}, nil).Once()

	rs := &resources.ResourceSet{APIPublications: []resources.APIPublicationResource{{
		Ref:      "publication",
		PortalID: nestedEnvExternalPlaceholder(t, tags.TagLookup, "PORTAL_LOOKUP_NAME"),
	}}}
	resolver := newExternalLookupResolver(NewPlanner(
		state.NewClient(state.ClientConfig{PortalAPI: portalAPI}),
		slog.Default(),
	))

	err := resolver.resolveInlineLookups(t.Context(), rs, resources.ResourceTypePortal)
	require.ErrorContains(t, err, `"name"="[redacted from !env]"`)
	require.NotContains(t, err.Error(), "secret-portal-name")
	portalAPI.AssertExpectations(t)
}

func TestExternalLookupResolverSavedPlanRetainsResolvedID(t *testing.T) {
	t.Setenv("PORTAL_LOOKUP_NAME", "Shared Portal")

	portalAPI := &MockPortalAPI{}
	portalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{newListPortal("portal-id", "Shared Portal", nil)},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
		},
	}, nil).Once()

	rs := &resources.ResourceSet{APIPublications: []resources.APIPublicationResource{{
		Ref:      "publication",
		PortalID: nestedEnvExternalPlaceholder(t, tags.TagExternal, "PORTAL_LOOKUP_NAME"),
	}}}
	resolver := newExternalLookupResolver(NewPlanner(
		state.NewClient(state.ClientConfig{PortalAPI: portalAPI}),
		slog.Default(),
	))
	require.NoError(t, resolver.resolveInlineLookups(t.Context(), rs, resources.ResourceTypePortal))
	require.Equal(t, "portal-id", rs.APIPublications[0].PortalID)

	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID:           "1:c:api_publication:publication",
		ResourceType: ResourceTypeAPIPublication,
		ResourceRef:  "publication",
		Action:       ActionCreate,
		Fields:       map[string]any{FieldPortalID: rs.APIPublications[0].PortalID},
	})
	payload, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "Shared Portal")
	require.NotContains(t, string(payload), tags.EnvPlaceholderPrefix)
	require.NotContains(t, string(payload), tags.ExternalPlaceholderPrefix)

	t.Setenv("PORTAL_LOOKUP_NAME", "Different Portal")
	var savedPlan Plan
	require.NoError(t, json.Unmarshal(payload, &savedPlan))
	require.Equal(t, "portal-id", savedPlan.Changes[0].Fields[FieldPortalID])
	portalAPI.AssertExpectations(t)
}

func TestExternalLookupResolverSkipsAPIImplementationWithoutService(t *testing.T) {
	t.Parallel()

	resolver := newExternalLookupResolver(NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default()))
	rs := &resources.ResourceSet{APIImplementations: []resources.APIImplementationResource{{
		Ref: "implementation",
	}}}

	require.NoError(t, resolver.resolveInlineLookups(
		t.Context(),
		rs,
		resources.ResourceTypeControlPlane,
		resources.ResourceTypeGatewayService,
	))
}

func TestEnsureInlineExternalParentBridgesRootChildPlanning(t *testing.T) {
	t.Parallel()

	rs := &resources.ResourceSet{AIGatewayProviders: []resources.AIGatewayProviderResource{{
		BaseResource: resources.BaseResource{Ref: "provider"},
		AIGateway:    "gateway-id",
	}}}
	require.NoError(t, ensureInlineExternalParent(rs, inlineExternalParent{
		resourceType: resources.ResourceTypeAIGateway,
		id:           "gateway-id",
	}))
	require.Len(t, rs.AIGateways, 1)
	require.True(t, rs.AIGateways[0].IsExternal())
	require.Equal(t, "gateway-id", rs.AIGateways[0].GetKonnectID())
	require.Len(t, rs.GetAIGatewayProvidersForGateway("gateway-id"), 1)
}

func TestEnsureInlineExternalTraversalBridgesScopedParent(t *testing.T) {
	t.Parallel()

	const (
		gatewayRef       = "external-gateway"
		gatewayID        = "gateway-id"
		virtualClusterID = "virtual-cluster-id"
	)
	rs := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{{
			BaseResource: resources.BaseResource{Ref: gatewayRef},
			External:     &resources.ExternalBlock{ID: gatewayID},
		}},
		SyncScope: resources.NewSyncScope(),
	}
	rs.EventGatewayControlPlanes[0].SetKonnectID(gatewayID)

	require.NoError(t, ensureInlineExternalTraversal(rs, inlineExternalParent{
		resourceType: resources.ResourceTypeEventGatewayVirtualCluster,
		id:           virtualClusterID,
		ref:          virtualClusterID,
		parentID:     gatewayID,
		parentRef:    gatewayRef,
	}))
	require.Len(t, rs.EventGatewayControlPlanes, 1)
	require.Len(t, rs.EventGatewayVirtualClusters, 1)
	require.Equal(t, gatewayRef, rs.EventGatewayVirtualClusters[0].EventGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeEventGatewayControlPlane,
		gatewayRef,
		resources.ResourceTypeEventGatewayVirtualCluster,
	))
}

func TestEnsureInlineExternalTraversalMaterializesLiteralIDAncestor(t *testing.T) {
	t.Parallel()

	rs := &resources.ResourceSet{SyncScope: resources.NewSyncScope()}
	require.NoError(t, ensureInlineExternalTraversal(rs, inlineExternalParent{
		resourceType: resources.ResourceTypeEventGatewayVirtualCluster,
		id:           "virtual-cluster-id",
		ref:          "virtual-cluster-id",
		parentID:     "gateway-id",
		parentRef:    "gateway-id",
	}))
	require.Len(t, rs.EventGatewayControlPlanes, 1)
	require.Equal(t, "gateway-id", rs.EventGatewayControlPlanes[0].GetKonnectID())
	require.Len(t, rs.EventGatewayVirtualClusters, 1)
	require.Equal(t, "gateway-id", rs.EventGatewayVirtualClusters[0].EventGateway)
}

func TestEnsureInlineExternalTraversalReusesResolvedTarget(t *testing.T) {
	t.Parallel()

	rs := &resources.ResourceSet{
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{{
			BaseResource: resources.BaseResource{Ref: "gateway-ref"},
			External:     &resources.ExternalBlock{ID: "gateway-id"},
		}},
		EventGatewayVirtualClusters: []resources.EventGatewayVirtualClusterResource{{
			Ref:          "virtual-cluster-ref",
			EventGateway: "gateway-ref",
			External:     &resources.ExternalBlock{ID: "virtual-cluster-id"},
		}},
		SyncScope: resources.NewSyncScope(),
	}
	rs.EventGatewayControlPlanes[0].SetKonnectID("gateway-id")
	rs.EventGatewayVirtualClusters[0].SetKonnectID("virtual-cluster-id")

	require.Equal(t, "virtual-cluster-ref", inlineExternalResourceRef(
		rs,
		resources.ResourceTypeEventGatewayVirtualCluster,
		"virtual-cluster-id",
	))
	require.NoError(t, ensureInlineExternalTraversal(rs, inlineExternalParent{
		resourceType: resources.ResourceTypeEventGatewayVirtualCluster,
		id:           "virtual-cluster-id",
		ref:          "virtual-cluster-ref",
		parentID:     "gateway-id",
		parentRef:    "gateway-ref",
	}))
	require.Len(t, rs.EventGatewayControlPlanes, 1)
	require.Len(t, rs.EventGatewayVirtualClusters, 1)
}

func TestEnsureInlineExternalTraversalRejectsWrongAncestorType(t *testing.T) {
	t.Parallel()

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{
			BaseResource: resources.BaseResource{Ref: "ancestor"},
		}},
		SyncScope: resources.NewSyncScope(),
	}
	err := ensureInlineExternalTraversal(rs, inlineExternalParent{
		resourceType: resources.ResourceTypeEventGatewayVirtualCluster,
		id:           "virtual-cluster-id",
		ref:          "virtual-cluster-id",
		parentID:     "gateway-id",
		parentRef:    "ancestor",
	})
	require.ErrorContains(t, err, "has portal ancestor")
}

func TestExternalLookupResolverDefersDeckServiceForNewControlPlane(t *testing.T) {
	t.Parallel()

	rs := &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{{
			BaseResource: resources.BaseResource{Ref: "control-plane"},
			Deck:         &resources.DeckConfig{Files: []string{"kong.yaml"}},
		}},
		GatewayServices: []resources.GatewayServiceResource{{
			Ref:          "gateway-service",
			ControlPlane: "control-plane",
			External: &resources.ExternalBlock{Selector: &resources.ExternalSelector{
				MatchFields: map[string]string{"name": "gateway-service"},
			}},
		}},
	}

	planner := NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default())
	resolver := newExternalLookupResolver(planner)
	require.NoError(t, resolver.resolveScopedDeclarations(t.Context(), rs))
	require.Empty(t, rs.GatewayServices[0].GetKonnectID())
}

func TestExternalControlPlaneContributesOnlyExternalNamespace(t *testing.T) {
	t.Parallel()

	planner := &Planner{}
	rs := &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{{
			BaseResource: resources.BaseResource{Ref: "external-control-plane"},
			External:     &resources.ExternalBlock{ID: "control-plane-123"},
		}},
	}

	require.Equal(t, []string{resources.NamespaceExternal}, planner.getResourceNamespaces(rs))
}

func externalPlaceholder(t *testing.T, tag string) string {
	return namedExternalPlaceholder(t, tag, "Shared Portal")
}

func namedExternalPlaceholder(t *testing.T, tag, name string) string {
	t.Helper()
	value, err := tags.NewExternalTagResolver(tag).Resolve(&yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "name"},
			{Kind: yaml.ScalarNode, Value: name},
		},
	})
	require.NoError(t, err)
	return value.(string)
}

func nestedEnvExternalPlaceholder(t *testing.T, tag string, variable string) string {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(
		"value: "+tag+" {name: !env "+variable+"}\n",
	), &doc))
	value, err := tags.NewExternalTagResolver(tag).Resolve(doc.Content[0].Content[1])
	require.NoError(t, err)
	placeholder, ok := value.(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(placeholder, tags.ExternalPlaceholderPrefix))
	return placeholder
}

func TestSetStringFieldByPathScopesServiceCheckToServicePaths(t *testing.T) {
	t.Parallel()

	// A service path on an implementation without a configured service still errors.
	svcErr := setStringFieldByPath(&resources.APIImplementationResource{}, "service.id", "svc")
	require.ErrorContains(t, svcErr, "service is not configured")

	// A non-service path falls through to field resolution instead of reporting
	// the service as unconfigured.
	otherErr := setStringFieldByPath(&resources.APIImplementationResource{}, "missing", "v")
	require.Error(t, otherErr)
	require.NotContains(t, otherErr.Error(), "service is not configured")
}

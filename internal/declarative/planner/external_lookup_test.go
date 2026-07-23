package planner

import (
	"context"
	"log/slog"
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

func externalPlaceholder(t *testing.T, tag string) string {
	t.Helper()
	value, err := tags.NewExternalTagResolver(tag).Resolve(&yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "name"},
			{Kind: yaml.ScalarNode, Value: "Shared Portal"},
		},
	})
	require.NoError(t, err)
	return value.(string)
}

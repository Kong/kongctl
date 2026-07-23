package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGeneratePlan_SyncWithNoScopeDoesNotListResources(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})

	plan, err := NewPlanner(
		client,
		slog.Default(),
	).GeneratePlan(ctx, &resources.ResourceSet{}, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Empty(t, plan.Changes)

	mockPortalAPI.AssertNotCalled(t, "ListPortals", mock.Anything, mock.Anything)
	mockAPIAPI.AssertNotCalled(t, "ListApis", mock.Anything, mock.Anything)
	mockAppAuthAPI.AssertNotCalled(t, "ListAppAuthStrategies", mock.Anything, mock.Anything)
}

func TestExcludeExternalOnlyControlPlaneSyncScope(t *testing.T) {
	t.Parallel()

	t.Run("external only", func(t *testing.T) {
		t.Parallel()

		scope := resources.NewSyncScope()
		scope.AddRoot(resources.ResourceTypeControlPlane)
		rs := &resources.ResourceSet{
			ControlPlanes: []resources.ControlPlaneResource{{
				BaseResource: resources.BaseResource{Ref: "external"},
				External:     &resources.ExternalBlock{ID: "control-plane-123"},
			}},
			SyncScope: scope,
		}

		excludeExternalOnlyControlPlaneSyncScope(rs)
		require.False(t, scope.RootInScope(resources.ResourceTypeControlPlane))
	})

	t.Run("managed and external", func(t *testing.T) {
		t.Parallel()

		scope := resources.NewSyncScope()
		scope.AddRoot(resources.ResourceTypeControlPlane)
		rs := &resources.ResourceSet{
			ControlPlanes: []resources.ControlPlaneResource{
				{BaseResource: resources.BaseResource{Ref: "managed"}},
				{
					BaseResource: resources.BaseResource{Ref: "external"},
					External:     &resources.ExternalBlock{ID: "control-plane-123"},
				},
			},
			SyncScope: scope,
		}

		excludeExternalOnlyControlPlaneSyncScope(rs)
		require.True(t, scope.RootInScope(resources.ResourceTypeControlPlane))
	})

	t.Run("explicit empty collection", func(t *testing.T) {
		t.Parallel()

		scope := resources.NewSyncScope()
		scope.AddRoot(resources.ResourceTypeControlPlane)
		rs := &resources.ResourceSet{SyncScope: scope}

		excludeExternalOnlyControlPlaneSyncScope(rs)
		require.True(t, scope.RootInScope(resources.ResourceTypeControlPlane))
	})
}

func TestValidateParentScopesAllowsInlineExternalParent(t *testing.T) {
	t.Parallel()

	placeholder := externalPlaceholder(t, "!external")
	scope := resources.NewSyncScope()
	scope.AddChild(resources.ResourceTypeAIGateway, placeholder, resources.ResourceTypeAIGatewayProvider)
	require.NoError(t, validateParentScopes(scope))

	scope.RebindChildParent(resources.ResourceTypeAIGateway, placeholder, "gateway-id")
	require.True(t, scope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"gateway-id",
		resources.ResourceTypeAIGatewayProvider,
	))
	require.False(t, scope.ChildInScope(
		resources.ResourceTypeAIGateway,
		placeholder,
		resources.ResourceTypeAIGatewayProvider,
	))
}

func TestGeneratePlan_SyncPortalScopeDoesNotListUnscopedRoots(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})

	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: 0},
			},
		},
	}, nil).Once()

	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypePortal)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "docs"},
				BaseResource: resources.BaseResource{Ref: "docs"},
			},
		},
		SyncScope: scope,
	}

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(ctx, rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, ResourceTypePortal, plan.Changes[0].ResourceType)

	mockPortalAPI.AssertExpectations(t)
	mockAPIAPI.AssertNotCalled(t, "ListApis", mock.Anything, mock.Anything)
	mockAppAuthAPI.AssertNotCalled(t, "ListAppAuthStrategies", mock.Anything, mock.Anything)
}

func TestGeneratePlan_SyncAuthStrategyScopeListsAuthStrategies(t *testing.T) {
	ctx := context.Background()
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		AppAuthAPI: mockAppAuthAPI,
	})

	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{Total: 0},
				},
			},
		}, nil).Once()

	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeApplicationAuthStrategy)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(
		ctx,
		&resources.ResourceSet{SyncScope: scope},
		Options{Mode: PlanModeSync},
	)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)

	mockAppAuthAPI.AssertExpectations(t)
}

func TestGeneratePlan_SyncRejectsRootLevelEmptyChildCollections(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRootChildCollection(resources.ResourceTypeAPIDocument)

	plan, err := NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default()).GeneratePlan(
		context.Background(),
		&resources.ResourceSet{SyncScope: scope},
		Options{Mode: PlanModeSync},
	)
	require.Error(t, err)
	require.Nil(t, plan)
	assert.Contains(t, err.Error(), "empty child collections")
	assert.Contains(t, err.Error(), "api_document")
}

func TestGeneratePlan_SyncChildScopeWithoutParentSuggestsExternalWhenSupported(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddChild(resources.ResourceTypePortal, "docs-portal", resources.ResourceTypePortalPage)

	plan, err := NewPlanner(state.NewClient(state.ClientConfig{}), slog.Default()).GeneratePlan(
		context.Background(),
		&resources.ResourceSet{SyncScope: scope},
		Options{Mode: PlanModeSync},
	)
	require.Error(t, err)
	require.Nil(t, plan)
	assert.Contains(t, err.Error(), "requires the parent collection")
	assert.Contains(t, err.Error(), "_external")
}

func TestEnsurePlanningSyncScopeInfersPortalTeamRolesWithTeams(t *testing.T) {
	rs := &resources.ResourceSet{
		PortalTeams: []resources.PortalTeamResource{
			{
				Ref:    "team-a",
				Portal: "docs-portal",
			},
		},
	}

	ensurePlanningSyncScope(rs)
	require.NotNil(t, rs.SyncScope)
	assert.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypePortal,
		"docs-portal",
		resources.ResourceTypePortalTeam,
	))
	assert.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypePortal,
		"docs-portal",
		resources.ResourceTypePortalTeamRole,
	))
}

func TestEnsurePlanningSyncScopeInfersOrganizationAssignmentScope(t *testing.T) {
	rs := &resources.ResourceSet{
		Organization: &resources.OrganizationResource{
			Users: []resources.OrganizationUserResource{
				{
					Ref: "alice",
					ID:  "user-123",
				},
			},
			SystemAccounts: []resources.OrganizationSystemAccountResource{
				{
					Ref: "ci-bot",
					ID:  "system-account-123",
				},
			},
		},
	}

	ensurePlanningSyncScope(rs)
	require.NotNil(t, rs.SyncScope)
	assert.True(t, rs.SyncScope.OrganizationUsersInScope())
	assert.True(t, rs.SyncScope.OrganizationSystemAccountsInScope())
}

func TestShouldPlanOrganizationSystemAccountsRequiresScope(t *testing.T) {
	plan := NewPlan("1.0", "test", PlanModeSync)
	scope := resources.NewSyncScope()
	planner := &Planner{
		resources: &resources.ResourceSet{SyncScope: scope},
	}

	assert.False(t, planner.shouldPlanOrganizationSystemAccounts(plan))

	scope.MarkOrganizationSystemAccountsScoped()
	assert.True(t, planner.shouldPlanOrganizationSystemAccounts(plan))
}

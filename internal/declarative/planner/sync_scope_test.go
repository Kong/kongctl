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

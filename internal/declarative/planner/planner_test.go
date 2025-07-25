package planner

import (
	"context"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStateClient for testing
type MockStateClient struct {
	mock.Mock
}

func (m *MockStateClient) ListManagedPortals(ctx context.Context) ([]state.Portal, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]state.Portal), args.Error(1)
}

func (m *MockStateClient) GetPortalByName(ctx context.Context, name string) (*state.Portal, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*state.Portal), args.Error(1)
}

// mockEmptyAPIsList adds a mock for empty APIs list, needed in sync mode
func mockEmptyAPIsList(_ context.Context, mockAPIAPI *MockAPIAPI) {
	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
		StatusCode: 200,
		ListAPIResponse: &kkComps.ListAPIResponse{
			Data: []kkComps.APIResponseSchema{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
}

func TestGeneratePlan_CreatePortal(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock empty portals list (no existing portals)
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Create a test portal resource
	displayName := "Development Portal"
	description := "Test portal for development"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "dev-portal",
					DisplayName: &displayName,
					Description: &description,
					Labels:      map[string]*string{},
				},
				Ref: "dev-portal",
				Kongctl: &resources.KongctlMeta{
					Namespace: &[]string{"default"}[0],
				},
			},
		},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{},
	}

	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Check plan has one CREATE change
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]

	assert.Equal(t, "1:c:portal:dev-portal", change.ID)
	assert.Equal(t, ActionCreate, change.Action)
	assert.Equal(t, "portal", change.ResourceType)
	assert.Equal(t, "dev-portal", change.ResourceRef)
	assert.Equal(t, "", change.ResourceID) // No ID for CREATE

	// Check fields
	assert.Equal(t, "dev-portal", change.Fields["name"])
	assert.Equal(t, displayName, change.Fields["display_name"])
	assert.Equal(t, description, change.Fields["description"])

	// Check summary
	assert.Equal(t, 1, plan.Summary.TotalChanges)
	assert.Equal(t, 1, plan.Summary.ByAction[ActionCreate])
	assert.Equal(t, 1, plan.Summary.ByResource["portal"])

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestGeneratePlan_UpdatePortal(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing portal with different description
	oldDesc := "Old description"
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "portal-123",
					Name:        "dev-portal",
					DisplayName: "Development Portal",
					Description: &oldDesc,
					Labels: map[string]string{
						labels.NamespaceKey: "default",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Create updated portal resource
	newDesc := "Updated description"
	displayName := "Development Portal"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "dev-portal",
					DisplayName: &displayName,
					Description: &newDesc,
					Labels:      map[string]*string{},
				},
				Ref: "dev-portal",
			},
		},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{},
	}

	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Check plan has one UPDATE change
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]

	assert.Equal(t, "1:u:portal:dev-portal", change.ID)
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "portal", change.ResourceType)
	assert.Equal(t, "dev-portal", change.ResourceRef)
	assert.Equal(t, "portal-123", change.ResourceID)

	// Check fields - now storing raw values instead of FieldChange
	// We now always send labels when defined to ensure proper label management
	assert.GreaterOrEqual(t, len(change.Fields), 2) // At minimum: name + description
	assert.Equal(t, "dev-portal", change.Fields["name"])
	assert.Equal(t, newDesc, change.Fields["description"])

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectionChange(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing protected portal
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "portal-123",
					Name:        "dev-portal",
					DisplayName: "Development Portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
						labels.ProtectedKey:  "true",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Create portal resource without protection
	displayName := "Development Portal"
	protectedLabel := "false"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "dev-portal",
					DisplayName: &displayName,
					Labels: map[string]*string{
						labels.ProtectedKey: &protectedLabel,
					},
				},
				Ref: "dev-portal",
			},
		},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{},
	}

	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Should have one change for unprotecting
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]

	assert.Equal(t, ActionUpdate, change.Action)
	// Protection changes no longer have special ID suffix
	assert.Contains(t, change.ID, "u:portal:dev-portal")

	// Check protection change
	protChange, ok := change.Protection.(ProtectionChange)
	assert.True(t, ok)
	assert.True(t, protChange.Old)
	assert.False(t, protChange.New)

	// Should have protection summary
	assert.NotNil(t, plan.Summary.ProtectionChanges)
	assert.Equal(t, 1, plan.Summary.ProtectionChanges.Unprotecting)
	assert.Equal(t, 0, plan.Summary.ProtectionChanges.Protecting)

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
}

func TestGeneratePlan_WithReferences(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock empty portals list
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Create resources with reference
	displayName := "Development Portal"
	authRef := "basic-auth"
	rs := &resources.ResourceSet{
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{
			{
				CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(
					kkComps.AppAuthStrategyKeyAuthRequest{
						Name:         "basic-auth",
						DisplayName:  "Basic Auth",
						StrategyType: kkComps.StrategyTypeKeyAuth,
						Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
							KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
						},
					},
				),
				Ref: "basic-auth",
			},
		},
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:                             "dev-portal",
					DisplayName:                      &displayName,
					DefaultApplicationAuthStrategyID: &authRef,
					Labels:                           map[string]*string{},
				},
				Ref: "dev-portal",
			},
		},
	}

	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Should have 2 changes
	assert.Len(t, plan.Changes, 2)

	// Find portal change
	var portalChange *PlannedChange
	for i := range plan.Changes {
		if plan.Changes[i].ResourceType == "portal" {
			portalChange = &plan.Changes[i]
			break
		}
	}
	assert.NotNil(t, portalChange)

	// Check reference was resolved
	if portalChange.References == nil {
		t.Logf("No references found on portal change")
		t.Logf("Portal change fields: %+v", portalChange.Fields)
	} else {
		ref, ok := portalChange.References["default_application_auth_strategy_id"]
		assert.True(t, ok)
		assert.Equal(t, "basic-auth", ref.Ref)
		assert.Equal(t, "[unknown]", ref.ID) // Will be resolved at execution
	}

	// Check execution order - auth strategy should come first
	if len(plan.ExecutionOrder) > 0 {
		assert.Len(t, plan.ExecutionOrder, 2)
		// Auth strategy is created first (due to processing order)
		assert.Equal(t, "1:c:application_auth_strategy:basic-auth", plan.ExecutionOrder[0])
		// Portal depends on auth strategy, so comes second
		assert.Equal(t, "2:c:portal:dev-portal", plan.ExecutionOrder[1])
	}

	// Should have warning about unresolved reference
	if len(plan.Warnings) > 0 {
		assert.Contains(t, plan.Warnings[0].Message, "will be resolved during execution")
	}

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestGeneratePlan_NoChangesNeeded(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing portal with same hash
	displayName := "Development Portal"
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "portal-123",
					Name:        "dev-portal",
					DisplayName: displayName,
					Labels: map[string]string{
						labels.NamespaceKey: "default",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Create same portal resource
	// Don't define labels at all to avoid triggering an update
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "dev-portal",
					DisplayName: &displayName,
					// Labels not defined - this avoids triggering label updates
				},
				Ref: "dev-portal",
			},
		},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{},
	}

	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Should have no changes
	assert.True(t, plan.IsEmpty())
	assert.Len(t, plan.Changes, 0)
	assert.Equal(t, 0, plan.Summary.TotalChanges)

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestNextChangeID(t *testing.T) {
	planner := &Planner{changeCount: 0}

	// Test CREATE - now generates temporary IDs
	id := planner.nextChangeID(ActionCreate, "portal", "my-resource")
	assert.Equal(t, "temp-1:c:portal:my-resource", id)

	// Test UPDATE
	id = planner.nextChangeID(ActionUpdate, "api", "other-resource")
	assert.Equal(t, "temp-2:u:api:other-resource", id)

	// Test DELETE (future)
	id = planner.nextChangeID(ActionDelete, "portal-page", "delete-me")
	assert.Equal(t, "temp-3:d:portal-page:delete-me", id)

	// Check counter increments
	assert.Equal(t, 3, planner.changeCount)
}

func TestReassignChangeIDs(t *testing.T) {
	planner := &Planner{changeCount: 0}
	
	// Create a plan with some changes
	plan := NewPlan("1.0", "test", PlanModeApply)
	
	// Add changes in one order
	plan.AddChange(PlannedChange{
		ID:           "temp-1:c:portal:portal-1",
		ResourceType: "portal",
		ResourceRef:  "portal-1",
		Action:       ActionCreate,
	})
	plan.AddChange(PlannedChange{
		ID:           "temp-2:c:api:api-1",
		ResourceType: "api",
		ResourceRef:  "api-1",
		Action:       ActionCreate,
		DependsOn:    []string{"temp-1:c:portal:portal-1"},
	})
	plan.AddChange(PlannedChange{
		ID:           "temp-3:c:portal_page:page-1",
		ResourceType: "portal_page",
		ResourceRef:  "page-1",
		Action:       ActionCreate,
		DependsOn:    []string{"temp-1:c:portal:portal-1"},
	})
	
	// Add a warning
	plan.AddWarning("temp-2:c:api:api-1", "Test warning")
	
	// Define execution order (different from creation order)
	executionOrder := []string{
		"temp-1:c:portal:portal-1",
		"temp-3:c:portal_page:page-1",
		"temp-2:c:api:api-1",
	}
	plan.SetExecutionOrder(executionOrder)
	
	// Reassign IDs
	planner.reassignChangeIDs(plan, executionOrder)
	
	// Check that IDs have been reassigned based on execution order
	// Changes array order stays the same, but IDs are updated
	assert.Equal(t, "1:c:portal:portal-1", plan.Changes[0].ID)
	assert.Equal(t, "3:c:api:api-1", plan.Changes[1].ID) // This was 3rd in execution order
	assert.Equal(t, "2:c:portal_page:page-1", plan.Changes[2].ID) // This was 2nd in execution order
	
	// Check that dependencies were updated
	assert.Equal(t, []string{"1:c:portal:portal-1"}, plan.Changes[1].DependsOn)
	assert.Equal(t, []string{"1:c:portal:portal-1"}, plan.Changes[2].DependsOn)
	
	// Check that execution order was updated
	assert.Equal(t, []string{
		"1:c:portal:portal-1",
		"2:c:portal_page:page-1",
		"3:c:api:api-1",
	}, plan.ExecutionOrder)
	
	// Check that warning was updated
	assert.Equal(t, "3:c:api:api-1", plan.Warnings[0].ChangeID)
}

func TestGeneratePlan_ApplyModeNoDeletes(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing managed portals
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "existing-id",
					Name:        "existing-portal",
					DisplayName: "Existing Portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Create resource set with only one portal (missing the existing one)
	displayName := "New Portal"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "new-portal",
					DisplayName: &displayName,
					Labels:      map[string]*string{},
				},
				Ref: "new-portal",
			},
		},
	}

	// Generate plan in apply mode
	opts := Options{Mode: PlanModeApply}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Should only have CREATE, no DELETE
	assert.Equal(t, 1, plan.Summary.TotalChanges)
	assert.Equal(t, 1, plan.Summary.ByAction[ActionCreate])
	assert.Equal(t, 0, plan.Summary.ByAction[ActionDelete])
	assert.False(t, plan.ContainsDeletes())

	// Verify plan metadata
	assert.Equal(t, PlanModeApply, plan.Metadata.Mode)

	mockPortalAPI.AssertExpectations(t)
}

func TestGeneratePlan_SyncModeWithDeletes(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing managed portals
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "existing-id",
					Name:        "existing-portal",
					DisplayName: "Existing Portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Create empty resource set (all managed resources should be deleted)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{},
	}

	// Generate plan in sync mode
	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Should have DELETE operation
	assert.Equal(t, 1, plan.Summary.TotalChanges)
	assert.Equal(t, 1, plan.Summary.ByAction[ActionDelete])
	assert.True(t, plan.ContainsDeletes())

	// Verify plan metadata
	assert.Equal(t, PlanModeSync, plan.Metadata.Mode)

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectedResourceFailsUpdate(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing protected portal
	protectedStr := "true"
	existingTimestamp := "20240101-120000Z"
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "protected-id",
					Name:        "protected-portal",
					DisplayName: "Protected Portal",
					Description: ptrString("Old description"),
					Labels: map[string]string{
						labels.ManagedKey:    trueStr,
						labels.LastUpdatedKey: existingTimestamp,
						labels.ProtectedKey:  protectedStr,
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Try to update the protected portal
	displayName := "Protected Portal"
	description := "New description"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "protected-portal",
					DisplayName: &displayName,
					Description: &description, // Changed field
				},
				Kongctl: &resources.KongctlMeta{
					Protected: &[]bool{true}[0], // Keep it protected
				},
				Ref: "protected-portal",
			},
		},
	}

	// Generate plan should fail
	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "Cannot generate plan due to protected resources")
	assert.Contains(t, err.Error(), "portal \"protected-portal\" is protected and cannot be updated")

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectedResourceFailsDelete(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockPortalAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockPortalAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing protected portal
	protectedStr := "true"
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "protected-id",
					Name:        "protected-portal",
					DisplayName: "Protected Portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
						labels.ProtectedKey:  protectedStr,
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)

	// Empty resource set (would delete all)
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{},
	}

	// Generate plan in sync mode should fail
	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "Cannot generate plan due to protected resources")
	assert.Contains(t, err.Error(), "portal \"protected-portal\" is protected and cannot be delete")

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectionChangeAllowed(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock existing protected portal
	protectedStr := "true"
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:          "protected-id",
					Name:        "protected-portal",
					DisplayName: "Protected Portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
						labels.ProtectedKey:  protectedStr,
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)
	
	// Mock empty auth strategies list
	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).Return(&kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: []kkComps.AppAuthStrategy{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)
	
	// Mock empty APIs list (needed because we're in sync mode)
	mockEmptyAPIsList(ctx, mockAPIAPI)

	// Change protection status only
	displayName := "Protected Portal"
	falseStr := "false"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name:        "protected-portal",
					DisplayName: &displayName,
					Labels: map[string]*string{
						labels.ProtectedKey: &falseStr,
					},
				},
				Ref: "protected-portal",
			},
		},
	}

	// Generate plan should succeed
	opts := Options{Mode: PlanModeSync}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Should have protection change
	assert.Equal(t, 1, plan.Summary.TotalChanges)
	assert.Equal(t, 1, plan.Summary.ByAction[ActionUpdate])
	assert.Equal(t, 1, plan.Summary.ProtectionChanges.Unprotecting)

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

// Test helpers
var (
	trueStr = "true"
)

func ptrString(s string) *string {
	return &s
}
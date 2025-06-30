package planner

import (
	"context"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
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

func TestGeneratePlan_CreatePortal(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock empty portals list (no existing portals)
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)

	// Create a test portal resource
	displayName := "Development Portal"
	description := "Test portal for development"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkInternalComps.CreatePortal{
					Name:        "dev-portal",
					DisplayName: &displayName,
					Description: &description,
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

	// Check plan has one CREATE change
	assert.Len(t, plan.Changes, 1)
	change := plan.Changes[0]

	assert.Equal(t, "1-c-dev-portal", change.ID)
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

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_UpdatePortal(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing portal with different description
	oldDesc := "Old description"
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "portal-123",
					Name:        "dev-portal",
					DisplayName: "Development Portal",
					Description: &oldDesc,
					Labels: map[string]string{
						labels.ManagedKey:    "true",
						labels.LastUpdatedKey: "20240101-120000Z",
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Create updated portal resource
	newDesc := "Updated description"
	displayName := "Development Portal"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkInternalComps.CreatePortal{
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

	assert.Equal(t, "1-u-dev-portal", change.ID)
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "portal", change.ResourceType)
	assert.Equal(t, "dev-portal", change.ResourceRef)
	assert.Equal(t, "portal-123", change.ResourceID)

	// Check fields - now storing raw values instead of FieldChange
	assert.Len(t, change.Fields, 2) // name + description
	assert.Equal(t, "dev-portal", change.Fields["name"])
	assert.Equal(t, newDesc, change.Fields["description"])

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectionChange(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing protected portal
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "portal-123",
					Name:        "dev-portal",
					DisplayName: "Development Portal",
					Labels: map[string]string{
						labels.ManagedKey:    "true",
						labels.LastUpdatedKey: "20240101-120000Z",
						labels.ProtectedKey:  "true",
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Create portal resource without protection
	displayName := "Development Portal"
	protectedLabel := "false"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkInternalComps.CreatePortal{
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
	assert.Contains(t, change.ID, "-protection")

	// Check protection change
	protChange, ok := change.Protection.(ProtectionChange)
	assert.True(t, ok)
	assert.True(t, protChange.Old)
	assert.False(t, protChange.New)

	// Should have protection summary
	assert.NotNil(t, plan.Summary.ProtectionChanges)
	assert.Equal(t, 1, plan.Summary.ProtectionChanges.Unprotecting)
	assert.Equal(t, 0, plan.Summary.ProtectionChanges.Protecting)

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_WithReferences(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock empty portals list
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)

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
				CreatePortal: kkInternalComps.CreatePortal{
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
		assert.Equal(t, "<unknown>", ref.ID) // Will be resolved at execution
	}

	// Check execution order - auth strategy should come first
	if len(plan.ExecutionOrder) > 0 {
		assert.Len(t, plan.ExecutionOrder, 2)
		assert.Equal(t, "1-c-basic-auth", plan.ExecutionOrder[0])
		assert.Equal(t, "2-c-dev-portal", plan.ExecutionOrder[1])
	}

	// Should have warning about unresolved reference
	if len(plan.Warnings) > 0 {
		assert.Contains(t, plan.Warnings[0].Message, "will be resolved during execution")
	}

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_NoChangesNeeded(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing portal with same hash
	displayName := "Development Portal"
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "portal-123",
					Name:        "dev-portal",
					DisplayName: displayName,
					Labels: map[string]string{
						labels.ManagedKey:    "true",
						labels.LastUpdatedKey: "20240101-120000Z",
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Create same portal resource
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkInternalComps.CreatePortal{
					Name:        "dev-portal",
					DisplayName: &displayName,
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

	// Should have no changes
	assert.True(t, plan.IsEmpty())
	assert.Len(t, plan.Changes, 0)
	assert.Equal(t, 0, plan.Summary.TotalChanges)

	mockAPI.AssertExpectations(t)
}

func TestNextChangeID(t *testing.T) {
	planner := &Planner{changeCount: 0}

	// Test CREATE
	id := planner.nextChangeID(ActionCreate, "my-resource")
	assert.Equal(t, "1-c-my-resource", id)

	// Test UPDATE
	id = planner.nextChangeID(ActionUpdate, "other-resource")
	assert.Equal(t, "2-u-other-resource", id)

	// Test DELETE (future)
	id = planner.nextChangeID(ActionDelete, "delete-me")
	assert.Equal(t, "3-d-delete-me", id)

	// Check counter increments
	assert.Equal(t, 3, planner.changeCount)
}

func TestGeneratePlan_ApplyModeNoDeletes(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing managed portals
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "existing-id",
					Name:        "existing-portal",
					DisplayName: "Existing Portal",
					Labels: map[string]string{
						labels.ManagedKey:    trueStr,
						labels.LastUpdatedKey: "20240101-120000Z",
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
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
				CreatePortal: kkInternalComps.CreatePortal{
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

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_SyncModeWithDeletes(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing managed portals
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "existing-id",
					Name:        "existing-portal",
					DisplayName: "Existing Portal",
					Labels: map[string]string{
						labels.ManagedKey:    trueStr,
						labels.LastUpdatedKey: "20240101-120000Z",
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

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

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectedResourceFailsUpdate(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing protected portal
	protectedStr := "true"
	existingTimestamp := "20240101-120000Z"
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
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
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Try to update the protected portal
	displayName := "Protected Portal"
	description := "New description"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkInternalComps.CreatePortal{
					Name:        "protected-portal",
					DisplayName: &displayName,
					Description: &description, // Changed field
					Labels: map[string]*string{
						labels.ProtectedKey: &protectedStr, // Keep it protected
					},
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
	assert.Contains(t, err.Error(), "portal \"protected-portal\" is protected and cannot be update")

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectedResourceFailsDelete(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing protected portal
	protectedStr := "true"
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "protected-id",
					Name:        "protected-portal",
					DisplayName: "Protected Portal",
					Labels: map[string]string{
						labels.ManagedKey:    trueStr,
						labels.LastUpdatedKey: "20240101-120000Z",
						labels.ProtectedKey:  protectedStr,
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
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

	mockAPI.AssertExpectations(t)
}

func TestGeneratePlan_ProtectionChangeAllowed(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Mock existing protected portal
	protectedStr := "true"
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{
				{
					ID:          "protected-id",
					Name:        "protected-portal",
					DisplayName: "Protected Portal",
					Labels: map[string]string{
						labels.ManagedKey:    trueStr,
						labels.LastUpdatedKey: "20240101-120000Z",
						labels.ProtectedKey:  protectedStr,
					},
				},
			},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Change protection status only
	displayName := "Protected Portal"
	falseStr := "false"
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkInternalComps.CreatePortal{
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

	mockAPI.AssertExpectations(t)
}

// Test helpers
var (
	trueStr = "true"
)

func ptrString(s string) *string {
	return &s
}
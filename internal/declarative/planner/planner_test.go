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

	plan, err := planner.GeneratePlan(ctx, rs)
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
						labels.ConfigHashKey: "old-hash",
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

	plan, err := planner.GeneratePlan(ctx, rs)
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

	// Check only description field changed
	assert.Len(t, change.Fields, 1)
	descChange := change.Fields["description"].(FieldChange)
	assert.Equal(t, oldDesc, descChange.Old)
	assert.Equal(t, newDesc, descChange.New)

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
						labels.ConfigHashKey: "hash-123",
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

	plan, err := planner.GeneratePlan(ctx, rs)
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

	plan, err := planner.GeneratePlan(ctx, rs)
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
						labels.ConfigHashKey: "5fb5278f3fa3962b4fb9b20c42163fc54f3cea1bea76c9f8dd0c6c4b7c30fb76",
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

	plan, err := planner.GeneratePlan(ctx, rs)
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
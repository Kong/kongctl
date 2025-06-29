package planner

import (
	"context"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGeneratePlan_Idempotency(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(mockAPI)
	planner := NewPlanner(client)

	// Create existing portal with API defaults
	displayName := "Developer Portal"
	description := "Test portal"
	authEnabled := true
	rbacEnabled := false
	autoApproveDev := true
	autoApproveApp := false
	
	existingPortal := kkInternalComps.Portal{
		ID:                      "portal-123",
		Name:                    "test-portal",
		DisplayName:             displayName,
		Description:             &description,
		AuthenticationEnabled:   authEnabled,
		RbacEnabled:            rbacEnabled,
		AutoApproveDevelopers:   autoApproveDev,
		AutoApproveApplications: autoApproveApp,
		Labels: map[string]string{
			labels.ManagedKey:    "true",
			labels.ConfigHashKey: "somehash",
		},
	}

	// Mock list returns existing portal
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkInternalOps.ListPortalsResponse{
		ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
			Data: []kkInternalComps.Portal{existingPortal},
			Meta: kkInternalComps.PaginatedMeta{
				Page: kkInternalComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	// Test case 1: Minimal config - should not trigger updates for API defaults
	t.Run("minimal config no changes", func(t *testing.T) {
		rs := &resources.ResourceSet{
			Portals: []resources.PortalResource{
				{
					CreatePortal: kkInternalComps.CreatePortal{
						Name:        "test-portal",
						Description: &description, // Only configured field
					},
					Ref: "test-portal",
				},
			},
		}

		opts := Options{Mode: PlanModeApply}
		plan, err := planner.GeneratePlan(ctx, rs, opts)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		// Should have no changes - description matches
		assert.Len(t, plan.Changes, 0)
		assert.True(t, plan.IsEmpty())
	})

	// Test case 2: Config with different value - should trigger update
	t.Run("config with changes", func(t *testing.T) {
		newDesc := "Updated description"
		rs := &resources.ResourceSet{
			Portals: []resources.PortalResource{
				{
					CreatePortal: kkInternalComps.CreatePortal{
						Name:        "test-portal",
						Description: &newDesc, // Changed value
					},
					Ref: "test-portal",
				},
			},
		}

		opts := Options{Mode: PlanModeApply}
		plan, err := planner.GeneratePlan(ctx, rs, opts)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		// Should have one update change
		assert.Len(t, plan.Changes, 1)
		change := plan.Changes[0]
		assert.Equal(t, ActionUpdate, change.Action)
		// Should have name and description fields
		assert.Equal(t, "test-portal", change.Fields["name"])
		assert.Equal(t, newDesc, change.Fields["description"])
	})

	// Test case 3: Add new field to existing config
	t.Run("add new field", func(t *testing.T) {
		newDisplayName := "Custom Portal"
		rs := &resources.ResourceSet{
			Portals: []resources.PortalResource{
				{
					CreatePortal: kkInternalComps.CreatePortal{
						Name:        "test-portal",
						Description: &description,      // Existing field
						DisplayName: &newDisplayName,   // New field
					},
					Ref: "test-portal",
				},
			},
		}

		opts := Options{Mode: PlanModeApply}
		plan, err := planner.GeneratePlan(ctx, rs, opts)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		// Should have one update with only display_name
		assert.Len(t, plan.Changes, 1)
		change := plan.Changes[0]
		assert.Equal(t, ActionUpdate, change.Action)
		assert.Equal(t, newDisplayName, change.Fields["display_name"])
		// Should not include description since it didn't change
		_, hasDesc := change.Fields["description"]
		assert.False(t, hasDesc)
	})

	// Test case 4: API defaults should not trigger updates
	t.Run("api defaults ignored", func(t *testing.T) {
		// Config doesn't specify display_name, auth_enabled, etc.
		// These have API defaults but shouldn't trigger updates
		rs := &resources.ResourceSet{
			Portals: []resources.PortalResource{
				{
					CreatePortal: kkInternalComps.CreatePortal{
						Name:        "test-portal",
						Description: &description,
						// Not specifying: DisplayName, AuthenticationEnabled, RbacEnabled, etc.
					},
					Ref: "test-portal",
				},
			},
		}

		opts := Options{Mode: PlanModeApply}
		plan, err := planner.GeneratePlan(ctx, rs, opts)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		// Should have no changes despite API having defaults
		assert.Len(t, plan.Changes, 0)
		assert.True(t, plan.IsEmpty())
	})

	// Test case 5: Sparse updates - only changed fields
	t.Run("sparse updates", func(t *testing.T) {
		newDisplayName := "New Name"
		newAuthEnabled := false
		rs := &resources.ResourceSet{
			Portals: []resources.PortalResource{
				{
					CreatePortal: kkInternalComps.CreatePortal{
						Name:                  "test-portal",
						Description:           &description,       // Same
						DisplayName:           &newDisplayName,    // Changed
						AuthenticationEnabled: &newAuthEnabled,    // Changed
						RbacEnabled:          &rbacEnabled,       // Same
					},
					Ref: "test-portal",
				},
			},
		}

		opts := Options{Mode: PlanModeApply}
		plan, err := planner.GeneratePlan(ctx, rs, opts)
		assert.NoError(t, err)
		assert.NotNil(t, plan)

		// Should have one update with only changed fields
		assert.Len(t, plan.Changes, 1)
		change := plan.Changes[0]
		assert.Equal(t, ActionUpdate, change.Action)
		
		// Should only have the two changed fields
		assert.Len(t, change.Fields, 3) // name + 2 changes
		assert.Equal(t, "test-portal", change.Fields["name"])
		assert.Equal(t, newDisplayName, change.Fields["display_name"])
		assert.Equal(t, newAuthEnabled, change.Fields["authentication_enabled"])
		
		// Should not have unchanged fields
		_, hasDesc := change.Fields["description"]
		assert.False(t, hasDesc)
		_, hasRbac := change.Fields["rbac_enabled"]
		assert.False(t, hasRbac)
	})

	mockAPI.AssertExpectations(t)
}
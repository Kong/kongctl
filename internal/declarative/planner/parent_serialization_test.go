package planner

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlannedChange_ParentSerialization(t *testing.T) {
	// Test that Parent field is properly serialized and deserialized
	tests := []struct {
		name   string
		change PlannedChange
	}{
		{
			name: "portal_customization_with_parent",
			change: PlannedChange{
				ID:           "1:UPDATE:portal_customization:test-customization",
				ResourceType: ResourceTypePortalCustomization,
				ResourceRef:  "test-customization",
				Action:       ActionUpdate,
				Fields: map[string]interface{}{
					"theme": map[string]interface{}{
						"name": "mint_rocket",
					},
				},
				Parent: &ParentInfo{
					Ref: "test-portal",
					ID:  "portal-123",
				},
				References: map[string]ReferenceInfo{
					"portal_id": {
						Ref: "test-portal",
						LookupFields: map[string]string{
							"name": "Test Portal",
						},
					},
				},
			},
		},
		{
			name: "portal_page_with_parent_no_id",
			change: PlannedChange{
				ID:           "2:CREATE:portal_page:test-page",
				ResourceType: ResourceTypePortalPage,
				ResourceRef:  "test-page",
				Action:       ActionCreate,
				Fields: map[string]interface{}{
					"slug":    "test",
					"content": "Test content",
				},
				Parent: &ParentInfo{
					Ref: "test-portal",
					ID:  "", // Empty when portal doesn't exist yet
				},
			},
		},
		{
			name: "resource_without_parent",
			change: PlannedChange{
				ID:           "3:CREATE:portal:test-portal",
				ResourceType: ResourceTypePortal,
				ResourceRef:  "test-portal",
				Action:       ActionCreate,
				Fields: map[string]interface{}{
					"name": "Test Portal",
				},
				// No Parent field
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize to JSON
			data, err := json.Marshal(tt.change)
			require.NoError(t, err)

			// Deserialize from JSON
			var deserialized PlannedChange
			err = json.Unmarshal(data, &deserialized)
			require.NoError(t, err)

			// Compare original and deserialized
			assert.Equal(t, tt.change.ID, deserialized.ID)
			assert.Equal(t, tt.change.ResourceType, deserialized.ResourceType)
			assert.Equal(t, tt.change.ResourceRef, deserialized.ResourceRef)
			assert.Equal(t, tt.change.Action, deserialized.Action)
			assert.Equal(t, tt.change.Fields, deserialized.Fields)

			// Check Parent field specifically
			if tt.change.Parent == nil {
				assert.Nil(t, deserialized.Parent)
			} else {
				require.NotNil(t, deserialized.Parent)
				assert.Equal(t, tt.change.Parent.Ref, deserialized.Parent.Ref)
				assert.Equal(t, tt.change.Parent.ID, deserialized.Parent.ID)
			}

			// Check References if present
			if len(tt.change.References) > 0 {
				assert.Equal(t, tt.change.References, deserialized.References)
			}
		})
	}
}

func TestPlan_ParentPreservationInJSON(t *testing.T) {
	// Test that Parent fields are preserved when plan is serialized/deserialized
	plan := NewPlan("1.0.0", "test", PlanModeSync)

	// Add changes with parent references
	plan.AddChange(PlannedChange{
		ID:           "1:CREATE:portal:test-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "test-portal",
		Action:       ActionCreate,
		Fields: map[string]interface{}{
			"name": "Test Portal",
		},
	})

	plan.AddChange(PlannedChange{
		ID:           "2:UPDATE:portal_customization:test-customization",
		ResourceType: ResourceTypePortalCustomization,
		ResourceRef:  "test-customization",
		Action:       ActionUpdate,
		Fields: map[string]interface{}{
			"theme": map[string]interface{}{
				"name": "mint_rocket",
			},
		},
		Parent: &ParentInfo{
			Ref: "test-portal",
			ID:  "portal-123",
		},
		DependsOn: []string{"1:CREATE:portal:test-portal"},
	})

	// Serialize plan to JSON
	data, err := json.MarshalIndent(plan, "", "  ")
	require.NoError(t, err)

	// Deserialize plan from JSON
	var deserializedPlan Plan
	err = json.Unmarshal(data, &deserializedPlan)
	require.NoError(t, err)

	// Check that we have the same number of changes
	require.Len(t, deserializedPlan.Changes, 2)

	// Check first change (no parent)
	assert.Nil(t, deserializedPlan.Changes[0].Parent)

	// Check second change (has parent)
	require.NotNil(t, deserializedPlan.Changes[1].Parent)
	assert.Equal(t, "test-portal", deserializedPlan.Changes[1].Parent.Ref)
	assert.Equal(t, "portal-123", deserializedPlan.Changes[1].Parent.ID)
}
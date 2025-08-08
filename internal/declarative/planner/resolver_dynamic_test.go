package planner

import (
	"context"
	"testing"

	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamicFieldDetection(t *testing.T) {
	// Create resolver
	client := state.NewClient(state.ClientConfig{})
	resolver := NewReferenceResolver(client, nil)

	t.Run("portal resource mappings", func(t *testing.T) {
		// Test that portal resource correctly reports its reference fields
		mappings := resolver.getResourceMappings(ResourceTypePortal)
		
		// Portal should have default_application_auth_strategy_id mapping
		authStratType, exists := mappings["default_application_auth_strategy_id"]
		assert.True(t, exists, "Portal should have default_application_auth_strategy_id mapping")
		assert.Equal(t, "application_auth_strategy", authStratType)
		
		// Portal should not have other random fields
		_, exists = mappings["name"]
		assert.False(t, exists, "Portal should not have 'name' as a reference field")
	})

	t.Run("api publication resource mappings", func(t *testing.T) {
		// Test that API publication resource correctly reports its reference fields
		mappings := resolver.getResourceMappings(ResourceTypeAPIPublication)
		
		// API publication should have portal_id mapping
		portalType, exists := mappings["portal_id"]
		assert.True(t, exists, "API publication should have portal_id mapping")
		assert.Equal(t, "portal", portalType)
		
		// API publication should have auth_strategy_ids mapping
		authType, exists := mappings["auth_strategy_ids"]
		assert.True(t, exists, "API publication should have auth_strategy_ids mapping")
		assert.Equal(t, "application_auth_strategy", authType)
	})
	
	t.Run("dynamic field detection", func(t *testing.T) {
		// Test isReferenceFieldDynamic
		isRef := resolver.isReferenceFieldDynamic(ResourceTypePortal, "default_application_auth_strategy_id")
		assert.True(t, isRef, "default_application_auth_strategy_id should be detected as reference field")
		
		isRef = resolver.isReferenceFieldDynamic(ResourceTypePortal, "name")
		assert.False(t, isRef, "name should not be detected as reference field")
		
		isRef = resolver.isReferenceFieldDynamic(ResourceTypeAPIPublication, "portal_id")
		assert.True(t, isRef, "portal_id should be detected as reference field")
	})
	
	t.Run("dynamic resource type lookup", func(t *testing.T) {
		// Test getResourceTypeForFieldDynamic
		resourceType := resolver.getResourceTypeForFieldDynamic(ResourceTypePortal, "default_application_auth_strategy_id")
		assert.Equal(t, "application_auth_strategy", resourceType)
		
		resourceType = resolver.getResourceTypeForFieldDynamic(ResourceTypeAPIPublication, "portal_id")
		assert.Equal(t, "portal", resourceType)
		
		resourceType = resolver.getResourceTypeForFieldDynamic(ResourceTypePortal, "unknown_field")
		assert.Equal(t, "", resourceType, "Unknown field should return empty string")
	})
	
	t.Run("mapping cache", func(t *testing.T) {
		// Clear cache
		resolver.mappingCache = make(map[string]map[string]string)
		
		// First call should populate cache
		mappings1 := resolver.getResourceMappings(ResourceTypePortal)
		require.NotNil(t, mappings1)
		
		// Cache should now contain the mappings
		assert.Contains(t, resolver.mappingCache, ResourceTypePortal)
		
		// Second call should use cache (we can't easily test this without mocking,
		// but we can verify the result is the same)
		mappings2 := resolver.getResourceMappings(ResourceTypePortal)
		assert.Equal(t, mappings1, mappings2)
	})
	
	t.Run("resources without mappings", func(t *testing.T) {
		// API resource has no outbound references
		mappings := resolver.getResourceMappings(ResourceTypeAPI)
		assert.Empty(t, mappings, "API resource should have no reference mappings")
		
		// Control plane resource has no outbound references
		mappings = resolver.getResourceMappings("control_plane")
		assert.Empty(t, mappings, "Control plane resource should have no reference mappings")
	})
	
	t.Run("unknown resource type", func(t *testing.T) {
		// Unknown resource type should return empty mappings
		mappings := resolver.getResourceMappings("unknown_resource_type")
		assert.Empty(t, mappings, "Unknown resource type should return empty mappings")
		
		// Should not crash when checking fields for unknown resource
		isRef := resolver.isReferenceFieldDynamic("unknown_resource_type", "some_field")
		assert.False(t, isRef, "Unknown resource type should return false for any field")
	})
}

func TestExtractReferenceValue(t *testing.T) {
	resolver := NewReferenceResolver(nil, nil)
	
	t.Run("string reference", func(t *testing.T) {
		ref, isRef := resolver.extractReferenceValue("my-portal-ref")
		assert.True(t, isRef)
		assert.Equal(t, "my-portal-ref", ref)
	})
	
	t.Run("UUID should not be reference", func(t *testing.T) {
		ref, isRef := resolver.extractReferenceValue("123e4567-e89b-12d3-a456-426614174000")
		assert.False(t, isRef)
		assert.Equal(t, "", ref)
	})
	
	t.Run("empty string", func(t *testing.T) {
		ref, isRef := resolver.extractReferenceValue("")
		assert.False(t, isRef)
		assert.Equal(t, "", ref)
	})
	
	t.Run("field change with new value", func(t *testing.T) {
		fc := FieldChange{
			Old: "old-ref",
			New: "new-ref",
		}
		ref, isRef := resolver.extractReferenceValue(fc)
		assert.True(t, isRef)
		assert.Equal(t, "new-ref", ref)
	})
	
	t.Run("field change with UUID new value", func(t *testing.T) {
		fc := FieldChange{
			Old: "old-ref",
			New: "123e4567-e89b-12d3-a456-426614174000",
		}
		ref, isRef := resolver.extractReferenceValue(fc)
		assert.False(t, isRef)
		assert.Equal(t, "", ref)
	})
	
	t.Run("non-string value", func(t *testing.T) {
		ref, isRef := resolver.extractReferenceValue(123)
		assert.False(t, isRef)
		assert.Equal(t, "", ref)
	})
}

func TestDynamicResolutionIntegration(t *testing.T) {
	// This test verifies the full dynamic resolution flow
	client := state.NewClient(state.ClientConfig{})
	resolver := NewReferenceResolver(client, nil)
	
	changes := []PlannedChange{
		{
			ID:           "change-1",
			Action:       ActionCreate,
			ResourceType: "application_auth_strategy",
			ResourceRef:  "auth-strategy-ref",
			Fields: map[string]interface{}{
				"name": "Auth Strategy",
			},
		},
		{
			ID:           "change-2",
			Action:       ActionCreate,
			ResourceType: ResourceTypePortal,
			ResourceRef:  "my-portal",
			Fields: map[string]interface{}{
				"name": "New Portal",
				"default_application_auth_strategy_id": "auth-strategy-ref",
			},
		},
		{
			ID:           "change-3",
			Action:       ActionCreate,
			ResourceType: ResourceTypeAPIPublication,
			ResourceRef:  "my-publication",
			Fields: map[string]interface{}{
				"portal_id": "my-portal",
			},
		},
	}
	
	// Test that references are detected using dynamic approach
	result, err := resolver.ResolveReferences(context.Background(), changes)
	require.NoError(t, err)
	require.NotNil(t, result)
	
	// Check that portal's auth strategy reference was detected
	// Since auth-strategy-ref is being created in the same plan, it should be marked as "[unknown]"
	portalRefs, exists := result.ChangeReferences["change-2"]
	assert.True(t, exists, "Portal change should have references")
	
	authRef, exists := portalRefs["default_application_auth_strategy_id"]
	assert.True(t, exists, "Portal's auth strategy field should be detected")
	assert.Equal(t, "auth-strategy-ref", authRef.Ref)
	assert.Equal(t, "[unknown]", authRef.ID, "Should be marked as unknown since it's in the same plan")
	
	// Name field should not be detected as reference
	_, exists = portalRefs["name"]
	assert.False(t, exists, "Name field should not be detected as reference")
	
	// Check that API publication's portal reference was detected
	pubRefs, exists := result.ChangeReferences["change-3"]
	assert.True(t, exists, "API publication change should have references")
	
	portalRef, exists := pubRefs["portal_id"]
	assert.True(t, exists, "API publication's portal_id field should be detected")
	assert.Equal(t, "my-portal", portalRef.Ref)
	assert.Equal(t, "[unknown]", portalRef.ID, "Should be marked as unknown since it's in the same plan")
}

func TestMixedDynamicAndHardcodedApproach(t *testing.T) {
	// Test that both dynamic and hardcoded approaches work together
	// This ensures smooth transition and backward compatibility
	client := state.NewClient(state.ClientConfig{})
	resolver := NewReferenceResolver(client, nil)
	
	// Test that a known resource type uses dynamic mappings
	t.Run("known resource uses dynamic", func(t *testing.T) {
		// Portal resource should use dynamic mappings
		isRef := resolver.isReferenceFieldDynamic(ResourceTypePortal, "default_application_auth_strategy_id")
		assert.True(t, isRef, "Portal should detect auth strategy field dynamically")
		
		// Get the resource type for the field
		resourceType := resolver.getResourceTypeForFieldDynamic(ResourceTypePortal, "default_application_auth_strategy_id")
		assert.Equal(t, "application_auth_strategy", resourceType)
	})
	
	// Test that unknown resource types don't crash
	t.Run("unknown resource doesn't crash", func(t *testing.T) {
		// Unknown resource should return false/empty without crashing
		isRef := resolver.isReferenceFieldDynamic("unknown_resource", "some_field")
		assert.False(t, isRef, "Unknown resource should return false")
		
		resourceType := resolver.getResourceTypeForFieldDynamic("unknown_resource", "some_field")
		assert.Equal(t, "", resourceType, "Unknown resource should return empty string")
	})
	
	// Test that hardcoded methods still work independently
	t.Run("hardcoded methods still work", func(t *testing.T) {
		// The hardcoded isReferenceField should still work
		isRef := resolver.isReferenceField("portal_id")
		assert.True(t, isRef, "Hardcoded method should still detect portal_id")
		
		// The hardcoded getResourceTypeForField should still work
		resourceType := resolver.getResourceTypeForField("portal_id")
		assert.Equal(t, ResourceTypePortal, resourceType, "Hardcoded method should return portal type")
	})
}
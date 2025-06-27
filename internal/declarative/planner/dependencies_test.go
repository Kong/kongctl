package planner

import (
	"testing"
)

func TestResolveDependencies_SimpleDependencyChain(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-auth",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "basic-auth",
			Action:       ActionCreate,
		},
		{
			ID:           "2-c-portal",
			ResourceType: "portal",
			ResourceRef:  "dev-portal",
			Action:       ActionCreate,
			DependsOn:    []string{"1-c-auth"},
		},
		{
			ID:           "3-c-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionCreate,
			DependsOn:    []string{"2-c-portal"},
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Check order
	expectedOrder := []string{"1-c-auth", "2-c-portal", "3-c-api"}
	if !equalSlices(order, expectedOrder) {
		t.Errorf("Expected order %v, got %v", expectedOrder, order)
	}
}

func TestResolveDependencies_ImplicitReferenceDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-portal",
			ResourceType: "portal",
			ResourceRef:  "dev-portal",
			Action:       ActionCreate,
			References: map[string]ReferenceInfo{
				"default_application_auth_strategy_id": {
					Ref: "basic-auth",
					ID:  "<unknown>",
				},
			},
		},
		{
			ID:           "2-c-auth",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "basic-auth",
			Action:       ActionCreate,
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Auth strategy should come before portal
	authIndex := indexOf(order, "2-c-auth")
	portalIndex := indexOf(order, "1-c-portal")

	if authIndex >= portalIndex {
		t.Errorf("Auth strategy (index %d) should come before portal (index %d)", authIndex, portalIndex)
	}
}

func TestResolveDependencies_ParentChildRelationship(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-api-version",
			ResourceType: "api_version",
			ResourceRef:  "my-api-v1",
			Action:       ActionCreate,
			Parent: &ParentInfo{
				Ref: "my-api",
				ID:  "<unknown>",
			},
		},
		{
			ID:           "2-c-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionCreate,
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// API should come before API version
	apiIndex := indexOf(order, "2-c-api")
	versionIndex := indexOf(order, "1-c-api-version")

	if apiIndex >= versionIndex {
		t.Errorf("API (index %d) should come before API version (index %d)", apiIndex, versionIndex)
	}
}

func TestResolveDependencies_ComplexDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-auth",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "basic-auth",
			Action:       ActionCreate,
		},
		{
			ID:           "2-c-portal",
			ResourceType: "portal",
			ResourceRef:  "dev-portal",
			Action:       ActionCreate,
			References: map[string]ReferenceInfo{
				"default_application_auth_strategy_id": {
					Ref: "basic-auth",
					ID:  "<unknown>",
				},
			},
		},
		{
			ID:           "3-c-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionCreate,
			DependsOn:    []string{"2-c-portal"},
		},
		{
			ID:           "4-c-api-version",
			ResourceType: "api_version",
			ResourceRef:  "my-api-v1",
			Action:       ActionCreate,
			Parent: &ParentInfo{
				Ref: "my-api",
				ID:  "<unknown>",
			},
		},
		{
			ID:           "5-u-portal",
			ResourceType: "portal",
			ResourceRef:  "existing-portal",
			Action:       ActionUpdate,
			References: map[string]ReferenceInfo{
				"default_application_auth_strategy_id": {
					Ref: "basic-auth",
					ID:  "<unknown>",
				},
			},
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Verify constraints
	authIndex := indexOf(order, "1-c-auth")
	portalIndex := indexOf(order, "2-c-portal")
	apiIndex := indexOf(order, "3-c-api")
	versionIndex := indexOf(order, "4-c-api-version")
	updateIndex := indexOf(order, "5-u-portal")

	// Auth should come before both portals
	if authIndex >= portalIndex {
		t.Error("Auth should come before portal creation")
	}
	if authIndex >= updateIndex {
		t.Error("Auth should come before portal update")
	}

	// Portal should come before API
	if portalIndex >= apiIndex {
		t.Error("Portal should come before API")
	}

	// API should come before API version
	if apiIndex >= versionIndex {
		t.Error("API should come before API version")
	}
}

func TestResolveDependencies_CircularDependency(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-a",
			ResourceType: "resource_a",
			ResourceRef:  "a",
			Action:       ActionCreate,
			DependsOn:    []string{"3-c-c"},
		},
		{
			ID:           "2-c-b",
			ResourceType: "resource_b",
			ResourceRef:  "b",
			Action:       ActionCreate,
			DependsOn:    []string{"1-c-a"},
		},
		{
			ID:           "3-c-c",
			ResourceType: "resource_c",
			ResourceRef:  "c",
			Action:       ActionCreate,
			DependsOn:    []string{"2-c-b"},
		},
	}

	_, err := resolver.ResolveDependencies(changes)
	if err == nil {
		t.Fatal("Expected error for circular dependency, got nil")
	}

	expectedErr := "circular dependency detected in plan"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestResolveDependencies_NoDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-auth",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "basic-auth",
			Action:       ActionCreate,
		},
		{
			ID:           "2-c-portal",
			ResourceType: "portal",
			ResourceRef:  "dev-portal",
			Action:       ActionCreate,
		},
		{
			ID:           "3-u-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionUpdate,
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// All changes should be in the order
	if len(order) != 3 {
		t.Errorf("Expected 3 changes in order, got %d", len(order))
	}

	// Check all IDs are present
	for _, change := range changes {
		if !contains(order, change.ID) {
			t.Errorf("Change %s missing from execution order", change.ID)
		}
	}
}

func TestResolveDependencies_DuplicateDependencies(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-auth",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "basic-auth",
			Action:       ActionCreate,
		},
		{
			ID:           "2-c-portal",
			ResourceType: "portal",
			ResourceRef:  "dev-portal",
			Action:       ActionCreate,
			DependsOn:    []string{"1-c-auth"}, // Explicit dependency
			References: map[string]ReferenceInfo{
				"default_application_auth_strategy_id": {
					Ref: "basic-auth",
					ID:  "<unknown>", // Implicit dependency (same as explicit)
				},
			},
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	// Should handle duplicate gracefully
	expectedOrder := []string{"1-c-auth", "2-c-portal"}
	if !equalSlices(order, expectedOrder) {
		t.Errorf("Expected order %v, got %v", expectedOrder, order)
	}
}

func TestGetParentType(t *testing.T) {
	resolver := NewDependencyResolver()

	tests := []struct {
		childType  string
		parentType string
	}{
		{"api_version", "api"},
		{"api_publication", "api"},
		{"api_implementation", "api"},
		{"portal_page", "portal"},
		{"unknown_type", ""},
	}

	for _, tt := range tests {
		t.Run(tt.childType, func(t *testing.T) {
			result := resolver.getParentType(tt.childType)
			if result != tt.parentType {
				t.Errorf("getParentType(%q) = %q, want %q", tt.childType, result, tt.parentType)
			}
		})
	}
}

// Helper functions
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
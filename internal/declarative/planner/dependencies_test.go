package planner

import (
	"strings"
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
					ID:  "[unknown]",
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

func TestResolveDependencies_ImplicitReferenceDependencies_WithPlaceholder(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1:u:application_auth_strategy:oidc",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "oidc",
			Action:       ActionUpdate,
			References: map[string]ReferenceInfo{
				FieldDCRProviderID: {
					Ref: "__REF__:http-dcr#id",
					ID:  "[unknown]",
				},
			},
		},
		{
			ID:           "2:c:dcr_provider:http-dcr",
			ResourceType: "dcr_provider",
			ResourceRef:  "http-dcr",
			Action:       ActionCreate,
		},
	}

	order, err := resolver.ResolveDependencies(changes)
	if err != nil {
		t.Fatalf("ResolveDependencies failed: %v", err)
	}

	dcrIndex := indexOf(order, "2:c:dcr_provider:http-dcr")
	authIndex := indexOf(order, "1:u:application_auth_strategy:oidc")

	if dcrIndex >= authIndex {
		t.Errorf("DCR provider create (index %d) should come before auth strategy update (index %d)", dcrIndex, authIndex)
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
				ID:  "[unknown]",
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
					ID:  "[unknown]",
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
				ID:  "[unknown]",
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
					ID:  "[unknown]",
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
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error to contain %q, got %q", expectedErr, err.Error())
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
					ID:  "[unknown]", // Implicit dependency (same as explicit)
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

// ── findImplicitDependencies ──────────────────────────────────────────────────

func TestFindImplicitDependencies_NoReferences(t *testing.T) {
	d := NewDependencyResolver()
	change := PlannedChange{
		ID:           "1-c-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
	}
	deps := d.findImplicitDependencies(change, []PlannedChange{change})
	if len(deps) != 0 {
		t.Errorf("expected no implicit deps, got %v", deps)
	}
}

func TestFindImplicitDependencies_ResolvedReference_NoDep(t *testing.T) {
	// A reference whose ID is already a real UUID should NOT generate a dependency.
	d := NewDependencyResolver()
	authCreate := PlannedChange{
		ID:           "1-c-auth",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "basic-auth",
		Action:       ActionCreate,
	}
	portal := PlannedChange{
		ID:           "2-c-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldDefaultApplicationStrategyID: {
				Ref: "basic-auth",
				ID:  "already-resolved-uuid",
			},
		},
	}
	deps := d.findImplicitDependencies(portal, []PlannedChange{authCreate, portal})
	if len(deps) != 0 {
		t.Errorf("expected no implicit deps for resolved reference, got %v", deps)
	}
}

func TestFindImplicitDependencies_UnknownID_PlainRef(t *testing.T) {
	// Reference with ID="[unknown]" and a plain (non-placeholder) ref should find
	// the create change whose ResourceRef matches.
	d := NewDependencyResolver()
	authCreate := PlannedChange{
		ID:           "1-c-auth",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "basic-auth",
		Action:       ActionCreate,
	}
	portal := PlannedChange{
		ID:           "2-c-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldDefaultApplicationStrategyID: {
				Ref: "basic-auth",
				ID:  "[unknown]",
			},
		},
	}
	deps := d.findImplicitDependencies(portal, []PlannedChange{authCreate, portal})
	if len(deps) != 1 || deps[0] != "1-c-auth" {
		t.Errorf("expected [1-c-auth], got %v", deps)
	}
}

func TestFindImplicitDependencies_EmptyID_PlainRef(t *testing.T) {
	// Empty ID is also unresolved and should generate an implicit dependency.
	d := NewDependencyResolver()
	authCreate := PlannedChange{
		ID:           "1-c-auth",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "basic-auth",
		Action:       ActionCreate,
	}
	portal := PlannedChange{
		ID:           "2-c-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldDefaultApplicationStrategyID: {
				Ref: "basic-auth",
				ID:  "",
			},
		},
	}
	deps := d.findImplicitDependencies(portal, []PlannedChange{authCreate, portal})
	if len(deps) != 1 || deps[0] != "1-c-auth" {
		t.Errorf("expected [1-c-auth], got %v", deps)
	}
}

func TestFindImplicitDependencies_UnknownID_RefPlaceholder(t *testing.T) {
	// __REF__:some-resource#id placeholders must be stripped before matching.
	d := NewDependencyResolver()
	dcrCreate := PlannedChange{
		ID:           "1-c-dcr",
		ResourceType: ResourceTypeDCRProvider,
		ResourceRef:  "http-dcr",
		Action:       ActionCreate,
	}
	authUpdate := PlannedChange{
		ID:           "2-u-auth",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "oidc",
		Action:       ActionUpdate,
		References: map[string]ReferenceInfo{
			FieldDCRProviderID: {
				Ref: "__REF__:http-dcr#id",
				ID:  "[unknown]",
			},
		},
	}
	deps := d.findImplicitDependencies(authUpdate, []PlannedChange{dcrCreate, authUpdate})
	if len(deps) != 1 || deps[0] != "1-c-dcr" {
		t.Errorf("expected [1-c-dcr], got %v", deps)
	}
}

func TestFindImplicitDependencies_RefPlaceholder_ResolvedIDStillDepends(t *testing.T) {
	// Placeholders should still infer dependency edges even if ID is populated.
	d := NewDependencyResolver()
	dcrCreate := PlannedChange{
		ID:           "1-c-dcr",
		ResourceType: ResourceTypeDCRProvider,
		ResourceRef:  "http-dcr",
		Action:       ActionCreate,
	}
	authUpdate := PlannedChange{
		ID:           "2-u-auth",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "oidc",
		Action:       ActionUpdate,
		References: map[string]ReferenceInfo{
			FieldDCRProviderID: {
				Ref: "__REF__:http-dcr#id",
				ID:  "already-resolved-id",
			},
		},
	}
	deps := d.findImplicitDependencies(authUpdate, []PlannedChange{dcrCreate, authUpdate})
	if len(deps) != 1 || deps[0] != "1-c-dcr" {
		t.Errorf("expected [1-c-dcr], got %v", deps)
	}
}

func TestFindImplicitDependencies_UnknownID_NoMatchingChange(t *testing.T) {
	// Reference can't be resolved because no create change exists for it — no dep added.
	d := NewDependencyResolver()
	portal := PlannedChange{
		ID:           "1-c-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldDefaultApplicationStrategyID: {
				Ref: "nonexistent-auth",
				ID:  "[unknown]",
			},
		},
	}
	deps := d.findImplicitDependencies(portal, []PlannedChange{portal})
	if len(deps) != 0 {
		t.Errorf("expected no deps for unresolvable reference, got %v", deps)
	}
}

func TestFindImplicitDependencies_UnknownID_UpdateNotCreate_NoDep(t *testing.T) {
	// Only ActionCreate changes satisfy an implicit dependency lookup.
	// An ActionUpdate of the same ref should not be considered a satisfying change.
	d := NewDependencyResolver()
	authUpdate := PlannedChange{
		ID:           "1-u-auth",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "basic-auth",
		Action:       ActionUpdate,
	}
	portal := PlannedChange{
		ID:           "2-c-portal",
		ResourceType: ResourceTypePortal,
		ResourceRef:  "my-portal",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldDefaultApplicationStrategyID: {
				Ref: "basic-auth",
				ID:  "[unknown]",
			},
		},
	}
	deps := d.findImplicitDependencies(portal, []PlannedChange{authUpdate, portal})
	if len(deps) != 0 {
		t.Errorf("expected no dep on an update change, got %v", deps)
	}
}

func TestFindImplicitDependencies_MultipleReferences(t *testing.T) {
	// Multiple [unknown] references each generate their own dependency.
	d := NewDependencyResolver()
	authCreate := PlannedChange{
		ID: "1-c-auth", ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef: "basic-auth", Action: ActionCreate,
	}
	dcrCreate := PlannedChange{
		ID: "2-c-dcr", ResourceType: ResourceTypeDCRProvider,
		ResourceRef: "http-dcr", Action: ActionCreate,
	}
	authUpdate := PlannedChange{
		ID:           "3-u-auth-oidc",
		ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef:  "oidc",
		Action:       ActionUpdate,
		References: map[string]ReferenceInfo{
			FieldDCRProviderID:                {Ref: "__REF__:http-dcr#id", ID: "[unknown]"},
			FieldDefaultApplicationStrategyID: {Ref: "basic-auth", ID: "[unknown]"},
		},
	}
	all := []PlannedChange{authCreate, dcrCreate, authUpdate}
	deps := d.findImplicitDependencies(authUpdate, all)
	if len(deps) != 2 {
		t.Errorf("expected 2 implicit deps, got %d: %v", len(deps), deps)
	}
	if !contains(deps, "1-c-auth") {
		t.Errorf("expected dep on 1-c-auth, got %v", deps)
	}
	if !contains(deps, "2-c-dcr") {
		t.Errorf("expected dep on 2-c-dcr, got %v", deps)
	}
}

func TestFindImplicitDependencies_ArrayReference(t *testing.T) {
	// IsArray references iterate Refs; each placeholder generates a dep.
	d := NewDependencyResolver()
	auth1 := PlannedChange{
		ID: "1-c-auth1", ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef: "auth-key", Action: ActionCreate,
	}
	auth2 := PlannedChange{
		ID: "2-c-auth2", ResourceType: ResourceTypeApplicationAuthStrategy,
		ResourceRef: "auth-jwt", Action: ActionCreate,
	}
	pub := PlannedChange{
		ID:           "3-c-pub",
		ResourceType: ResourceTypeAPIPublication,
		ResourceRef:  "my-pub",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldAuthStrategyIDs: {
				IsArray: true,
				Refs:    []string{"__REF__:auth-key#id", "__REF__:auth-jwt#id"},
			},
		},
	}
	deps := d.findImplicitDependencies(pub, []PlannedChange{auth1, auth2, pub})
	if len(deps) != 2 {
		t.Errorf("expected 2 array-ref deps, got %d: %v", len(deps), deps)
	}
	if !contains(deps, "1-c-auth1") || !contains(deps, "2-c-auth2") {
		t.Errorf("expected deps on auth creates, got %v", deps)
	}
}

func TestFindImplicitDependencies_ArrayReference_NonPlaceholderIgnored(t *testing.T) {
	// IsArray entries that are not ref placeholders are skipped.
	d := NewDependencyResolver()
	pub := PlannedChange{
		ID:           "1-c-pub",
		ResourceType: ResourceTypeAPIPublication,
		ResourceRef:  "my-pub",
		Action:       ActionCreate,
		References: map[string]ReferenceInfo{
			FieldAuthStrategyIDs: {
				IsArray: true,
				Refs:    []string{"already-resolved-uuid"},
			},
		},
	}
	deps := d.findImplicitDependencies(pub, []PlannedChange{pub})
	if len(deps) != 0 {
		t.Errorf("expected no deps for already-resolved array ref, got %v", deps)
	}
}

// ── ResolveDependenciesWithGroups ─────────────────────────────────────────────

func TestResolveDependenciesWithGroups_Empty(t *testing.T) {
	d := NewDependencyResolver()
	res, err := d.ResolveDependenciesWithGroups(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionOrder) != 0 {
		t.Errorf("expected empty order, got %v", res.ExecutionOrder)
	}
	if len(res.ExecutionGroups) != 0 {
		t.Errorf("expected no groups, got %v", res.ExecutionGroups)
	}
	if len(res.FullDepsMap) != 0 {
		t.Errorf("expected empty FullDepsMap, got %v", res.FullDepsMap)
	}
}

func TestResolveDependenciesWithGroups_SingleChange(t *testing.T) {
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "my-api", Action: ActionCreate},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionOrder) != 1 || res.ExecutionOrder[0] != "1-c-api" {
		t.Errorf("expected [1-c-api], got %v", res.ExecutionOrder)
	}
	if len(res.ExecutionGroups) != 1 || len(res.ExecutionGroups[0]) != 1 {
		t.Errorf("expected one group with one element, got %v", res.ExecutionGroups)
	}
}

func TestResolveDependenciesWithGroups_NoDependencies_AllInOneGroup(t *testing.T) {
	// Unrelated changes should all land in group 0 (concurrent).
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-auth", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "auth", Action: ActionCreate},
		{ID: "2-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "portal", Action: ActionCreate},
		{ID: "3-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "api", Action: ActionCreate},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionGroups) != 1 {
		t.Errorf("expected 1 group, got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
	if len(res.ExecutionGroups[0]) != 3 {
		t.Errorf("expected all 3 changes in group 0, got %v", res.ExecutionGroups[0])
	}
}

func TestResolveDependenciesWithGroups_LinearChain_ThreeGroups(t *testing.T) {
	// A→B→C must produce three groups each containing one element.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-auth", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "auth", Action: ActionCreate},
		{
			ID: "2-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "portal", Action: ActionCreate,
			DependsOn: []string{"1-c-auth"},
		},
		{
			ID: "3-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "api", Action: ActionCreate,
			DependsOn: []string{"2-c-portal"},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionGroups) != 3 {
		t.Errorf("expected 3 groups, got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
	if res.ExecutionGroups[0][0] != "1-c-auth" {
		t.Errorf("group 0 should be auth, got %v", res.ExecutionGroups[0])
	}
	if res.ExecutionGroups[1][0] != "2-c-portal" {
		t.Errorf("group 1 should be portal, got %v", res.ExecutionGroups[1])
	}
	if res.ExecutionGroups[2][0] != "3-c-api" {
		t.Errorf("group 2 should be api, got %v", res.ExecutionGroups[2])
	}
}

func TestResolveDependencies_ParentChildRelationship_EmptyParentID(t *testing.T) {
	resolver := NewDependencyResolver()

	changes := []PlannedChange{
		{
			ID:           "1-c-api-version",
			ResourceType: "api_version",
			ResourceRef:  "my-api-v1",
			Action:       ActionCreate,
			Parent: &ParentInfo{
				Ref: "my-api",
				ID:  "",
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

	apiIndex := indexOf(order, "2-c-api")
	versionIndex := indexOf(order, "1-c-api-version")

	if apiIndex >= versionIndex {
		t.Errorf("API (index %d) should come before API version (index %d)", apiIndex, versionIndex)
	}
}

func TestResolveDependenciesWithGroups_Diamond(t *testing.T) {
	// A→B, A→C, B→D, C→D  =>  groups: [A], [B,C], [D]
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "A", ResourceType: ResourceTypeAPI, ResourceRef: "a", Action: ActionCreate},
		{
			ID: "B", ResourceType: ResourceTypeAPIVersion, ResourceRef: "b", Action: ActionCreate,
			DependsOn: []string{"A"},
		},
		{
			ID: "C", ResourceType: ResourceTypeAPIVersion, ResourceRef: "c", Action: ActionCreate,
			DependsOn: []string{"A"},
		},
		{
			ID: "D", ResourceType: ResourceTypeAPIPublication, ResourceRef: "d", Action: ActionCreate,
			DependsOn: []string{"B", "C"},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionGroups) != 3 {
		t.Fatalf("expected 3 groups (diamond shape), got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
	if res.ExecutionGroups[0][0] != "A" {
		t.Errorf("group 0 should be [A], got %v", res.ExecutionGroups[0])
	}
	if len(res.ExecutionGroups[1]) != 2 ||
		!contains(res.ExecutionGroups[1], "B") ||
		!contains(res.ExecutionGroups[1], "C") {
		t.Errorf("group 1 should be [B,C], got %v", res.ExecutionGroups[1])
	}
	if res.ExecutionGroups[2][0] != "D" {
		t.Errorf("group 2 should be [D], got %v", res.ExecutionGroups[2])
	}
}

func TestResolveDependenciesWithGroups_CircularDependency(t *testing.T) {
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "A", ResourceType: ResourceTypeAPI, ResourceRef: "a", Action: ActionCreate, DependsOn: []string{"C"}},
		{ID: "B", ResourceType: ResourceTypeAPI, ResourceRef: "b", Action: ActionCreate, DependsOn: []string{"A"}},
		{ID: "C", ResourceType: ResourceTypeAPI, ResourceRef: "c", Action: ActionCreate, DependsOn: []string{"B"}},
	}
	_, err := d.ResolveDependenciesWithGroups(changes)
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("expected 'circular dependency' in error, got: %v", err)
	}
}

func TestResolveDependenciesWithGroups_ImplicitRef_SameGroup(t *testing.T) {
	// API and portal have no shared dep: they land in the same group.
	// api_version implicitly depends on api via [unknown] parent; it goes to the next group.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "my-api", Action: ActionCreate},
		{ID: "2-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "my-portal", Action: ActionCreate},
		{
			ID: "3-c-version", ResourceType: ResourceTypeAPIVersion, ResourceRef: "v1", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-api", ID: "[unknown]"},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Groups: [1-c-api, 2-c-portal], [3-c-version]
	if len(res.ExecutionGroups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
	g0 := res.ExecutionGroups[0]
	if !contains(g0, "1-c-api") || !contains(g0, "2-c-portal") {
		t.Errorf("expected api and portal in group 0, got %v", g0)
	}
	if res.ExecutionGroups[1][0] != "3-c-version" {
		t.Errorf("expected version in group 1, got %v", res.ExecutionGroups[1])
	}
}

func TestResolveDependenciesWithGroups_ParentChild_AllResourceTypes(t *testing.T) {
	// api_document, api_publication, api_implementation all depend on api create.
	d := NewDependencyResolver()
	apiCreate := PlannedChange{
		ID: "1-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "my-api", Action: ActionCreate,
	}
	children := []PlannedChange{
		{
			ID: "2-c-version", ResourceType: ResourceTypeAPIVersion, ResourceRef: "v1", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-api", ID: "[unknown]"},
		},
		{
			ID: "3-c-pub", ResourceType: ResourceTypeAPIPublication, ResourceRef: "pub1", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-api", ID: "[unknown]"},
		},
		{
			ID: "4-c-impl", ResourceType: ResourceTypeAPIImplementation, ResourceRef: "impl1", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-api", ID: "[unknown]"},
		},
		{
			ID: "5-c-doc", ResourceType: ResourceTypeAPIDocument, ResourceRef: "doc1", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-api", ID: "[unknown]"},
		},
	}
	all := append([]PlannedChange{apiCreate}, children...)
	res, err := d.ResolveDependenciesWithGroups(all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apiIdx := indexOf(res.ExecutionOrder, "1-c-api")
	for _, ch := range children {
		chIdx := indexOf(res.ExecutionOrder, ch.ID)
		if apiIdx >= chIdx {
			t.Errorf("%s (index %d) must come after api create (index %d)", ch.ID, chIdx, apiIdx)
		}
	}
}

func TestResolveDependenciesWithGroups_PortalPage_DependsOnPortal(t *testing.T) {
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "my-portal", Action: ActionCreate},
		{
			ID: "2-c-page", ResourceType: ResourceTypePortalPage, ResourceRef: "home", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-portal", ID: "[unknown]"},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if indexOf(res.ExecutionOrder, "1-c-portal") >= indexOf(res.ExecutionOrder, "2-c-page") {
		t.Error("portal must execute before portal_page")
	}
}

func TestResolveDependenciesWithGroups_ParentAlreadyResolved_NoDep(t *testing.T) {
	// Parent with a real (non-[unknown]) ID should not create an implicit dependency.
	d := NewDependencyResolver()
	apiCreate := PlannedChange{
		ID: "1-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "my-api", Action: ActionCreate,
	}
	version := PlannedChange{
		ID: "2-c-version", ResourceType: ResourceTypeAPIVersion, ResourceRef: "v1", Action: ActionCreate,
		Parent: &ParentInfo{Ref: "my-api", ID: "already-known-uuid"},
	}
	res, err := d.ResolveDependenciesWithGroups([]PlannedChange{apiCreate, version})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both should be in the same group since no edge exists.
	if len(res.ExecutionGroups) != 1 {
		t.Errorf("expected 1 group (no implicit parent dep), got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
}

func TestResolveDependenciesWithGroups_ExplicitPlusImplicit_NoDuplicate(t *testing.T) {
	// When explicit DependsOn and implicit ref point to the same change, there
	// should be only one edge (no double-counting in in-degree).
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-auth", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "basic-auth", Action: ActionCreate},
		{
			ID: "2-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "my-portal", Action: ActionCreate,
			DependsOn: []string{"1-c-auth"},
			References: map[string]ReferenceInfo{
				FieldDefaultApplicationStrategyID: {Ref: "basic-auth", ID: "[unknown]"},
			},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionGroups) != 2 {
		t.Errorf("expected 2 groups, got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
	// FullDepsMap for the portal should list 1-c-auth exactly once.
	deps := res.FullDepsMap["2-c-portal"]
	if len(deps) != 1 || deps[0] != "1-c-auth" {
		t.Errorf("expected FullDepsMap[2-c-portal] = [1-c-auth], got %v", deps)
	}
}

func TestResolveDependenciesWithGroups_FullDepsMap_ImplicitEdges(t *testing.T) {
	// FullDepsMap must capture implicit (reference-derived) edges.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-auth", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "auth", Action: ActionCreate},
		{
			ID: "2-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "portal", Action: ActionCreate,
			References: map[string]ReferenceInfo{
				FieldDefaultApplicationStrategyID: {Ref: "auth", ID: "[unknown]"},
			},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deps, ok := res.FullDepsMap["2-c-portal"]
	if !ok || !contains(deps, "1-c-auth") {
		t.Errorf("expected FullDepsMap to contain implicit dep 1-c-auth for 2-c-portal, got %v", res.FullDepsMap)
	}
	// Auth has no deps; should not appear in FullDepsMap.
	if _, ok := res.FullDepsMap["1-c-auth"]; ok {
		t.Errorf("expected 1-c-auth absent from FullDepsMap, got %v", res.FullDepsMap["1-c-auth"])
	}
}

func TestResolveDependenciesWithGroups_FullDepsMap_ParentEdge(t *testing.T) {
	// FullDepsMap must capture parent-derived edges.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "my-api", Action: ActionCreate},
		{
			ID: "2-c-version", ResourceType: ResourceTypeAPIVersion, ResourceRef: "v1", Action: ActionCreate,
			Parent: &ParentInfo{Ref: "my-api", ID: "[unknown]"},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deps, ok := res.FullDepsMap["2-c-version"]
	if !ok || !contains(deps, "1-c-api") {
		t.Errorf("expected FullDepsMap to contain parent dep 1-c-api for 2-c-version, got %v", res.FullDepsMap)
	}
}

func TestResolveDependenciesWithGroups_ExecutionOrder_FlatGroupConcat(t *testing.T) {
	// ExecutionOrder must be the concatenation of ExecutionGroups in order.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-auth", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "auth", Action: ActionCreate},
		{
			ID: "2-c-portal", ResourceType: ResourceTypePortal, ResourceRef: "portal", Action: ActionCreate,
			DependsOn: []string{"1-c-auth"},
		},
		{
			ID: "3-c-api", ResourceType: ResourceTypeAPI, ResourceRef: "api", Action: ActionCreate,
			DependsOn: []string{"2-c-portal"},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	flat := []string{}
	for _, g := range res.ExecutionGroups {
		flat = append(flat, g...)
	}
	if !equalSlices(flat, res.ExecutionOrder) {
		t.Errorf("ExecutionOrder %v != flattened groups %v", res.ExecutionOrder, flat)
	}
}

func TestResolveDependenciesWithGroups_GroupsAreSorted(t *testing.T) {
	// Items within each group must be sorted for deterministic output.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "z-last", ResourceType: ResourceTypeAPI, ResourceRef: "z", Action: ActionCreate},
		{ID: "a-first", ResourceType: ResourceTypePortal, ResourceRef: "a", Action: ActionCreate},
		{ID: "m-middle", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "m", Action: ActionCreate},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.ExecutionGroups) != 1 {
		t.Fatalf("expected 1 group, got %v", res.ExecutionGroups)
	}
	g := res.ExecutionGroups[0]
	for i := 1; i < len(g); i++ {
		if g[i] < g[i-1] {
			t.Errorf("group not sorted: %v", g)
			break
		}
	}
}

func TestResolveDependenciesWithGroups_ArrayRef_CreatesMultipleDeps(t *testing.T) {
	// Array references generate deps on all matching create changes.
	d := NewDependencyResolver()
	auth1 := PlannedChange{
		ID: "1-c-auth1", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "auth-key", Action: ActionCreate,
	}
	auth2 := PlannedChange{
		ID: "2-c-auth2", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "auth-jwt", Action: ActionCreate,
	}
	pub := PlannedChange{
		ID: "3-c-pub", ResourceType: ResourceTypeAPIPublication, ResourceRef: "pub", Action: ActionCreate,
		References: map[string]ReferenceInfo{
			FieldAuthStrategyIDs: {
				IsArray: true,
				Refs:    []string{"__REF__:auth-key#id", "__REF__:auth-jwt#id"},
			},
		},
	}
	res, err := d.ResolveDependenciesWithGroups([]PlannedChange{auth1, auth2, pub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pubIdx := indexOf(res.ExecutionOrder, "3-c-pub")
	for _, dep := range []string{"1-c-auth1", "2-c-auth2"} {
		if indexOf(res.ExecutionOrder, dep) >= pubIdx {
			t.Errorf("%s should come before 3-c-pub", dep)
		}
	}
	// Auth creates share no deps → should be in the same earlier group.
	if len(res.ExecutionGroups) != 2 {
		t.Errorf("expected 2 groups ([auths], [pub]), got %d: %v", len(res.ExecutionGroups), res.ExecutionGroups)
	}
}

func TestResolveDependenciesWithGroups_RefPlaceholder_ImplicitDep(t *testing.T) {
	// __REF__ placeholder in a scalar reference (not array) generates correct implicit dep.
	d := NewDependencyResolver()
	changes := []PlannedChange{
		{ID: "1-c-dcr", ResourceType: ResourceTypeDCRProvider, ResourceRef: "http-dcr", Action: ActionCreate},
		{
			ID: "2-u-auth", ResourceType: ResourceTypeApplicationAuthStrategy, ResourceRef: "oidc", Action: ActionUpdate,
			References: map[string]ReferenceInfo{
				FieldDCRProviderID: {Ref: "__REF__:http-dcr#id", ID: "[unknown]"},
			},
		},
	}
	res, err := d.ResolveDependenciesWithGroups(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if indexOf(res.ExecutionOrder, "1-c-dcr") >= indexOf(res.ExecutionOrder, "2-u-auth") {
		t.Error("dcr create must precede auth update")
	}
}

// ── Helper functions ──────────────────────────────────────────────────────────

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

package resources

import (
	"testing"

	"github.com/kong/kongctl/internal/util"
)

// Compile-time interface compliance checks
var (
	_ Resource           = (*PortalResource)(nil)
	_ ResourceWithLabels = (*PortalResource)(nil)

	_ Resource           = (*ApplicationAuthStrategyResource)(nil)
	_ ResourceWithLabels = (*ApplicationAuthStrategyResource)(nil)

	_ Resource           = (*APIResource)(nil)
	_ ResourceWithLabels = (*APIResource)(nil)
)

func TestPortalResourceInterface(t *testing.T) {
	portal := &PortalResource{
		Ref: "test-portal",
	}
	portal.Name = ptr("Test Portal")

	// Test Resource interface methods
	if got := portal.GetType(); got != ResourceTypePortal {
		t.Errorf("GetType() = %v, want %v", got, ResourceTypePortal)
	}

	if got := portal.GetRef(); got != "test-portal" {
		t.Errorf("GetRef() = %v, want %v", got, "test-portal")
	}

	if got := util.StringValue(portal.Name); got != "Test Portal" {
		t.Errorf("GetName() = %v, want %v", got, "Test Portal")
	}

	// Test ResourceWithLabels interface
	labels := map[string]string{"env": "test"}
	portal.SetLabels(labels)

	if got := portal.GetLabels(); got["env"] != "test" {
		t.Errorf("GetLabels() = %v, want %v", got, labels)
	}
}

func TestApplicationAuthStrategyResourceInterface(t *testing.T) {
	strategy := &ApplicationAuthStrategyResource{
		Ref: "test-strategy",
	}

	// Test Resource interface methods
	if got := strategy.GetType(); got != ResourceTypeApplicationAuthStrategy {
		t.Errorf("GetType() = %v, want %v", got, ResourceTypeApplicationAuthStrategy)
	}

	if got := strategy.GetRef(); got != "test-strategy" {
		t.Errorf("GetRef() = %v, want %v", got, "test-strategy")
	}

	// GetDependencies should return empty for auth strategies
	if deps := strategy.GetDependencies(); len(deps) != 0 {
		t.Errorf("GetDependencies() = %v, want empty", deps)
	}
}

func TestPortalResourceDependencies(t *testing.T) {
	authStrategyID := "my-auth-strategy"
	portal := &PortalResource{
		Ref: "test-portal",
	}
	portal.DefaultApplicationAuthStrategyID = &authStrategyID

	deps := portal.GetDependencies()
	if len(deps) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(deps))
	}

	if deps[0].Kind != "application_auth_strategy" {
		t.Errorf("Dependency kind = %v, want %v", deps[0].Kind, "application_auth_strategy")
	}

	if deps[0].Ref != authStrategyID {
		t.Errorf("Dependency ref = %v, want %v", deps[0].Ref, authStrategyID)
	}
}

func TestPortalResourceSetDefaults(t *testing.T) {
	portal := &PortalResource{
		Ref: "test-portal",
		// Name is not set
	}

	portal.SetDefaults()

	// Name should default to ref
	if util.StringValue(portal.Name) != "test-portal" {
		t.Errorf("SetDefaults() Name = %v, want %v", util.StringValue(portal.Name), "test-portal")
	}
}

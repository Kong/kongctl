package mesh

import (
	"strings"
	"testing"
)

func testDescriptors() []ResourceDescriptor {
	return []ResourceDescriptor{
		{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh, ShortName: "dp"},
		{Name: "Mesh", Path: "meshes", Scope: ScopeGlobal, ShortName: "m"},
		{Name: "MeshTrafficPermission", Path: "meshtrafficpermissions", Scope: ScopeMesh, ShortName: "mtp"},
		// An insight type carries no short name, so it is not KRI addressable
		// and offers no alias.
		{Name: "DataplaneInsight", Path: "dataplane-insights", Scope: ScopeMesh},
	}
}

func TestResolveType(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantType string
	}{
		{"by path", "dataplanes", "Dataplane"},
		{"by type name", "Dataplane", "Dataplane"},
		{"by short name", "dp", "Dataplane"},
		{"path is case insensitive", "DATAPLANES", "Dataplane"},
		{"type name is case insensitive", "dataplane", "Dataplane"},
		{"short name is case insensitive", "DP", "Dataplane"},
		{"global type by path", "meshes", "Mesh"},
		{"type with no short name", "dataplane-insights", "DataplaneInsight"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveType(testDescriptors(), tc.arg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tc.wantType {
				t.Errorf("resolved %q to %s, want %s", tc.arg, got.Name, tc.wantType)
			}
		})
	}
}

// A path match must win over a type name match, since the path is the form
// displayed in tables and therefore the one an operator is likeliest to copy.
func TestResolveTypePrefersPathOverName(t *testing.T) {
	descriptors := []ResourceDescriptor{
		{Name: "collision", Path: "other-path", Scope: ScopeGlobal},
		{Name: "Other", Path: "collision", Scope: ScopeGlobal},
	}

	got, err := ResolveType(descriptors, "collision")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Other" {
		t.Errorf("resolved to %s, want the descriptor whose path matched", got.Name)
	}
}

func TestResolveTypeUnknown(t *testing.T) {
	if _, err := ResolveType(testDescriptors(), ""); err == nil {
		t.Error("expected an error for an empty type")
	}

	// A type removed in Kong Mesh 3 is simply absent from discovery, so it
	// must report as unknown rather than produce a request that 404s.
	err := mustFailResolve(t, "zoneingresses")
	if !strings.Contains(err.Error(), "resource-types") {
		t.Errorf("error should point at the discovery command, got %q", err)
	}

	// A near miss should be offered rather than the whole type list.
	err = mustFailResolve(t, "dataplane-typo")
	if !strings.Contains(err.Error(), "did you mean") || !strings.Contains(err.Error(), "dataplanes") {
		t.Errorf("expected a near miss naming dataplanes, got %q", err)
	}
}

func mustFailResolve(t *testing.T, arg string) error {
	t.Helper()
	_, err := ResolveType(testDescriptors(), arg)
	if err == nil {
		t.Fatalf("expected %q to be unresolvable", arg)
	}
	return err
}

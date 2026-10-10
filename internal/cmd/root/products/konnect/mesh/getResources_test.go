package mesh

import (
	"strings"
	"testing"

	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	configtest "github.com/kong/kongctl/test/config"
)

// stubConfig returns a config hook answering only the given paths, so a test
// states exactly the configuration it depends on.
func stubConfig(values map[string]string, flags map[string]bool) *configtest.MockConfigHook {
	return &configtest.MockConfigHook{
		GetStringMock: func(key string) string { return values[key] },
		GetBoolMock:   func(key string) bool { return flags[key] },
	}
}

func TestRequestPathScoping(t *testing.T) {
	dataplanes := ResourceDescriptor{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh}
	zones := ResourceDescriptor{Name: "Zone", Path: "zones", Scope: ScopeGlobal}

	tests := []struct {
		name       string
		descriptor ResourceDescriptor
		mesh       string
		allMeshes  bool
		resource   string
		want       string
	}{
		{"mesh scoped list defaults to the default mesh", dataplanes, "", false, "", "/meshes/default/dataplanes"},
		{"mesh scoped list honours --mesh", dataplanes, "prod", false, "", "/meshes/prod/dataplanes"},
		{"mesh scoped item", dataplanes, "prod", false, "dp-1", "/meshes/prod/dataplanes/dp-1"},
		// Kuma registers mesh scoped lists at /{path} as well, which lists
		// across every mesh.
		{"--all-meshes drops the mesh segment", dataplanes, "prod", true, "", "/dataplanes"},
		// A global type takes no mesh, so --mesh must not appear in its path.
		{"global list ignores --mesh", zones, "prod", false, "", "/zones"},
		{"global item ignores --mesh", zones, "prod", false, "zone-1", "/zones/zone-1"},
		{"global type ignores --all-meshes", zones, "", true, "", "/zones"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := stubConfig(
				map[string]string{meshcommon.MeshConfigPath: tc.mesh},
				map[string]bool{meshcommon.AllMeshesConfigPath: tc.allMeshes},
			)

			got, err := requestPath(cfg, tc.descriptor, tc.resource)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("path = %q, want %q", got, tc.want)
			}
		})
	}
}

// --all-meshes lists across meshes, so it cannot also address one resource:
// the request would be ambiguous about which mesh's resource was meant.
func TestRequestPathRejectsAllMeshesWithAName(t *testing.T) {
	cfg := stubConfig(nil, map[string]bool{meshcommon.AllMeshesConfigPath: true})
	descriptor := ResourceDescriptor{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh}

	_, err := requestPath(cfg, descriptor, "dp-1")
	if err == nil {
		t.Fatal("expected --all-meshes with a resource name to be rejected")
	}
	if !strings.Contains(err.Error(), meshcommon.AllMeshesFlagName) {
		t.Errorf("error should name the offending flag, got %q", err)
	}
}

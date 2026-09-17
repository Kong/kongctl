package root

import (
	"strings"
	"testing"

	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
)

// Mesh is reachable both directly and under the explicit product path. The
// constructors promise both, but only the direct form was registered, so
// `kongctl get konnect mesh` failed with "unknown command". These assert the
// real command paths rather than the constructors.
func TestMeshCommandPathsResolve(t *testing.T) {
	// Every verb Kong Mesh serves here. For Plan, Sync, Diff, Export, Apply
	// and Delete, konnect.NewKonnectCmd replaces the konnect command with the
	// declarative command and returns before any product is added, so
	// `apply konnect mesh` and `delete konnect mesh` cannot be registered
	// without restructuring that subtree. Those two are served by the direct
	// form only, and fall through to the declarative command rather than
	// erroring -- asserted below so the fall-through stays deliberate.
	paths := [][]string{
		{"apply", "mesh", "--help"},
		{"get", "mesh", "--help"},
		{"get", "konnect", "mesh", "--help"},
		{"create", "mesh", "--help"},
		{"create", "konnect", "mesh", "--help"},
		{"delete", "mesh", "--help"},
	}

	for _, args := range paths {
		path := strings.Join(args[:len(args)-1], " ")

		t.Run(path, func(t *testing.T) {
			result := executeRootForTest(t, args...)

			if result.exitCode != 0 {
				t.Fatalf("expected %q to succeed\nstdout:\n%s\nstderr:\n%s",
					path, result.stdout, result.stderr)
			}
			if strings.Contains(result.stderr, "unknown command") {
				t.Fatalf("expected %q to be registered\nstderr:\n%s", path, result.stderr)
			}
			// The mesh command's own help, not a parent's help that merely
			// lists it: a swallowed argument prints the parent's help and
			// still exits zero.
			if !strings.Contains(result.stdout, "Kong Mesh control plane") {
				t.Fatalf("expected %q help to describe the mesh command\nstdout:\n%s",
					path, result.stdout)
			}
		})
	}
}

// The declarative verbs claim `<verb> konnect`, so mesh is not reachable
// there. That is a property of the konnect subtree rather than a mesh bug, but
// it is silent -- the declarative command accepts "mesh" as a positional
// argument and prints its own help -- so it is pinned here. If either of these
// ever resolves to the mesh command, meshVerbs and this test disagree.
func TestMeshIsNotReachableUnderTheDeclarativeVerbs(t *testing.T) {
	for _, verb := range []string{"apply", "delete"} {
		t.Run(verb, func(t *testing.T) {
			result := executeRootForTest(t, verb, "konnect", "mesh", "--help")

			if strings.Contains(result.stdout, "Kong Mesh control plane") {
				t.Fatalf("%s konnect mesh now resolves to the mesh command; "+
					"add %q to meshVerbs and move it into TestMeshCommandPathsResolve\nstdout:\n%s",
					verb, verb, result.stdout)
			}
		})
	}
}

// A selector given on the command line must reach configuration on both
// command trees.
//
// The explicit path passed the general konnect pre-run, which binds only the
// konnect-common flags, so every mesh flag there was registered but unbound.
// The failure was silent in the worst way: with a control plane already in
// configuration, `get konnect mesh meshes --control-plane-url <other>` ignored
// the flag and listed the configured control plane instead.
func TestMeshFlagsBindOnBothCommandTrees(t *testing.T) {
	// A port nothing listens on. The request must be attempted and fail to
	// connect, which proves the flag reached configuration. --help would not:
	// it returns before the pre-run binds anything, which is why a help-only
	// test cannot catch this.
	const wantURL = "http://127.0.0.1:1"

	paths := [][]string{
		{"get", "mesh", "meshes"},
		{"get", "konnect", "mesh", "meshes"},
	}

	for _, path := range paths {
		name := strings.Join(path, " ")

		t.Run(name, func(t *testing.T) {
			args := append(append([]string{}, path...), "--control-plane-url", wantURL)
			result := executeRootForTest(t, args...)

			if currConfig == nil {
				t.Fatal("expected config to be initialized")
			}
			if got := currConfig.GetString(meshcommon.ControlPlaneURLConfigPath); got != wantURL {
				t.Fatalf("%s: control plane URL = %q, want %q (flag registered but not bound)",
					name, got, wantURL)
			}
			// The selector must also decide the request. Binding it while
			// sending the request elsewhere is the failure this guards.
			if !strings.Contains(result.stderr, "127.0.0.1:1") {
				t.Fatalf("%s: expected the request to address %s\nstderr:\n%s",
					name, wantURL, result.stderr)
			}
		})
	}
}

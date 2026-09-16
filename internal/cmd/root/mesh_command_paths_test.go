package root

import (
	"strings"
	"testing"
)

// Mesh is reachable both directly and under the explicit product path. The
// constructors promise both, but only the direct form was registered, so
// `kongctl get konnect mesh` failed with "unknown command". These assert the
// real command paths rather than the constructors.
func TestMeshCommandPathsResolve(t *testing.T) {
	// Every verb Kong Mesh serves here. `apply konnect` and `delete konnect`
	// are replaced by the declarative commands and take their own arguments,
	// so those two are served by the direct form only.
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

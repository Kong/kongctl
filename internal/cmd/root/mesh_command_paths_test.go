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
	// Every verb Kong Mesh serves. `delete konnect` is replaced by the
	// declarative delete command and takes its own arguments, so mesh
	// deletion is served by the direct form only.
	paths := [][]string{
		{"get", "mesh", "--help"},
		{"get", "konnect", "mesh", "--help"},
		{"create", "mesh", "--help"},
		{"create", "konnect", "mesh", "--help"},
		{"dump", "mesh", "--help"},
		{"dump", "konnect", "mesh", "--help"},
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

// The export selection must not be called --profile: that name belongs to the
// global configuration profile, and a local flag of the same name shadowed it.
func TestMeshDumpDoesNotShadowProfileFlag(t *testing.T) {
	result := executeRootForTest(t, "dump", "mesh", "--help")
	if result.exitCode != 0 {
		t.Fatalf("expected dump mesh help to succeed\nstderr:\n%s", result.stderr)
	}

	if !strings.Contains(result.stdout, "--export-profile") {
		t.Fatalf("expected the export selection to be --export-profile\nstdout:\n%s", result.stdout)
	}
	// The global -p/--profile is still present and must stay: what must not
	// appear is an example telling operators to pass --profile for an export
	// selection, which is what the rename was for.
	for line := range strings.SplitSeq(result.stdout, "\n") {
		if strings.Contains(line, "dump mesh --profile") {
			t.Fatalf("example still uses --profile for the export selection: %q", line)
		}
	}
}

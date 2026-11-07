//go:build e2e

package scenario

import (
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestIdentityScenariosPreserveDefaultDirectory(t *testing.T) {
	paths, err := filepath.Glob("../../scenarios/identity/*/scenario.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	shared := 0
	for _, path := range paths {
		s, err := Load(path)
		require.NoError(t, err, path)
		if filepath.Base(filepath.Dir(path)) == "directories" {
			require.Equal(t, "KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY", s.Test.EnabledByEnvVar)
			continue
		}
		for _, resource := range s.Test.CleanupResources {
			if resource.Resource == string(resources.ResourceTypeIdentityDirectory) {
				require.True(t, resource.UseDefaultDirectory, path)
				shared++
			}
		}
		for _, step := range s.Steps {
			for _, command := range step.Commands {
				if command.Create != nil && command.Create.Resource == string(resources.ResourceTypeIdentityDirectory) {
					require.True(t, command.Create.UseDefaultDirectory, "%s: %s", path, command.Name)
				}
			}
		}
	}
	t.Logf("validated %d Identity scenarios; %d use the shared default directory", len(paths), shared)
}

func TestExtraDirectoryCapacityIsExplicit(t *testing.T) {
	t.Setenv("KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY", "0")
	s, err := Load("../../scenarios/identity/directories/scenario.yaml")
	require.NoError(t, err)
	require.Contains(t, skipScenarioReason(s), "KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY")
	t.Setenv("KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY", "1")
	require.Empty(t, skipScenarioReason(s))
}

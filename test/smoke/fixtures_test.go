package smoke_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/stretchr/testify/require"
)

func TestGeneratedAIGatewayFixturesEnforceNamespace(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "kongctl")
	script, err := os.ReadFile("fake_kongctl.py")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fake, script, 0o600))
	require.NoError(t, os.Chmod(fake, 0o700))
	t.Setenv("FAKE_KONGCTL_LOG", filepath.Join(dir, "commands.log"))
	t.Setenv("FAKE_KONGCTL_STATE", filepath.Join(dir, "state.json"))
	for _, key := range []string{
		"FAKE_FAIL_ON", "FAKE_FAIL_DELETE", "FAKE_BAD_SCAFFOLD", "FAKE_FAULT", "KONGCTL_SMOKE_YES",
	} {
		t.Setenv(key, "")
	}
	cmd := exec.CommandContext(t.Context(), "bash", "../../scripts/smoke-test.sh",
		"--quick", "--resources", "ai_gateway", "--binary", fake, "--artifacts-dir", dir)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	paths, err := filepath.Glob(filepath.Join(dir, "kongctl-smoke.*", "fixtures", "ai_gateway", "*.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, 4)
	t.Setenv("KONGCTL_SMOKE_NAMESPACE_AI_GATEWAY", "smoke-test")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			if filepath.Base(path) != "pruned.yaml" {
				require.Len(t, rs.AIGatewayPolicies, 1)
				require.Len(t, rs.AIGatewayProviders, 1)
				require.Len(t, rs.AIGatewayModels, 1)
				require.Len(t, rs.AIGatewayMCPServers, 1)
			}
			v := validator.NewNamespaceValidator()
			require.NoError(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
				Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"smoke-test"},
			}))
			require.Error(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
				Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"another-team"},
			}))
		})
	}
}

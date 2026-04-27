package extensions

import (
	"testing"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/stretchr/testify/require"
)

func TestHostEnvironmentPropagatesReentrantContext(t *testing.T) {
	cfg := newTestHook()

	env := hostEnvironment(cfg, "/tmp/context.json")

	require.Contains(t, env, "KONGCTL_EXTENSION_CONTEXT=/tmp/context.json")
	require.Contains(t, env, "KONGCTL_PROFILE=default")
}

func TestHostEnvironmentPropagatesPATForProfile(t *testing.T) {
	cfg := newTestHook()
	cfg.SetString(konnectcommon.PATConfigPath, "test-pat")

	env := hostEnvironment(cfg, "/tmp/context.json")

	require.Contains(t, env, "KONGCTL_EXTENSION_KONNECT_PAT=test-pat")
	require.Contains(t, env, "KONGCTL_DEFAULT_KONNECT_PAT=test-pat")
}

func TestProfileEnvNameNormalizesProfileAndConfigPath(t *testing.T) {
	require.Equal(t,
		"KONGCTL_TEAM_A_KONNECT_PAT",
		profileEnvName("team-a", konnectcommon.PATConfigPath),
	)
}

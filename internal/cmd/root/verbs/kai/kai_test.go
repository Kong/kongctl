package kai

import (
	"context"
	"testing"

	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	utilviper "github.com/kong/kongctl/internal/util/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKaiCmd_UsesPATFlag(t *testing.T) {
	cmd, err := NewKaiCmd()
	require.NoError(t, err)

	cfg := buildKaiConfig(t, "default")
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, cfg))

	require.NoError(t, cmd.Flags().Set(konnectcommon.PATFlagName, "pat-from-flag"))
	require.NoError(t, bindFlags(cmd, []string{}))

	token, err := konnectcommon.GetAccessToken(cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, "pat-from-flag", token)
}

func TestKaiCmd_UsesPATEnvVar(t *testing.T) {
	t.Setenv("KONGCTL_DEFAULT_KONNECT_PAT", "pat-from-env")

	cmd, err := NewKaiCmd()
	require.NoError(t, err)

	cfg := buildKaiConfig(t, "default")
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, cfg))

	require.NoError(t, bindFlags(cmd, []string{}))

	token, err := konnectcommon.GetAccessToken(cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, "pat-from-env", token)
}

func buildKaiConfig(t *testing.T, profile string) *config.ProfiledConfig {
	t.Helper()

	mainv := utilviper.NewViper("nonexistent.yaml")
	mainv.Set(profile, map[string]any{})

	return config.BuildProfiledConfig(profile, "nonexistent.yaml", mainv)
}

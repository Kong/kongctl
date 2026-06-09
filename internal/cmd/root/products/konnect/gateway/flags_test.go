package gateway

import (
	"context"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCmd() *cobra.Command {
	return &cobra.Command{Use: "test"}
}

func TestAddGatewayFlags_CommonFlagsAlwaysRegistered(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List, verbs.Create, verbs.Delete} {
		t.Run(verb.String(), func(t *testing.T) {
			c := newTestCmd()
			AddGatewayFlags(verb, c)

			assert.NotNil(t, c.Flags().Lookup(common.BaseURLFlagName),
				"expected --%s flag to be registered", common.BaseURLFlagName)
			assert.NotNil(t, c.Flags().Lookup(common.RegionFlagName),
				"expected --%s flag to be registered", common.RegionFlagName)
			assert.NotNil(t, c.Flags().Lookup(common.PATFlagName),
				"expected --%s flag to be registered", common.PATFlagName)
		})
	}
}

func TestAddGatewayFlags_PageSizeOnlyForGetAndList(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		t.Run(verb.String(), func(t *testing.T) {
			c := newTestCmd()
			AddGatewayFlags(verb, c)

			f := c.Flags().Lookup(common.RequestPageSizeFlagName)
			require.NotNil(t, f, "expected --%s flag for verb %s", common.RequestPageSizeFlagName, verb)
			assert.Equal(t, "int", f.Value.Type())
		})
	}

	for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Delete} {
		t.Run(verb.String(), func(t *testing.T) {
			c := newTestCmd()
			AddGatewayFlags(verb, c)

			assert.Nil(t, c.Flags().Lookup(common.RequestPageSizeFlagName),
				"expected no --%s flag for verb %s", common.RequestPageSizeFlagName, verb)
		})
	}
}

func TestBindGatewayFlags_BindsRegisteredFlags(t *testing.T) {
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	c := newTestCmd()
	c.SetContext(ctx)
	AddGatewayFlags(verbs.Get, c)

	err := BindGatewayFlags(c, nil)
	require.NoError(t, err)
}

func TestBindGatewayFlags_SkipsMissingPageSizeFlag(t *testing.T) {
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Create verb does not register --page-size; BindGatewayFlags must not fail
	c := newTestCmd()
	c.SetContext(ctx)
	AddGatewayFlags(verbs.Create, c)

	err := BindGatewayFlags(c, nil)
	require.NoError(t, err)
}

func TestBindGatewayFlags_ErrorWhenNoConfig(t *testing.T) {
	c := newTestCmd()
	// Context has no config.ConfigKey value
	c.SetContext(context.Background())
	AddGatewayFlags(verbs.Get, c)

	err := BindGatewayFlags(c, nil)
	require.Error(t, err)
}

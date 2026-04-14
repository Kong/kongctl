package eventgateway

import (
	"context"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSchemaRegistryFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "both gateway flags mutually exclusive",
			args:    []string{"--gateway-id", "gw-1", "--gateway-name", "gw"},
			wantErr: "if any flags in the group [gateway-id gateway-name] are set none of the others can be",
		},
		{
			name: "both schema registry flags mutually exclusive",
			args: []string{
				"--gateway-id", "gw-1",
				"--schema-registry-id", "sr-1",
				"--schema-registry-name", "sr",
			},
			wantErr: "if any flags in the group [schema-registry-id schema-registry-name] are set none of the others can be",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(common.OutputConfigPath, "text")

			ctx := context.Background()
			ctx = context.WithValue(ctx, config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.Default())
			ctx = context.WithValue(ctx, iostreams.StreamsKey, iostreams.NewTestIOStreamsOnly())

			cmd := newGetEventGatewaySchemaRegistriesCmd(verbs.Get, nil, nil)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestSchemaRegistryPositionalArgConflictWithFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "positional arg conflicts with schema-registry-id flag",
			args: []string{
				"--gateway-id", "gw-1",
				"--schema-registry-id", "sr-1",
				"my-registry",
			},
			wantErr: "cannot specify both positional argument",
		},
		{
			name: "positional arg conflicts with schema-registry-name flag",
			args: []string{
				"--gateway-id", "gw-1",
				"--schema-registry-name", "sr",
				"my-registry",
			},
			wantErr: "cannot specify both positional argument",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(common.OutputConfigPath, "text")

			ctx := context.Background()
			ctx = context.WithValue(ctx, config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.Default())
			ctx = context.WithValue(ctx, iostreams.StreamsKey, iostreams.NewTestIOStreamsOnly())

			cmd := newGetEventGatewaySchemaRegistriesCmd(verbs.Get, nil, nil)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

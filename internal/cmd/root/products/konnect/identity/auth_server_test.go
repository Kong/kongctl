package identity

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type authServerOutputSDK struct {
	helpers.SDKAPI
	api helpers.IdentityAuthServersAPI
}

func (s authServerOutputSDK) GetIdentityAuthServersAPI() helpers.IdentityAuthServersAPI { return s.api }

func TestAuthServerCommandAppliesJQToAPIOutput(t *testing.T) {
	server := fixture.AuthServer("demo-issuer", "test", false)
	server.Issuer = "https://issuer.example.test/auth"
	server.JwksURI = new(server.Issuer + "/.well-known/jwks")
	for _, tc := range []struct {
		name, format, want, wantError string
		verb                          verbs.VerbValue
		args                          []string
	}{
		{
			name: "issuer scalar by name", format: "json", verb: verbs.Get,
			args: []string{server.Name, "--jq", ".issuer", "-r"}, want: server.Issuer + "\n",
		},
		{
			name: "JWKS scalar by UUID", format: "json", verb: verbs.Get,
			args: []string{server.ID, "--jq", ".jwks_uri", "-r"}, want: *server.JwksURI + "\n",
		},
		{
			name: "quoted JSON scalar", format: "json", verb: verbs.Get,
			args: []string{server.Name, "--jq", ".issuer"}, want: `"` + server.Issuer + "\"\n",
		},
		{
			name: "get collection projection", format: "json", verb: verbs.Get,
			args: []string{"--jq", ".[].issuer", "-r"}, want: server.Issuer + "\n",
		},
		{
			name: "list collection projection", format: "json", verb: verbs.List,
			args: []string{"--jq", ".[].jwks_uri", "-r"}, want: *server.JwksURI + "\n",
		},
		{
			name: "invalid filter fails", format: "json", verb: verbs.Get,
			args: []string{server.Name, "--jq", ".["}, wantError: "invalid jq expression",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newAuthServerCommand(tc.verb, nil, nil)
			jqoutput.AddFlags(command.Flags())
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(cmdCommon.OutputConfigPath, tc.format)
			require.NoError(t, jqoutput.BindFlags(cfg, command.Flags()))
			streams, _, output, _ := iostreams.NewTestIOStreams()
			ctx := context.WithValue(t.Context(), config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
			ctx = context.WithValue(ctx, logctx.LoggerKey, slog.New(slog.DiscardHandler))
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey,
				helpers.SDKAPIFactory(func(config.Hook, *slog.Logger) (helpers.SDKAPI, error) {
					return authServerOutputSDK{api: &fixture.AuthServerAPI{Servers: []kkComps.AuthServer{server}}}, nil
				}))
			command.SetArgs(tc.args)
			err := command.ExecuteContext(ctx)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, output.String())
		})
	}
}

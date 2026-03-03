package eventgateway

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/log"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDataPlaneCertFlagValidation(t *testing.T) {
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
			name:    "both data plane certificate flags mutually exclusive",
			args:    []string{"--gateway-id", "gw-1", "--data-plane-certificate-id", "cert-1", "--data-plane-certificate-name", "cert"},                                                                          //nolint:lll
			wantErr: "if any flags in the group [data-plane-certificate-id data-plane-certificate-name] are set none of the others can be; [data-plane-certificate-id data-plane-certificate-name] were all set", //nolint:lll
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Minimal context - Cobra validates before SDK is needed
			cfg := config.BuildProfiledConfig("default", "", viper.New())
			cfg.Set(common.OutputConfigPath, "text")

			ctx := context.Background()
			ctx = context.WithValue(ctx, config.ConfigKey, cfg)
			ctx = context.WithValue(ctx, log.LoggerKey, slog.Default())
			ctx = context.WithValue(ctx, iostreams.StreamsKey, iostreams.NewTestIOStreamsOnly())

			cmd := newGetEventGatewayDataPlaneCertificatesCmd(verbs.Get, nil, nil)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestFindDataPlaneCertByName(t *testing.T) {
	certs := []kkComps.EventGatewayDataPlaneCertificate{
		{ID: "cert-1", Name: new("Alpha-Cert")},
		{ID: "cert-2", Name: nil},
	}

	// Case-insensitive match
	assert.Equal(t, "cert-1", findDataPlaneCertByName(certs, "alpha-cert").ID)
	// Not found
	assert.Nil(t, findDataPlaneCertByName(certs, "missing"))
}

func TestFormatCertificateMetadata(t *testing.T) {
	// Nil returns n/a
	assert.Contains(t, formatCertificateMetadata(nil), valueNA)
	// Populated field appears
	meta := &kkComps.CertificateMetadata{Subject: new("CN=test")}
	assert.Contains(t, formatCertificateMetadata(meta), "subject: CN=test")
}

//go:fix inline
func ptr(s string) *string { return new(s) }

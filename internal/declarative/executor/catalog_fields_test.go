package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestPortalDomainMapsCertificateUpdate(t *testing.T) {
	var request kkComps.UpdatePortalCustomDomainRequest
	err := (&PortalDomainAdapter{}).MapUpdateFields(t.Context(), nil, map[string]any{
		planner.FieldSSL: map[string]any{
			"domain_verification_method": "custom_certificate",
			"custom_certificate":         "public-certificate", "custom_private_key": "resolved-key",
			"skip_ca_check": true,
		},
	}, &request, nil)
	require.NoError(t, err)
	require.NotNil(t, request.Ssl)
	require.Equal(t, "resolved-key", *request.Ssl.CustomPrivateKey)
	require.Equal(t, "public-certificate", *request.Ssl.CustomCertificate)
	require.True(t, *request.Ssl.SkipCaCheck)
	data, err := json.Marshal(request)
	require.NoError(t, err)
	require.NotContains(t, string(data), "domain_verification_method")
}

func TestBackendUpdatePreservesOmittedPassword(t *testing.T) {
	for _, kind := range []string{"sasl_plain", "sasl_scram"} {
		for _, write := range []bool{false, true} {
			auth := map[string]any{"type": kind, "username": "user"}
			if kind == "sasl_scram" {
				auth["algorithm"] = "sha256"
			}
			if write {
				auth["password"] = "resolved-password"
			}
			var request kkComps.UpdateBackendClusterRequest
			err := (&EventGatewayBackendClusterAdapter{}).MapUpdateFields(t.Context(), nil,
				map[string]any{planner.FieldAuthentication: auth}, &request, nil)
			require.NoError(t, err)
			data, err := json.Marshal(request.Authentication)
			require.NoError(t, err)
			if write {
				require.Contains(t, string(data), `"password":"resolved-password"`)
			} else {
				require.NotContains(t, string(data), "password")
			}
		}
	}
}

func TestBackendUpdateRejectsMalformedAuthentication(t *testing.T) {
	for name, auth := range map[string]any{
		"unsupported type":        map[string]any{"type": "unsupported"},
		"missing type":            map[string]any{"username": "user"},
		"missing plain username":  map[string]any{"type": "sasl_plain"},
		"missing scram username":  map[string]any{"type": "sasl_scram", "algorithm": "sha256"},
		"missing scram algorithm": map[string]any{"type": "sasl_scram", "username": "user"},
		"missing typed member": kkComps.BackendClusterAuthenticationScheme{
			Type: kkComps.BackendClusterAuthenticationSchemeTypeSaslPlain,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var request kkComps.UpdateBackendClusterRequest
			err := (&EventGatewayBackendClusterAdapter{}).MapUpdateFields(t.Context(), nil,
				map[string]any{planner.FieldAuthentication: auth}, &request, nil)
			require.Error(t, err)
		})
	}
}

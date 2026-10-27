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

func TestBackendAWSIAMSavedPlanFields(t *testing.T) {
	for _, config := range []string{
		`{"type":"default_provider_chain"}`,
		`{"type":"assume_role","arn":"arn:aws:iam::123456789012:role/backend","session_name":"kong"}`,
	} {
		t.Run(config, func(t *testing.T) {
			body := `{"type":"sasl_aws_iam","sasl_aws_iam":` + config + `}`
			var auth map[string]any
			require.NoError(t, json.Unmarshal([]byte(body), &auth))
			var typedAuth kkComps.BackendClusterAuthenticationScheme
			require.NoError(t, json.Unmarshal([]byte(body), &typedAuth))
			for name, authField := range map[string]any{
				"saved plan":    auth,
				"typed request": typedAuth,
				"planner fields": map[string]any{
					planner.FieldType: "sasl_aws_iam",
					"sasl_aws_iam":    typedAuth.BackendClusterAuthenticationSaslAwsIam.SaslAwsIam,
				},
			} {
				t.Run(name, func(t *testing.T) {
					fields := map[string]any{
						planner.FieldName:             "backend",
						planner.FieldAuthentication:   authField,
						planner.FieldBootstrapServers: []any{"broker.example.test:9098"},
						planner.FieldTLS:              map[string]any{"enabled": true},
					}
					adapter := &EventGatewayBackendClusterAdapter{}
					var create kkComps.CreateBackendClusterRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
					encoded, err := json.Marshal(create.Authentication)
					require.NoError(t, err)
					require.JSONEq(t, body, string(encoded))

					var update kkComps.UpdateBackendClusterRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
					encoded, err = json.Marshal(update.Authentication)
					require.NoError(t, err)
					require.JSONEq(t, body, string(encoded))
				})
			}
		})
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

func TestBackendAWSIAMRejectsMalformedAuthentication(t *testing.T) {
	for name, auth := range map[string]any{
		"missing inner object": map[string]any{"type": "sasl_aws_iam"},
		"missing arn": map[string]any{
			"type": "sasl_aws_iam", "sasl_aws_iam": map[string]any{"type": "assume_role"},
		},
		"empty arn": map[string]any{
			"type": "sasl_aws_iam", "sasl_aws_iam": map[string]any{"type": "assume_role", "arn": ""},
		},
		"typed empty arn": kkComps.CreateBackendClusterAuthenticationSchemeSaslAwsIam(
			kkComps.BackendClusterAuthenticationSaslAwsIam{
				SaslAwsIam: kkComps.CreateSaslAwsIamAssumeRole(kkComps.BackendClusterAuthenticationSaslAwsIamAssumeRole{}),
			},
		),
	} {
		t.Run(name, func(t *testing.T) {
			fields := map[string]any{
				planner.FieldName:             "backend",
				planner.FieldAuthentication:   auth,
				planner.FieldBootstrapServers: []any{"broker.example.test:9098"},
				planner.FieldTLS:              map[string]any{"enabled": true},
			}
			adapter := &EventGatewayBackendClusterAdapter{}
			var create kkComps.CreateBackendClusterRequest
			require.Error(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
			var update kkComps.UpdateBackendClusterRequest
			require.Error(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
		})
	}
}

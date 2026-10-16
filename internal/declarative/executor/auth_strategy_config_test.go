package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthStrategyConfigSavedPlanLists(t *testing.T) {
	cases := []struct {
		name                   string
		raw                    any
		ordinary, createClaims []string
	}{
		{"absent or null", nil, nil, []string{"sub"}},
		{"typed nil", []string(nil), nil, nil},
		{"typed empty", []string{}, []string{}, []string{}},
		{"decoded empty", []any{}, nil, []string{"sub"}},
		{"decoded nonstrings", []any{1, false}, nil, []string{"sub"}},
		{"decoded mixed", []any{"b", false, "a"}, []string{"b", "a"}, []string{"b", "a"}},
		{"typed populated", []string{"b", "a"}, []string{"b", "a"}, []string{"b", "a"}},
		{"wrong type", "claim", nil, []string{"sub"}},
	}
	adapter := NewAuthStrategyAdapter(nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, kind := range []string{"key_auth", "openid_connect"} {
				t.Run(kind, func(t *testing.T) {
					configs := map[string]any{"key-auth": map[string]any{"key_names": tc.raw}}
					if kind == "openid_connect" {
						configs = map[string]any{"openid-connect": map[string]any{
							planner.FieldDCRProviderIssuer: "", "credential_claim": tc.raw, "scopes": tc.raw, "auth_methods": tc.raw,
						}}
					}
					fields := map[string]any{
						planner.FieldName: "strategy", planner.FieldAuthStrategyType: kind,
						planner.FieldStrategyType: kind, planner.FieldConfigs: configs,
					}
					var create kkComps.CreateAppAuthStrategyRequest
					require.NoError(t, adapter.MapCreateFields(t.Context(), &ExecutionContext{}, fields, &create))
					var update kkComps.UpdateAppAuthStrategyRequest
					require.NoError(t, adapter.MapUpdateFields(t.Context(), &ExecutionContext{}, fields, &update, nil))
					require.NotNil(t, update.Configs)
					if kind == "key_auth" {
						require.NotNil(t, create.AppAuthStrategyKeyAuthRequest)
						require.NotNil(t, update.Configs.UpdateAppAuthStrategyRequestKeyAuth)
						assert.Equal(t, kkComps.ConfigsTypeUpdateAppAuthStrategyRequestKeyAuth, update.Configs.Type)
						assert.Equal(t, tc.ordinary, create.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames)
						assert.Equal(
							t,
							tc.ordinary,
							update.Configs.UpdateAppAuthStrategyRequestKeyAuth.KeyAuth.KeyNames,
						)
					} else {
						require.NotNil(t, create.AppAuthStrategyOpenIDConnectRequest)
						require.NotNil(t, update.Configs.UpdateAppAuthStrategyRequestOpenIDConnect)
						assert.Equal(t, kkComps.ConfigsTypeUpdateAppAuthStrategyRequestOpenIDConnect, update.Configs.Type)
						c := create.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect
						u := update.Configs.UpdateAppAuthStrategyRequestOpenIDConnect.OpenidConnect
						assert.Equal(t, tc.createClaims, c.CredentialClaim)
						assert.Equal(t, tc.ordinary, u.CredentialClaim)
						assert.Equal(t, tc.ordinary, c.Scopes)
						assert.Equal(t, tc.ordinary, u.Scopes)
						assert.Equal(t, tc.ordinary, c.AuthMethods)
						assert.Equal(t, tc.ordinary, u.AuthMethods)
						require.NotNil(t, u.Issuer)
						assert.Empty(t, *u.Issuer)
					}
				})
			}
		})
	}
}

func TestAuthStrategyConfigMappingErrors(t *testing.T) {
	cases := []struct {
		kind                     string
		configs                  any
		createError, updateError string
	}{
		{"key_auth", nil, "configs is required for key_auth strategy", ""},
		{"openid_connect", nil, "configs is required for openid_connect strategy", ""},
		{
			"key_auth",
			map[string]any{},
			"configs.key-auth is required for key_auth strategy",
			"failed to build configs: key-auth config missing for key_auth strategy",
		},
		{
			"openid_connect",
			map[string]any{},
			"configs.openid-connect is required for openid_connect strategy",
			"failed to build configs: openid-connect config missing for openid_connect strategy",
		},
		{
			"openid_connect",
			map[string]any{"openid-connect": map[string]any{}},
			"issuer is required for openid_connect strategy", "",
		},
		{
			"unknown",
			map[string]any{},
			"unsupported strategy_type: unknown",
			"failed to build configs: unsupported strategy type: unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.createError, func(t *testing.T) {
			adapter := NewAuthStrategyAdapter(nil)
			fields := map[string]any{
				planner.FieldAuthStrategyType: tc.kind,
				planner.FieldStrategyType:     tc.kind, planner.FieldConfigs: tc.configs,
			}
			var create kkComps.CreateAppAuthStrategyRequest
			require.EqualError(
				t,
				adapter.MapCreateFields(t.Context(), &ExecutionContext{}, fields, &create),
				tc.createError,
			)
			assert.Nil(t, create.AppAuthStrategyKeyAuthRequest)
			assert.Nil(t, create.AppAuthStrategyOpenIDConnectRequest)
			var update kkComps.UpdateAppAuthStrategyRequest
			err := adapter.MapUpdateFields(t.Context(), &ExecutionContext{}, fields, &update, nil)
			if tc.updateError != "" {
				require.EqualError(t, err, tc.updateError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

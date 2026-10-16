package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthStrategyConfigListPlanning(t *testing.T) {
	fields := []struct{ kind, config, field string }{
		{"key_auth", "key-auth", "key_names"},
		{"openid_connect", "openid-connect", "credential_claim"},
		{"openid_connect", "openid-connect", "scopes"},
		{"openid_connect", "openid-connect", "auth_methods"},
	}
	cases := []struct {
		name     string
		desired  []string
		observed any
		old      []string
		changed  bool
	}{
		{"omitted", nil, []string{"old"}, nil, false},
		{"clear", []string{}, []string{"old"}, []string{"old"}, true},
		{"empty matches missing", []string{}, nil, nil, false},
		{"ordered", []string{"a", "b"}, []string{"b", "a"}, []string{"b", "a"}, true},
		{"decoded filtering", []string{"a", "b"}, []any{"a", 42, "b"}, nil, false},
		{"decoded diagnostic", []string{"new"}, []any{false, "old"}, []string{"old"}, true},
		{"wrong type", []string{"new"}, true, []string{}, true},
		{"typed nil diagnostic", []string{"new"}, []string(nil), nil, true},
	}
	for _, field := range fields {
		t.Run(field.field, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					desired := authConfigTestResource(field.field, tc.desired)
					p := &authStrategyPlannerImpl{BasePlanner: NewBasePlanner(&Planner{})}
					plan := &Plan{}
					p.planAuthStrategyCreate(desired, plan)
					require.Len(t, plan.Changes, 1)
					config := map[string]any{}
					if tc.desired != nil {
						config[field.field] = tc.desired
					}
					assert.Equal(t, map[string]any{field.config: config}, plan.Changes[0].Fields[FieldConfigs])
					assert.Equal(t, field.kind, plan.Changes[0].Fields[FieldAuthStrategyType])
					current := state.ApplicationAuthStrategy{
						StrategyType: field.kind,
						Configs:      map[string]any{field.config: map[string]any{field.field: tc.observed}},
					}
					changed, updates, diagnostics := p.shouldUpdateAuthStrategy(current, desired)
					require.Equal(t, tc.changed, changed)
					if !changed {
						assert.Empty(t, updates)
						assert.Empty(t, diagnostics)
						return
					}
					expectedNew := map[string]any{field.config: map[string]any{field.field: tc.desired}}
					expectedOld := map[string]any{field.config: map[string]any{field.field: tc.old}}
					assert.Equal(t, map[string]any{FieldConfigs: expectedNew}, updates)
					assert.Equal(
						t,
						map[string]FieldChange{FieldConfigs: {Old: expectedOld, New: expectedNew}},
						diagnostics,
					)
				})
			}
		})
	}
}

func authConfigTestResource(field string, values []string) resources.ApplicationAuthStrategyResource {
	var request kkComps.CreateAppAuthStrategyRequest
	if field == "key_names" {
		request = kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(kkComps.AppAuthStrategyKeyAuthRequest{
			Name: "strategy", Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
				KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{KeyNames: values},
			},
		})
	} else {
		config := kkComps.AppAuthStrategyConfigOpenIDConnect{}
		switch field {
		case "credential_claim":
			config.CredentialClaim = values
		case "scopes":
			config.Scopes = values
		case "auth_methods":
			config.AuthMethods = values
		}
		request = kkComps.CreateCreateAppAuthStrategyRequestOpenidConnect(kkComps.AppAuthStrategyOpenIDConnectRequest{
			Name: "strategy", Configs: kkComps.AppAuthStrategyOpenIDConnectRequestConfigs{OpenidConnect: config},
		})
	}
	return resources.ApplicationAuthStrategyResource{
		BaseResource: resources.BaseResource{Ref: "strategy"}, CreateAppAuthStrategyRequest: request,
	}
}

func TestAuthStrategyConfigIssuerAndTypePlanning(t *testing.T) {
	p := &authStrategyPlannerImpl{BasePlanner: NewBasePlanner(&Planner{})}
	desired := authConfigTestResource("scopes", nil)
	desired.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Issuer = "https://issuer.test"
	current := state.ApplicationAuthStrategy{StrategyType: "openid_connect", Configs: map[string]any{
		"openid-connect": map[string]any{FieldDCRProviderIssuer: "https://issuer.test/"},
	}}
	changed, fields, diagnostic := p.shouldUpdateAuthStrategy(current, desired)
	require.True(t, changed)
	want := map[string]any{"openid-connect": map[string]any{FieldDCRProviderIssuer: "https://issuer.test"}}
	assert.Equal(t, want, fields[FieldConfigs])
	assert.Equal(t, FieldChange{Old: current.Configs, New: want}, diagnostic[FieldConfigs])
	plan := &Plan{}
	p.planAuthStrategyCreate(desired, plan)
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, want, plan.Changes[0].Fields[FieldConfigs])
	desired.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Issuer = ""
	changed, fields, diagnostic = p.shouldUpdateAuthStrategy(current, desired)
	assert.False(t, changed)
	assert.Empty(t, fields)
	assert.Empty(t, diagnostic)
	current.StrategyType = "key_auth"
	changed, fields, diagnostic = p.shouldUpdateAuthStrategy(current, desired)
	assert.True(t, changed)
	assert.Contains(t, fields[FieldError], "changing strategy_type from key_auth to openid_connect is not supported")
	assert.Empty(t, diagnostic)
}

package loader

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // custom tag tests
)

func TestStoredEnvSDKFieldsAndTemplates(t *testing.T) {
	t.Setenv("STORED_DESCRIPTION", "42")
	t.Setenv("STORED_ENABLED", "true")
	t.Setenv("STORED_LABELS", `{"owner":"42"}`)
	t.Setenv("DEFERRED_DISPLAY", "display")
	for _, tag := range []string{
		"!env_store STORED_ENABLED",
		"!env {var: STORED_ENABLED, store: true}",
		"!env\n      var: STORED_ENABLED\n      store: true",
	} {
		input := `
_templates:
  portal_defaults:
    description: !env_store STORED_DESCRIPTION
    labels: !env_store STORED_LABELS
portals:
  - ref: stored
    _extends: portal_defaults
    name: stored
    display_name: !env {var: DEFERRED_DISPLAY, store: false}
    auto_approve_developers: ` + tag + "\n"
		rs, err := New().parseYAML(strings.NewReader(input), "stored.yaml", "")
		require.NoError(t, err)
		require.Equal(t, "42", *rs.Portals[0].Description)
		require.True(t, *rs.Portals[0].AutoApproveDevelopers)
		require.Equal(t, "42", *rs.Portals[0].Labels["owner"])
		require.Equal(
			t,
			map[string]map[string]string{"stored": {"/display_name": "__ENV__:DEFERRED_DISPLAY"}},
			rs.EnvSources,
		)
	}
}

func TestStoredEnvCrossFileTemplate(t *testing.T) {
	t.Setenv("STORED_TEMPLATE_BOOL", "true")
	templates := writeLoaderTestFile(t, `_templates:
  stored:
    auto_approve_developers: !env_store STORED_TEMPLATE_BOOL
`)
	consumer := writeLoaderTestFile(t, `portals:
  - ref: stored
    name: stored
    _extends: stored
`)
	rs, err := New().LoadFromSources([]Source{
		{Path: templates, Type: SourceTypeFile},
		{Path: consumer, Type: SourceTypeFile},
	}, false)
	require.NoError(t, err)
	require.True(t, *rs.Portals[0].AutoApproveDevelopers)
	require.Empty(t, rs.EnvSources)
}

func TestStoredEnvDynamicPolicyConfig(t *testing.T) {
	t.Setenv("STORED_BOOL", "true")
	t.Setenv("STORED_RATE", "0.5")
	t.Setenv("STORED_OBJECT", `{"values":[null,9007199254740991,"42",false]}`)
	input := `ai_gateway_policies:
  - ref: rate
    ai_gateway: gateway
    name: rate
    type: rate-limiting
    config:
      redis:
        ssl: !env_store {var: STORED_BOOL, type: boolean}
        ssl_verify: !env {var: STORED_BOOL, store: true, type: boolean}
      sync_rate: !env_store {var: STORED_RATE, type: number}
      extra: !env_store {var: STORED_OBJECT, type: object}
`
	rs, err := New().parseYAML(strings.NewReader(input), "policy.yaml", "")
	require.NoError(t, err)
	require.Empty(t, rs.EnvSources)
	config := rs.AIGatewayPolicies[0].Config
	require.Equal(t, map[string]any{"ssl": true, "ssl_verify": true}, config["redis"])
	require.Equal(t, 0.5, config["sync_rate"])
	require.Equal(t, []any{nil, float64(9007199254740991), "42", false}, config["extra"].(map[string]any)["values"])
	_, err = New().parseYAML(strings.NewReader(strings.Replace(input, ", type: number", "", 1)), "policy.yaml", "")
	require.ErrorContains(t, err, "cannot infer type")
	require.ErrorContains(t, err, "config.sync_rate")
	require.ErrorContains(t, err, "STORED_RATE")
	t.Setenv("STORED_OBJECT", `{"values":[9007199254740993]}`)
	_, err = New().parseYAML(strings.NewReader(input), "policy.yaml", "")
	require.ErrorContains(t, err, "2^53")
}

func TestStoredEnvTypeInference(t *testing.T) {
	type branchA struct {
		Value bool `json:"value"`
	}
	type branchB struct {
		Value string `json:"value"`
	}
	type union struct {
		A *branchA `union:"member"`
		B *branchB `union:"member"`
	}
	type fixture struct {
		Known   int32             `json:"known"`
		Numbers []int64           `json:"numbers"`
		Labels  map[string]string `json:"labels"`
		Dynamic map[string]any    `json:"dynamic"`
		Union   union             `json:"union"`
	}
	for _, tc := range []struct{ input, raw, wantErr string }{
		{"known: !env_store VALUE", "42", ""},
		{"numbers: [!env_store VALUE]", "42", ""},
		{"labels: {owner: !env_store VALUE}", "true", ""},
		{"dynamic: {value: !env_store VALUE}", "42", "cannot infer type"},
		{"union: {value: !env_store VALUE}", "true", "cannot infer type"},
		{"union: {value: !env_store {var: VALUE, type: boolean}}", "true", ""},
		{"known: !env_store {var: VALUE, type: string}", "42", "conflicts"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Setenv("VALUE", tc.raw)
			var document yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(tc.input), &document))
			err := resolveStoredEnvNode(&document, []reflect.Type{reflect.TypeFor[fixture]()}, nil)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStoredEnvRestrictions(t *testing.T) {
	t.Setenv("VALUE", "value")
	_, err := New().LoadFile(writeLoaderTestFile(t, portalSecretConfig("!env_store VALUE")))
	require.ErrorContains(t, err, "requires !secret with a deferred source")
	for _, tc := range []struct{ input, message string }{
		{"portals: [{ref: !env_store VALUE, name: portal}]", "not supported on refs"},
		{"portals: [{ref: portal, name: portal, kongctl: {namespace: !env_store VALUE}}]", "kongctl.namespace"},
		{"portals: [{ref: portal, name: portal, auto_approve_developers: !env_store VALUE}]", "expected boolean"},
		{"portals: [{ref: portal, name: portal, description: !env_store {var: VALUE, type: boolean}}]", "conflicts"},
		{"portals: [{ref: portal, name: portal, description: !env {var: VALUE, store: 'false'}}]", "store must be a boolean"},
	} {
		_, err := New().parseYAML(strings.NewReader(tc.input), "invalid.yaml", "")
		require.ErrorContains(t, err, tc.message)
	}
}

func TestStoredEnvSDKIntegerField(t *testing.T) {
	t.Setenv("STORED_INTERVAL", "60")
	input := `event_gateways:
  - ref: gateway
    name: gateway
    backend_clusters:
      - ref: backend
        name: backend
        bootstrap_servers: ["localhost:9092"]
        authentication: {type: anonymous}
        metadata_update_interval_seconds: !env_store STORED_INTERVAL
`
	rs, err := New().parseYAML(strings.NewReader(input), "integer.yaml", "")
	require.NoError(t, err)
	require.Equal(t, int64(60), *rs.EventGatewayBackendClusters[0].MetadataUpdateIntervalSeconds)
	t.Setenv("STORED_INTERVAL", "0.5")
	_, err = New().parseYAML(strings.NewReader(input), "integer.yaml", "")
	require.ErrorContains(t, err, "expected integer")
	t.Setenv("STORED_INTERVAL", "9007199254740993")
	_, err = New().parseYAML(strings.NewReader(input), "integer.yaml", "")
	require.ErrorContains(t, err, "2^53")
}

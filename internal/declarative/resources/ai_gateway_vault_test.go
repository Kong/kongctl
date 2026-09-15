package resources

import (
	"reflect"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAIGatewayVaultExplainTracksSDKRequests(t *testing.T) {
	actual, err := aiGatewayVaultExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	for _, typ := range []reflect.Type{
		reflect.TypeFor[kkComps.CreateAIGatewayVaultRequest](),
		reflect.TypeFor[kkComps.UpdateAIGatewayVaultRequest](),
	} {
		expected, ok, err := autoExplainSDKUnionNode(typ, nil, defaultExplainHints(""), nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.Len(t, actual.OneOf, len(expected.OneOf))
		for _, branch := range expected.OneOf {
			kind, ok := branch.property("type")
			require.True(t, ok)
			assertExplainNodeDeeplySupportsSDKShape(t, "", branch,
				explainUnionBranchByType(t, actual, kind.Node.Const.(string)),
				func(path, name string) bool {
					return path == "" && (name == SchemaFieldRef || name == SchemaFieldAIGateway)
				})
		}
	}
}

func TestAIGatewayVaultExplainAPIRequirementsAndDefaults(t *testing.T) {
	node, err := aiGatewayVaultExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	for _, tc := range []struct {
		kind         string
		field        string
		required     bool
		defaultValue any
	}{
		{"konnect", "config_store_id", true, nil},
		{"env", "prefix", false, nil},
		{"aws", "assume_role_arn", false, nil},
		{"aws", "role_session_name", false, "KongVault"},
		{"gcp", "project_id", true, nil},
		{"azure", "vault_uri", true, nil},
		{"azure", "location", true, nil},
		{"azure", "type", false, "secrets"},
		{"conjur", "account", true, nil},
		{"conjur", "endpoint_url", true, nil},
		{"conjur", "login", true, nil},
	} {
		t.Run(tc.kind+"/"+tc.field, func(t *testing.T) {
			config, ok := explainUnionBranchByType(t, node, tc.kind).property("config")
			require.True(t, ok)
			field, ok := config.Node.property(tc.field)
			require.True(t, ok)
			require.Equal(t, tc.required, field.Required)
			require.Equal(t, tc.defaultValue, field.Node.Default)
			require.False(t, field.Node.Nullable)
		})
	}
	config, ok := explainUnionBranchByType(t, node, "hcv").property("config")
	require.True(t, ok)
	require.Len(t, config.Node.OneOf, 10)
	for _, branch := range config.Node.OneOf {
		auth, ok := branch.property("auth_method")
		require.True(t, ok)
		require.True(t, auth.Required)
		for _, name := range []string{"host", "port"} {
			field, ok := branch.property(name)
			require.True(t, ok)
			require.True(t, field.Required)
		}
		for _, name := range []string{"key", "token", "client_secret", "secret_access_key", "secret_id"} {
			if field, ok := branch.property(name); ok {
				require.Equal(t, "!secret", field.Node.PreferredTag)
			}
		}
	}
}

func TestAIGatewayVaultScaffoldDescribesAllBranches(t *testing.T) {
	subject, err := ResolveExplainSubject("ai_gateway_vault")
	require.NoError(t, err)
	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)
	for _, kind := range []string{"env", "konnect", "aws", "gcp", "azure", "conjur", "hcv"} {
		require.Contains(t, scaffold, "oneOf option: type="+kind)
	}
	for _, method := range []string{
		"token", "cert", "jwt", "approle", "kubernetes", "gcp_iam", "gcp_gce", "aws_ec2", "aws_iam", "azure",
	} {
		require.Contains(t, scaffold, "oneOf option: auth_method="+method)
	}
	require.Contains(t, scaffold, "assume_role_arn:")
	require.Contains(t, scaffold, "location:")
	require.Contains(t, scaffold, "!secret {source: !env VAULT_KEY}")
	require.Equal(t, "env", scaffoldActiveNode(subject.Node).propIndex["type"].Node.Const)
	var document struct {
		Vaults []map[string]any `json:"ai_gateway_vaults"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(scaffold), &document))
	require.Len(t, document.Vaults, 1)
	require.Equal(t, "", document.Vaults[0]["description"])
}

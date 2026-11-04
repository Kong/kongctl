package loader

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/stretchr/testify/require"
)

func TestEnvNamespacesResolveBeforeDefaultsAndScope(t *testing.T) {
	t.Setenv("ENV_NAMESPACE", "team-prod")
	t.Setenv("ENV_OVERRIDE", `{"namespace":"override-prod"}`)
	for _, expression := range []string{
		"!env ENV_NAMESPACE", "!env {var: ENV_NAMESPACE}",
		"!env {var: ENV_NAMESPACE, store: false}", "!env {var: ENV_NAMESPACE, store: true}",
	} {
		t.Run(expression, func(t *testing.T) {
			input := fmt.Sprintf(`_defaults:
  kongctl:
    namespace: %s
control_planes:
  - ref: inherited
    name: inherited
  - ref: explicit
    name: explicit
    kongctl:
      namespace: !env {var: ENV_OVERRIDE, extract: namespace}
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    kongctl:
      namespace: !env ENV_NAMESPACE
    models: []
`, expression)
			rs, err := New().LoadFile(writeLoaderTestFile(t, input))
			require.NoError(t, err)
			require.Equal(t, "team-prod", resources.GetNamespace(rs.ControlPlanes[0].Kongctl))
			require.Equal(t, resources.NamespaceOriginFileDefault, rs.ControlPlanes[0].Kongctl.NamespaceOrigin)
			require.Equal(t, "override-prod", resources.GetNamespace(rs.ControlPlanes[1].Kongctl))
			require.Equal(t, resources.NamespaceOriginExplicit, rs.ControlPlanes[1].Kongctl.NamespaceOrigin)
			require.Equal(t, "team-prod", resources.GetNamespace(rs.AIGateways[0].Kongctl))
			require.Equal(t, resources.NamespaceOriginExplicit, rs.AIGateways[0].Kongctl.NamespaceOrigin)
			require.Contains(t, rs.SyncScope.GatewayNamespaces, "team-prod")
			require.Contains(t, rs.SyncScope.ModelNamespaces["gateway"], "team-prod")
			require.Empty(t, rs.EnvSources)
		})
	}
}

func TestEnvNamespacesRejectInvalidValues(t *testing.T) {
	for _, target := range []string{
		"_defaults: {kongctl: {namespace: !env ENV_NAMESPACE}}",
		"control_planes: [{ref: cp, name: cp, kongctl: {namespace: !env ENV_NAMESPACE}}]",
		"ai_gateways: [{ref: gw, name: gw, display_name: Gateway, kongctl: {namespace: !env ENV_NAMESPACE}}]",
	} {
		for _, value := range []string{
			"", "UPPER", "has space", "-leading", "trailing-", strings.Repeat("n", 64), "__ENV__:OTHER",
		} {
			t.Run(target+value, func(t *testing.T) {
				t.Setenv("ENV_NAMESPACE", value)
				_, err := New().LoadFile(writeLoaderTestFile(t, target))
				require.Error(t, err)
			})
		}
		t.Run(target+"unset", func(t *testing.T) {
			t.Setenv("ENV_NAMESPACE", "unused")
			require.NoError(t, os.Unsetenv("ENV_NAMESPACE"))
			_, err := New().LoadFile(writeLoaderTestFile(t, target))
			require.ErrorContains(t, err, "environment variable not set")
		})
	}
}

func TestEnvNamespaceGuardrailsAndSourceSemantics(t *testing.T) {
	t.Setenv("ENV_NAMESPACE", "team-prod")
	for _, suffix := range []string{
		"", "\nportals: [{ref: portal, name: portal}]",
		"\nportals: [{ref: portal, _external: {id: external-id}}]",
	} {
		rs, err := New().LoadFile(writeLoaderTestFile(t, "_defaults: {kongctl: {namespace: !env ENV_NAMESPACE}}"+suffix))
		require.NoError(t, err)
		v := validator.NewNamespaceValidator()
		require.NoError(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
			Mode: validator.NamespaceRequirementAny,
		}))
		require.NoError(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
			Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"team-prod"},
		}))
		require.Error(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
			Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"team-dev"},
		}))
	}
	sources := []Source{}
	for i, namespace := range []string{"team-dev", "team-prod"} {
		t.Setenv(fmt.Sprintf("MULTI_NS_%d", i), namespace)
		input := fmt.Sprintf("_defaults: {kongctl: {namespace: !env MULTI_NS_%d}}\n"+
			"portals: [{ref: portal-%d, name: portal-%d}]", i, i, i)
		sources = append(sources, Source{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile})
	}
	rs, err := New().LoadFromSources(sources, false)
	require.NoError(t, err)
	require.Len(t, rs.GetPortalsByNamespace("team-dev"), 1)
	require.Len(t, rs.GetPortalsByNamespace("team-prod"), 1)
	require.Empty(t, rs.EnvSources)
}

func TestEnvNamespaceNestedModelsAndTemplates(t *testing.T) {
	t.Setenv("MODEL_DEFAULT_NS", "team-prod")
	t.Setenv("MODEL_EXPLICIT_NS", "models-prod")
	t.Setenv("GATEWAY_NS", "platform-prod")
	for _, explicit := range []bool{false, true} {
		input := "_defaults: {kongctl: {namespace: !env MODEL_DEFAULT_NS}}\n" + aiGatewayModelYAML
		input = strings.Replace(input, "    name: support-gateway",
			"    kongctl: {namespace: !env GATEWAY_NS}\n    name: support-gateway", 1)
		want := "team-prod"
		if explicit {
			input = strings.Replace(input, "      - ref: support-gpt",
				"      - ref: support-gpt\n        kongctl: {namespace: !env MODEL_EXPLICIT_NS}", 1)
			want = "models-prod"
		}
		rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
		require.NoError(t, err)
		require.Equal(t, want, resources.GetNamespace(rs.AIGatewayModels[0].Kongctl))
		require.Contains(t, rs.SyncScope.ModelNamespaces["support-gateway"], want)
		require.NotContains(t, rs.SyncScope.ModelNamespaces["support-gateway"], "platform-prod")
		require.Empty(t, rs.EnvSources)
	}
	sources := []Source{
		{
			Path: writeLoaderTestFile(t, "_templates: {env-portal: {kongctl: {namespace: !env MODEL_DEFAULT_NS}}}"),
			Type: SourceTypeFile,
		},
		{Path: writeLoaderTestFile(t, "portals: [{_extends: env-portal, ref: portal, name: portal}]"), Type: SourceTypeFile},
	}
	rs, err := New().LoadFromSources(sources, false)
	require.NoError(t, err)
	require.Equal(t, "team-prod", resources.GetNamespace(rs.Portals[0].Kongctl))
	require.Equal(t, resources.NamespaceOriginExplicit, rs.Portals[0].Kongctl.NamespaceOrigin)
	require.Empty(t, rs.EnvSources)
}

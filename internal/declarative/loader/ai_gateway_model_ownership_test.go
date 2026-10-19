package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/stretchr/testify/require"
)

func TestModelNamespaceDefaultsAreIndependentOfGateway(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "file-default", true: "explicit"}[explicit], func(t *testing.T) {
			input := "_defaults:\n  kongctl:\n    namespace: team-a\n    protected: true\n" + aiGatewayModelYAML
			input = strings.Replace(
				input,
				"    name: support-gateway",
				"    kongctl:\n      namespace: platform\n      protected: false\n    name: support-gateway",
				1,
			)
			want := "team-a"
			if explicit {
				input = strings.Replace(
					input,
					"      - ref: support-gpt",
					"      - ref: support-gpt\n        kongctl:\n          namespace: team-b\n          protected: false",
					1,
				)
				want = "team-b"
			}
			rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.Equal(t, "platform", resources.GetNamespace(rs.AIGateways[0].Kongctl))
			require.Equal(t, want, resources.GetNamespace(rs.AIGatewayModels[0].Kongctl))
			require.Equal(t, !explicit, rs.AIGatewayModels[0].Protected())
			require.Contains(t, rs.SyncScope.ModelNamespaces["support-gateway"], want)
		})
	}
}

func TestEmptyModelScopeRetainsEachSourcesNamespace(t *testing.T) {
	var sources []Source
	for _, namespace := range []string{"team-a", "team-b"} {
		input := "_defaults:\n  kongctl:\n    namespace: " + namespace + "\nai_gateways:\n" +
			"  - ref: " + namespace + "-gateway\n    _external:\n      selector:\n" +
			"        matchFields:\n          name: shared\n    models: []\n"
		sources = append(sources, Source{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile})
	}
	rs, err := New().LoadFromSources(sources, false)
	require.NoError(t, err)
	for _, namespace := range []string{"team-a", "team-b"} {
		require.Contains(t, rs.SyncScope.ModelNamespaces[namespace+"-gateway"], namespace)
		require.Len(t, rs.SyncScope.ModelNamespaces[namespace+"-gateway"], 1)
	}
}

func TestModelsParticipateInNamespaceGuardrails(t *testing.T) {
	input := strings.Replace(
		aiGatewayModelYAML,
		"      - ref: support-gpt",
		"      - ref: support-gpt\n        kongctl:\n          namespace: team-b",
		1,
	)
	input = "_defaults:\n  kongctl:\n    namespace: team-a\n" + input
	loader := New()
	rs, err := loader.LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	err = validator.NewNamespaceValidator().ValidateNamespaceRequirement(rs,
		validator.NamespaceRequirement{
			Mode:              validator.NamespaceRequirementSpecific,
			AllowedNamespaces: []string{"team-a"},
		})
	require.ErrorContains(t, err, "team-b")
}

package tags

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag tests
)

func TestExternalTagResolverAliases(t *testing.T) {
	t.Parallel()

	for _, tag := range []string{"!external", "!lookup"} {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()
			resolver := NewExternalTagResolver(tag)
			value, err := resolver.Resolve(&yaml.Node{Kind: yaml.ScalarNode, Value: "name:Shared: Portal"})
			require.NoError(t, err)
			lookup, ok := ParseExternalPlaceholder(value.(string))
			require.True(t, ok)
			require.Equal(t, map[string]string{"name": "Shared: Portal"}, lookup.MatchFields)
		})
	}
}

func TestExternalTagResolverMapping(t *testing.T) {
	t.Parallel()

	resolver := NewExternalTagResolver("!external")
	value, err := resolver.Resolve(&yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "name"},
			{Kind: yaml.ScalarNode, Value: "shared"},
			{Kind: yaml.ScalarNode, Value: "display_name"},
			{Kind: yaml.ScalarNode, Value: "Shared Gateway"},
		},
	})
	require.NoError(t, err)
	lookup, ok := ParseExternalPlaceholder(value.(string))
	require.True(t, ok)
	require.Equal(t, map[string]string{"name": "shared", "display_name": "Shared Gateway"}, lookup.MatchFields)
}

func TestExternalTagResolverRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node *yaml.Node
	}{
		{name: "missing delimiter", node: &yaml.Node{Kind: yaml.ScalarNode, Value: "shared"}},
		{name: "empty selector", node: &yaml.Node{Kind: yaml.MappingNode}},
		{name: "id combined", node: &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "id"},
			{Kind: yaml.ScalarNode, Value: "123"},
			{Kind: yaml.ScalarNode, Value: "name"},
			{Kind: yaml.ScalarNode, Value: "shared"},
		}}},
		{name: "non-string value", node: &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
			{Kind: yaml.ScalarNode, Tag: "!!int", Value: "123"},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewExternalTagResolver("!external").Resolve(tt.node)
			require.Error(t, err)
		})
	}
}

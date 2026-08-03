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

func TestExternalTagResolverNestedEnvSelector(t *testing.T) {
	t.Setenv("EXTERNAL_SELECTOR", "Shared Portal")

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("value: !lookup {name: !env EXTERNAL_SELECTOR}\n"), &doc))
	node := doc.Content[0].Content[1]

	value, err := NewExternalTagResolver(TagLookup).Resolve(node)
	require.NoError(t, err)
	lookup, ok := ParseExternalPlaceholder(value.(string))
	require.True(t, ok)
	require.Equal(t, map[string]string{"name": "Shared Portal"}, lookup.MatchFields)
	require.Equal(t, []string{"name"}, lookup.SensitiveFields)
	require.Equal(
		t,
		`"name"="[redacted from !env]"`,
		ExternalLookupDisplayKey(lookup.MatchFields, lookup.SensitiveFields),
	)
}

func TestExternalTagResolverNestedEnvMapExtraction(t *testing.T) {
	t.Setenv("EXTERNAL_SELECTOR_DOCUMENT", "portal:\n  name: Shared Portal\n")

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`value: !external
  name: !env
    var: EXTERNAL_SELECTOR_DOCUMENT
    extract: portal.name
`), &doc))
	node := doc.Content[0].Content[1]

	value, err := NewExternalTagResolver(TagExternal).Resolve(node)
	require.NoError(t, err)
	lookup, ok := ParseExternalPlaceholder(value.(string))
	require.True(t, ok)
	require.Equal(t, map[string]string{"name": "Shared Portal"}, lookup.MatchFields)
	require.Equal(t, []string{"name"}, lookup.SensitiveFields)
}

func TestExternalTagResolverRejectsInvalidNestedEnvSelector(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		var doc yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("value: !lookup {name: !env UNSET_EXTERNAL_SELECTOR}\n"), &doc))

		_, err := NewExternalTagResolver(TagLookup).Resolve(doc.Content[0].Content[1])
		require.ErrorContains(t, err, "environment variable not set: UNSET_EXTERNAL_SELECTOR")
	})

	t.Run("empty", func(t *testing.T) {
		t.Setenv("EMPTY_EXTERNAL_SELECTOR", "")
		var doc yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("value: !lookup {name: !env EMPTY_EXTERNAL_SELECTOR}\n"), &doc))

		_, err := NewExternalTagResolver(TagLookup).Resolve(doc.Content[0].Content[1])
		require.ErrorContains(t, err, "mapping keys and values cannot be empty")
	})

	t.Run("non-string extraction", func(t *testing.T) {
		t.Setenv("NON_STRING_EXTERNAL_SELECTOR", "portal:\n  enabled: true\n")
		var doc yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(
			"value: !lookup {name: !env NON_STRING_EXTERNAL_SELECTOR#portal.enabled}\n",
		), &doc))

		_, err := NewExternalTagResolver(TagLookup).Resolve(doc.Content[0].Content[1])
		require.ErrorContains(t, err, "!env value must resolve to a string")
	})
}

func TestExternalLookupKeyIsUnambiguous(t *testing.T) {
	t.Parallel()

	separateSelectors := ExternalLookupKey(map[string]string{"a": "b", "c": "d"})
	delimitersInValue := ExternalLookupKey(map[string]string{"a": "b,c=d"})
	require.NotEqual(t, separateSelectors, delimitersInValue)
	require.Equal(t, separateSelectors, ExternalLookupKey(map[string]string{"c": "d", "a": "b"}))
}

func TestExternalTagResolverRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		node    *yaml.Node
		wantErr string
	}{
		{name: "missing delimiter", node: &yaml.Node{Kind: yaml.ScalarNode, Value: "shared"}},
		{name: "invalid node kind", node: &yaml.Node{}},
		{name: "empty selector", node: &yaml.Node{Kind: yaml.MappingNode}},
		{
			name: "non-scalar key",
			node: &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.MappingNode},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "shared"},
			}},
			wantErr: "mapping keys must be strings",
		},
		{
			name: "non-string key",
			node: &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!int", Value: "123"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "shared"},
			}},
			wantErr: "mapping keys must be strings",
		},
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
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag processing
)

func TestSecretTagResolverParsesSourceAndComposition(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		parts []SecretPart
	}{
		{
			name: "single source",
			yaml: "value: !secret {source: !env PORTAL_SECRET}\n",
			parts: []SecretPart{{Source: &SecretSource{
				Kind: "env", Reference: "PORTAL_SECRET",
			}}},
		},
		{
			name: "ordered composition",
			yaml: "value: !secret\n  parts:\n    - 'Bearer '\n    - !env ACCESS_TOKEN\n    - ':'\n    - !env TOKEN_SUFFIX\n",
			parts: []SecretPart{
				{Literal: new("Bearer ")},
				{Source: &SecretSource{Kind: "env", Reference: "ACCESS_TOKEN"}},
				{Literal: new(":")},
				{Source: &SecretSource{Kind: "env", Reference: "TOKEN_SUFFIX"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var document yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &document))
			node := document.Content[0].Content[1]
			placeholder, err := NewSecretTagResolver().Resolve(node)
			require.NoError(t, err)
			expression, err := ParseSecretPlaceholder(placeholder.(string))
			require.NoError(t, err)
			assert.Equal(t, tt.parts, expression.Parts)
		})
	}
}

func TestSecretTagResolverRejectsInvalidForms(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		message string
	}{
		{name: "not a map", yaml: "value: !secret TOKEN\n", message: "must be a map"},
		{name: "neither", yaml: "value: !secret {}\n", message: "exactly one"},
		{name: "both", yaml: "value: !secret {source: !env TOKEN, parts: [!env TOKEN]}\n", message: "exactly one"},
		{name: "empty parts", yaml: "value: !secret {parts: []}\n", message: "non-empty sequence"},
		{name: "all literal", yaml: "value: !secret {parts: [Bearer, token]}\n", message: "at least one deferred source"},
		{name: "eager file", yaml: "value: !secret {source: !file secret.txt}\n", message: "only !env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var document yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &document))
			_, err := NewSecretTagResolver().Resolve(document.Content[0].Content[1])
			require.ErrorContains(t, err, tt.message)
		})
	}
}

func TestResolveSecretExpressionPreservesExactComposition(t *testing.T) {
	t.Setenv("SECRET_TOKEN", " token ")
	expression := SecretExpression{Parts: []SecretPart{
		{Literal: new("Bearer[")},
		{Source: &SecretSource{Kind: "env", Reference: "SECRET_TOKEN"}},
		{Literal: new("]")},
	}}

	actual, err := ResolveSecretExpression(expression)
	require.NoError(t, err)
	assert.Equal(t, "Bearer[ token ]", actual)
}

func TestResolveSecretExpressionRejectsMissingAndEmptySources(t *testing.T) {
	expression := SecretExpression{Parts: []SecretPart{{Source: &SecretSource{
		Kind: "env", Reference: "MISSING_SECRET",
	}}}}
	_, err := ResolveSecretExpression(expression)
	require.ErrorContains(t, err, "environment variable not set")

	t.Setenv("EMPTY_SECRET", "")
	expression.Parts[0].Source.Reference = "EMPTY_SECRET"
	_, err = ResolveSecretExpression(expression)
	require.ErrorContains(t, err, "empty value")
}

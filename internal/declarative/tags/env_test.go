package tags

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

func TestEnvTagResolver_Tag(t *testing.T) {
	resolver := NewEnvTagResolver(EnvTagModeResolve)
	assert.Equal(t, "!env", resolver.Tag())
}

func TestEnvTagResolver_Resolve(t *testing.T) {
	t.Setenv("TEST_ENV_SIMPLE", "secret-value")
	t.Setenv("TEST_ENV_CONFIG", "credentials:\n  username: admin\n")

	tests := []struct {
		name     string
		mode     EnvTagMode
		node     *yaml.Node
		expected any
		wantErr  string
	}{
		{
			name: "resolve scalar value",
			mode: EnvTagModeResolve,
			node: &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: "TEST_ENV_SIMPLE",
			},
			expected: "secret-value",
		},
		{
			name: "resolve extracted value",
			mode: EnvTagModeResolve,
			node: &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: "TEST_ENV_CONFIG#credentials.username",
			},
			expected: "admin",
		},
		{
			name: "preserve placeholder",
			mode: EnvTagModePlaceholder,
			node: &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: "TEST_ENV_SIMPLE",
			},
			expected: "__ENV__:TEST_ENV_SIMPLE",
		},
		{
			name: "missing env var",
			mode: EnvTagModeResolve,
			node: &yaml.Node{
				Kind:  yaml.ScalarNode,
				Value: "TEST_ENV_MISSING",
			},
			wantErr: "environment variable not set: TEST_ENV_MISSING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewEnvTagResolver(tt.mode)
			result, err := resolver.Resolve(tt.node)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveEnvPlaceholder(t *testing.T) {
	t.Setenv("TEST_ENV_PLACEHOLDER", "resolved-value")

	value, err := ResolveEnvPlaceholder("__ENV__:TEST_ENV_PLACEHOLDER")
	require.NoError(t, err)
	assert.Equal(t, "resolved-value", value)
}

func TestParseEnvPlaceholder(t *testing.T) {
	varRef, extractPath, ok := ParseEnvPlaceholder("__ENV__:TEST_ENV_CONFIG#credentials.username")
	require.True(t, ok)
	assert.Equal(t, "TEST_ENV_CONFIG", varRef)
	assert.Equal(t, "credentials.username", extractPath)
}

package tags

import (
	"os"
	"path/filepath"
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

func TestSecretTagResolverDefersFileSourceUntilExecution(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "runtime.key")
	require.NoError(t, os.WriteFile(secretPath, []byte("first-private-key"), 0o600))

	var document yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("value: !secret {source: !file runtime.key}\n"), &document))
	placeholder, err := NewSecretTagResolverWithFileScope(dir, dir).Resolve(document.Content[0].Content[1])
	require.NoError(t, err)
	assert.NotContains(t, placeholder, "first-private-key")

	expression, err := ParseSecretPlaceholder(placeholder.(string))
	require.NoError(t, err)
	require.Len(t, expression.Parts, 1)
	assert.Equal(t, "file", expression.Parts[0].Source.Kind)
	assert.Equal(t, secretPath, expression.Parts[0].Source.Reference)
	assert.Equal(t, dir, expression.Parts[0].Source.RootDir)

	require.NoError(t, os.WriteFile(secretPath, []byte("rotated-private-key"), 0o600))
	resolved, err := ResolveSecretExpression(expression)
	require.NoError(t, err)
	assert.Equal(t, "rotated-private-key", resolved)
}

func TestSecretTagResolverRejectsFileSourceOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "runtime.key"), []byte("private-key"), 0o600))

	var document yaml.Node
	reference, err := filepath.Rel(root, filepath.Join(outside, "runtime.key"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal([]byte("value: !secret {source: !file "+reference+"}\n"), &document))
	_, err = NewSecretTagResolverWithFileScope(root, root).Resolve(document.Content[0].Content[1])
	require.ErrorContains(t, err, "outside base dir")
}

func TestResolveSecretExpressionFromBaseUsesTrustedPlanDirectory(t *testing.T) {
	planDir := t.TempDir()
	secretDir := filepath.Join(planDir, "certs")
	require.NoError(t, os.Mkdir(secretDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(secretDir, "runtime.key"), []byte("private-key"), 0o600))
	expression := SecretExpression{Parts: []SecretPart{{Source: &SecretSource{
		Kind: "file", Reference: filepath.Join("certs", "runtime.key"), RootDir: "/",
	}}}}

	resolved, err := ResolveSecretExpressionFromBase(expression, planDir)
	require.NoError(t, err)
	assert.Equal(t, "private-key", resolved)

	expression.Parts[0].Source.Reference = filepath.Join("..", "outside.key")
	_, err = ResolveSecretExpressionFromBase(expression, planDir)
	require.ErrorContains(t, err, "outside base dir")

	expression.Parts[0].Source.Reference = filepath.Join(planDir, "certs", "runtime.key")
	_, err = ResolveSecretExpressionFromBase(expression, planDir)
	require.ErrorContains(t, err, "must be relative")
}

func TestResolveSecretExpressionFromBaseRejectsSymlinkEscape(t *testing.T) {
	planDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.key")
	require.NoError(t, os.WriteFile(outsidePath, []byte("private-key"), 0o600))
	require.NoError(t, os.Symlink(outsidePath, filepath.Join(planDir, "runtime.key")))
	expression := SecretExpression{Parts: []SecretPart{{Source: &SecretSource{
		Kind: "file", Reference: "runtime.key",
	}}}}

	_, err := ResolveSecretExpressionFromBase(expression, planDir)
	require.ErrorContains(t, err, "outside base dir")
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
		{name: "unscoped file", yaml: "value: !secret {source: !file secret.txt}\n", message: "requires file scope"},
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

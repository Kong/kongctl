package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestAPIVersionResource_UnmarshalYAML_WithSpec(t *testing.T) {
	yamlContent := `
ref: api-v1
version: "1.0.0"
spec:
  openapi: "3.0.0"
  info:
    title: "Test API"
    version: "1.0.0"
    description: "Test API description"
  paths:
    /test:
      get:
        summary: "Test endpoint"
        responses:
          '200':
            description: "Success"
`

	var apiVersion APIVersionResource
	err := yaml.Unmarshal([]byte(yamlContent), &apiVersion)
	require.NoError(t, err)

	assert.Equal(t, "api-v1", apiVersion.Ref)
	assert.Equal(t, "1.0.0", *apiVersion.Version)
	require.NotNil(t, apiVersion.Spec, "Spec should not be nil")
	if apiVersion.Spec != nil {
		require.NotNil(t, apiVersion.Spec.Content, "Spec.Content should not be nil")
		if apiVersion.Spec.Content != nil {
			t.Logf("Spec content: %q", *apiVersion.Spec.Content)
			// Check that spec content is a JSON string
			expectedJSON := `{"info":{"description":"Test API description","title":"Test API","version":"1.0.0"},` +
				`"openapi":"3.0.0","paths":{"/test":{"get":{"responses":{"200":{"description":"Success"}},` +
				`"summary":"Test endpoint"}}}}`
			assert.JSONEq(t, expectedJSON, *apiVersion.Spec.Content)
		}
	}
}

func TestAPIVersionResource_UnmarshalYAML_WithoutSpec(t *testing.T) {
	yamlContent := `
ref: api-v1
version: "2.0.0"
`

	var apiVersion APIVersionResource
	err := yaml.Unmarshal([]byte(yamlContent), &apiVersion)
	require.NoError(t, err)

	assert.Equal(t, "api-v1", apiVersion.Ref)
	assert.Equal(t, "2.0.0", *apiVersion.Version)
	assert.Nil(t, apiVersion.Spec)
}

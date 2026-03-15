package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveExplainSubject_Resource(t *testing.T) {
	subject, err := ResolveExplainSubject("api")
	require.NoError(t, err)

	assert.Equal(t, "api", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPI, subject.Doc.ResourceType)
	assert.True(t, subject.Doc.SupportsRoot)
	assert.True(t, subject.Doc.SupportsNested)
	assert.True(t, subject.ResourceTarget)
	assert.Empty(t, subject.FieldPath)
	assert.Equal(t, "object", subject.Node.Kind)
}

func TestResolveExplainSubject_NestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api.versions")
	require.NoError(t, err)

	assert.Equal(t, "api.versions", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPIVersion, subject.Doc.ResourceType)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []string{"versions"}, subject.FieldPath)
	assert.Equal(t, []ResourceType{ResourceTypeAPI}, subject.AncestorTypes)
}

func TestResolveExplainSubject_FieldPath(t *testing.T) {
	subject, err := ResolveExplainSubject("api.publications.portal_id")
	require.NoError(t, err)

	assert.Equal(t, "api.publications.portal_id", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPIPublication, subject.Doc.ResourceType)
	assert.False(t, subject.ResourceTarget)
	assert.Equal(t, []string{"publications", "portal_id"}, subject.FieldPath)
	assert.Equal(t, "string", subject.Node.Kind)
	assert.Equal(t, "portal", subject.Node.RefKind)
	assert.Equal(t, "!ref", subject.Node.PreferredTag)
}

func TestResolveExplainSubject_RecursivePortalResource(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)

	pagesField, ok := subject.Node.property("pages")
	require.True(t, ok)
	require.NotNil(t, pagesField.Node)
	require.NotNil(t, pagesField.Node.Items)

	childrenField, ok := pagesField.Node.Items.property("children")
	require.True(t, ok)
	require.NotNil(t, childrenField.Node)
	require.NotNil(t, childrenField.Node.Items)

	assert.Equal(t, "object", childrenField.Node.Items.Kind)
	assert.Contains(t, childrenField.Node.Items.Description, "Recursive")
	assert.Contains(t, childrenField.Node.Items.Notes, "schema recursion truncated after the first expansion")
}

func TestRenderExplainSchema_Metadata(t *testing.T) {
	subject, err := ResolveExplainSubject("api")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema.Schema)
	assert.Equal(t, "kongctl://declarative/api", schema.ID)
	assert.Equal(t, "api", schema.XKongctlResource)
	assert.Equal(t, "api", schema.XKongctlPath)
	assert.Equal(t, "apis", schema.XKongctlRootKey)
}

func TestRenderScaffoldYAML_RootResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "apis:")
	assert.Contains(t, scaffold, "- ref: my-resource")
	assert.Contains(t, scaffold, "name: my-resource")
	assert.Contains(t, scaffold, "# versions:")
}

func TestRenderScaffoldYAML_NestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api.versions")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "apis:")
	assert.Contains(t, scaffold, "- ref: my-resource")
	assert.Contains(t, scaffold, "versions:")
	assert.Contains(t, scaffold, "- ref: my-resource")
	assert.NotContains(t, scaffold, "api: value")
	assert.NotContains(t, scaffold, "kongctl:")
}

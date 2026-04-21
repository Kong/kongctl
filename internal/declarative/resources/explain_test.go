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
	assert.False(t, subject.Doc.SupportsNestedDeclaration)
	assert.Equal(t, explainResourceClassTopLevel, subject.Doc.ResourceClass)
	assert.True(t, subject.ResourceTarget)
	assert.Empty(t, subject.FieldPath)
	assert.Equal(t, "object", subject.Node.Kind)
}

func TestResolveExplainSubject_NestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api.versions")
	require.NoError(t, err)

	assert.Equal(t, "api.versions", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPIVersion, subject.Doc.ResourceType)
	assert.Equal(t, explainResourceClassChild, subject.Doc.ResourceClass)
	assert.True(t, subject.Doc.SupportsNestedDeclaration)
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
	assert.Equal(t, []string{"portal_id"}, subject.FieldRelativePath)
	assert.True(t, subject.FieldRequired)
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
	assert.Equal(t, "api", schema.XResource)
	assert.Equal(t, "api", schema.XPath)
	assert.Equal(t, "apis", schema.XRootKey)
	assert.Equal(t, explainResourceClassTopLevel, schema.XClass)
	require.NotNil(t, schema.XNestedDecl)
	assert.False(t, *schema.XNestedDecl)
}

func TestRenderExplainText_ResourceSubject(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "RESOURCE\nRESOURCE CLASS: top-level")
	assert.Contains(t, text, "ROOT KEY: portals[]")
	assert.Contains(t, text, "SUPPORTS ROOT: true")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: false")
	assert.Contains(t, text, "ACCEPTS kongctl metadata: yes")
	assert.Contains(
		t,
		text,
		"CHILD RESOURCES: auth_settings, custom_domain, customization, email_config, identity_providers, pages, snippets, teams", //nolint:lll
	)
	assert.Contains(t, text, "\nFIELD DETAILS: use --extended")
	assert.NotContains(t, text, "RESOURCE: portal")
	assert.NotContains(t, text, "PATH: portal")
	assert.NotContains(t, text, "NESTED PATHS:")
	assert.NotContains(t, text, "\nFIELDS\n")
}

func TestRenderExplainText_ResourceSubjectExtended(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)

	text := RenderExplainText(subject, true)

	assert.Contains(t, text, "\nFIELDS\n- ref: string required")
	assert.NotContains(t, text, "FIELD DETAILS: use --extended")
}

func TestRenderExplainText_FieldSubject(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.name")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "FIELD\nPATH: portal.name")
	assert.Contains(t, text, "TYPE: string")
	assert.Contains(t, text, "OPTIONAL: true")
	assert.Contains(t, text, "RECOMMENDED: yes")
	assert.Contains(t, text, "DEFAULT FROM: ref")
	assert.Contains(t, text, "\nRESOURCE\nRESOURCE CLASS: top-level")
	assert.Contains(t, text, "ROOT KEY: portals[]")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: false")
	assert.Contains(
		t,
		text,
		"CHILD RESOURCES: auth_settings, custom_domain, customization, email_config, identity_providers, pages, snippets, teams", //nolint:lll
	)
	assert.NotContains(t, text, "PLACEMENT")
	assert.NotContains(t, text, "YAML PATH:")
}

func TestRenderExplainText_NestedFieldSubjectPlacement(t *testing.T) {
	subject, err := ResolveExplainSubject("api.publications.portal_id")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "FIELD\nPATH: api.publications.portal_id")
	assert.Contains(t, text, "OPTIONAL: false")
	assert.Contains(t, text, "NESTED YAML PATH: apis[].publications[].portal_id")
	assert.Contains(t, text, "ROOT YAML PATH: api_publications[].portal_id")
	assert.Contains(t, text, "\nRESOURCE\nRESOURCE CLASS: child")
	assert.Contains(t, text, "ROOT KEY: api_publications[]")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: true")
	assert.Contains(t, text, "ACCEPTS kongctl metadata: no")
	assert.NotContains(t, text, "FIELD DETAILS: use --extended")
}

func TestRenderExplainText_NestedChildResourceSummary(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.pages")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "RESOURCE\nRESOURCE CLASS: child")
	assert.Contains(t, text, "ROOT KEY: portal_pages[]")
	assert.Contains(t, text, "SUPPORTS ROOT: true")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: true")
	assert.Contains(t, text, "ACCEPTS kongctl metadata: no")
	assert.Contains(t, text, "CHILD RESOURCES: children")
}

func TestRenderExplainSchema_FieldSubjectMetadata(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.name")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	require.NotNil(t, schema.XSubject)
	assert.Equal(t, "field", schema.XSubject.Kind)
	assert.Equal(t, "portal.name", schema.XSubject.Path)
	require.NotNil(t, schema.XSubject.Required)
	assert.False(t, *schema.XSubject.Required)
	require.NotNil(t, schema.XSubject.Recommended)
	assert.True(t, *schema.XSubject.Recommended)
	require.NotNil(t, schema.XPlacement)
	assert.Equal(t, "portals[].name", schema.XPlacement.YAMLPath)
	resource, ok := schema.XResource.(*ExplainSchemaResource)
	require.True(t, ok)
	assert.Equal(t, "portal", resource.Name)
	assert.Equal(t, explainResourceClassTopLevel, resource.ResourceClass)
	assert.Equal(t, "ref", schema.XDefault)
	assert.Empty(t, schema.XPath)
	assert.Empty(t, schema.XClass)
	assert.Nil(t, schema.XRoot)
	assert.Nil(t, schema.XNestedDecl)
}

func TestRenderExplainSchema_NestedFieldSubjectPlacement(t *testing.T) {
	subject, err := ResolveExplainSubject("api.publications.portal_id")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)
	require.NotNil(t, schema.XPlacement)

	assert.Equal(t, "apis[].publications[].portal_id", schema.XPlacement.NestedYAMLPath)
	assert.Equal(t, "api_publications[].portal_id", schema.XPlacement.RootYAMLPath)
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

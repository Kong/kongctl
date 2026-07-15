package resources

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/maturity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const maturityTestResourceType ResourceType = "maturity_test_resource"

func registerMaturityTestResource(t *testing.T, options ...ResourceRegistrationOption) {
	t.Helper()
	registerResourceType(
		maturityTestResourceType,
		func(resourceSet *ResourceSet) *[]PortalResource { return &resourceSet.Portals },
		AutoExplain[PortalResource](),
		options...,
	)
	t.Cleanup(func() {
		delete(registry, maturityTestResourceType)
		explainDocCacheMu.Lock()
		delete(explainDocCache, maturityTestResourceType)
		explainDocCacheMu.Unlock()
	})
}

func TestResourceMaturityDefaultsAndOperations(t *testing.T) {
	portal, err := MaturityFor(ResourceTypePortal)
	require.NoError(t, err)
	assert.Equal(t, maturity.LevelGA, portal.Effective.Level)
	assert.Nil(t, portal.Declared)

	registerMaturityTestResource(
		t,
		WithMaturity(maturity.Metadata{Level: maturity.LevelBeta, Message: "Beta resource."}),
		WithOperationMaturity(OperationDelete, maturity.Metadata{Level: maturity.LevelTechPreview}),
		WithOperationMaturity(OperationCreate, maturity.Metadata{Level: maturity.LevelGA}),
	)

	resource, err := MaturityFor(maturityTestResourceType)
	require.NoError(t, err)
	assert.Equal(t, maturity.LevelBeta, resource.Effective.Level)
	assert.Equal(t, maturity.KindResource, resource.Source.Kind)

	deleteMaturity, err := MaturityFor(maturityTestResourceType, OperationDelete)
	require.NoError(t, err)
	assert.Equal(t, maturity.LevelTechPreview, deleteMaturity.Effective.Level)
	assert.Equal(t, maturity.KindOperation, deleteMaturity.Source.Kind)

	createMaturity, err := MaturityFor(maturityTestResourceType, OperationCreate)
	require.NoError(t, err)
	require.NotNil(t, createMaturity.Declared)
	assert.Equal(t, maturity.LevelGA, createMaturity.Declared.Level)
	assert.Equal(t, maturity.LevelBeta, createMaturity.Effective.Level)
	assert.Equal(t, maturity.KindResource, createMaturity.Source.Kind)

	_, err = MaturityFor(maturityTestResourceType, Operation("invalid"))
	require.Error(t, err)
	_, err = MaturityFor(ResourceType("missing"))
	require.Error(t, err)
}

func TestResourceMaturityExplainAndScaffold(t *testing.T) {
	registerMaturityTestResource(
		t,
		WithMaturity(maturity.Metadata{
			Level:        maturity.LevelBeta,
			Message:      "This resource may change before GA.",
			ReferenceURL: "https://example.test/resource",
		}),
		WithOperationMaturity(OperationDelete, maturity.Metadata{
			Level:   maturity.LevelTechPreview,
			Message: "Delete is experimental.",
		}),
	)

	subject, err := ResolveExplainSubject(string(maturityTestResourceType))
	require.NoError(t, err)
	text := RenderExplainText(subject, false)
	assert.Contains(t, text, "RESOURCE\nMATURITY: Beta\nRESOURCE CLASS:")
	assert.Contains(t, text, "OPERATION MATURITY:\n  delete: Tech Preview")

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema.XMaturity)
	assert.Equal(t, maturity.LevelBeta, schema.XMaturity.Level)
	assert.Equal(t, "This resource may change before GA.", schema.XMaturity.Message)
	assert.Equal(t, "https://example.test/resource", schema.XMaturity.ReferenceURL)
	assert.Equal(t, maturity.LevelTechPreview, schema.XMaturity.Operations["delete"].Level)
	assert.NotContains(t, schema.XMaturity.Operations, "create")

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(scaffold,
		"# Maturity: Beta\n# This resource may change before GA.\n\n"))
}

func TestGAScaffoldAndResourceSerializationExcludeMaturity(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)
	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)
	assert.False(t, strings.HasPrefix(scaffold, "# Maturity:"))

	resourceSet := ResourceSet{Portals: []PortalResource{{BaseResource: BaseResource{Ref: "portal"}}}}
	jsonData, err := json.Marshal(resourceSet)
	require.NoError(t, err)
	yamlData, err := yaml.Marshal(resourceSet)
	require.NoError(t, err)
	assert.NotContains(t, string(jsonData), "maturity")
	assert.NotContains(t, string(yamlData), "maturity")
}

func TestInvalidResourceMaturityPanicsDuringRegistration(t *testing.T) {
	assert.Panics(t, func() {
		registerResourceType(
			ResourceType("invalid_maturity_test"),
			func(resourceSet *ResourceSet) *[]PortalResource { return &resourceSet.Portals },
			AutoExplain[PortalResource](),
			WithMaturity(maturity.Metadata{Level: "invalid"}),
		)
	})
}

package resources

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nullableLoadSchemaFixture struct {
	Slice  *[]string                  `yaml:"slice,omitempty"`
	Array  *[1]string                 `yaml:"array,omitempty"`
	Nested *nullableLoadSchemaFixture `yaml:"nested,omitempty"`
}

func TestRenderLoadSchemaComposesRegisteredResources(t *testing.T) {
	schema, err := RenderLoadSchema(LoadSchemaProfileShape)
	require.NoError(t, err)

	assert.Equal(t, jsonSchemaDraft202012, schema.Schema)
	assert.Equal(t, loadSchemaID, schema.ID)
	assert.False(t, schema.Additional.(bool))
	for _, resourceType := range RegisteredTypes() {
		assert.Contains(t, schema.Defs, string(resourceType))
	}

	aiGateways := schema.Properties["ai_gateways"]
	require.NotNil(t, aiGateways)
	require.NotNil(t, aiGateways.Items)
	assert.Equal(t, "#/$defs/ai_gateway", aiGateways.Items.Ref)
}

func TestRenderLoadSchemaClosesObjectsAndKeepsMapsOpen(t *testing.T) {
	schema, err := RenderLoadSchema(LoadSchemaProfileShape)
	require.NoError(t, err)

	controlPlane := schema.Defs[string(ResourceTypeControlPlane)]
	require.NotNil(t, controlPlane)
	assert.False(t, controlPlane.Additional.(bool))

	labels := controlPlane.Properties["labels"]
	require.NotNil(t, labels)
	require.NotNil(t, labels.Additional)
	assert.NotEqual(t, false, labels.Additional)

	proxyURLs := controlPlane.Properties["proxy_urls"]
	require.NotNil(t, proxyURLs)
	require.NotNil(t, proxyURLs.Items)
	assert.False(t, proxyURLs.Items.Additional.(bool))
}

func TestReflectLoadSchemaPreservesPointerNullability(t *testing.T) {
	schema := reflectLoadSchema(reflect.TypeFor[nullableLoadSchemaFixture](), nil)

	assert.Equal(t, []string{explainKindArray, jsonNullLiteral}, schema.Properties["slice"].Type)
	assert.Equal(t, []string{explainKindArray, jsonNullLiteral}, schema.Properties["array"].Type)
	assert.Equal(t, []string{explainKindObject, jsonNullLiteral}, schema.Properties["nested"].Type)
	assert.True(t, schema.Properties["nested"].Additional.(bool))
}

func TestRenderLoadSchemaRetainsOnlyUnionDiscriminatorScalars(t *testing.T) {
	schema, err := RenderLoadSchema(LoadSchemaProfileShape)
	require.NoError(t, err)

	authStrategy := schema.Defs[string(ResourceTypeApplicationAuthStrategy)]
	require.Len(t, authStrategy.OneOf, 2)
	for _, branch := range authStrategy.OneOf {
		strategyType := branch.Properties["strategy_type"]
		require.NotNil(t, strategyType)
		assert.NotNil(t, strategyType.Const)
		assert.Contains(t, branch.Required, "strategy_type")
	}

	implementation := schema.Defs[string(ResourceTypeAPIImplementation)]
	require.NotNil(t, implementation)
	assert.Nil(t, implementation.Properties["type"].Const)
	assert.Nil(t, implementation.Properties["type"].Type)
}

func TestRenderShapeSchemaRetainsScalarTypesOnlyForUnionBranches(t *testing.T) {
	ports := renderShapeSchema(&ExplainNode{
		OneOf: []*ExplainNode{
			{Kind: explainKindString},
			{Kind: explainKindInteger},
		},
	})
	require.Len(t, ports.OneOf, 2)
	assert.Equal(t, explainKindString, ports.OneOf[0].Type)
	assert.Equal(t, explainKindInteger, ports.OneOf[1].Type)

	resourceNames := renderShapeSchema(&ExplainNode{
		OneOf: []*ExplainNode{
			{
				Kind: explainKindArray,
				Items: &ExplainNode{
					Kind: explainKindObject,
					Properties: []*ExplainField{
						{Name: "match", Node: &ExplainNode{Kind: explainKindString}},
					},
				},
			},
			{Kind: explainKindString},
		},
	})
	require.Len(t, resourceNames.OneOf, 2)
	assert.Equal(t, explainKindArray, resourceNames.OneOf[0].Type)
	assert.Equal(t, explainKindString, resourceNames.OneOf[1].Type)
	assert.Nil(t, resourceNames.OneOf[0].Items.Properties["match"].Type)

	ordinaryScalar := renderShapeSchema(&ExplainNode{Kind: explainKindString})
	assert.Nil(t, ordinaryScalar.Type)
}

func TestRenderShapeSchemaDoesNotRequireOptionalConstFields(t *testing.T) {
	schema := renderShapeSchema(explainUnionNode(
		explainObject(
			explainField("type", explainConstStringNode("vertex"), true, true),
			explainField(
				"use_gcp_service_account",
				&ExplainNode{Kind: explainKindBoolean, Const: true},
				false,
				false,
			),
		),
		explainObject(explainField("type", explainConstStringNode("basic"), true, true)),
	))

	require.Len(t, schema.OneOf, 2)
	assert.Contains(t, schema.OneOf[0].Required, "type")
	assert.NotContains(t, schema.OneOf[0].Required, "use_gcp_service_account")
	assert.Equal(t, true, schema.OneOf[0].Properties["use_gcp_service_account"].Const)
}

func TestRenderLoadSchemaRetainsKnownRejectedFieldDiagnostics(t *testing.T) {
	schema, err := RenderLoadSchema(LoadSchemaProfileShape)
	require.NoError(t, err)

	mcpServer := schema.Defs[string(ResourceTypeAIGatewayMCPServer)]
	require.NotNil(t, mcpServer)
	var conversionOnly *JSONSchema
	for _, branch := range mcpServer.OneOf {
		if branch.Properties["type"].Const == "conversion-only" {
			conversionOnly = branch
			break
		}
	}
	require.NotNil(t, conversionOnly)
	assert.NotContains(t, conversionOnly.Properties, "access")
	assert.Equal(
		t,
		aiGatewayMCPServerConversionOnlyAccessMessage,
		conversionOnly.LoadRejectedFieldMessage("access"),
	)

	portalAuthSettings := schema.Defs[string(ResourceTypePortalAuthSettings)]
	require.NotNil(t, portalAuthSettings)
	assert.NotContains(t, portalAuthSettings.Properties, "oidc_auth_enabled")
	assert.Equal(
		t,
		"portal_auth_settings "+PortalAuthSettingsDeprecatedFieldMessage("oidc_auth_enabled"),
		portalAuthSettings.LoadRejectedFieldMessage("oidc_auth_enabled"),
	)
}

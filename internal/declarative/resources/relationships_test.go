package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRelationshipDescriptorsDistinguishFieldOrigins(t *testing.T) {
	t.Parallel()

	publication := RelationshipDescriptorsForType(ResourceTypeAPIPublication)
	require.Contains(t, publication, RelationshipDescriptor{
		FieldPath: "portal_id", TargetType: ResourceTypePortal, Kind: RelationshipKindAPIForeignKey,
		Cardinality: RelationshipCardinalityScalar, ResultField: RelationshipResultFieldID,
	})

	model := RelationshipDescriptorsForType(ResourceTypeAIGatewayModel)
	require.Contains(t, model, RelationshipDescriptor{
		FieldPath: SchemaFieldAIGateway, TargetType: ResourceTypeAIGateway,
		Kind: RelationshipKindKongctlParentSelector, Cardinality: RelationshipCardinalityScalar,
		ResultField: RelationshipResultFieldRef, RootOnly: true,
	})
	require.Contains(t, model, RelationshipDescriptor{
		FieldPath:  SchemaFieldAccess + "." + SchemaFieldAuthStrategies,
		TargetType: ResourceTypeAIGatewayAuthStrategy,
		Kind:       RelationshipKindAPIForeignKey, Cardinality: RelationshipCardinalityList,
		ResultField: RelationshipResultFieldID, ScopeFieldPath: SchemaFieldAIGateway,
	})

	agent := RelationshipDescriptorsForType(ResourceTypeAIGatewayAgent)
	require.Contains(t, agent, RelationshipDescriptor{
		FieldPath:  SchemaFieldAccess + "." + SchemaFieldAuthStrategies,
		TargetType: ResourceTypeAIGatewayAuthStrategy,
		Kind:       RelationshipKindAPIForeignKey, Cardinality: RelationshipCardinalityList,
		ResultField: RelationshipResultFieldID, ScopeFieldPath: SchemaFieldAIGateway,
	})

	mcpServer := RelationshipDescriptorsForType(ResourceTypeAIGatewayMCPServer)
	require.Contains(t, mcpServer, RelationshipDescriptor{
		FieldPath:  SchemaFieldAccess + "." + SchemaFieldAuthStrategies,
		TargetType: ResourceTypeAIGatewayAuthStrategy,
		Kind:       RelationshipKindAPIForeignKey, Cardinality: RelationshipCardinalityList,
		ResultField: RelationshipResultFieldID, ScopeFieldPath: SchemaFieldAIGateway,
	})
}

func TestExplainSchemaIncludesRelationshipContract(t *testing.T) {
	t.Parallel()

	subject, err := ResolveExplainSubject("api_publication.portal_id")
	require.NoError(t, err)
	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema.XRelationship)
	require.Equal(t, ResourceTypePortal, schema.XRelationship.Target)
	require.Equal(t, RelationshipKindAPIForeignKey, schema.XRelationship.Kind)
	require.Equal(t, []string{"!ref", "!external", "!lookup"}, schema.XRelationship.AcceptedTags)
	require.Equal(t, []string{"name"}, schema.XRelationship.Selectors)
}

func TestAIGatewayVaultExplainIncludesConfigStoreRelationship(t *testing.T) {
	t.Parallel()

	subject, err := ResolveExplainSubject("ai_gateway_vault")
	require.NoError(t, err)
	schema := RenderExplainSchema(subject)
	var configStore *JSONSchema
	for _, variant := range schema.OneOf {
		if variant.Properties["type"].Const == "konnect" {
			configStore = variant.Properties["config"].Properties[SchemaFieldConfigStoreID]
			break
		}
	}
	require.NotNil(t, configStore)
	require.Equal(t, string(ResourceTypeAIGatewayConfigStore), configStore.XRefKind)
	require.Equal(t, "!ref", configStore.XTag)
	require.Equal(t, ResourceTypeAIGatewayConfigStore, configStore.XRelationship.Target)
	require.Equal(t, RelationshipKindAPIForeignKey, configStore.XRelationship.Kind)
}

func TestRelationshipExplainNoteMatchesExternalCapability(t *testing.T) {
	t.Parallel()

	require.Contains(
		t,
		relationshipExplainNote(RelationshipKindAPIForeignKey, true),
		"use !lookup",
	)
	note := relationshipExplainNote(RelationshipKindAPIForeignKey, false)
	require.Contains(t, note, "use !ref")
	require.NotContains(t, note, "!lookup")
	require.NotContains(t, note, "!external")
}

func TestEveryRelationshipTargetDeclaresExternalCapabilityOrExclusion(t *testing.T) {
	t.Parallel()

	for _, targetType := range RelationshipTargetTypes() {
		_, supported := ExternalResolutionFor(targetType)
		_, excluded := ExternalUnsupportedReason(targetType)
		require.NotEqualf(
			t,
			supported,
			excluded,
			"relationship target %s must declare exactly one external-resolution disposition",
			targetType,
		)
	}
}

func TestRBACRelationshipExplainUsesDiscriminatorContract(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"organization.users.roles.entity_id",
		"portal.teams.roles.entity_id",
	} {
		subject, err := ResolveExplainSubject(path)
		require.NoError(t, err)
		schema := RenderExplainSchema(subject)
		require.NotNil(t, schema.XRelationship)
		require.Equal(t, "entity_type_name", schema.XRelationship.TargetDiscriminator)
		require.ElementsMatch(
			t,
			[]ResourceType{ResourceTypeAPI, ResourceTypePortal, ResourceTypeControlPlane},
			schema.XRelationship.Targets,
		)
		require.Equal(t, []string{"!ref", "!external", "!lookup"}, schema.XRelationship.AcceptedTags)
		require.Equal(t, []string{"name"}, schema.XRelationship.Selectors)
	}
}

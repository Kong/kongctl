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
	})

	model := RelationshipDescriptorsForType(ResourceTypeAIGatewayModel)
	require.Contains(t, model, RelationshipDescriptor{
		FieldPath: SchemaFieldAIGateway, TargetType: ResourceTypeAIGateway,
		Kind: RelationshipKindKongctlParentSelector, RootOnly: true,
	})
	require.Contains(t, model, RelationshipDescriptor{
		FieldPath:  SchemaFieldAccess + "." + SchemaFieldIdentityProviders,
		TargetType: ResourceTypeAIGatewayIdentityProvider,
		Kind:       RelationshipKindAPIForeignKey, ScopeFieldPath: SchemaFieldAIGateway,
	})

	agent := RelationshipDescriptorsForType(ResourceTypeAIGatewayAgent)
	require.Contains(t, agent, RelationshipDescriptor{
		FieldPath:  SchemaFieldAccess + "." + SchemaFieldIdentityProviders,
		TargetType: ResourceTypeAIGatewayIdentityProvider,
		Kind:       RelationshipKindAPIForeignKey, ScopeFieldPath: SchemaFieldAIGateway,
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

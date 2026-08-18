package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateSchemaRegistryComparesPublicVaultReferences(t *testing.T) {
	t.Parallel()

	const currentReference = "${vault['support-vault']['schema-registry-password']}"
	current := schemaRegistryStateWithPassword(currentReference)

	needsUpdate, fields, changes := (&Planner{}).shouldUpdateSchemaRegistry(
		current,
		schemaRegistryResourceWithPassword(currentReference),
	)
	require.False(t, needsUpdate)
	require.Empty(t, fields)
	require.Empty(t, changes)

	const desiredReference = "${vault['support-vault']['schema-registry-password-rotated']}"
	needsUpdate, fields, changes = (&Planner{}).shouldUpdateSchemaRegistry(
		current,
		schemaRegistryResourceWithPassword(desiredReference),
	)
	require.True(t, needsUpdate)
	require.Equal(t, FieldChange{Old: currentReference, New: desiredReference},
		changes["config.authentication.password"])

	config := fields[FieldConfig].(kkComps.SchemaRegistryConfluentConfig)
	require.Equal(t, desiredReference, config.Authentication.SchemaRegistryAuthenticationBasic.Password)
}

func TestShouldUpdateSchemaRegistryIgnoresSecretMaterial(t *testing.T) {
	t.Parallel()

	current := schemaRegistryStateWithPassword("")
	needsUpdate, fields, changes := (&Planner{}).shouldUpdateSchemaRegistry(
		current,
		schemaRegistryResourceWithPassword("secret-material"),
	)

	require.False(t, needsUpdate)
	require.Empty(t, fields)
	require.Empty(t, changes)
}

func schemaRegistryStateWithPassword(password string) state.EventGatewaySchemaRegistry {
	authentication := map[string]any{
		FieldType:  "basic",
		"username": "testuser",
	}
	if password != "" {
		authentication["password"] = password
	}

	return state.EventGatewaySchemaRegistry{
		SchemaRegistry: kkComps.SchemaRegistry{
			Name: "schema-registry",
			Type: "confluent",
		},
		RawConfig: map[string]any{
			"schema_type":       "json",
			"endpoint":          "https://schema-registry.example.com",
			FieldAuthentication: authentication,
		},
	}
}

func schemaRegistryResourceWithPassword(password string) resources.EventGatewaySchemaRegistryResource {
	authentication := kkComps.CreateSchemaRegistryAuthenticationSchemeBasic(
		kkComps.SchemaRegistryAuthenticationBasic{
			Username: "testuser",
			Password: password,
		},
	)

	return resources.EventGatewaySchemaRegistryResource{
		Ref: "schema-registry",
		SchemaRegistryCreate: kkComps.CreateSchemaRegistryCreateConfluent(
			kkComps.SchemaRegistryConfluent{
				Name: "schema-registry",
				Config: kkComps.SchemaRegistryConfluentConfig{
					SchemaType:     kkComps.SchemaTypeJSON,
					Endpoint:       "https://schema-registry.example.com",
					Authentication: &authentication,
				},
			},
		),
	}
}

package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVirtualClusterAuthentication_ClientCertificate(t *testing.T) {
	input := []any{
		map[string]any{
			"type": "client_certificate",
		},
	}

	result, err := buildVirtualClusterAuthentication(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, kkComps.VirtualClusterAuthenticationSchemeTypeClientCertificate, result[0].Type)
	assert.NotNil(t, result[0].VirtualClusterAuthenticationClientCertificate)
}

func TestBuildVirtualClusterAuthentication_UnsupportedType(t *testing.T) {
	input := []any{
		map[string]any{
			"type": "unknown_type",
		},
	}

	_, err := buildVirtualClusterAuthentication(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported authentication type")
}

func TestConvertToVirtualClusterSensitiveDataAwareAuth_ClientCertificate(t *testing.T) {
	auth := kkComps.CreateVirtualClusterAuthenticationSchemeClientCertificate(
		kkComps.VirtualClusterAuthenticationClientCertificate{},
	)

	result, err := convertToVirtualClusterSensitiveDataAwareAuth(auth)
	require.NoError(t, err)
	assert.Equal(t, kkComps.VirtualClusterAuthenticationSensitiveDataAwareSchemeTypeClientCertificate, result.Type)
	assert.NotNil(t, result.VirtualClusterAuthenticationClientCertificate)
}

func TestConvertToVirtualClusterSensitiveDataAwareAuth_ClientCertificate_MissingData(t *testing.T) {
	auth := kkComps.VirtualClusterAuthenticationScheme{
		Type: kkComps.VirtualClusterAuthenticationSchemeTypeClientCertificate,
		// VirtualClusterAuthenticationClientCertificate is intentionally nil
	}

	_, err := convertToVirtualClusterSensitiveDataAwareAuth(auth)
	require.Error(t, err)
	assert.Equal(t, "client certificate authentication data is missing", err.Error())
}

func TestEventGatewayVirtualClusterAdapterMapCreateFieldsTopicAliasesFromPlanJSON(t *testing.T) {
	fields := baseVirtualClusterCreateFields()
	fields[planner.FieldTopicAliases] = []any{
		map[string]any{
			"alias":     "public-orders",
			"topic":     "tenant-a.orders",
			"condition": "context.auth.type == 'anonymous'",
			"conflict":  "warn",
		},
	}

	var create kkComps.CreateVirtualClusterRequest
	err := (&EventGatewayVirtualClusterAdapter{}).MapCreateFields(context.Background(), nil, fields, &create)
	require.NoError(t, err)
	require.Len(t, create.TopicAliases, 1)

	alias := create.TopicAliases[0]
	assert.Equal(t, "public-orders", alias.Alias)
	assert.Equal(t, "tenant-a.orders", alias.Topic)
	require.NotNil(t, alias.Condition)
	assert.Equal(t, "context.auth.type == 'anonymous'", *alias.Condition)
	require.NotNil(t, alias.Conflict)
	assert.Equal(t, kkComps.VirtualClusterTopicAliasConflictWarn, *alias.Conflict)
}

func TestEventGatewayVirtualClusterAdapterMapUpdateFieldsTopicAliasesEmptyFromPlanJSON(t *testing.T) {
	var update kkComps.UpdateVirtualClusterRequest
	err := (&EventGatewayVirtualClusterAdapter{}).MapUpdateFields(
		context.Background(),
		nil,
		map[string]any{
			planner.FieldTopicAliases: []any{},
		},
		&update,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, update.TopicAliases)
	require.Empty(t, update.TopicAliases)
}

func TestBuildVirtualClusterTopicAliasesTyped(t *testing.T) {
	condition := "context.auth.type == 'anonymous'"
	typedAliases := []kkComps.VirtualClusterTopicAlias{{
		Alias:     "public-orders",
		Topic:     "tenant-a.orders",
		Condition: &condition,
	}}

	aliases, err := buildVirtualClusterTopicAliases(typedAliases)
	require.NoError(t, err)
	require.Equal(t, typedAliases, aliases)
}

func TestBuildVirtualClusterTopicAliasesTreatsEmptyConflictAsUnset(t *testing.T) {
	aliases, err := buildVirtualClusterTopicAliases([]any{
		map[string]any{
			"alias":    "public-orders",
			"topic":    "tenant-a.orders",
			"conflict": "",
		},
	})

	require.NoError(t, err)
	require.Len(t, aliases, 1)
	require.Nil(t, aliases[0].Conflict)
}

func TestBuildVirtualClusterTopicAliasesRejectsInvalidConflict(t *testing.T) {
	_, err := buildVirtualClusterTopicAliases([]any{
		map[string]any{
			"alias":    "public-orders",
			"topic":    "tenant-a.orders",
			"conflict": "fail",
		},
	})

	require.EqualError(t, err, "topic_aliases[0].conflict must be one of: warn, ignore")
}

func TestBuildVirtualClusterTopicAliasesValidatesTypedConflicts(t *testing.T) {
	emptyConflict := kkComps.VirtualClusterTopicAliasConflict("")
	aliases, err := buildVirtualClusterTopicAliases([]kkComps.VirtualClusterTopicAlias{{
		Alias:    "public-orders",
		Topic:    "tenant-a.orders",
		Conflict: &emptyConflict,
	}})

	require.NoError(t, err)
	require.Len(t, aliases, 1)
	require.Nil(t, aliases[0].Conflict)

	invalidConflict := kkComps.VirtualClusterTopicAliasConflict("fail")
	_, err = buildVirtualClusterTopicAliases([]kkComps.VirtualClusterTopicAlias{{
		Alias:    "public-orders",
		Topic:    "tenant-a.orders",
		Conflict: &invalidConflict,
	}})

	require.EqualError(t, err, "topic_aliases[0].conflict must be one of: warn, ignore")
}

func TestBuildVirtualClusterTopicAliasesRejectsInvalidOptionalFieldTypes(t *testing.T) {
	_, err := buildVirtualClusterTopicAliases([]any{
		map[string]any{
			"alias":     "public-orders",
			"topic":     "tenant-a.orders",
			"condition": 1,
		},
	})

	require.EqualError(t, err, "topic_aliases[0].condition must be a string")
}

func baseVirtualClusterCreateFields() map[string]any {
	return map[string]any{
		planner.FieldName: "virtual-cluster",
		planner.FieldDestination: map[string]any{
			"name": "backend-cluster",
		},
		planner.FieldAuthentication: []any{
			map[string]any{
				"type": "anonymous",
			},
		},
		planner.FieldACLMode:  "passthrough",
		planner.FieldDNSLabel: "vc-default",
	}
}

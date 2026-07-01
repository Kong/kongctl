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

func TestBuildBackendClusterReferencePrefersHydratedDependencyID(t *testing.T) {
	destination := kkComps.CreateBackendClusterReferenceModifyBackendClusterReferenceByName(
		kkComps.BackendClusterReferenceByName{Name: "backend-name"},
	)
	execCtx := &ExecutionContext{PlannedChange: &planner.PlannedChange{
		References: map[string]planner.ReferenceInfo{
			planner.FieldEventGatewayBackendClusterID: {
				Ref: "backend-ref",
				ID:  "backend-id",
			},
		},
	}}

	result, err := buildBackendClusterReference(destination, execCtx)
	require.NoError(t, err)
	require.NotNil(t, result.BackendClusterReferenceByID)
	assert.Equal(t, "backend-id", result.BackendClusterReferenceByID.ID)
	assert.Nil(t, result.BackendClusterReferenceByName)
}

func TestBuildBackendClusterReferenceKeepsNameWithoutPlannedDependency(t *testing.T) {
	destination := kkComps.CreateBackendClusterReferenceModifyBackendClusterReferenceByName(
		kkComps.BackendClusterReferenceByName{Name: "backend-name"},
	)

	result, err := buildBackendClusterReference(destination, nil)
	require.NoError(t, err)
	require.NotNil(t, result.BackendClusterReferenceByName)
	assert.Equal(t, "backend-name", result.BackendClusterReferenceByName.Name)
	assert.Nil(t, result.BackendClusterReferenceByID)
}

func TestBuildVirtualClusterAuthentication_FetchKongIdentityPrincipal(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
		fetch func(kkComps.VirtualClusterAuthenticationScheme) *kkComps.FetchKongIdentityPrincipal
	}{
		{
			name: "sasl_plain",
			input: map[string]any{
				"type":                          "sasl_plain",
				"mediation":                     "passthrough",
				"fetch_kong_identity_principal": fetchKongIdentityPrincipalMap(),
			},
			fetch: func(auth kkComps.VirtualClusterAuthenticationScheme) *kkComps.FetchKongIdentityPrincipal {
				return auth.VirtualClusterAuthenticationSaslPlain.FetchKongIdentityPrincipal
			},
		},
		{
			name: "sasl_scram",
			input: map[string]any{
				"type":                          "sasl_scram",
				"algorithm":                     "sha256",
				"fetch_kong_identity_principal": fetchKongIdentityPrincipalMap(),
			},
			fetch: func(auth kkComps.VirtualClusterAuthenticationScheme) *kkComps.FetchKongIdentityPrincipal {
				return auth.VirtualClusterAuthenticationSaslScram.FetchKongIdentityPrincipal
			},
		},
		{
			name: "client_certificate",
			input: map[string]any{
				"type":                          "client_certificate",
				"fetch_kong_identity_principal": fetchKongIdentityPrincipalMap(),
			},
			fetch: func(auth kkComps.VirtualClusterAuthenticationScheme) *kkComps.FetchKongIdentityPrincipal {
				return auth.VirtualClusterAuthenticationClientCertificate.FetchKongIdentityPrincipal
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildVirtualClusterAuthentication([]any{tt.input})
			require.NoError(t, err)
			require.Len(t, result, 1)

			fetch := tt.fetch(result[0])
			require.NotNil(t, fetch)
			assert.Equal(t, "identity-directory", fetch.Directory)
			assert.Equal(t, "principal-key", fetch.FetchBy.Key)
			assert.Equal(t, kkComps.FetchKongIdentityPrincipalFailureModeIgnore, fetch.FailureMode)
		})
	}
}

func TestBuildVirtualClusterAuthentication_FetchKongIdentityPrincipalOauthBearer(t *testing.T) {
	result, err := buildVirtualClusterAuthentication([]any{
		map[string]any{
			"type":      "oauth_bearer",
			"mediation": "validate_forward",
			"fetch_kong_identity_principal": map[string]any{
				"directory":    "identity-directory",
				"failure_mode": "error",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)

	fetch := result[0].VirtualClusterAuthenticationOauthBearer.FetchKongIdentityPrincipal
	require.NotNil(t, fetch)
	assert.Equal(t, "identity-directory", fetch.Directory)
	assert.Equal(t, kkComps.FetchKongIdentityPrincipalFailureModeError, fetch.FailureMode)
}

func TestBuildVirtualClusterAuthentication_FetchKongIdentityPrincipalRejectsOauthBearerFetchBy(t *testing.T) {
	_, err := buildVirtualClusterAuthentication([]any{
		map[string]any{
			"type":      "oauth_bearer",
			"mediation": "validate_forward",
			"fetch_kong_identity_principal": map[string]any{
				"directory":    "identity-directory",
				"fetch_by":     map[string]any{"key": "principal-key"},
				"failure_mode": "error",
			},
		},
	})

	require.EqualError(t, err,
		"authentication[0].fetch_kong_identity_principal.fetch_by is not supported for oauth_bearer")
}

func TestBuildVirtualClusterAuthentication_FetchKongIdentityPrincipalRejectsInvalidFailureMode(t *testing.T) {
	_, err := buildVirtualClusterAuthentication([]any{
		map[string]any{
			"type":      "sasl_scram",
			"algorithm": "sha256",
			"fetch_kong_identity_principal": map[string]any{
				"directory":    "identity-directory",
				"fetch_by":     map[string]any{"key": "principal-key"},
				"failure_mode": "skip",
			},
		},
	})

	require.EqualError(t, err,
		"authentication[0].fetch_kong_identity_principal.failure_mode must be one of: error, ignore")
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

func TestConvertToVirtualClusterSensitiveDataAwareAuth_PreservesFetchKongIdentityPrincipal(t *testing.T) {
	fetch := &kkComps.FetchKongIdentityPrincipal{
		Directory: "identity-directory",
		FetchBy: kkComps.FetchKongIdentityPrincipalFetchBy{
			Key: "principal-key",
		},
		FailureMode: kkComps.FetchKongIdentityPrincipalFailureModeIgnore,
	}

	tests := []struct {
		name  string
		auth  kkComps.VirtualClusterAuthenticationScheme
		fetch func(kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme) *kkComps.FetchKongIdentityPrincipal
	}{
		{
			name: "sasl_plain",
			auth: kkComps.CreateVirtualClusterAuthenticationSchemeSaslPlain(
				kkComps.VirtualClusterAuthenticationSaslPlain{
					Mediation:                  kkComps.VirtualClusterAuthenticationSaslPlainMediationPassthrough,
					FetchKongIdentityPrincipal: fetch,
				},
			),
			fetch: func(auth kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme) *kkComps.FetchKongIdentityPrincipal {
				return auth.VirtualClusterAuthenticationSaslPlainSensitiveDataAware.FetchKongIdentityPrincipal
			},
		},
		{
			name: "sasl_scram",
			auth: kkComps.CreateVirtualClusterAuthenticationSchemeSaslScram(
				kkComps.VirtualClusterAuthenticationSaslScram{
					Algorithm:                  kkComps.AlgorithmSha256,
					FetchKongIdentityPrincipal: fetch,
				},
			),
			fetch: func(auth kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme) *kkComps.FetchKongIdentityPrincipal {
				return auth.VirtualClusterAuthenticationSaslScram.FetchKongIdentityPrincipal
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToVirtualClusterSensitiveDataAwareAuth(tt.auth)
			require.NoError(t, err)

			require.Equal(t, fetch, tt.fetch(result))
		})
	}
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

func TestBuildVirtualClusterTopicAliasesTreatsEmptyConditionAsUnset(t *testing.T) {
	aliases, err := buildVirtualClusterTopicAliases([]any{
		map[string]any{
			"alias":     "public-orders",
			"topic":     "tenant-a.orders",
			"condition": "",
		},
	})

	require.NoError(t, err)
	require.Len(t, aliases, 1)
	require.Nil(t, aliases[0].Condition)
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

func fetchKongIdentityPrincipalMap() map[string]any {
	return map[string]any{
		"directory": "identity-directory",
		"fetch_by": map[string]any{
			"key": "principal-key",
		},
		"failure_mode": "ignore",
	}
}

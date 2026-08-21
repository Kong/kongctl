package executor

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSecretWritePreflightResolvesAllSourcesBeforeInjection(t *testing.T) {
	t.Setenv("FIRST_SECRET", "first")
	t.Setenv("SECOND_SECRET", "second")
	plan := secretExecutionPlan(
		secretExecutionIntent("/config/first", "FIRST_SECRET"),
		secretExecutionIntent("/config/second", "SECOND_SECRET"),
	)
	executor := &Executor{}

	require.NoError(t, executor.preflightSecretWrites(plan))
	change, err := cloneChangeForExecution(&plan.Changes[0])
	require.NoError(t, err)
	require.NoError(t, executor.injectResolvedSecretWrites(change))

	config := change.Fields[planner.FieldConfig].(map[string]any)
	assert.Equal(t, "first", config["first"])
	assert.Equal(t, "second", config["second"])
	assert.Empty(t, plan.Changes[0].Fields[planner.FieldConfig])
}

func TestSecretWritePreflightFailurePublishesNoPartialValues(t *testing.T) {
	t.Setenv("AVAILABLE_SECRET", "available")
	plan := secretExecutionPlan(
		secretExecutionIntent("/config/available", "AVAILABLE_SECRET"),
		secretExecutionIntent("/config/missing", "MISSING_SECRET"),
	)
	executor := &Executor{resolvedSecrets: map[string]map[string]string{"old": {"/value": "old"}}}

	err := executor.preflightSecretWrites(plan)
	require.ErrorContains(t, err, "MISSING_SECRET")
	assert.Empty(t, executor.resolvedSecrets)
}

func TestSecretWritePreflightRejectsInvalidTargetBeforeExecution(t *testing.T) {
	t.Setenv("SECRET", "value")
	plan := secretExecutionPlan(secretExecutionIntent("/config/headers/2/value", "SECRET"))
	plan.Changes[0].Fields[planner.FieldConfig] = map[string]any{
		"headers": []any{map[string]any{}},
	}

	err := (&Executor{}).preflightSecretWrites(plan)
	require.ErrorContains(t, err, "array index")
}

func TestSecretWritePreflightRejectsMissingArrayContainer(t *testing.T) {
	t.Setenv("SECRET", "value")
	plan := secretExecutionPlan(secretExecutionIntent("/config/client_secret/0", "SECRET"))

	err := (&Executor{}).preflightSecretWrites(plan)
	require.ErrorContains(t, err, "array container \"client_secret\" is missing")
}

func TestSecretWritePreflightPreservesAndPopulatesArrayShape(t *testing.T) {
	t.Setenv("FIRST_SECRET", "first")
	t.Setenv("SECOND_SECRET", "second")
	plan := secretExecutionPlan(
		secretExecutionIntent("/config/client_secret/0", "FIRST_SECRET"),
		secretExecutionIntent("/config/client_secret/1", "SECOND_SECRET"),
	)
	plan.Changes[0].Fields[planner.FieldConfig] = map[string]any{
		"client_id":     []any{"first-client", "second-client"},
		"client_secret": []any{nil, nil},
	}
	executor := &Executor{}

	require.NoError(t, executor.preflightSecretWrites(plan))
	change, err := cloneChangeForExecution(&plan.Changes[0])
	require.NoError(t, err)
	require.NoError(t, executor.injectResolvedSecretWrites(change))

	config := change.Fields[planner.FieldConfig].(map[string]any)
	assert.Equal(t, []any{"first-client", "second-client"}, config["client_id"])
	assert.Equal(t, []any{"first", "second"}, config["client_secret"])
	original := plan.Changes[0].Fields[planner.FieldConfig].(map[string]any)
	assert.Equal(t, []any{nil, nil}, original["client_secret"])
}

func TestAIGatewayIdentityProviderAdapterMapsInjectedClientSecrets(t *testing.T) {
	t.Setenv("FIRST_SECRET", "first")
	t.Setenv("SECOND_SECRET", "second")
	plan := secretExecutionPlan(
		secretExecutionIntent("/config/client_secret/0", "FIRST_SECRET"),
		secretExecutionIntent("/config/client_secret/1", "SECOND_SECRET"),
	)
	plan.Changes[0].Fields = map[string]any{
		planner.FieldName:        "support-oidc",
		planner.FieldType:        "openid-connect",
		planner.FieldDisplayName: "Support OIDC",
		planner.FieldConfig: map[string]any{
			"cache_tokens_salt": "support-cache-salt",
			"client_id":         []any{"first-client", "second-client"},
			"client_secret":     []any{nil, nil},
		},
	}
	executor := &Executor{}

	require.NoError(t, executor.preflightSecretWrites(plan))
	change, err := cloneChangeForExecution(&plan.Changes[0])
	require.NoError(t, err)
	require.NoError(t, executor.injectResolvedSecretWrites(change))

	var request kkComps.UpdateAIGatewayIdentityProviderRequest
	err = NewAIGatewayIdentityProviderAdapter(nil).MapUpdateFields(
		t.Context(), nil, change.Fields, &request, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect.Config)
	assert.Equal(
		t,
		[]string{"first", "second"},
		request.AIGatewayIdentityProviderOpenIDConnect.Config.ClientSecret,
	)
}

func TestAIGatewayIdentityProviderAdapterPreservesVaultReferenceBesideInjectedSecret(t *testing.T) {
	const reference = "{vault://support-secrets/primary-client-secret}"
	t.Setenv("FALLBACK_SECRET", "fallback")
	plan := secretExecutionPlan(secretExecutionIntent("/config/client_secret/1", "FALLBACK_SECRET"))
	plan.Changes[0].Fields = map[string]any{
		planner.FieldName:        "support-oidc",
		planner.FieldType:        "openid-connect",
		planner.FieldDisplayName: "Support OIDC",
		planner.FieldConfig: map[string]any{
			"cache_tokens_salt": "support-cache-salt",
			"client_id":         []any{"primary-client", "fallback-client"},
			"client_secret":     []any{reference, nil},
		},
	}
	executor := &Executor{}

	require.NoError(t, executor.preflightSecretWrites(plan))
	change, err := cloneChangeForExecution(&plan.Changes[0])
	require.NoError(t, err)
	require.NoError(t, executor.injectResolvedSecretWrites(change))

	var request kkComps.UpdateAIGatewayIdentityProviderRequest
	err = NewAIGatewayIdentityProviderAdapter(nil).MapUpdateFields(
		t.Context(), nil, change.Fields, &request, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect.Config)
	assert.Equal(
		t,
		[]string{reference, "fallback"},
		request.AIGatewayIdentityProviderOpenIDConnect.Config.ClientSecret,
	)
}

func TestResolvedSecretDoesNotReachPlanReporterOrExecutionResult(t *testing.T) {
	const sentinel = "resolved-secret-sentinel"
	t.Setenv("SECRET", sentinel)
	reporter := &MockProgressReporter{}
	reporter.On("StartExecution", mock.Anything).Return()
	reporter.On("StartChange", mock.Anything).Return()
	reporter.On("SkipChange", mock.Anything, "dry-run mode").Return()
	reporter.On("FinishExecution", mock.Anything).Return()
	plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "change-1",
		ResourceType: planner.ResourceTypeAIGatewayConsumerCredential,
		ResourceRef:  "consumer-key",
		Action:       planner.ActionCreate,
		Fields: map[string]any{
			planner.FieldName:        "consumer-key",
			planner.FieldDisplayName: "Consumer key",
			planner.FieldType:        "api-key",
		},
		SecretWrites: []planner.SecretWriteIntent{secretExecutionIntent("/api_key", "SECRET")},
	})
	plan.SetExecutionOrder([]string{"change-1"})

	result := New(nil, reporter, true).Execute(context.Background(), plan)

	require.Zero(t, result.FailureCount)
	require.Len(t, reporter.StartChangeCalls, 1)
	require.Len(t, reporter.SkipChangeCalls, 1)
	observable := struct {
		Plan         *planner.Plan
		StartChanges []planner.PlannedChange
		SkipChanges  []planner.PlannedChange
		Result       *ExecutionResult
	}{plan, reporter.StartChangeCalls, reporter.SkipChangeCalls, result}
	data, err := json.Marshal(observable)
	require.NoError(t, err)
	assert.NotContains(t, string(data), sentinel)
}

func TestPayloadValidationInjectsSecretPlaceholderWithoutResolvingSource(t *testing.T) {
	const resourceType = "secret-payload-test"
	plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "change-1",
		ResourceType: resourceType,
		ResourceRef:  "resource",
		Action:       planner.ActionCreate,
		Fields:       map[string]any{planner.FieldConfig: map[string]any{}},
		SecretWrites: []planner.SecretWriteIntent{
			secretExecutionIntent("/config/secret", "INTENTIONALLY_UNSET_SECRET"),
		},
	})

	contract := &secretPayloadValidationContract{resourceType: resourceType}
	executor := &Executor{
		payloadContracts: map[string]payloadContract{resourceType: contract},
	}

	require.NoError(t, executor.validatePlanPayloads(t.Context(), plan))
	require.Len(t, contract.changes, 1)
	config := contract.changes[0].Fields[planner.FieldConfig].(map[string]any)
	assert.Equal(t, "payload-validation-secret", config["secret"])
	assert.Empty(t, plan.Changes[0].Fields[planner.FieldConfig].(map[string]any))
}

func TestPayloadValidationNormalizesSchemaRegistrySDKConfigBeforeSecretInjection(t *testing.T) {
	authentication := kkComps.CreateSchemaRegistryAuthenticationSchemeBasic(
		kkComps.SchemaRegistryAuthenticationBasic{
			Username: "testuser",
			Password: "***",
		},
	)
	fields := map[string]any{
		planner.FieldName: "schema-registry",
		planner.FieldType: "confluent",
		planner.FieldConfig: kkComps.SchemaRegistryConfluentConfig{
			SchemaType:     kkComps.SchemaTypeJSON,
			Endpoint:       "https://schema-registry.example.com",
			Authentication: &authentication,
		},
	}
	contract := NewBaseExecutor[kkComps.SchemaRegistryCreate, kkComps.SchemaRegistryUpdate](
		NewEventGatewaySchemaRegistryAdapter(nil),
		nil,
		false,
	)

	for _, action := range []planner.ActionType{planner.ActionCreate, planner.ActionUpdate} {
		t.Run(string(action), func(t *testing.T) {
			plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
			plan.AddChange(planner.PlannedChange{
				ID:           "change-1",
				ResourceType: planner.ResourceTypeEventGatewaySchemaRegistry,
				ResourceRef:  "schema-registry",
				Action:       action,
				Fields:       fields,
				SecretWrites: []planner.SecretWriteIntent{
					secretExecutionIntent("/config/authentication/password", "INTENTIONALLY_UNSET_SECRET"),
				},
			})
			executor := &Executor{payloadContracts: map[string]payloadContract{
				planner.ResourceTypeEventGatewaySchemaRegistry: contract,
			}}

			require.NoError(t, executor.validatePlanPayloads(t.Context(), plan))
			assert.IsType(t, kkComps.SchemaRegistryConfluentConfig{}, plan.Changes[0].Fields[planner.FieldConfig])
		})
	}
}

func TestPayloadValidationAcceptsDCRProviderSecretOnlyUpdate(t *testing.T) {
	plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID:           "change-1",
		ResourceType: planner.ResourceTypeDCRProvider,
		ResourceRef:  "http-dcr",
		Action:       planner.ActionUpdate,
		Fields: map[string]any{
			planner.FieldName:                  "http-dcr",
			planner.FieldDCRProviderUpdateType: "http",
			planner.FieldDCRProviderConfig: map[string]any{
				"dcr_base_url": "https://dcr.example.test",
			},
		},
		SecretWrites: []planner.SecretWriteIntent{
			secretExecutionIntent("/dcr_config/api_key", "INTENTIONALLY_UNSET_SECRET"),
		},
	})
	contract := NewBaseExecutor[kkComps.CreateDcrProviderRequest, kkComps.UpdateDcrProviderRequest](
		NewDCRProviderAdapter(nil),
		nil,
		false,
	)
	executor := &Executor{payloadContracts: map[string]payloadContract{
		planner.ResourceTypeDCRProvider: contract,
	}}

	require.NoError(t, executor.validatePlanPayloads(t.Context(), plan))
	config := plan.Changes[0].Fields[planner.FieldDCRProviderConfig].(map[string]any)
	assert.NotContains(t, config, planner.FieldAPIKey)
}

func TestSecretWritePreflightRejectsEmptySourceAndComposesPrivately(t *testing.T) {
	t.Setenv("EMPTY_SECRET", "")
	plan := secretExecutionPlan(secretExecutionIntent("/config/value", "EMPTY_SECRET"))
	executor := &Executor{}
	require.ErrorContains(t, executor.preflightSecretWrites(plan), "empty value")

	t.Setenv("TOKEN", "opaque-token")
	literal := "Bearer "
	plan.Changes[0].SecretWrites[0].Expression.Parts = []tags.SecretPart{
		{Literal: &literal},
		{Source: &tags.SecretSource{Kind: "env", Reference: "TOKEN"}},
	}
	require.NoError(t, executor.preflightSecretWrites(plan))
	assert.Equal(t, "Bearer opaque-token", executor.resolvedSecrets["change-1"]["/config/value"])
	assert.NotContains(t, plan.Changes[0].Fields, "opaque-token")
}

func secretExecutionPlan(intents ...planner.SecretWriteIntent) *planner.Plan {
	return &planner.Plan{
		Changes: []planner.PlannedChange{{
			ID: "change-1", Fields: map[string]any{planner.FieldConfig: map[string]any{}}, SecretWrites: intents,
		}},
	}
}

func secretExecutionIntent(field, reference string) planner.SecretWriteIntent {
	return planner.SecretWriteIntent{
		Field: field,
		Expression: tags.SecretExpression{Parts: []tags.SecretPart{{Source: &tags.SecretSource{
			Kind: "env", Reference: reference,
		}}}},
	}
}

type secretPayloadValidationContract struct {
	resourceType string
	changes      []planner.PlannedChange
}

func (c *secretPayloadValidationContract) ResourceType() string {
	return c.resourceType
}

func (c *secretPayloadValidationContract) ValidatePayload(
	_ context.Context,
	change planner.PlannedChange,
) error {
	c.changes = append(c.changes, change)
	return nil
}

package planner

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySecretWriteIntentsCreatesSecretOnlyUpdate(t *testing.T) {
	resourceSet := dcrSecretResourceSet(t)
	plan := NewPlan("1.0", "test", PlanModeApply)
	planner := &Planner{}

	err := planner.applySecretWriteIntents(context.Background(), plan, resourceSet, Options{
		WriteSecretSelectors: []string{"dcr_provider:dcr#dcr_config.api_key"},
	})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "remote-dcr-id", change.ResourceID)
	require.Len(t, change.SecretWrites, 1)
	assert.Equal(t, "/dcr_config/api_key", change.SecretWrites[0].Field)
	config := change.Fields[FieldDCRProviderConfig].(map[string]any)
	assert.NotContains(t, config, FieldAPIKey)
	assert.Equal(t, 1, plan.Summary.SecretWrites)
}

func TestApplySecretWriteIntentsMergesIntoOrdinaryUpdate(t *testing.T) {
	resourceSet := dcrSecretResourceSet(t)
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID: "1:update:dcr_provider:dcr", ResourceType: string(resources.ResourceTypeDCRProvider),
		ResourceRef: "dcr", ResourceID: "remote-dcr-id", Action: ActionUpdate,
		Fields: map[string]any{FieldDisplayName: "Updated"}, Namespace: DefaultNamespace,
	})

	err := (&Planner{}).applySecretWriteIntents(context.Background(), plan, resourceSet, Options{WriteSecrets: true})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, "Updated", plan.Changes[0].Fields[FieldDisplayName])
	require.Len(t, plan.Changes[0].SecretWrites, 1)
}

func TestApplySecretWriteIntentsAutomaticallyIncludesCreateSecrets(t *testing.T) {
	resourceSet := dcrSecretResourceSet(t)
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID: "1:create:dcr_provider:dcr", ResourceType: string(resources.ResourceTypeDCRProvider),
		ResourceRef: "dcr", Action: ActionCreate, Fields: map[string]any{FieldName: "dcr"},
		Namespace: DefaultNamespace,
	})

	err := (&Planner{}).applySecretWriteIntents(context.Background(), plan, resourceSet, Options{})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Len(t, plan.Changes[0].SecretWrites, 1)
	assert.Equal(t, ActionCreate, plan.Changes[0].Action)
}

func TestApplySecretWriteIntentsWarnsForExistingCreateOnlyCredentialWithWriteSecrets(t *testing.T) {
	credential := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key"},
	}
	credential.SetKonnectID("remote-credential-id")
	resourceSet := &resources.ResourceSet{AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{
		credential,
	}}
	resourceSet.AddSecretSource("consumer-key", "/api_key", envSecretExpression("CONSUMER_KEY"), false)

	plan := NewPlan("1.0", "test", PlanModeApply)
	require.NoError(t, (&Planner{}).applySecretWriteIntents(
		context.Background(), plan, resourceSet, Options{WriteSecrets: true},
	))
	require.Len(t, plan.Warnings, 2)
	assert.Contains(t, plan.Warnings[0].Message, "--write-secrets skipped")
	assert.Contains(t, plan.Warnings[0].Message, "create-only")
	assert.Contains(t, plan.Warnings[1].Message, "did not select any writable secret fields")
	assert.Zero(t, plan.Summary.SecretWrites)
}

func TestApplySecretWriteIntentsReportsAllUnwritableSecrets(t *testing.T) {
	resourceSet := dcrSecretResourceSet(t)
	first := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key-a"},
	}
	first.SetKonnectID("remote-credential-a")
	second := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key-b"},
	}
	second.SetKonnectID("remote-credential-b")
	resourceSet.AIGatewayConsumerCredentials = []resources.AIGatewayConsumerCredentialResource{first, second}
	resourceSet.AddSecretSource("consumer-key-a", "/api_key", envSecretExpression("CONSUMER_KEY_A"), false)
	resourceSet.AddSecretSource("consumer-key-b", "/api_key", envSecretExpression("CONSUMER_KEY_B"), false)
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := (&Planner{}).applySecretWriteIntents(
		context.Background(),
		plan,
		resourceSet,
		Options{WriteSecrets: true},
	)
	require.NoError(t, err)
	require.Len(t, plan.Warnings, 2)
	assert.Contains(t, plan.Warnings[0].Message, "consumer-key-a")
	assert.Contains(t, plan.Warnings[1].Message, "consumer-key-b")
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, "dcr", plan.Changes[0].ResourceRef)
	assert.Equal(t, 1, plan.Summary.SecretWrites)
}

func TestApplySecretWriteIntentsAllowsWriteSecretsWithNoConfiguredSecrets(t *testing.T) {
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := (&Planner{}).applySecretWriteIntents(
		context.Background(),
		plan,
		&resources.ResourceSet{},
		Options{WriteSecrets: true},
	)
	require.NoError(t, err)
	assert.Empty(t, plan.Changes)
	assert.Zero(t, plan.Summary.SecretWrites)
	require.Len(t, plan.Warnings, 1)
	assert.Contains(t, plan.Warnings[0].Message, "did not select any writable secret fields")
}

func TestApplySecretWriteIntentsRejectsExplicitExistingCreateOnlyCredential(t *testing.T) {
	credential := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key"},
	}
	credential.SetKonnectID("remote-credential-id")
	resourceSet := &resources.ResourceSet{AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{
		credential,
	}}
	resourceSet.AddSecretSource("consumer-key", "/api_key", envSecretExpression("CONSUMER_KEY"), false)

	err := (&Planner{}).applySecretWriteIntents(
		context.Background(),
		NewPlan("1.0", "test", PlanModeApply),
		resourceSet,
		Options{WriteSecretSelectors: []string{"consumer-key#api_key"}},
	)
	require.ErrorContains(t, err, "create-only")
	require.ErrorContains(t, err, "declare a new resource")
}

func TestApplySecretWriteIntentsExplicitSelectorRemainsStrictWithWriteSecrets(t *testing.T) {
	credential := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key"},
	}
	credential.SetKonnectID("remote-credential-id")
	resourceSet := &resources.ResourceSet{AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{
		credential,
	}}
	resourceSet.AddSecretSource("consumer-key", "/api_key", envSecretExpression("CONSUMER_KEY"), false)

	err := (&Planner{}).applySecretWriteIntents(
		context.Background(),
		NewPlan("1.0", "test", PlanModeApply),
		resourceSet,
		Options{WriteSecrets: true, WriteSecretSelectors: []string{"consumer-key#api_key"}},
	)
	require.ErrorContains(t, err, "create-only")
}

func TestApplySecretWriteIntentsRejectsSelectingCreateOnlyFieldDuringReplacement(t *testing.T) {
	credential := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key"},
	}
	credential.SetKonnectID("remote-credential-id")
	resourceSet := &resources.ResourceSet{AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{
		credential,
	}}
	resourceSet.AddSecretSource("consumer-key", "/api_key", envSecretExpression("CONSUMER_KEY"), false)
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID:           "2:create:ai_gateway_consumer_credential:consumer-key",
		ResourceType: string(resources.ResourceTypeAIGatewayConsumerCredential), ResourceRef: "consumer-key",
		Action: ActionCreate, Fields: map[string]any{},
	})

	err := (&Planner{}).applySecretWriteIntents(context.Background(), plan, resourceSet,
		Options{WriteSecretSelectors: []string{"consumer-key#api_key"}})
	require.ErrorContains(t, err, "belongs to an existing resource")
}

func TestSecretWritePlanSerializationContainsMetadataButNoValue(t *testing.T) {
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID: "1:update:dcr_provider:dcr", ResourceType: string(resources.ResourceTypeDCRProvider),
		ResourceRef: "dcr", Action: ActionUpdate, Fields: map[string]any{},
		SecretWrites: []SecretWriteIntent{{
			Field: "/dcr_config/api_key", Expression: tags.SecretExpression{Parts: []tags.SecretPart{
				{Literal: new("Bearer ")},
				{Source: &tags.SecretSource{Kind: "env", Reference: "DCR_API_KEY"}},
			}},
		}},
	})

	data, err := json.Marshal(plan)
	require.NoError(t, err)
	assert.Contains(t, string(data), "DCR_API_KEY")
	assert.Contains(t, string(data), "Bearer ")
	assert.NotContains(t, string(data), "resolved-secret-value")

	var roundTrip Plan
	require.NoError(t, json.Unmarshal(data, &roundTrip))
	assert.Equal(t, plan.Changes[0].SecretWrites, roundTrip.Changes[0].SecretWrites)
}

func TestStripDeclaredSecretPathsRemovesArrayElementsWithoutNullPlaceholders(t *testing.T) {
	fieldsConfig := map[string]any{
		"issuer":        "https://issuer.example.test",
		"client_secret": []any{"__SECRET__:first", "__SECRET__:second"},
	}
	changedConfig := map[string]any{
		"issuer":        "https://issuer.example.test",
		"client_secret": []any{"__SECRET__:first", "__SECRET__:second"},
	}
	change := PlannedChange{
		Fields: map[string]any{"config": fieldsConfig},
		ChangedFields: map[string]FieldChange{
			"config": {New: changedConfig},
		},
	}
	declarations := map[string]resources.SecretSourceDeclaration{
		"/config/client_secret/0": {},
		"/config/client_secret/1": {},
	}

	stripDeclaredSecretPaths(&change, declarations)

	assert.NotContains(t, fieldsConfig, "client_secret")
	assert.NotContains(t, changedConfig, "client_secret")
	data, err := json.Marshal(change)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"client_secret":[null]`)
	assert.NotContains(t, string(data), "__SECRET__:")
}

func TestAIGatewayConsumerCredentialAPIKeyIsExcludedFromComparison(t *testing.T) {
	ttl := int64(0)
	apiKey := "configured-key"
	current := state.AIGatewayConsumerCredential{AIGatewayConsumerCredential: kkComps.AIGatewayConsumerCredential{
		Name: "consumer-key", DisplayName: "Consumer Key", Type: kkComps.AIGatewayConsumerCredentialTypeAPIKey,
		TTL: &ttl,
	}}
	desired := resources.AIGatewayConsumerCredentialResource{
		BaseResource: resources.BaseResource{Ref: "consumer-key"},
		CreateAIGatewayConsumerCredentialRequest: kkComps.CreateAIGatewayConsumerCredentialRequest{
			Name: "consumer-key", DisplayName: "Consumer Key",
			Type: kkComps.CreateAIGatewayConsumerCredentialRequestTypeAPIKey, TTL: &ttl, APIKey: &apiKey,
		},
	}

	replace, changed, err := shouldReplaceAIGatewayConsumerCredential(current, desired)
	require.NoError(t, err)
	assert.False(t, replace)
	assert.Empty(t, changed)
}

func dcrSecretResourceSet(t *testing.T) *resources.ResourceSet {
	t.Helper()
	placeholder, err := tags.BuildSecretPlaceholder(envSecretExpression("DCR_API_KEY"))
	require.NoError(t, err)
	dcr := resources.DCRProviderResource{
		BaseResource: resources.BaseResource{Ref: "dcr"}, Name: "dcr", ProviderType: "http",
		Issuer: "https://issuer.example.test", DCRConfig: map[string]any{
			"dcr_base_url": "https://dcr.example.test", FieldAPIKey: placeholder,
		},
	}
	dcr.SetKonnectID("remote-dcr-id")
	rs := &resources.ResourceSet{DCRProviders: []resources.DCRProviderResource{dcr}}
	rs.AddSecretSource("dcr", "/dcr_config/api_key", envSecretExpression("DCR_API_KEY"), false)
	return rs
}

func envSecretExpression(reference string) tags.SecretExpression {
	return tags.SecretExpression{Parts: []tags.SecretPart{{Source: &tags.SecretSource{
		Kind: "env", Reference: reference,
	}}}}
}

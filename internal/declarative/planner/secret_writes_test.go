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
	assert.Equal(t, "http", change.Fields[FieldDCRProviderUpdateType])
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
	config := plan.Changes[0].Fields[FieldDCRProviderConfig].(map[string]any)
	assert.NotContains(t, config, FieldAPIKey)
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
	assert.NotContains(t, plan.Changes[0].Fields, FieldDCRProviderUpdateType)
}

func TestApplySecretWriteIntentsKeepsConsumerCredentialParentOutOfCreateFields(t *testing.T) {
	placeholder, err := tags.BuildSecretPlaceholder(envSecretExpression("CONSUMER_API_KEY"))
	require.NoError(t, err)
	ttl := int64(0)
	credential := resources.AIGatewayConsumerCredentialResource{
		BaseResource:      resources.BaseResource{Ref: "consumer-key"},
		AIGatewayConsumer: "consumer",
		CreateAIGatewayConsumerCredentialRequest: kkComps.CreateAIGatewayConsumerCredentialRequest{
			Name:        "consumer-key",
			DisplayName: "Consumer key",
			Type:        kkComps.CreateAIGatewayConsumerCredentialRequestTypeAPIKey,
			TTL:         &ttl,
			APIKey:      &placeholder,
		},
	}
	resourceSet := &resources.ResourceSet{
		AIGatewayConsumerCredentials: []resources.AIGatewayConsumerCredentialResource{credential},
	}
	resourceSet.AddSecretSource("consumer-key", "/api_key", envSecretExpression("CONSUMER_API_KEY"), false)
	fields, err := credential.MutablePayloadMap()
	require.NoError(t, err)
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ID:           "1:create:ai_gateway_consumer_credential:consumer-key",
		ResourceType: string(resources.ResourceTypeAIGatewayConsumerCredential),
		ResourceRef:  "consumer-key",
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    DefaultNamespace,
	})

	require.NoError(t, (&Planner{}).applySecretWriteIntents(context.Background(), plan, resourceSet, Options{}))
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	assert.NotContains(t, change.Fields, resources.SchemaFieldAIGatewayConsumer)
	assert.Equal(t, "consumer-key", change.Fields[FieldName])
	assert.Equal(t, "Consumer key", change.Fields[FieldDisplayName])
	assert.NotContains(t, change.Fields, FieldAPIKey)
	require.Len(t, change.SecretWrites, 1)
	assert.Equal(t, "/api_key", change.SecretWrites[0].Field)
}

func TestSecretResourceFieldsUsesMutablePayloadContract(t *testing.T) {
	provider := resources.AIGatewayProviderResource{
		BaseResource: resources.BaseResource{Ref: "provider"},
		AIGateway:    "gateway",
		Name:         "provider",
		Type:         "openai",
		DisplayName:  "Provider",
		Config:       map[string]any{"auth": map[string]any{"header": "placeholder"}},
	}

	fields, err := secretResourceFields(&provider, ActionCreate)
	require.NoError(t, err)
	assert.Equal(t, "provider", fields[FieldName])
	assert.Contains(t, fields, FieldConfig)
	assert.NotContains(t, fields, resources.SchemaFieldRef)
	assert.NotContains(t, fields, resources.SchemaFieldAIGateway)
}

func TestSecretResourceFieldsUsesPortalIdentityProviderUpdateContract(t *testing.T) {
	providerType := kkComps.IdentityProviderTypeOidc
	provider := resources.PortalIdentityProviderResource{
		CreateIdentityProvider: kkComps.CreateIdentityProvider{Type: &providerType},
		Ref:                    "identity-provider",
		Portal:                 "portal",
	}

	fields, err := secretResourceFields(&provider, ActionUpdate)
	require.NoError(t, err)
	assert.NotContains(t, fields, FieldType)
	assert.NotContains(t, fields, resources.SchemaFieldPortal)
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

func TestPrepareDeclaredSecretPathsRemovesUnselectedArrayWithoutNullPlaceholders(t *testing.T) {
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

	require.NoError(t, prepareDeclaredSecretPaths(&change, declarations, nil))

	assert.NotContains(t, fieldsConfig, "client_secret")
	assert.NotContains(t, changedConfig, "client_secret")
	data, err := json.Marshal(change)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"client_secret":[null]`)
	assert.NotContains(t, string(data), "__SECRET__:")
}

func TestPrepareDeclaredSecretPathsPreservesSelectedArrayShape(t *testing.T) {
	fieldsConfig := map[string]any{
		"client_id":     []any{"first-client", "second-client"},
		"client_secret": []any{"__SECRET__:first", "__SECRET__:second"},
	}
	change := PlannedChange{Fields: map[string]any{"config": fieldsConfig}}
	declarations := map[string]resources.SecretSourceDeclaration{
		"/config/client_secret/0": {},
		"/config/client_secret/1": {},
	}
	selected := []SecretWriteIntent{
		{Field: "/config/client_secret/0"},
		{Field: "/config/client_secret/1"},
	}

	require.NoError(t, prepareDeclaredSecretPaths(&change, declarations, selected))
	assert.Equal(t, []any{"first-client", "second-client"}, fieldsConfig["client_id"])
	assert.Equal(t, []any{nil, nil}, fieldsConfig["client_secret"])
}

func TestPrepareDeclaredSecretPathsPreservesVaultReferenceSibling(t *testing.T) {
	const reference = "{vault://support-secrets/primary-client-secret}"
	fieldsConfig := map[string]any{
		"client_id":     []any{"primary-client", "fallback-client"},
		"client_secret": []any{reference, "__SECRET__:fallback"},
	}
	change := PlannedChange{Fields: map[string]any{"config": fieldsConfig}}
	declarations := map[string]resources.SecretSourceDeclaration{
		"/config/client_secret/1": {},
	}

	require.NoError(t, prepareDeclaredSecretPaths(&change, declarations, []SecretWriteIntent{{
		Field: "/config/client_secret/1",
	}}))
	assert.Equal(t, []any{"primary-client", "fallback-client"}, fieldsConfig["client_id"])
	assert.Equal(t, []any{reference, nil}, fieldsConfig["client_secret"])
}

func TestPrepareDeclaredSecretPathsRejectsPartialArraySelection(t *testing.T) {
	change := PlannedChange{Fields: map[string]any{"config": map[string]any{
		"client_secret": []any{"__SECRET__:first", "__SECRET__:second"},
	}}}
	declarations := map[string]resources.SecretSourceDeclaration{
		"/config/client_secret/0": {},
		"/config/client_secret/1": {},
	}

	err := prepareDeclaredSecretPaths(&change, declarations, []SecretWriteIntent{{
		Field: "/config/client_secret/0",
	}})
	require.ErrorContains(t, err, "must be written as a complete group")
}

func TestApplySecretWriteIntentsOrdersSecretOnlyChangesDeterministically(t *testing.T) {
	first := dcrSecretResourceSet(t).DCRProviders[0]
	first.Ref = "z-provider"
	first.Name = "z-provider"
	second := dcrSecretResourceSet(t).DCRProviders[0]
	second.Ref = "a-provider"
	second.Name = "a-provider"
	resourceSet := &resources.ResourceSet{DCRProviders: []resources.DCRProviderResource{first, second}}
	resourceSet.AddSecretSource("z-provider", "/dcr_config/api_key", envSecretExpression("Z_API_KEY"), false)
	resourceSet.AddSecretSource("a-provider", "/dcr_config/api_key", envSecretExpression("A_API_KEY"), false)

	for range 20 {
		plan := NewPlan("1.0", "test", PlanModeApply)
		err := (&Planner{}).applySecretWriteIntents(
			context.Background(), plan, resourceSet, Options{WriteSecrets: true},
		)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 2)
		assert.Equal(t, "a-provider", plan.Changes[0].ResourceRef)
		assert.Equal(t, "temp-1:u:dcr_provider:a-provider", plan.Changes[0].ID)
		assert.Equal(t, "z-provider", plan.Changes[1].ResourceRef)
		assert.Equal(t, "temp-2:u:dcr_provider:z-provider", plan.Changes[1].ID)
	}
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

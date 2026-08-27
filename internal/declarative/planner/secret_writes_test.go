package planner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/secrets"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAIGatewaySecretRef = "gateway"

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
	assert.NotContains(t, change.Fields, FieldDCRProviderProviderType)
	assert.Equal(t, 1, plan.Summary.SecretWrites)
}

func TestAIGatewayWriteOnlyFieldsPlanSecretOnlyUpdates(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		resourceType resources.ResourceType
		resourceRef  string
		field        string
		selector     string
		resourceSet  func(*testing.T, string) *resources.ResourceSet
	}

	provider := func(config func(string) map[string]any) func(*testing.T, string) *resources.ResourceSet {
		return func(t *testing.T, placeholder string) *resources.ResourceSet {
			t.Helper()
			resource := resources.AIGatewayProviderResource{
				BaseResource: resources.BaseResource{Ref: "provider"},
				AIGateway:    testAIGatewaySecretRef,
				Name:         "provider",
				Type:         "openai",
				DisplayName:  "Provider",
				Config:       config(placeholder),
			}
			resource.SetKonnectID("remote-id")
			return aiGatewaySecretResourceSet(&resource)
		}
	}
	authStrategy := func(config func(string) map[string]any) func(*testing.T, string) *resources.ResourceSet {
		return func(t *testing.T, placeholder string) *resources.ResourceSet {
			t.Helper()
			resource := resources.AIGatewayAuthStrategyResource{
				BaseResource: resources.BaseResource{Ref: "auth-strategy"},
				AIGateway:    testAIGatewaySecretRef,
				Name:         "auth-strategy",
				Type:         "openid-connect",
				DisplayName:  "Auth Strategy",
				Config:       config(placeholder),
			}
			resource.SetKonnectID("remote-id")
			return aiGatewaySecretResourceSet(&resource)
		}
	}
	vault := func(config func(string) map[string]any, vaultType string) func(*testing.T, string) *resources.ResourceSet {
		return func(t *testing.T, placeholder string) *resources.ResourceSet {
			t.Helper()
			payload := map[string]any{
				"ref":        "vault",
				"ai_gateway": testAIGatewaySecretRef,
				"type":       vaultType,
				"name":       "vault",
				"config":     config(placeholder),
			}
			data, err := json.Marshal(payload)
			require.NoError(t, err)
			var resource resources.AIGatewayVaultResource
			require.NoError(t, json.Unmarshal(data, &resource))
			resource.SetKonnectID("remote-id")
			return aiGatewaySecretResourceSet(&resource)
		}
	}

	cases := []testCase{
		{
			name: "provider header value", resourceType: resources.ResourceTypeAIGatewayProvider,
			resourceRef: "provider", field: "/config/auth/headers/0/value",
			selector: "config.auth.headers[].value",
			resourceSet: provider(func(secret string) map[string]any {
				return map[string]any{"auth": map[string]any{
					"type": "basic", "headers": []any{map[string]any{"name": "Authorization", "value": secret}},
				}}
			}),
		},
		{
			name: "provider client secret", resourceType: resources.ResourceTypeAIGatewayProvider,
			resourceRef: "provider", field: "/config/auth/client_secret", selector: "config.auth.client_secret",
			resourceSet: provider(func(secret string) map[string]any {
				return map[string]any{"auth": map[string]any{"type": "oauth2", FieldClientSecret: secret}}
			}),
		},
		{
			name: "provider AWS secret access key", resourceType: resources.ResourceTypeAIGatewayProvider,
			resourceRef: "provider", field: "/config/auth/secret_access_key",
			selector: "config.auth.secret_access_key",
			resourceSet: provider(func(secret string) map[string]any {
				return map[string]any{"auth": map[string]any{"type": "aws", "secret_access_key": secret}}
			}),
		},
		{
			name: "provider nested AWS secret access key", resourceType: resources.ResourceTypeAIGatewayProvider,
			resourceRef: "provider", field: "/config/auth/aws/secret_access_key",
			selector: "config.auth.aws.secret_access_key",
			resourceSet: provider(func(secret string) map[string]any {
				return map[string]any{"auth": map[string]any{
					"type": "aws", "aws": map[string]any{"secret_access_key": secret},
				}}
			}),
		},
		{
			name: "provider GCP service account", resourceType: resources.ResourceTypeAIGatewayProvider,
			resourceRef: "provider", field: "/config/auth/service_account_json",
			selector: "config.auth.service_account_json",
			resourceSet: provider(func(secret string) map[string]any {
				return map[string]any{"auth": map[string]any{"type": "gcp", "service_account_json": secret}}
			}),
		},
		{
			name: "auth strategy client secret array", resourceType: resources.ResourceTypeAIGatewayAuthStrategy,
			resourceRef: "auth-strategy", field: "/config/client_secret/0", selector: "config.client_secret[]",
			resourceSet: authStrategy(func(secret string) map[string]any {
				return map[string]any{"issuer": "https://issuer.example.test", FieldClientSecret: []any{secret}}
			}),
		},
		{
			name: "auth strategy scalar client secret", resourceType: resources.ResourceTypeAIGatewayAuthStrategy,
			resourceRef: "auth-strategy", field: "/config/client_secret", selector: "config.client_secret",
			resourceSet: authStrategy(func(secret string) map[string]any {
				return map[string]any{"issuer": "https://issuer.example.test", FieldClientSecret: secret}
			}),
		},
		{
			name: "Conjur vault API key", resourceType: resources.ResourceTypeAIGatewayVault,
			resourceRef: "vault", field: "/config/api_key", selector: "config.api_key",
			resourceSet: vault(func(secret string) map[string]any {
				return map[string]any{
					"account": "account", "endpoint_url": "https://conjur.example.test", "login": "login",
					"api_key": secret,
				}
			}, "conjur"),
		},
		{
			name: "HashiCorp vault token", resourceType: resources.ResourceTypeAIGatewayVault,
			resourceRef: "vault", field: "/config/token", selector: "config.token",
			resourceSet: vault(func(secret string) map[string]any {
				return map[string]any{
					"auth_method": "token", "host": "vault.example.test", "port": 8200, "token": secret,
				}
			}, "hcv"),
		},
		{
			name: "HashiCorp vault OAuth client secret", resourceType: resources.ResourceTypeAIGatewayVault,
			resourceRef: "vault", field: "/config/client_secret", selector: "config.client_secret",
			resourceSet: vault(func(secret string) map[string]any {
				config := map[string]any{ //nolint:gosec // Non-secret OAuth test fixture values.
					"auth_method": "jwt", "host": "vault.example.test", "port": 8200, "role": "role",
					"token_endpoint": "https://issuer.example.test/token", "client_id": "client",
				}
				config[FieldClientSecret] = secret
				return config
			}, "hcv"),
		},
		{
			name: "HashiCorp vault AWS secret access key", resourceType: resources.ResourceTypeAIGatewayVault,
			resourceRef: "vault", field: "/config/secret_access_key", selector: "config.secret_access_key",
			resourceSet: vault(func(secret string) map[string]any {
				return map[string]any{
					"auth_method": "aws_iam", "host": "vault.example.test", "port": 8200, "role": "role",
					"region": "us-east-1", "access_key_id": "access-key", "secret_access_key": secret,
				}
			}, "hcv"),
		},
		{
			name: "HashiCorp vault AppRole secret ID", resourceType: resources.ResourceTypeAIGatewayVault,
			resourceRef: "vault", field: "/config/secret_id", selector: "config.secret_id",
			resourceSet: vault(func(secret string) map[string]any {
				return map[string]any{
					"auth_method": "approle", "host": "vault.example.test", "port": 8200,
					"role_id": "role", "secret_id": secret,
				}
			}, "hcv"),
		},
		{
			name: "config store secret value", resourceType: resources.ResourceTypeAIGatewayConfigStoreSecret,
			resourceRef: "config-store-secret", field: "/value", selector: "value",
			resourceSet: func(t *testing.T, placeholder string) *resources.ResourceSet {
				t.Helper()
				secret := resources.AIGatewayConfigStoreSecretResource{
					BaseResource:         resources.BaseResource{Ref: "config-store-secret"},
					AIGatewayConfigStore: "config-store", Key: "secret", Value: placeholder,
				}
				secret.SetKonnectID("remote-id")
				store := resources.AIGatewayConfigStoreResource{
					BaseResource: resources.BaseResource{Ref: "config-store"},
					AIGateway:    testAIGatewaySecretRef,
					Name:         "store",
				}
				store.SetKonnectID("store-id")
				rs := aiGatewaySecretResourceSet(&secret)
				rs.AIGatewayConfigStores = []resources.AIGatewayConfigStoreResource{store}
				return rs
			},
		},
	}
	coveredCapabilities := make(map[string]bool, len(cases))
	for _, tc := range cases {
		capability, ok := secrets.Match(tc.resourceType, tc.field)
		require.True(t, ok, "%s is not in the reviewed secret catalog", tc.field)
		coveredCapabilities[string(capability.ResourceType)+":"+capability.PathPattern] = true
	}
	expectedCapabilities := make(map[string]bool)
	for _, capability := range secrets.Capabilities() {
		if capability.Update && strings.HasPrefix(string(capability.ResourceType), "ai_gateway") {
			expectedCapabilities[string(capability.ResourceType)+":"+capability.PathPattern] = true
		}
	}
	require.Equal(t, expectedCapabilities, coveredCapabilities)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, mode := range []struct {
				name    string
				options Options
			}{
				{name: "without write flag", options: Options{Mode: PlanModeApply}},
				{name: "with --write-secret", options: Options{
					Mode: PlanModeApply, WriteSecretSelectors: []string{tc.resourceRef + "#" + tc.selector},
				}},
				{name: "with --write-secrets", options: Options{Mode: PlanModeApply, WriteSecrets: true}},
			} {
				t.Run(mode.name, func(t *testing.T) {
					t.Parallel()
					expression := envSecretExpression("AI_GATEWAY_SECRET")
					placeholder, err := tags.BuildSecretPlaceholder(expression)
					require.NoError(t, err)
					rs := tc.resourceSet(t, placeholder)
					rs.AddSecretSource(tc.resourceRef, tc.field, expression, false)
					plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)

					err = (&Planner{}).applySecretWriteIntents(t.Context(), plan, rs, mode.options)
					require.NoError(t, err)
					if mode.name == "without write flag" {
						require.Empty(t, plan.Changes)
						return
					}

					require.Len(t, plan.Changes, 1)
					change := plan.Changes[0]
					require.Equal(t, ActionUpdate, change.Action)
					require.Equal(t, string(tc.resourceType), change.ResourceType)
					require.Equal(t, tc.resourceRef, change.ResourceRef)
					require.Equal(t, "remote-id", change.ResourceID)
					require.Len(t, change.SecretWrites, 1)
					require.Equal(t, tc.field, change.SecretWrites[0].Field)
					require.Equal(t, 1, plan.Summary.SecretWrites)
					data, err := json.Marshal(plan)
					require.NoError(t, err)
					require.NotContains(t, string(data), placeholder)
				})
			}
		})
	}
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
	assert.Equal(t, "http", plan.Changes[0].Fields[FieldDCRProviderProviderType])
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

func TestResolveSecretResourceIDLooksUpAIGatewayConfigStoreSecretByKey(t *testing.T) {
	api := &testAIGatewayConfigStoreAPI{
		getSecret: &kkComps.AIGatewayConfigStoreSecret{Key: "openai-auth-header"},
	}
	client := state.NewClient(state.ClientConfig{AIGatewayConfigStoresAPI: api})
	gateway := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: "support-gateway"}}
	gateway.SetKonnectID("gateway-id")
	store := resources.AIGatewayConfigStoreResource{
		BaseResource: resources.BaseResource{Ref: "support-store"},
		AIGateway:    "support-gateway",
		Name:         "support-store",
	}
	store.SetKonnectID("store-id")
	rs := &resources.ResourceSet{
		AIGateways:            []resources.AIGatewayResource{gateway},
		AIGatewayConfigStores: []resources.AIGatewayConfigStoreResource{store},
		AIGatewayConfigStoreSecrets: []resources.AIGatewayConfigStoreSecretResource{{
			BaseResource:         resources.BaseResource{Ref: "support-openai-header"},
			AIGatewayConfigStore: "support-store",
			Key:                  "openai-auth-header",
		}},
	}

	id, err := (&Planner{client: client}).resolveSecretResourceID(
		t.Context(),
		rs,
		&rs.AIGatewayConfigStoreSecrets[0],
	)
	require.NoError(t, err)
	assert.Equal(t, "openai-auth-header", id)
	require.Len(t, api.getSecretRequests, 1)
	assert.Equal(t, "gateway-id", api.getSecretRequests[0].GatewayID)
	assert.Equal(t, "store-id", api.getSecretRequests[0].ConfigStoreIDOrName)
	assert.Equal(t, "openai-auth-header", api.getSecretRequests[0].Key)
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

func TestSecretResourceFieldsUsesDCRProviderUpdateContract(t *testing.T) {
	provider := resources.DCRProviderResource{
		BaseResource: resources.BaseResource{Ref: "http-dcr"},
		Name:         "http-dcr",
		ProviderType: "http",
		Issuer:       "https://issuer.example.test",
		DCRConfig:    map[string]any{"dcr_base_url": "https://dcr.example.test"},
	}

	fields, err := secretResourceFields(&provider, ActionUpdate)
	require.NoError(t, err)
	assert.Equal(t, "http", fields[FieldDCRProviderUpdateType])
	assert.NotContains(t, fields, FieldDCRProviderProviderType)
	assert.Contains(t, fields, FieldDCRProviderConfig)
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

func aiGatewaySecretResourceSet(resource resources.Resource) *resources.ResourceSet {
	gateway := resources.AIGatewayResource{BaseResource: resources.BaseResource{Ref: testAIGatewaySecretRef}}
	gateway.SetKonnectID("gateway-id")
	rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{gateway}}
	switch typed := resource.(type) {
	case *resources.AIGatewayProviderResource:
		rs.AIGatewayProviders = []resources.AIGatewayProviderResource{*typed}
	case *resources.AIGatewayAuthStrategyResource:
		rs.AIGatewayAuthStrategies = []resources.AIGatewayAuthStrategyResource{*typed}
	case *resources.AIGatewayVaultResource:
		rs.AIGatewayVaults = []resources.AIGatewayVaultResource{*typed}
	case *resources.AIGatewayConfigStoreSecretResource:
		rs.AIGatewayConfigStoreSecrets = []resources.AIGatewayConfigStoreSecretResource{*typed}
	}
	return rs
}

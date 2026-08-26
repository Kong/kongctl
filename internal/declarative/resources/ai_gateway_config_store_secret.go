package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayConfigStoreSecretFieldKey   = "key"
	aiGatewayConfigStoreSecretFieldValue = "value"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayConfigStoreSecret,
		func(rs *ResourceSet) *[]AIGatewayConfigStoreSecretResource {
			return &rs.AIGatewayConfigStoreSecrets
		},
		AutoExplain[AIGatewayConfigStoreSecretResource](
			WithExplainAliases(
				"ai_gateway_config_store_secrets",
				"ai-gateway-config-store-secret",
				"ai-gateway-config-store-secrets",
				"ai_gateway.config_stores.secrets",
				"aigw-config-store-secret",
			),
			WithExplainRecommendedFields(
				SchemaFieldRef,
				SchemaFieldAIGatewayConfigStore,
				aiGatewayConfigStoreSecretFieldKey,
				aiGatewayConfigStoreSecretFieldValue,
			),
			WithExplainSchemaBuilder(aiGatewayConfigStoreSecretExplainNode),
		),
		WithMaturity(aiGatewayMaturity),
	)
}

// AIGatewayConfigStoreSecretResource represents a write-only secret in an AI Gateway Config Store.
type AIGatewayConfigStoreSecretResource struct {
	BaseResource         `yaml:",inline" json:",inline"`
	AIGatewayConfigStore string `yaml:"ai_gateway_config_store,omitempty" json:"ai_gateway_config_store,omitempty"`
	Key                  string `yaml:"key" json:"key"`
	Value                string `yaml:"value,omitempty" json:"value,omitempty"`
}

func (a AIGatewayConfigStoreSecretResource) GetType() ResourceType {
	return ResourceTypeAIGatewayConfigStoreSecret
}

func (a AIGatewayConfigStoreSecretResource) GetMoniker() string { return a.Key }

func (a AIGatewayConfigStoreSecretResource) GetDependencies() []ResourceRef {
	if a.AIGatewayConfigStore == "" {
		return nil
	}
	return []ResourceRef{{
		Kind: ResourceTypeAIGatewayConfigStore,
		Ref:  NormalizeResourceRef(a.AIGatewayConfigStore),
	}}
}

func (a AIGatewayConfigStoreSecretResource) GetParentRef() *ResourceRef {
	if a.AIGatewayConfigStore == "" {
		return nil
	}
	return &ResourceRef{
		Kind: ResourceTypeAIGatewayConfigStore,
		Ref:  NormalizeResourceRef(a.AIGatewayConfigStore),
	}
}

func (a AIGatewayConfigStoreSecretResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGatewayConfigStore == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGatewayConfigStore: string(ResourceTypeAIGatewayConfigStore)}
}

func (a AIGatewayConfigStoreSecretResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Config Store Secret ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Config Store Secret %s", a.Ref)
	}
	if a.AIGatewayConfigStore == "" {
		return fmt.Errorf("ai_gateway_config_store is required for AI Gateway Config Store Secret %s", a.Ref)
	}
	if a.Key == "" {
		return fmt.Errorf("key is required for AI Gateway Config Store Secret %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayConfigStoreSecretResource) SetDefaults() {
	if a == nil {
		return
	}
	if a.Ref == "" {
		a.Ref = a.Key
	}
}

func (a AIGatewayConfigStoreSecretResource) GetKonnectMonikerFilter() string { return a.Key }

func (a *AIGatewayConfigStoreSecretResource) TryMatchKonnectResource(resource any) bool {
	var key string
	switch typed := resource.(type) {
	case kkComps.AIGatewayConfigStoreSecret:
		key = typed.Key
	case *kkComps.AIGatewayConfigStoreSecret:
		if typed != nil {
			key = typed.Key
		}
	}
	if key == "" || key != a.Key {
		return false
	}
	a.SetKonnectID(key)
	return true
}

func (a AIGatewayConfigStoreSecretResource) PayloadMap() (map[string]any, error) {
	payload := map[string]any{aiGatewayConfigStoreSecretFieldKey: a.Key}
	if a.Value != "" {
		payload[aiGatewayConfigStoreSecretFieldValue] = a.Value
	}
	return payload, nil
}

func (a AIGatewayConfigStoreSecretResource) MutablePayloadMap() (map[string]any, error) {
	return a.PayloadMap()
}

// AIGatewayConfigStoreSecretResourceFromResponse maps public secret metadata to declarative form.
func AIGatewayConfigStoreSecretResourceFromResponse(
	storeRef string,
	secret kkComps.AIGatewayConfigStoreSecret,
) AIGatewayConfigStoreSecretResource {
	return AIGatewayConfigStoreSecretResource{
		BaseResource:         BaseResource{Ref: secret.Key},
		AIGatewayConfigStore: storeRef,
		Key:                  secret.Key,
	}
}

func aiGatewayConfigStoreSecretExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGatewayConfigStore, ResourceTypeAIGatewayConfigStore, true),
		explainField(aiGatewayConfigStoreSecretFieldKey, explainStringNode("openai-auth-header"), true, true),
		explainField(
			aiGatewayConfigStoreSecretFieldValue,
			explainSecretEnvNode("OPENAI_AUTH_HEADER"),
			false,
			false,
		),
	), nil
}

func aiGatewayConfigStoreSecretInlineExplainNode() *ExplainNode {
	node, err := aiGatewayConfigStoreSecretExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

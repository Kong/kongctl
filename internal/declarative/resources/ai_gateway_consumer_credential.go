package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayConsumerCredentialFieldID          = "id"
	aiGatewayConsumerCredentialFieldName        = "name"
	aiGatewayConsumerCredentialFieldType        = "type"
	aiGatewayConsumerCredentialFieldDisplayName = "display_name"
	aiGatewayConsumerCredentialFieldLabels      = "labels"
	aiGatewayConsumerCredentialFieldTTL         = "ttl"
	aiGatewayConsumerCredentialFieldAPIKey      = "api_key"
	aiGatewayConsumerCredentialFieldUpdatedAt   = "updated_at"
)

func init() {
	registerChildResourceType(
		ResourceTypeAIGatewayConsumerCredential,
		func(rs *ResourceSet) *[]AIGatewayConsumerCredentialResource {
			return &rs.AIGatewayConsumerCredentials
		},
		AutoExplain[AIGatewayConsumerCredentialResource](
			WithExplainAliases(
				"ai_gateway_consumer_credentials",
				"ai-gateway-consumer-credential",
				"ai-gateway-consumer-credentials",
				"ai_gateway.consumers.credentials",
				"aigw-consumer-credential",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGatewayConsumer,
				"name",
				"type",
				"display_name",
			),
			WithExplainSchemaBuilder(aiGatewayConsumerCredentialExplainNode),
		),
		childLoad[AIGatewayConsumerCredentialResource, AIGatewayConsumerResource]{
			family:        ResourceTypeAIGateway,
			extractOrder:  10,
			validateOrder: 60,
			nested: func(parent *AIGatewayConsumerResource) *[]AIGatewayConsumerCredentialResource {
				return &parent.Credentials
			},
			setParent: func(child *AIGatewayConsumerCredentialResource, ref string) { child.AIGatewayConsumer = ref },
			validate:  validateAIGatewayConsumerCredentials,
		},
		WithChildSyncScope(
			ResourceTypeAIGatewayConsumer,
			WithEmptyRootCollectionError(70, "each Credential must declare an ai_gateway_consumer parent"),
		),
	)
}

// AIGatewayConsumerCredentialResource represents a Credential nested under a Konnect AI Gateway Consumer.
type AIGatewayConsumerCredentialResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway Consumer reference for root-level declarations.
	AIGatewayConsumer string `yaml:"ai_gateway_consumer,omitempty" json:"ai_gateway_consumer,omitempty"`

	kkComps.CreateAIGatewayConsumerCredentialRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayConsumerCredentialResource) GetType() ResourceType {
	return ResourceTypeAIGatewayConsumerCredential
}

func (a AIGatewayConsumerCredentialResource) GetMoniker() string {
	return a.Name
}

func (a AIGatewayConsumerCredentialResource) GetDependencies() []ResourceRef {
	if a.AIGatewayConsumer == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGatewayConsumer, Ref: NormalizeResourceRef(a.AIGatewayConsumer)}}
}

func (a AIGatewayConsumerCredentialResource) GetParentRef() *ResourceRef {
	if a.AIGatewayConsumer == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGatewayConsumer, Ref: NormalizeResourceRef(a.AIGatewayConsumer)}
}

func (a AIGatewayConsumerCredentialResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGatewayConsumer == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGatewayConsumer: string(ResourceTypeAIGatewayConsumer)}
}

func (a AIGatewayConsumerCredentialResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Consumer Credential ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Consumer Credential %s", a.Ref)
	}
	if a.AIGatewayConsumer == "" {
		return fmt.Errorf("ai_gateway_consumer is required for AI Gateway Consumer Credential %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Consumer Credential %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Consumer Credential %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Consumer Credential %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayConsumerCredentialResource) SetDefaults() {
	if a == nil {
		return
	}
	if a.Ref == "" {
		a.Ref = a.Name
	}
	if a.Name == "" {
		a.Name = a.Ref
	}
	if a.DisplayName == "" {
		a.DisplayName = a.Name
	}
	if a.TTL == nil {
		ttl := int64(0)
		a.TTL = &ttl
	}
}

func (a AIGatewayConsumerCredentialResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

func (a *AIGatewayConsumerCredentialResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name
	if name == "" {
		return false
	}
	if id := AIGatewayConsumerCredentialID(konnectResource); id != "" &&
		(util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayConsumerCredentialID(konnectResource); id != "" &&
		AIGatewayConsumerCredentialName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayConsumerCredentialResource) CreateRequest() kkComps.CreateAIGatewayConsumerCredentialRequest {
	return a.CreateAIGatewayConsumerCredentialRequest
}

func (a AIGatewayConsumerCredentialResource) PayloadMap() (map[string]any, error) {
	payload, err := marshalObjectToMap(a.CreateRequest(), "AI Gateway Consumer Credential payload")
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (a AIGatewayConsumerCredentialResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayConsumerCredentialServerFields(payload)
	return payload, nil
}

func (a AIGatewayConsumerCredentialResource) MarshalJSON() ([]byte, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	payload[SchemaFieldRef] = a.Ref
	if a.AIGatewayConsumer != "" {
		payload[SchemaFieldAIGatewayConsumer] = a.AIGatewayConsumer
	}
	return json.Marshal(payload)
}

func (a AIGatewayConsumerCredentialResource) MarshalYAML() (any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	payload[SchemaFieldRef] = a.Ref
	if a.AIGatewayConsumer != "" {
		payload[SchemaFieldAIGatewayConsumer] = a.AIGatewayConsumer
	}
	return payload, nil
}

func (a *AIGatewayConsumerCredentialResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var meta struct {
		Ref               string          `json:"ref"`
		AIGatewayConsumer string          `json:"ai_gateway_consumer,omitempty"`
		Kongctl           json.RawMessage `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	if len(meta.Kongctl) > 0 && string(meta.Kongctl) != jsonNullLiteral {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	delete(raw, SchemaFieldRef)
	delete(raw, SchemaFieldAIGatewayConsumer)
	delete(raw, SchemaFieldKongctl)

	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var req kkComps.CreateAIGatewayConsumerCredentialRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGatewayConsumer = meta.AIGatewayConsumer
	a.CreateAIGatewayConsumerCredentialRequest = req
	return nil
}

func AIGatewayConsumerCredentialID(credential any) string {
	return aiGatewayConsumerCredentialStringField(credential, aiGatewayConsumerCredentialFieldID)
}

func AIGatewayConsumerCredentialName(credential any) string {
	return aiGatewayConsumerCredentialStringField(credential, aiGatewayConsumerCredentialFieldName)
}

func AIGatewayConsumerCredentialDisplayName(credential any) string {
	return aiGatewayConsumerCredentialStringField(credential, aiGatewayConsumerCredentialFieldDisplayName)
}

func AIGatewayConsumerCredentialLabels(credential any) map[string]string {
	payload, err := marshalObjectToMap(credential, "AI Gateway Consumer Credential")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayConsumerCredentialFieldLabels].(map[string]any)
	if !ok {
		return nil
	}
	labels := make(map[string]string, len(raw))
	for key, value := range raw {
		if stringValue, ok := value.(string); ok {
			labels[key] = stringValue
		}
	}
	return labels
}

func AIGatewayConsumerCredentialUpdatedAt(credential any) time.Time {
	payload, err := marshalObjectToMap(credential, "AI Gateway Consumer Credential")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayConsumerCredentialFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayConsumerCredentialMutablePayloadMap(
	credential kkComps.AIGatewayConsumerCredential,
) (map[string]any, error) {
	payload, err := marshalObjectToMap(credential, "AI Gateway Consumer Credential response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayConsumerCredentialServerFields(payload)
	stripAIGatewayConsumerCredentialWriteOnlyFields(payload)
	return payload, nil
}

func AIGatewayConsumerCredentialResourceFromResponse(
	consumerRef string,
	credential kkComps.AIGatewayConsumerCredential,
) (AIGatewayConsumerCredentialResource, error) {
	payload, err := AIGatewayConsumerCredentialMutablePayloadMap(credential)
	if err != nil {
		return AIGatewayConsumerCredentialResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayConsumerCredentialResource{}, err
	}
	var req kkComps.CreateAIGatewayConsumerCredentialRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayConsumerCredentialResource{}, err
	}

	ref := AIGatewayConsumerCredentialID(credential)
	if ref == "" {
		ref = AIGatewayConsumerCredentialName(credential)
	}
	return AIGatewayConsumerCredentialResource{
		BaseResource:                             BaseResource{Ref: ref},
		AIGatewayConsumer:                        consumerRef,
		CreateAIGatewayConsumerCredentialRequest: req,
	}, nil
}

func stripAIGatewayConsumerCredentialServerFields(payload map[string]any) {
	delete(payload, aiGatewayConsumerCredentialFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, aiGatewayConsumerCredentialFieldUpdatedAt)
}

func stripAIGatewayConsumerCredentialWriteOnlyFields(payload map[string]any) {
	delete(payload, aiGatewayConsumerCredentialFieldAPIKey)
}

func aiGatewayConsumerCredentialStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Consumer Credential")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayConsumerCredentialExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGatewayConsumer, ResourceTypeAIGatewayConsumer, true),
		explainField("name", explainStringNode("support-user-key"), true, true),
		explainField("type", explainStringNode("api-key"), true, true),
		explainField("display_name", explainStringNode("Support User API Key"), true, true),
		explainField("api_key", explainSecretEnvNode("AI_GATEWAY_CONSUMER_API_KEY"), false, false),
		explainField(aiGatewayConsumerCredentialFieldTTL, &ExplainNode{Kind: "integer", Literal: "0"}, false, true),
		explainField(
			"labels",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")},
			false,
			false,
		),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	), nil
}

func validateAIGatewayConsumerCredentials(rs *ResourceSet, children []AIGatewayConsumerCredentialResource) error {
	namesByConsumer := make(map[string]string)

	for i := range children {
		credential := &children[i]

		if err := credential.Validate(); err != nil {
			return fmt.Errorf("invalid ai_gateway_consumer_credential %q: %w", credential.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(credential.GetRef()); found {
			if existing.GetType() != ResourceTypeAIGatewayConsumerCredential {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					credential.GetRef(), existing.GetType())
			}
		}

		consumerRef := NormalizeResourceRef(credential.AIGatewayConsumer)
		consumer, found := rs.GetResourceByRef(consumerRef)
		if !found {
			return fmt.Errorf(
				"ai_gateway_consumer_credential %q references unknown ai_gateway_consumer %q",
				credential.GetRef(),
				credential.AIGatewayConsumer,
			)
		}
		if consumer.GetType() != ResourceTypeAIGatewayConsumer {
			return fmt.Errorf(
				"ai_gateway_consumer_credential %q references %q which is %s, not ai_gateway_consumer",
				credential.GetRef(),
				credential.AIGatewayConsumer,
				consumer.GetType(),
			)
		}

		nameKey := consumerRef + "\x00" + credential.Name
		if existingRef, exists := namesByConsumer[nameKey]; exists {
			return fmt.Errorf(
				"duplicate ai_gateway_consumer_credential name %q for ai_gateway_consumer %q (ref: %s conflicts with ref: %s)",
				credential.Name,
				consumerRef,
				credential.GetRef(),
				existingRef,
			)
		}
		namesByConsumer[nameKey] = credential.GetRef()
	}

	return nil
}

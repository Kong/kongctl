package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayConsumerFieldID          = "id"
	aiGatewayConsumerFieldName        = "name"
	aiGatewayConsumerFieldType        = "type"
	aiGatewayConsumerFieldDisplayName = "display_name"
	aiGatewayConsumerFieldCustomID    = "custom_id"
	aiGatewayConsumerFieldPolicies    = "policies"
	aiGatewayConsumerFieldLabels      = "labels"
	aiGatewayConsumerFieldUpdatedAt   = "updated_at"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayConsumer,
		func(rs *ResourceSet) *[]AIGatewayConsumerResource { return &rs.AIGatewayConsumers },
		AutoExplain[AIGatewayConsumerResource](
			WithExplainAliases(
				"ai_gateway_consumers",
				"ai-gateway-consumer",
				"ai-gateway-consumers",
				"ai_gateway.consumers",
				"aigw-consumer",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"name",
				"type",
				"display_name",
				aiGatewayConsumerFieldPolicies,
			),
			WithExplainSchemaBuilder(aiGatewayConsumerExplainNode),
		),
	)
}

// AIGatewayConsumerResource represents a Consumer nested under a Konnect AI Gateway.
type AIGatewayConsumerResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayConsumerRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayConsumerResource) GetType() ResourceType {
	return ResourceTypeAIGatewayConsumer
}

func (a AIGatewayConsumerResource) GetMoniker() string {
	return a.Name
}

func (a AIGatewayConsumerResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayConsumerResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayConsumerResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Consumer ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Consumer %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Consumer %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Consumer %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Consumer %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Consumer %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayConsumerResource) SetDefaults() {
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
}

func (a AIGatewayConsumerResource) GetKonnectMonikerFilter() string {
	return a.Name
}

func (a *AIGatewayConsumerResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name
	if name == "" {
		return false
	}
	if id := AIGatewayConsumerID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayConsumerID(konnectResource); id != "" && AIGatewayConsumerName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayConsumerResource) CreateRequest() kkComps.CreateAIGatewayConsumerRequest {
	return a.CreateAIGatewayConsumerRequest
}

func (a AIGatewayConsumerResource) UpdateRequest() kkComps.UpdateAIGatewayConsumerRequest {
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayConsumerRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayConsumerRequest{}
	}
	var req kkComps.UpdateAIGatewayConsumerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayConsumerRequest{}
	}
	return req
}

func (a AIGatewayConsumerResource) PayloadMap() (map[string]any, error) {
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway Consumer payload")
}

func (a AIGatewayConsumerResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayConsumerServerFields(payload)
	return payload, nil
}

func (a AIGatewayConsumerResource) MarshalJSON() ([]byte, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	payload[SchemaFieldRef] = a.Ref
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return json.Marshal(payload)
}

func (a AIGatewayConsumerResource) MarshalYAML() (any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	payload[SchemaFieldRef] = a.Ref
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return payload, nil
}

func (a *AIGatewayConsumerResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var meta struct {
		Ref       string          `json:"ref"`
		AIGateway string          `json:"ai_gateway,omitempty"`
		Kongctl   json.RawMessage `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	if len(meta.Kongctl) > 0 && string(meta.Kongctl) != jsonNullLiteral {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	delete(raw, SchemaFieldRef)
	delete(raw, SchemaFieldAIGateway)
	delete(raw, "kongctl")

	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var req kkComps.CreateAIGatewayConsumerRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayConsumerRequest = req
	return nil
}

func AIGatewayConsumerID(consumer any) string {
	return aiGatewayConsumerStringField(consumer, aiGatewayConsumerFieldID)
}

func AIGatewayConsumerName(consumer any) string {
	return aiGatewayConsumerStringField(consumer, aiGatewayConsumerFieldName)
}

func AIGatewayConsumerDisplayName(consumer any) string {
	return aiGatewayConsumerStringField(consumer, aiGatewayConsumerFieldDisplayName)
}

func AIGatewayConsumerLabels(consumer any) map[string]string {
	payload, err := marshalObjectToMap(consumer, "AI Gateway Consumer")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayConsumerFieldLabels].(map[string]any)
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

func AIGatewayConsumerUpdatedAt(consumer any) time.Time {
	payload, err := marshalObjectToMap(consumer, "AI Gateway Consumer")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayConsumerFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayConsumerMutablePayloadMap(consumer kkComps.AIGatewayConsumer) (map[string]any, error) {
	payload, err := marshalObjectToMap(consumer, "AI Gateway Consumer response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayConsumerServerFields(payload)
	return payload, nil
}

func AIGatewayConsumerResourceFromResponse(
	gatewayRef string,
	consumer kkComps.AIGatewayConsumer,
) (AIGatewayConsumerResource, error) {
	payload, err := AIGatewayConsumerMutablePayloadMap(consumer)
	if err != nil {
		return AIGatewayConsumerResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayConsumerResource{}, err
	}
	var req kkComps.CreateAIGatewayConsumerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayConsumerResource{}, err
	}

	ref := AIGatewayConsumerID(consumer)
	if ref == "" {
		ref = AIGatewayConsumerName(consumer)
	}
	return AIGatewayConsumerResource{
		BaseResource:                   BaseResource{Ref: ref},
		AIGateway:                      gatewayRef,
		CreateAIGatewayConsumerRequest: req,
	}, nil
}

func stripAIGatewayConsumerServerFields(payload map[string]any) {
	delete(payload, aiGatewayConsumerFieldID)
	delete(payload, "created_at")
	delete(payload, aiGatewayConsumerFieldUpdatedAt)
}

func aiGatewayConsumerStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Consumer")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayConsumerExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("support-user"), true, true),
		explainField("type", explainStringNode("api-key"), true, true),
		explainField("display_name", explainStringNode("Support User"), true, true),
		explainField(aiGatewayConsumerFieldCustomID, explainStringNode("support-user"), false, false),
		explainField(
			aiGatewayConsumerFieldPolicies,
			explainArrayOf(explainStringNode("!ref mask-sensitive-data")),
			false,
			true,
		),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	), nil
}

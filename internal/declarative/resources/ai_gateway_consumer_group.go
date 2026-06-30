package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayConsumerGroupFieldID          = "id"
	aiGatewayConsumerGroupFieldName        = "name"
	aiGatewayConsumerGroupFieldDisplayName = "display_name"
	aiGatewayConsumerGroupFieldPolicies    = "policies"
	aiGatewayConsumerGroupFieldLabels      = "labels"
	aiGatewayConsumerGroupFieldUpdatedAt   = "updated_at"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayConsumerGroup,
		func(rs *ResourceSet) *[]AIGatewayConsumerGroupResource { return &rs.AIGatewayConsumerGroups },
		AutoExplain[AIGatewayConsumerGroupResource](
			WithExplainAliases(
				"ai_gateway_consumer_groups",
				"ai-gateway-consumer-group",
				"ai-gateway-consumer-groups",
				"ai_gateway.consumer_groups",
				"aigw-consumer-group",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"name",
				"display_name",
				aiGatewayConsumerGroupFieldPolicies,
			),
			WithExplainSchemaBuilder(aiGatewayConsumerGroupExplainNode),
		),
	)
}

// AIGatewayConsumerGroupResource represents a Consumer Group nested under a Konnect AI Gateway.
type AIGatewayConsumerGroupResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayConsumerGroupRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayConsumerGroupResource) GetType() ResourceType {
	return ResourceTypeAIGatewayConsumerGroup
}

func (a AIGatewayConsumerGroupResource) GetMoniker() string {
	return a.Name
}

func (a AIGatewayConsumerGroupResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayConsumerGroupResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayConsumerGroupResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayConsumerGroupResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Consumer Group ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Consumer Group %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Consumer Group %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Consumer Group %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Consumer Group %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayConsumerGroupResource) SetDefaults() {
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

func (a AIGatewayConsumerGroupResource) GetKonnectMonikerFilter() string {
	return a.Name
}

func (a *AIGatewayConsumerGroupResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name
	if name == "" {
		return false
	}
	if id := AIGatewayConsumerGroupID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayConsumerGroupID(konnectResource); id != "" && AIGatewayConsumerGroupName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayConsumerGroupResource) CreateRequest() kkComps.CreateAIGatewayConsumerGroupRequest {
	return a.CreateAIGatewayConsumerGroupRequest
}

func (a AIGatewayConsumerGroupResource) UpdateRequest() kkComps.UpdateAIGatewayConsumerGroupRequest {
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayConsumerGroupRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayConsumerGroupRequest{}
	}
	var req kkComps.UpdateAIGatewayConsumerGroupRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayConsumerGroupRequest{}
	}
	return req
}

func (a AIGatewayConsumerGroupResource) PayloadMap() (map[string]any, error) {
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway Consumer Group payload")
}

func (a AIGatewayConsumerGroupResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayConsumerGroupServerFields(payload)
	return payload, nil
}

func (a AIGatewayConsumerGroupResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayConsumerGroupResource) MarshalYAML() (any, error) {
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

func (a *AIGatewayConsumerGroupResource) UnmarshalJSON(data []byte) error {
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
	delete(raw, SchemaFieldKongctl)

	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var req kkComps.CreateAIGatewayConsumerGroupRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayConsumerGroupRequest = req
	return nil
}

func AIGatewayConsumerGroupID(group any) string {
	return aiGatewayConsumerGroupStringField(group, aiGatewayConsumerGroupFieldID)
}

func AIGatewayConsumerGroupName(group any) string {
	return aiGatewayConsumerGroupStringField(group, aiGatewayConsumerGroupFieldName)
}

func AIGatewayConsumerGroupDisplayName(group any) string {
	return aiGatewayConsumerGroupStringField(group, aiGatewayConsumerGroupFieldDisplayName)
}

func AIGatewayConsumerGroupLabels(group any) map[string]string {
	payload, err := marshalObjectToMap(group, "AI Gateway Consumer Group")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayConsumerGroupFieldLabels].(map[string]any)
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

func AIGatewayConsumerGroupUpdatedAt(group any) time.Time {
	payload, err := marshalObjectToMap(group, "AI Gateway Consumer Group")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayConsumerGroupFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayConsumerGroupMutablePayloadMap(group kkComps.AIGatewayConsumerGroup) (map[string]any, error) {
	payload, err := marshalObjectToMap(group, "AI Gateway Consumer Group response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayConsumerGroupServerFields(payload)
	return payload, nil
}

func AIGatewayConsumerGroupResourceFromResponse(
	gatewayRef string,
	group kkComps.AIGatewayConsumerGroup,
) (AIGatewayConsumerGroupResource, error) {
	payload, err := AIGatewayConsumerGroupMutablePayloadMap(group)
	if err != nil {
		return AIGatewayConsumerGroupResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayConsumerGroupResource{}, err
	}
	var req kkComps.CreateAIGatewayConsumerGroupRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayConsumerGroupResource{}, err
	}

	ref := AIGatewayConsumerGroupID(group)
	if ref == "" {
		ref = AIGatewayConsumerGroupName(group)
	}
	return AIGatewayConsumerGroupResource{
		BaseResource:                        BaseResource{Ref: ref},
		AIGateway:                           gatewayRef,
		CreateAIGatewayConsumerGroupRequest: req,
	}, nil
}

func stripAIGatewayConsumerGroupServerFields(payload map[string]any) {
	delete(payload, aiGatewayConsumerGroupFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, aiGatewayConsumerGroupFieldUpdatedAt)
}

func aiGatewayConsumerGroupStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Consumer Group")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayConsumerGroupExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("premium-users"), true, true),
		explainField("display_name", explainStringNode("Premium Users"), true, true),
		explainField(
			aiGatewayConsumerGroupFieldPolicies,
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

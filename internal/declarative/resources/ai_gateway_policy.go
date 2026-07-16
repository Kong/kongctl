package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayPolicyFieldID          = "id"
	aiGatewayPolicyFieldName        = "name"
	aiGatewayPolicyFieldType        = "type"
	aiGatewayPolicyFieldDisplayName = "display_name"
	aiGatewayPolicyFieldEnabled     = "enabled"
	aiGatewayPolicyFieldGlobal      = "global"
	aiGatewayPolicyFieldLabels      = "labels"
	aiGatewayPolicyFieldUpdatedAt   = "updated_at"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayPolicy,
		func(rs *ResourceSet) *[]AIGatewayPolicyResource { return &rs.AIGatewayPolicies },
		AutoExplain[AIGatewayPolicyResource](
			WithExplainAliases(
				"ai_gateway_policies",
				"ai-gateway-policy",
				"ai-gateway-policies",
				"ai_gateway.policies",
				"aigw-policy",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"name",
				"type",
				"display_name",
				"config",
			),
			WithExplainSchemaBuilder(aiGatewayPolicyExplainNode),
		),
		WithMaturity(aiGatewayMaturity),
	)
}

// AIGatewayPolicyResource represents a Policy nested under a Konnect AI Gateway.
type AIGatewayPolicyResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayPolicyRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayPolicyResource) GetType() ResourceType {
	return ResourceTypeAIGatewayPolicy
}

func (a AIGatewayPolicyResource) GetMoniker() string {
	return a.Name
}

func (a AIGatewayPolicyResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayPolicyResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayPolicyResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayPolicyResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway policy ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway policy %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway policy %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway policy %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway policy %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway policy %s", a.Ref)
	}
	if a.Config == nil {
		return fmt.Errorf("config is required for AI Gateway policy %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayPolicyResource) SetDefaults() {
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
	enabled := true
	if a.Enabled == nil {
		a.Enabled = &enabled
	}
	global := false
	if a.Global == nil {
		a.Global = &global
	}
}

func (a AIGatewayPolicyResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

func (a *AIGatewayPolicyResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name
	if name == "" {
		return false
	}
	if id := AIGatewayPolicyID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayPolicyID(konnectResource); id != "" && AIGatewayPolicyName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayPolicyResource) CreateRequest() kkComps.CreateAIGatewayPolicyRequest {
	return a.CreateAIGatewayPolicyRequest
}

func (a AIGatewayPolicyResource) UpdateRequest() kkComps.UpdateAIGatewayPolicyRequest {
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayPolicyRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayPolicyRequest{}
	}
	var req kkComps.UpdateAIGatewayPolicyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayPolicyRequest{}
	}
	return req
}

func (a AIGatewayPolicyResource) PayloadMap() (map[string]any, error) {
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway policy payload")
}

func (a AIGatewayPolicyResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayPolicyServerFields(payload)
	return payload, nil
}

func (a AIGatewayPolicyResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayPolicyResource) MarshalYAML() (any, error) {
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

func (a *AIGatewayPolicyResource) UnmarshalJSON(data []byte) error {
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
	var req kkComps.CreateAIGatewayPolicyRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayPolicyRequest = req
	return nil
}

func AIGatewayPolicyID(policy any) string {
	return aiGatewayPolicyStringField(policy, aiGatewayPolicyFieldID)
}

func AIGatewayPolicyName(policy any) string {
	return aiGatewayPolicyStringField(policy, aiGatewayPolicyFieldName)
}

func AIGatewayPolicyDisplayName(policy any) string {
	return aiGatewayPolicyStringField(policy, aiGatewayPolicyFieldDisplayName)
}

func AIGatewayPolicyType(policy any) string {
	return aiGatewayPolicyStringField(policy, aiGatewayPolicyFieldType)
}

func AIGatewayPolicyEnabled(policy any) *bool {
	payload, err := marshalObjectToMap(policy, "AI Gateway policy")
	if err != nil {
		return nil
	}
	value, ok := payload[aiGatewayPolicyFieldEnabled].(bool)
	if !ok {
		return nil
	}
	return &value
}

func AIGatewayPolicyGlobal(policy any) *bool {
	payload, err := marshalObjectToMap(policy, "AI Gateway policy")
	if err != nil {
		return nil
	}
	value, ok := payload[aiGatewayPolicyFieldGlobal].(bool)
	if !ok {
		return nil
	}
	return &value
}

func AIGatewayPolicyLabels(policy any) map[string]string {
	payload, err := marshalObjectToMap(policy, "AI Gateway policy")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayPolicyFieldLabels].(map[string]any)
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

func AIGatewayPolicyUpdatedAt(policy any) time.Time {
	payload, err := marshalObjectToMap(policy, "AI Gateway policy")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayPolicyFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayPolicyMutablePayloadMap(policy kkComps.AIGatewayPolicy) (map[string]any, error) {
	payload, err := marshalObjectToMap(policy, "AI Gateway policy response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayPolicyServerFields(payload)
	return payload, nil
}

func AIGatewayPolicyResourceFromResponse(
	gatewayRef string,
	policy kkComps.AIGatewayPolicy,
) (AIGatewayPolicyResource, error) {
	payload, err := AIGatewayPolicyMutablePayloadMap(policy)
	if err != nil {
		return AIGatewayPolicyResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayPolicyResource{}, err
	}
	var req kkComps.CreateAIGatewayPolicyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayPolicyResource{}, err
	}

	ref := AIGatewayPolicyID(policy)
	if ref == "" {
		ref = AIGatewayPolicyName(policy)
	}
	return AIGatewayPolicyResource{
		BaseResource:                 BaseResource{Ref: ref},
		AIGateway:                    gatewayRef,
		CreateAIGatewayPolicyRequest: req,
	}, nil
}

func stripAIGatewayPolicyServerFields(payload map[string]any) {
	delete(payload, aiGatewayPolicyFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, aiGatewayPolicyFieldUpdatedAt)
}

func aiGatewayPolicyStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway policy")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayPolicyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("mask-sensitive-data"), true, true),
		explainField("type", explainStringNode("ai-sanitizer"), true, true),
		explainField("display_name", explainStringNode("Mask Sensitive Data"), true, true),
		explainField("enabled", explainBoolNode("true"), false, true),
		explainField("global", explainBoolNode("false"), false, true),
		explainField("config", &ExplainNode{
			Kind:       explainKindObject,
			Additional: &ExplainNode{},
		}, true, true),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	), nil
}

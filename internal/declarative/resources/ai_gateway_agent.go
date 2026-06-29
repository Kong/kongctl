package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayAgentFieldID          = "id"
	aiGatewayAgentFieldName        = "name"
	aiGatewayAgentFieldType        = "type"
	aiGatewayAgentFieldDisplayName = "display_name"
	aiGatewayAgentFieldEnabled     = "enabled"
	aiGatewayAgentFieldPolicies    = "policies"
	aiGatewayAgentFieldConfig      = "config"
	aiGatewayAgentFieldLabels      = "labels"
	aiGatewayAgentFieldUpdatedAt   = "updated_at"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayAgent,
		func(rs *ResourceSet) *[]AIGatewayAgentResource { return &rs.AIGatewayAgents },
		AutoExplain[AIGatewayAgentResource](
			WithExplainAliases(
				"ai_gateway_agents",
				"ai-gateway-agent",
				"ai-gateway-agents",
				"ai_gateway.agents",
				"aigw-agent",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"name",
				"type",
				"display_name",
				aiGatewayAgentFieldConfig,
				aiGatewayAgentFieldPolicies,
			),
			WithExplainSchemaBuilder(aiGatewayAgentExplainNode),
		),
	)
}

// AIGatewayAgentResource represents an Agent nested under a Konnect AI Gateway.
type AIGatewayAgentResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayAgentRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayAgentResource) GetType() ResourceType {
	return ResourceTypeAIGatewayAgent
}

func (a AIGatewayAgentResource) GetMoniker() string {
	return a.Name
}

func (a AIGatewayAgentResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayAgentResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayAgentResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Agent ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Agent %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Agent %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Agent %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Agent %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Agent %s", a.Ref)
	}
	if a.Config.URL == "" {
		return fmt.Errorf("config.url is required for AI Gateway Agent %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayAgentResource) SetDefaults() {
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
	if a.Enabled == nil {
		enabled := true
		a.Enabled = &enabled
	}
}

func (a AIGatewayAgentResource) GetKonnectMonikerFilter() string {
	return a.Name
}

func (a *AIGatewayAgentResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name
	if name == "" {
		return false
	}
	if id := AIGatewayAgentID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayAgentID(konnectResource); id != "" && AIGatewayAgentName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayAgentResource) CreateRequest() kkComps.CreateAIGatewayAgentRequest {
	return a.CreateAIGatewayAgentRequest
}

func (a AIGatewayAgentResource) UpdateRequest() kkComps.UpdateAIGatewayAgentRequest {
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayAgentRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayAgentRequest{}
	}
	var req kkComps.UpdateAIGatewayAgentRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayAgentRequest{}
	}
	return req
}

func (a AIGatewayAgentResource) PayloadMap() (map[string]any, error) {
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway Agent payload")
}

func (a AIGatewayAgentResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayAgentServerFields(payload)
	return payload, nil
}

func (a AIGatewayAgentResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayAgentResource) MarshalYAML() (any, error) {
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

func (a *AIGatewayAgentResource) UnmarshalJSON(data []byte) error {
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
	var req kkComps.CreateAIGatewayAgentRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayAgentRequest = req
	return nil
}

func AIGatewayAgentID(agent any) string {
	return aiGatewayAgentStringField(agent, aiGatewayAgentFieldID)
}

func AIGatewayAgentName(agent any) string {
	return aiGatewayAgentStringField(agent, aiGatewayAgentFieldName)
}

func AIGatewayAgentDisplayName(agent any) string {
	return aiGatewayAgentStringField(agent, aiGatewayAgentFieldDisplayName)
}

func AIGatewayAgentType(agent any) string {
	return aiGatewayAgentStringField(agent, aiGatewayAgentFieldType)
}

func AIGatewayAgentEnabled(agent any) *bool {
	payload, err := marshalObjectToMap(agent, "AI Gateway Agent")
	if err != nil {
		return nil
	}
	if enabled, ok := payload[aiGatewayAgentFieldEnabled].(bool); ok {
		return &enabled
	}
	return nil
}

func AIGatewayAgentLabels(agent any) map[string]string {
	payload, err := marshalObjectToMap(agent, "AI Gateway Agent")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayAgentFieldLabels].(map[string]any)
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

func AIGatewayAgentUpdatedAt(agent any) time.Time {
	payload, err := marshalObjectToMap(agent, "AI Gateway Agent")
	if err != nil {
		return time.Time{}
	}
	switch value := payload[aiGatewayAgentFieldUpdatedAt].(type) {
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	case time.Time:
		return value
	}
	return time.Time{}
}

func AIGatewayAgentMutablePayloadMap(agent kkComps.AIGatewayAgent) (map[string]any, error) {
	payload, err := marshalObjectToMap(agent, "AI Gateway Agent response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayAgentServerFields(payload)
	return payload, nil
}

func AIGatewayAgentResourceFromResponse(
	gatewayRef string,
	agent kkComps.AIGatewayAgent,
) (AIGatewayAgentResource, error) {
	payload, err := AIGatewayAgentMutablePayloadMap(agent)
	if err != nil {
		return AIGatewayAgentResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayAgentResource{}, err
	}
	var req kkComps.CreateAIGatewayAgentRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayAgentResource{}, err
	}

	ref := AIGatewayAgentID(agent)
	if ref == "" {
		ref = AIGatewayAgentName(agent)
	}
	return AIGatewayAgentResource{
		BaseResource:                BaseResource{Ref: ref},
		AIGateway:                   gatewayRef,
		CreateAIGatewayAgentRequest: req,
	}, nil
}

func stripAIGatewayAgentServerFields(payload map[string]any) {
	delete(payload, aiGatewayAgentFieldID)
	delete(payload, "created_at")
	delete(payload, aiGatewayAgentFieldUpdatedAt)
}

func aiGatewayAgentStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Agent")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayAgentExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("booking-agent"), true, true),
		explainField("type", explainStringNode("a2a"), true, true),
		explainField("display_name", explainStringNode("Booking Agent"), true, true),
		explainField(aiGatewayAgentFieldEnabled, explainBoolNode("true"), false, false),
		explainField(
			aiGatewayAgentFieldConfig,
			explainObject(
				explainField("url", explainStringNode("https://booking-agent.example.com"), true, true),
				explainField(
					"route",
					explainObject(
						explainField("paths", explainArrayOf(explainStringNode("/booking")), false, false),
						explainField("hosts", explainArrayOf(explainStringNode("agents.example.com")), false, false),
						explainField("methods", explainArrayOf(explainStringNode("POST")), false, false),
					),
					false,
					false,
				),
				explainField("max_request_body_size", &ExplainNode{Kind: explainKindInteger, Literal: "1048576"}, false, false),
				explainField(
					"logging",
					explainObject(
						explainField("payloads", explainBoolNode("true"), false, false),
						explainField("statistics", explainBoolNode("true"), false, false),
						explainField("max_payload_size", &ExplainNode{Kind: explainKindInteger, Literal: "524288"}, false, false),
					),
					false,
					false,
				),
			),
			true,
			true,
		),
		explainField(
			aiGatewayAgentFieldPolicies,
			explainArrayOf(explainStringNode("!ref mask-sensitive-data")),
			false,
			true,
		),
		explainField(
			"acls",
			explainObject(explainField("allow", explainArrayOf(explainStringNode("support-user")), false, false)),
			false,
			false,
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

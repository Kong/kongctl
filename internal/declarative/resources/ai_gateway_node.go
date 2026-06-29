package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayNodeFieldID                  = "id"
	aiGatewayNodeFieldVersion             = "version"
	aiGatewayNodeFieldHostname            = "hostname"
	aiGatewayNodeFieldLastPing            = "last_ping"
	aiGatewayNodeFieldType                = "type"
	aiGatewayNodeFieldConfigVersion       = "config_version"
	aiGatewayNodeFieldErrors              = "errors"
	aiGatewayNodeFieldCompatibilityStatus = "compatibility_status"
	aiGatewayNodeFieldCreatedAt           = "created_at"
	aiGatewayNodeFieldUpdatedAt           = "updated_at"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayNode,
		func(rs *ResourceSet) *[]AIGatewayNodeResource { return &rs.AIGatewayNodes },
		AutoExplain[AIGatewayNodeResource](
			WithExplainAliases(
				"ai_gateway_nodes",
				"ai-gateway-node",
				"ai-gateway-nodes",
				"ai_gateway.nodes",
				"aigw-node",
			),
			WithExplainRecommendedFields(
				SchemaFieldRef,
				SchemaFieldAIGateway,
				aiGatewayNodeFieldID,
				aiGatewayNodeFieldVersion,
				aiGatewayNodeFieldHostname,
				aiGatewayNodeFieldType,
			),
			WithExplainSchemaBuilder(aiGatewayNodeExplainNode),
		),
	)
}

type (
	aiGatewayNodeError               = kkComps.AIGatewayDataPlaneNodeError
	aiGatewayNodeCompatibilityStatus = kkComps.CompatibilityStatus
)

// AIGatewayNodeResource represents a data plane node nested under a Konnect AI Gateway.
type AIGatewayNodeResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	ID                  string                            `yaml:"id,omitempty"                   json:"id,omitempty"`
	Version             string                            `yaml:"version,omitempty"              json:"version,omitempty"`
	Hostname            string                            `yaml:"hostname,omitempty"             json:"hostname,omitempty"`
	LastPing            *int64                            `yaml:"last_ping,omitempty"            json:"last_ping,omitempty"`
	Type                string                            `yaml:"type,omitempty"                 json:"type,omitempty"`
	ConfigVersion       *string                           `yaml:"config_version,omitempty"       json:"config_version,omitempty"`
	Errors              []aiGatewayNodeError              `yaml:"errors,omitempty" json:"errors,omitempty"`
	CompatibilityStatus *aiGatewayNodeCompatibilityStatus `yaml:"compatibility_status,omitempty" json:"compatibility_status,omitempty"`
}

func (a AIGatewayNodeResource) GetType() ResourceType {
	return ResourceTypeAIGatewayNode
}

func (a AIGatewayNodeResource) GetMoniker() string {
	return a.ID
}

func (a AIGatewayNodeResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayNodeResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayNodeResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Node ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Node %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Node %s", a.Ref)
	}
	if a.ID == "" {
		return fmt.Errorf("id is required for AI Gateway Node %s", a.Ref)
	}
	if a.Version == "" {
		return fmt.Errorf("version is required for AI Gateway Node %s", a.Ref)
	}
	if a.Hostname == "" {
		return fmt.Errorf("hostname is required for AI Gateway Node %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Node %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayNodeResource) SetDefaults() {
	if a == nil {
		return
	}
	if a.Ref == "" {
		a.Ref = a.ID
	}
	if a.ID == "" {
		a.ID = a.Ref
	}
}

func (a AIGatewayNodeResource) GetKonnectMonikerFilter() string {
	return a.ID
}

func (a *AIGatewayNodeResource) TryMatchKonnectResource(konnectResource any) bool {
	id := AIGatewayNodeID(konnectResource)
	if id == "" || a.ID == "" {
		return false
	}
	if a.ID == id || a.Ref == id || a.GetKonnectID() == id {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayNodeResource) PayloadMap() (map[string]any, error) {
	payload := map[string]any{
		aiGatewayNodeFieldID:       a.ID,
		aiGatewayNodeFieldVersion:  a.Version,
		aiGatewayNodeFieldHostname: a.Hostname,
		aiGatewayNodeFieldType:     a.Type,
	}
	if a.LastPing != nil {
		payload[aiGatewayNodeFieldLastPing] = *a.LastPing
	}
	if a.ConfigVersion != nil {
		payload[aiGatewayNodeFieldConfigVersion] = *a.ConfigVersion
	}
	if len(a.Errors) > 0 {
		payload[aiGatewayNodeFieldErrors] = a.Errors
	}
	if a.CompatibilityStatus != nil {
		payload[aiGatewayNodeFieldCompatibilityStatus] = a.CompatibilityStatus
	}
	return normalizeAIGatewayNodePayloadMap(payload, "AI Gateway Node payload")
}

func (a AIGatewayNodeResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayNodeServerFields(payload)
	return payload, nil
}

func (a AIGatewayNodeResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayNodeResource) MarshalYAML() (any, error) {
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

func AIGatewayNodeID(node any) string {
	return aiGatewayNodeStringField(node, aiGatewayNodeFieldID)
}

func AIGatewayNodeVersion(node any) string {
	return aiGatewayNodeStringField(node, aiGatewayNodeFieldVersion)
}

func AIGatewayNodeHostname(node any) string {
	return aiGatewayNodeStringField(node, aiGatewayNodeFieldHostname)
}

func AIGatewayNodeType(node any) string {
	return aiGatewayNodeStringField(node, aiGatewayNodeFieldType)
}

func AIGatewayNodeConfigVersion(node any) string {
	return aiGatewayNodeStringField(node, aiGatewayNodeFieldConfigVersion)
}

func AIGatewayNodeUpdatedAt(node any) time.Time {
	payload, err := marshalObjectToMap(node, "AI Gateway Node")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayNodeFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayNodeMutablePayloadMap(node kkComps.AIGatewayDataPlaneNode) (map[string]any, error) {
	payload, err := marshalObjectToMap(node, "AI Gateway Node response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayNodeServerFields(payload)
	return payload, nil
}

func AIGatewayNodeResourceFromResponse(
	gatewayRef string,
	node kkComps.AIGatewayDataPlaneNode,
) (AIGatewayNodeResource, error) {
	payload, err := AIGatewayNodeMutablePayloadMap(node)
	if err != nil {
		return AIGatewayNodeResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayNodeResource{}, err
	}
	var resource AIGatewayNodeResource
	if err := json.Unmarshal(data, &resource); err != nil {
		return AIGatewayNodeResource{}, err
	}
	resource.BaseResource = BaseResource{Ref: AIGatewayNodeID(node)}
	resource.AIGateway = gatewayRef
	return resource, nil
}

func stripAIGatewayNodeServerFields(payload map[string]any) {
	delete(payload, aiGatewayNodeFieldLastPing)
	delete(payload, aiGatewayNodeFieldErrors)
	delete(payload, aiGatewayNodeFieldCompatibilityStatus)
	delete(payload, aiGatewayNodeFieldCreatedAt)
	delete(payload, aiGatewayNodeFieldUpdatedAt)
}

func aiGatewayNodeStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Node")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func normalizeAIGatewayNodePayloadMap(payload map[string]any, label string) (map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s: %w", label, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", label, err)
	}
	return result, nil
}

func aiGatewayNodeExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("id", explainStringNode("3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c"), true, true),
		explainField("version", explainStringNode("3.11.0"), true, true),
		explainField("hostname", explainStringNode("ai-gateway-node-1"), true, true),
		explainField("type", explainStringNode("data-plane"), true, true),
		explainField("last_ping", &ExplainNode{Kind: explainKindInteger, Literal: "1719350400"}, false, false),
		explainField("config_version", &ExplainNode{Kind: explainKindString, Nullable: true}, false, false),
		explainField("errors", explainArrayOf(&ExplainNode{Kind: explainKindObject}), false, false),
		explainField("compatibility_status", explainObject(
			explainField("state", &ExplainNode{Kind: explainKindString, Nullable: true}, false, false),
			explainField("issues", explainArrayOf(&ExplainNode{Kind: explainKindObject}), false, false),
		), false, false),
	), nil
}

func aiGatewayNodeInlineExplainNode() *ExplainNode {
	node, err := aiGatewayNodeExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

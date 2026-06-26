package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayMCPServerFieldID          = "id"
	aiGatewayMCPServerFieldName        = "name"
	aiGatewayMCPServerFieldType        = "type"
	aiGatewayMCPServerFieldDisplayName = "display_name"
	aiGatewayMCPServerFieldEnabled     = "enabled"
	aiGatewayMCPServerFieldLabels      = "labels"
	aiGatewayMCPServerFieldUpdatedAt   = "updated_at"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayMCPServer,
		func(rs *ResourceSet) *[]AIGatewayMCPServerResource { return &rs.AIGatewayMCPServers },
		AutoExplain[AIGatewayMCPServerResource](
			WithExplainAliases(
				"ai_gateway_mcp_servers",
				"ai-gateway-mcp-server",
				"ai-gateway-mcp-servers",
				"ai_gateway.mcp_servers",
				"aigw-mcp-server",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"type",
				"name",
				"display_name",
				"config",
				"tools",
			),
			WithExplainSchemaBuilder(aiGatewayMCPServerExplainNode),
		),
	)
}

// AIGatewayMCPServerResource represents an MCP Server nested under a Konnect AI Gateway.
type AIGatewayMCPServerResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayMCPServerRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayMCPServerResource) GetType() ResourceType {
	return ResourceTypeAIGatewayMCPServer
}

func (a AIGatewayMCPServerResource) GetMoniker() string {
	return a.Name()
}

func (a AIGatewayMCPServerResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayMCPServerResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayMCPServerResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway MCP Server ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway MCP Server %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway MCP Server %s", a.Ref)
	}
	if a.Name() == "" {
		return fmt.Errorf("name is required for AI Gateway MCP Server %s", a.Ref)
	}
	if a.DisplayName() == "" {
		return fmt.Errorf("display_name is required for AI Gateway MCP Server %s", a.Ref)
	}
	if a.MCPServerType() == "" {
		return fmt.Errorf("type is required for AI Gateway MCP Server %s", a.Ref)
	}
	if !a.hasPayload() {
		return fmt.Errorf("AI Gateway MCP Server %s must specify a valid MCP Server payload", a.Ref)
	}
	return nil
}

func (a *AIGatewayMCPServerResource) SetDefaults() {
	if a == nil || !a.hasPayload() {
		return
	}

	payload, err := a.PayloadMap()
	if err != nil {
		return
	}
	if a.Ref == "" {
		if name, _ := payload[aiGatewayMCPServerFieldName].(string); name != "" {
			a.Ref = name
		}
	}
	if name, _ := payload[aiGatewayMCPServerFieldName].(string); name == "" && a.Ref != "" {
		payload[aiGatewayMCPServerFieldName] = a.Ref
	}
	if displayName, _ := payload[aiGatewayMCPServerFieldDisplayName].(string); displayName == "" {
		if name, _ := payload[aiGatewayMCPServerFieldName].(string); name != "" {
			payload[aiGatewayMCPServerFieldDisplayName] = name
		}
	}
	if _, ok := payload[aiGatewayMCPServerFieldEnabled]; !ok {
		payload[aiGatewayMCPServerFieldEnabled] = true
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var req kkComps.CreateAIGatewayMCPServerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	a.CreateAIGatewayMCPServerRequest = req
}

func (a AIGatewayMCPServerResource) GetKonnectMonikerFilter() string {
	return a.Name()
}

func (a *AIGatewayMCPServerResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name()
	if name == "" {
		return false
	}
	if id := AIGatewayMCPServerID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayMCPServerID(konnectResource); id != "" && AIGatewayMCPServerName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayMCPServerResource) Name() string {
	return aiGatewayMCPServerStringField(a.CreateAIGatewayMCPServerRequest, aiGatewayMCPServerFieldName)
}

func (a AIGatewayMCPServerResource) DisplayName() string {
	return aiGatewayMCPServerStringField(a.CreateAIGatewayMCPServerRequest, aiGatewayMCPServerFieldDisplayName)
}

func (a AIGatewayMCPServerResource) MCPServerType() string {
	if a.Type != "" {
		return string(a.Type)
	}
	return aiGatewayMCPServerStringField(a.CreateAIGatewayMCPServerRequest, aiGatewayMCPServerFieldType)
}

func (a AIGatewayMCPServerResource) CreateRequest() kkComps.CreateAIGatewayMCPServerRequest {
	return a.CreateAIGatewayMCPServerRequest
}

func (a AIGatewayMCPServerResource) UpdateRequest() kkComps.UpdateAIGatewayMCPServerRequest {
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayMCPServerRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayMCPServerRequest{}
	}
	var req kkComps.UpdateAIGatewayMCPServerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayMCPServerRequest{}
	}
	return req
}

func (a AIGatewayMCPServerResource) PayloadMap() (map[string]any, error) {
	if !a.hasPayload() {
		return map[string]any{}, nil
	}
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway MCP Server payload")
}

func (a AIGatewayMCPServerResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayMCPServerServerFields(payload)
	return payload, nil
}

func (a AIGatewayMCPServerResource) hasPayload() bool {
	return a.AIGatewayMCPServerConversionOnly != nil ||
		a.AIGatewayMCPServerConversionListener != nil ||
		a.AIGatewayMCPServerListener != nil ||
		a.AIGatewayMCPServerPassthroughListener != nil ||
		a.AIGatewayMCPServerUpstreamServer != nil
}

func (a AIGatewayMCPServerResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayMCPServerResource) MarshalYAML() (any, error) {
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

func (a *AIGatewayMCPServerResource) UnmarshalJSON(data []byte) error {
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
	if len(meta.Kongctl) > 0 && string(meta.Kongctl) != "null" {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	delete(raw, SchemaFieldRef)
	delete(raw, SchemaFieldAIGateway)
	delete(raw, "kongctl")

	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var req kkComps.CreateAIGatewayMCPServerRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayMCPServerRequest = req
	return nil
}

func AIGatewayMCPServerID(server any) string {
	return aiGatewayMCPServerStringField(server, aiGatewayMCPServerFieldID)
}

func AIGatewayMCPServerName(server any) string {
	return aiGatewayMCPServerStringField(server, aiGatewayMCPServerFieldName)
}

func AIGatewayMCPServerDisplayName(server any) string {
	return aiGatewayMCPServerStringField(server, aiGatewayMCPServerFieldDisplayName)
}

func AIGatewayMCPServerType(server any) string {
	return aiGatewayMCPServerStringField(server, aiGatewayMCPServerFieldType)
}

func AIGatewayMCPServerEnabled(server any) *bool {
	payload, err := marshalObjectToMap(server, "AI Gateway MCP Server")
	if err != nil {
		return nil
	}
	value, ok := payload[aiGatewayMCPServerFieldEnabled].(bool)
	if !ok {
		return nil
	}
	return &value
}

func AIGatewayMCPServerLabels(server any) map[string]string {
	payload, err := marshalObjectToMap(server, "AI Gateway MCP Server")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayMCPServerFieldLabels].(map[string]any)
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

func AIGatewayMCPServerUpdatedAt(server any) time.Time {
	payload, err := marshalObjectToMap(server, "AI Gateway MCP Server")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayMCPServerFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayMCPServerMutablePayloadMap(server kkComps.AIGatewayMCPServer) (map[string]any, error) {
	payload, err := marshalObjectToMap(server, "AI Gateway MCP Server response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayMCPServerServerFields(payload)
	return payload, nil
}

func AIGatewayMCPServerResourceFromResponse(
	gatewayRef string,
	server kkComps.AIGatewayMCPServer,
) (AIGatewayMCPServerResource, error) {
	payload, err := AIGatewayMCPServerMutablePayloadMap(server)
	if err != nil {
		return AIGatewayMCPServerResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayMCPServerResource{}, err
	}
	var req kkComps.CreateAIGatewayMCPServerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayMCPServerResource{}, err
	}

	ref := AIGatewayMCPServerID(server)
	if ref == "" {
		ref = AIGatewayMCPServerName(server)
	}
	return AIGatewayMCPServerResource{
		BaseResource:                    BaseResource{Ref: ref},
		AIGateway:                       gatewayRef,
		CreateAIGatewayMCPServerRequest: req,
	}, nil
}

func stripAIGatewayMCPServerServerFields(payload map[string]any) {
	delete(payload, aiGatewayMCPServerFieldID)
	delete(payload, "created_at")
	delete(payload, aiGatewayMCPServerFieldUpdatedAt)
}

func aiGatewayMCPServerStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway MCP Server")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayMCPServerExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	commonFields := []*ExplainField{
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("customer-support-tools"), true, true),
		explainField("display_name", explainStringNode("Customer Support Tools"), true, true),
		explainField("enabled", explainBoolNode("true"), false, true),
		explainField("config", explainObject(
			explainField("url", explainStringNode("https://support-tools.example.com"), true, true),
		), true, true),
		explainField("tools", explainArrayOf(explainObject(
			explainField("name", explainStringNode("lookup-customer"), true, true),
			explainField("description", explainStringNode("Look up a customer profile"), true, true),
			explainField("method", explainStringNode("GET"), true, true),
			explainField("path", explainStringNode("/customers/{customer_id}"), false, true),
		)), true, true),
		explainField("policies", explainArrayOf(explainStringNode("policy-name")), true, false),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	}

	return explainUnionNode(
		explainObject(append(
			slicesCloneExplainFields(commonFields),
			explainField("type", explainConstStringNode("conversion-only"), true, true),
		)...),
		explainObject(append(
			slicesCloneExplainFields(commonFields),
			explainField("type", explainConstStringNode("conversion-listener"), true, true),
			explainField("acl_attribute_type", explainStringNode("consumer"), true, true),
		)...),
		explainObject(append(
			slicesCloneExplainFields(commonFields),
			explainField("type", explainConstStringNode("listener"), true, true),
			explainField("acl_attribute_type", explainStringNode("consumer"), true, true),
		)...),
		explainObject(append(
			slicesCloneExplainFields(commonFields),
			explainField("type", explainConstStringNode("passthrough-listener"), true, true),
			explainField("acl_attribute_type", explainStringNode("consumer"), true, true),
		)...),
		explainObject(append(
			slicesCloneExplainFields(commonFields),
			explainField("type", explainConstStringNode("upstream-server"), true, true),
			explainField("acl_attribute_type", explainStringNode("consumer"), true, true),
		)...),
	), nil
}

package resources

import (
	"encoding/json"
	"fmt"
	"slices"
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

func (a AIGatewayMCPServerResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
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

	// Defaults are best-effort; validation and planning surface malformed payloads through PayloadMap.
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
	return a.BaseResource.GetKonnectMonikerFilter(a.Name())
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
	// UpdateRequest is best-effort for legacy callers; MutablePayloadMap surfaces payload errors to planners.
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
	if len(meta.Kongctl) > 0 && string(meta.Kongctl) != jsonNullLiteral {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	delete(raw, SchemaFieldRef)
	delete(raw, SchemaFieldAIGateway)
	delete(raw, SchemaFieldKongctl)
	if err := rejectLegacyAIGatewayMCPServerAccessFields(raw); err != nil {
		return err
	}
	if err := rejectUnsupportedAIGatewayMCPServerAccess(raw); err != nil {
		return err
	}

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

func rejectLegacyAIGatewayMCPServerAccessFields(raw map[string]json.RawMessage) error {
	legacyFields := []string{
		"acl_attribute_type",
		"access_token_claim_field",
		"acls",
		"default_tool_acls",
	}
	for _, field := range legacyFields {
		if _, ok := raw[field]; ok {
			return fmt.Errorf("AI Gateway MCP Server field %q must be nested under access", field)
		}
	}
	return nil
}

func rejectUnsupportedAIGatewayMCPServerAccess(raw map[string]json.RawMessage) error {
	if _, hasAccess := raw["access"]; !hasAccess {
		return nil
	}

	var serverType string
	if err := json.Unmarshal(raw[aiGatewayMCPServerFieldType], &serverType); err != nil {
		return fmt.Errorf("invalid AI Gateway MCP Server type: %w", err)
	}
	if serverType == "conversion-only" {
		return fmt.Errorf(`AI Gateway MCP Server field "access" is not supported when type is "conversion-only"`)
	}
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
	delete(payload, SchemaFieldCreatedAt)
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
		explainField("config", aiGatewayMCPServerConfigExplainNode(), true, true),
		explainField("tools", explainArrayOf(explainObject(
			explainField("access", explainObject(
				explainField("acls", aiGatewayMCPACLsExplainNode(), false, false),
			), false, false),
			explainField("annotations", explainObject(
				explainField("destructive_hint", explainBoolNode("false"), false, false),
				explainField("idempotent_hint", explainBoolNode("true"), false, false),
				explainField("open_world_hint", explainBoolNode("false"), false, false),
				explainField("read_only_hint", explainBoolNode("true"), false, false),
				explainField("title", explainStringNode("Lookup customer"), false, false),
			), false, false),
			explainField("name", explainStringNode("lookup-customer"), true, true),
			explainField("description", explainStringNode("Look up a customer profile"), true, true),
			explainField(
				"headers",
				&ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}},
				false,
				false,
			),
			explainField("host", explainStringNode("api.example.com"), false, false),
			explainField("method", explainStringNode("GET"), false, true),
			explainField("path", explainStringNode("/customers/{customer_id}"), false, true),
			explainField(
				"query",
				&ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}},
				false,
				false,
			),
			explainField(
				"request_body",
				&ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}},
				false,
				false,
			),
			explainField(
				"responses",
				&ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}},
				false,
				false,
			),
			explainField("scheme", explainStringNode("https"), false, false),
			explainField("parameters", explainArrayOf(explainObject(
				explainField("name", explainStringNode("customer_id"), true, true),
				explainField("in", explainStringNode("path"), true, true),
				explainField("description", explainStringNode("Customer identifier"), false, false),
				explainField("required", explainBoolNode("true"), false, false),
				explainField("schema", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
			)), false, false),
			explainField("input_schema", &ExplainNode{}, false, false),
			explainField("output_schema", &ExplainNode{}, false, false),
		)), false, true),
		explainField("policies", explainArrayOf(explainStringNode("policy-name")), false, false),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	}
	accessFields := append(
		slices.Clone(commonFields),
		explainField("access", aiGatewayMCPServerAccessExplainNode(), false, false),
	)
	return explainUnionNode(
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("conversion-only"), true, true),
		)...),
		explainObject(append(
			slices.Clone(accessFields),
			explainField("type", explainConstStringNode("conversion-listener"), true, true),
		)...),
		explainObject(append(
			slices.Clone(accessFields),
			explainField("type", explainConstStringNode("listener"), true, true),
		)...),
		explainObject(append(
			slices.Clone(accessFields),
			explainField("type", explainConstStringNode("passthrough-listener"), true, true),
		)...),
		explainObject(append(
			slices.Clone(accessFields),
			explainField("type", explainConstStringNode("upstream-server"), true, true),
		)...),
	), nil
}

func aiGatewayMCPServerAccessExplainNode() *ExplainNode {
	return explainObject(
		explainField("acl_attribute_type", explainStringNode("consumer"), true, true),
		explainField("access_token_claim_field", explainStringNode("sub"), false, false),
		explainField("acls", aiGatewayMCPACLsExplainNode(), false, false),
		explainField("default_tool_acls", aiGatewayMCPACLsExplainNode(), false, false),
	)
}

func aiGatewayMCPServerConfigExplainNode() *ExplainNode {
	return explainObject(
		explainField("route", aiGatewayRouteExplainNode(), false, false),
		explainField("logging", explainObject(
			explainField("payloads", explainBoolNode("false"), false, false),
			explainField("statistics", explainBoolNode("true"), false, false),
			explainField("audits", explainBoolNode("false"), false, false),
		), false, false),
		explainField("max_request_body_size", &ExplainNode{Kind: explainKindInteger, Literal: "8388608"}, false, false),
		explainField("server", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
		explainField("url", explainStringNode("https://support-tools.example.com"), false, true),
		explainField("proxy", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
		explainField("tools_cache_ttl_seconds", &ExplainNode{Kind: explainKindInteger, Literal: "60"}, false, false),
	)
}

func aiGatewayMCPACLsExplainNode() *ExplainNode {
	return explainObject(
		explainField("allow", explainArrayOf(explainStringNode("consumer-group")), false, false),
		explainField("deny", explainArrayOf(explainStringNode("consumer-group")), false, false),
	)
}

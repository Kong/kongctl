package resources

import (
	"encoding/json"
	"fmt"
	"slices"
)

const (
	aiGatewayIdentityProviderFieldName        = "name"
	aiGatewayIdentityProviderFieldType        = "type"
	aiGatewayIdentityProviderFieldDisplayName = "display_name"
	aiGatewayIdentityProviderFieldLabels      = "labels"
	aiGatewayIdentityProviderFieldManagedBy   = "managed_by"
	aiGatewayIdentityProviderFieldConfig      = "config"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayIdentityProvider,
		func(rs *ResourceSet) *[]AIGatewayIdentityProviderResource { return &rs.AIGatewayIdentityProviders },
		AutoExplain[AIGatewayIdentityProviderResource](
			WithExplainAliases(
				"ai_gateway_identity_providers",
				"ai-gateway-identity-provider",
				"ai-gateway-identity-providers",
				"aigw-identity-provider",
			),
			WithExplainRecommendedFields("ref", SchemaFieldAIGateway, "name", "type", "display_name", "config"),
			WithExplainSchemaBuilder(aiGatewayIdentityProviderExplainNode),
		),
	)
}

// AIGatewayIdentityProviderResource represents a Konnect AI Gateway Identity Provider in declarative configuration.
type AIGatewayIdentityProviderResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level identity provider declarations.
	AIGateway   string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name        string            `yaml:"name"                 json:"name"`
	Type        string            `yaml:"type"                 json:"type"`
	DisplayName string            `yaml:"display_name"         json:"display_name"`
	Labels      map[string]string `yaml:"labels,omitempty"     json:"labels,omitempty"`
	ManagedBy   map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
	Config      map[string]any    `yaml:"config"               json:"config"`
}

// GetType returns the resource type.
func (a AIGatewayIdentityProviderResource) GetType() ResourceType {
	return ResourceTypeAIGatewayIdentityProvider
}

// GetMoniker returns the provider name used for matching within the parent gateway.
func (a AIGatewayIdentityProviderResource) GetMoniker() string {
	return a.Name
}

// GetDependencies returns references to other resources this provider depends on.
func (a AIGatewayIdentityProviderResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

// Validate ensures the AI Gateway Identity Provider resource is valid.
func (a AIGatewayIdentityProviderResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Identity Provider ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Identity Provider %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Identity Provider %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Identity Provider %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Identity Provider %s", a.Ref)
	}
	if a.Config == nil {
		return fmt.Errorf("config is required for AI Gateway Identity Provider %s", a.Ref)
	}
	return nil
}

// SetDefaults applies default values to AI Gateway Identity Provider resources.
func (a *AIGatewayIdentityProviderResource) SetDefaults() {
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

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (a AIGatewayIdentityProviderResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

// TryMatchKonnectResource attempts to match this provider with a Konnect resource.
func (a *AIGatewayIdentityProviderResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", a.Name); id != "" {
		a.SetKonnectID(id)
		return true
	}
	return false
}

// GetParentRef returns the parent AI Gateway reference.
func (a AIGatewayIdentityProviderResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayIdentityProviderResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayIdentityProviderResource) PayloadMap() (map[string]any, error) {
	payload := map[string]any{
		aiGatewayIdentityProviderFieldName:        a.Name,
		aiGatewayIdentityProviderFieldType:        a.Type,
		aiGatewayIdentityProviderFieldDisplayName: a.DisplayName,
		aiGatewayIdentityProviderFieldConfig:      a.Config,
	}
	if a.Labels != nil {
		payload[aiGatewayIdentityProviderFieldLabels] = a.Labels
	}
	if a.ManagedBy != nil {
		payload[aiGatewayIdentityProviderFieldManagedBy] = a.ManagedBy
	}
	return payload, nil
}

func (a AIGatewayIdentityProviderResource) MutablePayloadMap() (map[string]any, error) {
	return a.PayloadMap()
}

func (a AIGatewayIdentityProviderResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayIdentityProviderResource) MarshalYAML() (any, error) {
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

// UnmarshalJSON rejects kongctl metadata on child provider resources.
func (a *AIGatewayIdentityProviderResource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref         string            `json:"ref"`
		AIGateway   string            `json:"ai_gateway,omitempty"`
		Name        string            `json:"name"`
		Type        string            `json:"type"`
		DisplayName string            `json:"display_name"`
		Labels      map[string]string `json:"labels,omitempty"`
		ManagedBy   map[string]string `json:"managed_by,omitempty"`
		Config      map[string]any    `json:"config"`
		Kongctl     any               `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	a.BaseResource = BaseResource{Ref: raw.Ref}
	a.AIGateway = raw.AIGateway
	a.Name = raw.Name
	a.Type = raw.Type
	a.DisplayName = raw.DisplayName
	a.Labels = raw.Labels
	a.ManagedBy = raw.ManagedBy
	a.Config = raw.Config
	return nil
}

func aiGatewayIdentityProviderExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	commonFields := []*ExplainField{
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("support-key-auth"), true, true),
		explainField("display_name", explainStringNode("Support Key Auth"), true, true),
		explainField("labels", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("value"),
		}, false, false),
		explainField("managed_by", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("kongctl"),
		}, false, false),
	}

	return explainUnionNode(
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("key-auth"), true, true),
			explainField("config", explainObject(
				explainField("hide_credentials", explainBoolNode("true"), false, true),
				explainField("key_in_body", explainBoolNode("false"), false, false),
				explainField("key_in_header", explainBoolNode("true"), false, false),
				explainField("key_in_query", explainBoolNode("true"), false, false),
				explainField("key_names", explainArrayOf(explainStringNode("apikey")), false, true),
			), true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("openid-connect"), true, true),
			explainField("config", explainObject(
				explainField("auth_methods", explainArrayOf(explainStringNode("bearer")), false, true),
				explainField("cache_tokens_salt", explainStringNode("support-cache-salt"), true, true),
				explainField("client_id", explainArrayOf(explainStringNode("support-client")), false, true),
				explainField("client_secret", explainArrayOf(explainStringNode("${OIDC_CLIENT_SECRET}")), false, false),
				explainField("consumer_claims", explainArrayOf(explainArrayOf(explainStringNode("sub"))), false, false),
				explainField("consumer_optional", explainBoolNode("false"), false, false),
				explainField("issuer", explainStringNode("https://issuer.example.com"), false, true),
				explainField("scopes", explainArrayOf(explainStringNode("openid")), false, false),
				explainField("ssl_verify", explainBoolNode("true"), false, false),
			), true, true),
		)...),
	), nil
}

func aiGatewayIdentityProviderInlineExplainNode() *ExplainNode {
	node, err := aiGatewayIdentityProviderExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject(
			explainResourceRefField(),
			explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
			explainField("name", explainStringNode("support-key-auth"), true, true),
			explainField("type", explainStringNode("key-auth"), true, true),
			explainField("display_name", explainStringNode("Support Key Auth"), true, true),
			explainField("config", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, true, true),
		)
	}
	return node
}

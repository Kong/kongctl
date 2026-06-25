package resources

import (
	"encoding/json"
	"fmt"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayProvider,
		func(rs *ResourceSet) *[]AIGatewayProviderResource { return &rs.AIGatewayProviders },
		AutoExplain[AIGatewayProviderResource](
			WithExplainAliases("ai_gateway_providers", "ai-gateway-provider", "ai-gateway-providers", "aigw-provider"),
			WithExplainRecommendedFields("ref", "ai_gateway", "name", "type", "display_name", "config"),
			WithExplainSchemaBuilder(aiGatewayProviderExplainNode),
		),
	)
}

// AIGatewayProviderResource represents a Konnect AI Gateway Provider in declarative configuration.
type AIGatewayProviderResource struct {
	Ref string `yaml:"ref" json:"ref"`
	// Parent AI Gateway reference for root-level provider declarations.
	AIGateway   string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name        string            `yaml:"name"                 json:"name"`
	Type        string            `yaml:"type"                 json:"type"`
	DisplayName string            `yaml:"display_name"         json:"display_name"`
	Labels      map[string]string `yaml:"labels,omitempty"     json:"labels,omitempty"`
	ManagedBy   map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
	Config      map[string]any    `yaml:"config"               json:"config"`

	konnectID string `yaml:"-" json:"-"`
}

// GetType returns the resource type.
func (a AIGatewayProviderResource) GetType() ResourceType {
	return ResourceTypeAIGatewayProvider
}

// GetRef returns the declarative ref.
func (a AIGatewayProviderResource) GetRef() string {
	return a.Ref
}

// GetMoniker returns the provider name used for matching within the parent gateway.
func (a AIGatewayProviderResource) GetMoniker() string {
	return a.Name
}

// GetDependencies returns references to other resources this provider depends on.
func (a AIGatewayProviderResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: a.AIGateway}}
}

// Validate ensures the AI Gateway Provider resource is valid.
func (a AIGatewayProviderResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Provider ref: %w", err)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Provider %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Provider %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Provider %s", a.Ref)
	}
	if a.Config == nil {
		return fmt.Errorf("config is required for AI Gateway Provider %s", a.Ref)
	}
	return nil
}

// SetDefaults applies default values to AI Gateway Provider resources.
func (a *AIGatewayProviderResource) SetDefaults() {
}

// GetKonnectID returns the resolved Konnect ID.
func (a AIGatewayProviderResource) GetKonnectID() string {
	return a.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (a AIGatewayProviderResource) GetKonnectMonikerFilter() string {
	return fmt.Sprintf("name[eq]=%s", a.Name)
}

// TryMatchKonnectResource attempts to match this provider with a Konnect resource.
func (a *AIGatewayProviderResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", a.Name); id != "" {
		a.konnectID = id
		return true
	}
	return false
}

// GetParentRef returns the parent AI Gateway reference.
func (a AIGatewayProviderResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: a.AIGateway}
}

// UnmarshalJSON rejects kongctl metadata on child provider resources.
func (a *AIGatewayProviderResource) UnmarshalJSON(data []byte) error {
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

	a.Ref = raw.Ref
	a.AIGateway = raw.AIGateway
	a.Name = raw.Name
	a.Type = raw.Type
	a.DisplayName = raw.DisplayName
	a.Labels = raw.Labels
	a.ManagedBy = raw.ManagedBy
	a.Config = raw.Config
	return nil
}

func aiGatewayProviderExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField("ai_gateway", ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("openai-provider"), true, true),
		explainField("type", explainStringNode("openai"), true, true),
		explainField("display_name", explainStringNode("OpenAI Provider"), true, true),
		explainField("config", explainObject(
			explainField("auth", explainObject(
				explainField("type", explainStringNode("basic"), true, true),
				explainField("header_name", explainStringNode("Authorization"), false, false),
				explainField("header_value", explainStringNode("Bearer ${OPENAI_API_KEY}"), false, false),
			), true, true),
		), true, true),
		explainField("labels", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("value"),
		}, false, false),
		explainField("managed_by", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("kongctl"),
		}, false, false),
	), nil
}

package resources

import (
	"encoding/json"
	"fmt"
)

const (
	aiGatewayProviderFieldName        = "name"
	aiGatewayProviderFieldType        = "type"
	aiGatewayProviderFieldDisplayName = "display_name"
	aiGatewayProviderFieldLabels      = "labels"
	aiGatewayProviderFieldManagedBy   = "managed_by"
	aiGatewayProviderFieldConfig      = "config"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayProvider,
		func(rs *ResourceSet) *[]AIGatewayProviderResource { return &rs.AIGatewayProviders },
		AutoExplain[AIGatewayProviderResource](
			WithExplainAliases(
				"ai_gateway_model_providers",
				"ai-gateway-model-provider",
				"ai-gateway-model-providers",
				"aigw-model-provider",
			),
			WithExplainRecommendedFields("ref", SchemaFieldAIGateway, "name", "type", "display_name", "config"),
			WithExplainSchemaBuilder(aiGatewayProviderExplainNode),
		),
	)
}

// AIGatewayProviderResource represents a Konnect AI Gateway Model Provider in declarative configuration.
type AIGatewayProviderResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level model provider declarations.
	AIGateway   string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name        string            `yaml:"name"                 json:"name"`
	Type        string            `yaml:"type"                 json:"type"`
	DisplayName string            `yaml:"display_name"         json:"display_name"`
	Labels      map[string]string `yaml:"labels,omitempty"     json:"labels,omitempty"`
	ManagedBy   map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
	Config      map[string]any    `yaml:"config"               json:"config"`
}

// GetType returns the resource type.
func (a AIGatewayProviderResource) GetType() ResourceType {
	return ResourceTypeAIGatewayProvider
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
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

// Validate ensures the AI Gateway Model Provider resource is valid.
func (a AIGatewayProviderResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Model Provider ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Model Provider %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Model Provider %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Model Provider %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Model Provider %s", a.Ref)
	}
	if a.Config == nil {
		return fmt.Errorf("config is required for AI Gateway Model Provider %s", a.Ref)
	}
	return nil
}

// SetDefaults applies default values to AI Gateway Model Provider resources.
func (a *AIGatewayProviderResource) SetDefaults() {
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
func (a AIGatewayProviderResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

// TryMatchKonnectResource attempts to match this provider with a Konnect resource.
func (a *AIGatewayProviderResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", a.Name); id != "" {
		a.SetKonnectID(id)
		return true
	}
	return false
}

// GetParentRef returns the parent AI Gateway reference.
func (a AIGatewayProviderResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayProviderResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayProviderResource) PayloadMap() (map[string]any, error) {
	payload := map[string]any{
		aiGatewayProviderFieldName:        a.Name,
		aiGatewayProviderFieldType:        a.Type,
		aiGatewayProviderFieldDisplayName: a.DisplayName,
		aiGatewayProviderFieldConfig:      a.Config,
	}
	if a.Labels != nil {
		payload[aiGatewayProviderFieldLabels] = a.Labels
	}
	if a.ManagedBy != nil {
		payload[aiGatewayProviderFieldManagedBy] = a.ManagedBy
	}
	return payload, nil
}

func (a AIGatewayProviderResource) MutablePayloadMap() (map[string]any, error) {
	return a.PayloadMap()
}

func (a AIGatewayProviderResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayProviderResource) MarshalYAML() (any, error) {
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

func aiGatewayProviderExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
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

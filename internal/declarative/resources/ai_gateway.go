package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeAIGateway,
		func(rs *ResourceSet) *[]AIGatewayResource { return &rs.AIGateways },
		AutoExplain[AIGatewayResource](
			WithExplainAliases("ai_gateways", "ai-gateway", "ai-gateways", "aigw"),
			WithExplainRecommendedFields("ref", "display_name"),
			WithExplainSchemaBuilder(aiGatewayExplainNode),
		),
	)
}

// AIGatewayResource represents a Konnect AI Gateway in declarative configuration.
type AIGatewayResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	kkComps.CreateAIGatewayRequest
}

func (a AIGatewayResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.aiGatewayAlias())
}

// MarshalYAML ensures YAML output mirrors the custom JSON encoding.
func (a AIGatewayResource) MarshalYAML() (any, error) {
	return a.aiGatewayAlias(), nil
}

type aiGatewayAlias struct {
	Ref         string                      `json:"ref"                   yaml:"ref"`
	Kongctl     *KongctlMeta                `json:"kongctl,omitempty"     yaml:"kongctl,omitempty"`
	DisplayName string                      `json:"display_name"          yaml:"display_name"`
	Description *string                     `json:"description,omitempty" yaml:"description,omitempty"`
	ProxyURLs   []kkComps.AIGatewayProxyURL `json:"proxy_urls,omitempty"  yaml:"proxy_urls,omitempty"`
	Labels      map[string]string           `json:"labels,omitempty"      yaml:"labels,omitempty"`
}

func (a AIGatewayResource) aiGatewayAlias() aiGatewayAlias {
	return aiGatewayAlias{
		Ref:         a.Ref,
		Kongctl:     a.Kongctl,
		DisplayName: a.DisplayName,
		Description: a.Description,
		ProxyURLs:   a.ProxyUrls,
		Labels:      a.Labels,
	}
}

// UnmarshalYAML decodes AI Gateway fields explicitly because the SDK request
// type only carries JSON tags.
func (a *AIGatewayResource) UnmarshalYAML(unmarshal func(any) error) error {
	var raw struct {
		Ref         string                      `yaml:"ref"`
		Kongctl     *KongctlMeta                `yaml:"kongctl,omitempty"`
		DisplayName string                      `yaml:"display_name"`
		Description *string                     `yaml:"description,omitempty"`
		ProxyURLs   []kkComps.AIGatewayProxyURL `yaml:"proxy_urls,omitempty"`
		Labels      map[string]string           `yaml:"labels,omitempty"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	a.BaseResource = BaseResource{
		Ref:     raw.Ref,
		Kongctl: raw.Kongctl,
	}
	a.CreateAIGatewayRequest = kkComps.CreateAIGatewayRequest{
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		ProxyUrls:   raw.ProxyURLs,
		Labels:      raw.Labels,
	}

	return nil
}

// UnmarshalJSON decodes AI Gateways explicitly because YAML loading goes
// through JSON tags and the embedded SDK request type has a custom unmarshaler.
func (a *AIGatewayResource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref         string                      `json:"ref"`
		Kongctl     *KongctlMeta                `json:"kongctl,omitempty"`
		DisplayName string                      `json:"display_name"`
		Description *string                     `json:"description,omitempty"`
		ProxyURLs   []kkComps.AIGatewayProxyURL `json:"proxy_urls,omitempty"`
		Labels      map[string]string           `json:"labels,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.BaseResource = BaseResource{
		Ref:     raw.Ref,
		Kongctl: raw.Kongctl,
	}
	a.CreateAIGatewayRequest = kkComps.CreateAIGatewayRequest{
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		ProxyUrls:   raw.ProxyURLs,
		Labels:      raw.Labels,
	}

	return nil
}

// GetType returns the resource type.
func (a AIGatewayResource) GetType() ResourceType {
	return ResourceTypeAIGateway
}

// GetMoniker returns the resource moniker.
func (a AIGatewayResource) GetMoniker() string {
	return a.DisplayName
}

// GetDependencies returns references to other resources this gateway depends on.
func (a AIGatewayResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

// GetLabels returns the labels for this resource.
func (a AIGatewayResource) GetLabels() map[string]string {
	return a.Labels
}

// SetLabels sets the labels for this resource.
func (a *AIGatewayResource) SetLabels(labels map[string]string) {
	a.Labels = labels
}

// Validate ensures the AI Gateway resource is valid.
func (a AIGatewayResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway ref: %w", err)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway %s", a.Ref)
	}
	return nil
}

// SetDefaults applies default values to AI Gateway resource.
func (a *AIGatewayResource) SetDefaults() {
	if a.DisplayName == "" {
		a.DisplayName = a.Ref
	}
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (a AIGatewayResource) GetKonnectMonikerFilter() string {
	return a.DisplayName
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (a *AIGatewayResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "DisplayName", a.DisplayName); id != "" {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func aiGatewayExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainKongctlField(),
		explainField("display_name", explainStringNode("My AI Gateway"), true, true),
		explainField("description", &ExplainNode{Kind: explainKindString, Nullable: true}, false, false),
		explainField("proxy_urls", explainArrayOf(explainObject(
			explainField("host", explainStringNode("proxy.example.com"), true, true),
			explainField("port", &ExplainNode{Kind: "integer", Literal: "443"}, true, true),
			explainField("protocol", explainStringNode("https"), true, true),
		)), false, false),
		explainField("labels", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("value"),
		}, false, false),
	), nil
}

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
	External       *ExternalBlock                   `yaml:"_external,omitempty" json:"_external,omitempty"`
	Providers      []AIGatewayProviderResource      `yaml:"providers,omitempty" json:"providers,omitempty"`
	Policies       []AIGatewayPolicyResource        `yaml:"policies,omitempty" json:"policies,omitempty"`
	Consumers      []AIGatewayConsumerResource      `yaml:"consumers,omitempty" json:"consumers,omitempty"`
	ConsumerGroups []AIGatewayConsumerGroupResource `yaml:"consumer_groups,omitempty" json:"consumer_groups,omitempty"`
	Models         []AIGatewayModelResource         `yaml:"models,omitempty"    json:"models,omitempty"`
	MCPServers     []AIGatewayMCPServerResource     `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
	Vaults         []AIGatewayVaultResource         `yaml:"vaults,omitempty" json:"vaults,omitempty"`
}

func (a AIGatewayResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.aiGatewayAlias())
}

// MarshalYAML ensures YAML output mirrors the custom JSON encoding.
func (a AIGatewayResource) MarshalYAML() (any, error) {
	return a.aiGatewayAlias(), nil
}

type aiGatewayAlias struct {
	Ref            string                           `json:"ref"                   yaml:"ref"`
	Kongctl        *KongctlMeta                     `json:"kongctl,omitempty"     yaml:"kongctl,omitempty"`
	External       *ExternalBlock                   `json:"_external,omitempty"   yaml:"_external,omitempty"`
	DisplayName    string                           `json:"display_name"          yaml:"display_name"`
	Description    *string                          `json:"description,omitempty" yaml:"description,omitempty"`
	ProxyURLs      []kkComps.AIGatewayProxyURL      `json:"proxy_urls,omitempty"  yaml:"proxy_urls,omitempty"`
	Labels         map[string]string                `json:"labels,omitempty"      yaml:"labels,omitempty"`
	Providers      []AIGatewayProviderResource      `json:"providers,omitempty"   yaml:"providers,omitempty"`
	Policies       []AIGatewayPolicyResource        `json:"policies,omitempty"    yaml:"policies,omitempty"`
	Consumers      []AIGatewayConsumerResource      `json:"consumers,omitempty"   yaml:"consumers,omitempty"`
	ConsumerGroups []AIGatewayConsumerGroupResource `json:"consumer_groups,omitempty" yaml:"consumer_groups,omitempty"`
	Models         []AIGatewayModelResource         `json:"models,omitempty"      yaml:"models,omitempty"`
	MCPServers     []AIGatewayMCPServerResource     `json:"mcp_servers,omitempty" yaml:"mcp_servers,omitempty"`
	Vaults         []AIGatewayVaultResource         `json:"vaults,omitempty"      yaml:"vaults,omitempty"`
}

func (a AIGatewayResource) aiGatewayAlias() aiGatewayAlias {
	return aiGatewayAlias{
		Ref:            a.Ref,
		Kongctl:        a.Kongctl,
		External:       a.External,
		DisplayName:    a.DisplayName,
		Description:    a.Description,
		ProxyURLs:      a.ProxyUrls,
		Labels:         a.Labels,
		Providers:      a.Providers,
		Policies:       a.Policies,
		Consumers:      a.Consumers,
		ConsumerGroups: a.ConsumerGroups,
		Models:         a.Models,
		MCPServers:     a.MCPServers,
		Vaults:         a.Vaults,
	}
}

// UnmarshalYAML decodes AI Gateway fields explicitly because the SDK request
// type only carries JSON tags.
func (a *AIGatewayResource) UnmarshalYAML(unmarshal func(any) error) error {
	var raw struct {
		Ref            string                           `yaml:"ref"`
		Kongctl        *KongctlMeta                     `yaml:"kongctl,omitempty"`
		External       *ExternalBlock                   `yaml:"_external,omitempty"`
		DisplayName    string                           `yaml:"display_name"`
		Description    *string                          `yaml:"description,omitempty"`
		ProxyURLs      []kkComps.AIGatewayProxyURL      `yaml:"proxy_urls,omitempty"`
		Labels         map[string]string                `yaml:"labels,omitempty"`
		Providers      []AIGatewayProviderResource      `yaml:"providers,omitempty"`
		Policies       []AIGatewayPolicyResource        `yaml:"policies,omitempty"`
		Consumers      []AIGatewayConsumerResource      `yaml:"consumers,omitempty"`
		ConsumerGroups []AIGatewayConsumerGroupResource `yaml:"consumer_groups,omitempty"`
		Models         []AIGatewayModelResource         `yaml:"models,omitempty"`
		MCPServers     []AIGatewayMCPServerResource     `yaml:"mcp_servers,omitempty"`
		Vaults         []AIGatewayVaultResource         `yaml:"vaults,omitempty"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	a.BaseResource = BaseResource{
		Ref:     raw.Ref,
		Kongctl: raw.Kongctl,
	}
	a.External = raw.External
	a.CreateAIGatewayRequest = kkComps.CreateAIGatewayRequest{
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		ProxyUrls:   raw.ProxyURLs,
		Labels:      raw.Labels,
	}
	a.Providers = raw.Providers
	a.Policies = raw.Policies
	a.Consumers = raw.Consumers
	a.ConsumerGroups = raw.ConsumerGroups
	a.Models = raw.Models
	a.MCPServers = raw.MCPServers
	a.Vaults = raw.Vaults

	return nil
}

// UnmarshalJSON decodes AI Gateways explicitly because YAML loading goes
// through JSON tags and the embedded SDK request type has a custom unmarshaler.
func (a *AIGatewayResource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref            string                           `json:"ref"`
		Kongctl        *KongctlMeta                     `json:"kongctl,omitempty"`
		External       *ExternalBlock                   `json:"_external,omitempty"`
		DisplayName    string                           `json:"display_name"`
		Description    *string                          `json:"description,omitempty"`
		ProxyURLs      []kkComps.AIGatewayProxyURL      `json:"proxy_urls,omitempty"`
		Labels         map[string]string                `json:"labels,omitempty"`
		Providers      []AIGatewayProviderResource      `json:"providers,omitempty"`
		Policies       []AIGatewayPolicyResource        `json:"policies,omitempty"`
		Consumers      []AIGatewayConsumerResource      `json:"consumers,omitempty"`
		ConsumerGroups []AIGatewayConsumerGroupResource `json:"consumer_groups,omitempty"`
		Models         []AIGatewayModelResource         `json:"models,omitempty"`
		MCPServers     []AIGatewayMCPServerResource     `json:"mcp_servers,omitempty"`
		Vaults         []AIGatewayVaultResource         `json:"vaults,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.BaseResource = BaseResource{
		Ref:     raw.Ref,
		Kongctl: raw.Kongctl,
	}
	a.External = raw.External
	a.CreateAIGatewayRequest = kkComps.CreateAIGatewayRequest{
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		ProxyUrls:   raw.ProxyURLs,
		Labels:      raw.Labels,
	}
	a.Providers = raw.Providers
	a.Policies = raw.Policies
	a.Consumers = raw.Consumers
	a.ConsumerGroups = raw.ConsumerGroups
	a.Models = raw.Models
	a.MCPServers = raw.MCPServers
	a.Vaults = raw.Vaults

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

// IsExternal returns true if this AI Gateway is externally managed.
func (a AIGatewayResource) IsExternal() bool {
	return a.External != nil && a.External.IsExternal()
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
	if a.External != nil {
		if err := a.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
		return nil
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
	for i := range a.Policies {
		a.Policies[i].SetDefaults()
	}
	for i := range a.Consumers {
		a.Consumers[i].SetDefaults()
	}
	for i := range a.ConsumerGroups {
		a.ConsumerGroups[i].SetDefaults()
	}
	for i := range a.Models {
		a.Models[i].SetDefaults()
	}
	for i := range a.MCPServers {
		a.MCPServers[i].SetDefaults()
	}
	for i := range a.Vaults {
		a.Vaults[i].SetDefaults()
	}
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (a AIGatewayResource) GetKonnectMonikerFilter() string {
	if a.IsExternal() {
		return ""
	}
	return a.DisplayName
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (a *AIGatewayResource) TryMatchKonnectResource(konnectResource any) bool {
	if a.IsExternal() {
		if id, ok := tryMatchByNameWithExternal(a.DisplayName, konnectResource, matchOptions{}, a.External); ok {
			a.SetKonnectID(id)
			return true
		}
	}
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
		explainField("providers", explainArrayOf(aiGatewayProviderInlineExplainNode()), false, false),
		explainField("policies", explainArrayOf(aiGatewayPolicyInlineExplainNode()), false, false),
		explainField("consumers", explainArrayOf(aiGatewayConsumerInlineExplainNode()), false, false),
		explainField("consumer_groups", explainArrayOf(aiGatewayConsumerGroupInlineExplainNode()), false, false),
		explainField("models", explainArrayOf(&ExplainNode{Kind: explainKindObject}), false, false),
		explainField("mcp_servers", explainArrayOf(aiGatewayMCPServerInlineExplainNode()), false, false),
		explainField("vaults", explainArrayOf(aiGatewayVaultInlineExplainNode()), false, false),
	), nil
}

func aiGatewayProviderInlineExplainNode() *ExplainNode {
	node, err := aiGatewayProviderExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

func aiGatewayPolicyInlineExplainNode() *ExplainNode {
	node, err := aiGatewayPolicyExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

func aiGatewayConsumerGroupInlineExplainNode() *ExplainNode {
	node, err := aiGatewayConsumerGroupExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

func aiGatewayConsumerInlineExplainNode() *ExplainNode {
	node, err := aiGatewayConsumerExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

func aiGatewayMCPServerInlineExplainNode() *ExplainNode {
	node, err := aiGatewayMCPServerExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

func aiGatewayVaultInlineExplainNode() *ExplainNode {
	node, err := aiGatewayVaultExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

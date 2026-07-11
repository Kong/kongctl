package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayModelProvidersField  = "model_providers"
	aiGatewayLegacyProvidersField = "providers"
)

func init() {
	registerResourceType(
		ResourceTypeAIGateway,
		func(rs *ResourceSet) *[]AIGatewayResource { return &rs.AIGateways },
		AutoExplain[AIGatewayResource](
			WithExplainAliases("ai_gateways", "ai-gateway", "ai-gateways", "aigw"),
			WithExplainRecommendedFields("ref", "name", "display_name"),
			WithExplainSchemaBuilder(aiGatewayExplainNode),
		),
	)
}

// AIGatewayResource represents a Konnect AI Gateway in declarative configuration.
type AIGatewayResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	kkComps.CreateAIGatewayRequest
	External              *ExternalBlock                          `yaml:"_external,omitempty" json:"_external,omitempty"`
	Providers             []AIGatewayProviderResource             `yaml:"model_providers,omitempty" json:"model_providers,omitempty"`       //nolint:lll
	IdentityProviders     []AIGatewayIdentityProviderResource     `yaml:"identity_providers,omitempty" json:"identity_providers,omitempty"` //nolint:lll
	Policies              []AIGatewayPolicyResource               `yaml:"policies,omitempty" json:"policies,omitempty"`
	Agents                []AIGatewayAgentResource                `yaml:"agents,omitempty" json:"agents,omitempty"`
	Consumers             []AIGatewayConsumerResource             `yaml:"consumers,omitempty" json:"consumers,omitempty"`
	ConsumerGroups        []AIGatewayConsumerGroupResource        `yaml:"consumer_groups,omitempty" json:"consumer_groups,omitempty"` //nolint:lll
	Models                []AIGatewayModelResource                `yaml:"models,omitempty"    json:"models,omitempty"`
	MCPServers            []AIGatewayMCPServerResource            `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"` //nolint:lll
	Vaults                []AIGatewayVaultResource                `yaml:"vaults,omitempty" json:"vaults,omitempty"`
	DataPlaneCertificates []AIGatewayDataPlaneCertificateResource `yaml:"data_plane_certificates,omitempty" json:"data_plane_certificates,omitempty"` //nolint:lll
}

func (a AIGatewayResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.aiGatewayAlias())
}

// MarshalYAML ensures YAML output mirrors the custom JSON encoding.
func (a AIGatewayResource) MarshalYAML() (any, error) {
	return a.aiGatewayAlias(), nil
}

type aiGatewayAlias struct {
	Ref                   string                                  `json:"ref"                   yaml:"ref"`
	Kongctl               *KongctlMeta                            `json:"kongctl,omitempty"     yaml:"kongctl,omitempty"`
	External              *ExternalBlock                          `json:"_external,omitempty"   yaml:"_external,omitempty"`
	Name                  string                                  `json:"name,omitempty"        yaml:"name,omitempty"`
	DisplayName           string                                  `json:"display_name"          yaml:"display_name"`
	Description           *string                                 `json:"description,omitempty" yaml:"description,omitempty"` //nolint:lll
	ProxyURLs             []kkComps.AIGatewayProxyURL             `json:"proxy_urls,omitempty"  yaml:"proxy_urls,omitempty"`  //nolint:lll
	Labels                map[string]string                       `json:"labels,omitempty"      yaml:"labels,omitempty"`
	Providers             []AIGatewayProviderResource             `json:"model_providers,omitempty" yaml:"model_providers,omitempty"`       //nolint:lll
	IdentityProviders     []AIGatewayIdentityProviderResource     `json:"identity_providers,omitempty" yaml:"identity_providers,omitempty"` //nolint:lll
	Policies              []AIGatewayPolicyResource               `json:"policies,omitempty"    yaml:"policies,omitempty"`
	Agents                []AIGatewayAgentResource                `json:"agents,omitempty"      yaml:"agents,omitempty"`
	Consumers             []AIGatewayConsumerResource             `json:"consumers,omitempty"   yaml:"consumers,omitempty"`
	ConsumerGroups        []AIGatewayConsumerGroupResource        `json:"consumer_groups,omitempty" yaml:"consumer_groups,omitempty"` //nolint:lll
	Models                []AIGatewayModelResource                `json:"models,omitempty"      yaml:"models,omitempty"`
	MCPServers            []AIGatewayMCPServerResource            `json:"mcp_servers,omitempty" yaml:"mcp_servers,omitempty"` //nolint:lll
	Vaults                []AIGatewayVaultResource                `json:"vaults,omitempty"      yaml:"vaults,omitempty"`
	DataPlaneCertificates []AIGatewayDataPlaneCertificateResource `json:"data_plane_certificates,omitempty" yaml:"data_plane_certificates,omitempty"` //nolint:lll
}

func (a AIGatewayResource) aiGatewayAlias() aiGatewayAlias {
	return aiGatewayAlias{
		Ref:                   a.Ref,
		Kongctl:               a.Kongctl,
		External:              a.External,
		Name:                  a.Name,
		DisplayName:           a.DisplayName,
		Description:           a.Description,
		ProxyURLs:             a.ProxyUrls,
		Labels:                a.Labels,
		Providers:             a.Providers,
		IdentityProviders:     a.IdentityProviders,
		Policies:              a.Policies,
		Agents:                a.Agents,
		Consumers:             a.Consumers,
		ConsumerGroups:        a.ConsumerGroups,
		Models:                a.Models,
		MCPServers:            a.MCPServers,
		Vaults:                a.Vaults,
		DataPlaneCertificates: a.DataPlaneCertificates,
	}
}

// UnmarshalYAML decodes AI Gateway fields explicitly because the SDK request
// type only carries JSON tags.
func (a *AIGatewayResource) UnmarshalYAML(unmarshal func(any) error) error {
	var fields map[string]any
	if err := unmarshal(&fields); err != nil {
		return err
	}
	if _, ok := fields[aiGatewayLegacyProvidersField]; ok {
		return fmt.Errorf(
			"ai_gateways.%s is not supported; use ai_gateways.%s",
			aiGatewayLegacyProvidersField,
			aiGatewayModelProvidersField,
		)
	}

	var raw struct {
		Ref                   string                                  `yaml:"ref"`
		Kongctl               *KongctlMeta                            `yaml:"kongctl,omitempty"`
		External              *ExternalBlock                          `yaml:"_external,omitempty"`
		Name                  string                                  `yaml:"name,omitempty"`
		DisplayName           string                                  `yaml:"display_name"`
		Description           *string                                 `yaml:"description,omitempty"`
		ProxyURLs             []kkComps.AIGatewayProxyURL             `yaml:"proxy_urls,omitempty"`
		Labels                map[string]string                       `yaml:"labels,omitempty"`
		Providers             []AIGatewayProviderResource             `yaml:"model_providers,omitempty"`
		IdentityProviders     []AIGatewayIdentityProviderResource     `yaml:"identity_providers,omitempty"`
		Policies              []AIGatewayPolicyResource               `yaml:"policies,omitempty"`
		Agents                []AIGatewayAgentResource                `yaml:"agents,omitempty"`
		Consumers             []AIGatewayConsumerResource             `yaml:"consumers,omitempty"`
		ConsumerGroups        []AIGatewayConsumerGroupResource        `yaml:"consumer_groups,omitempty"`
		Models                []AIGatewayModelResource                `yaml:"models,omitempty"`
		MCPServers            []AIGatewayMCPServerResource            `yaml:"mcp_servers,omitempty"`
		Vaults                []AIGatewayVaultResource                `yaml:"vaults,omitempty"`
		DataPlaneCertificates []AIGatewayDataPlaneCertificateResource `yaml:"data_plane_certificates,omitempty"`
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
		Name:        raw.Name,
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		ProxyUrls:   raw.ProxyURLs,
		Labels:      raw.Labels,
	}
	a.Providers = raw.Providers
	a.IdentityProviders = raw.IdentityProviders
	a.Policies = raw.Policies
	a.Agents = raw.Agents
	a.Consumers = raw.Consumers
	a.ConsumerGroups = raw.ConsumerGroups
	a.Models = raw.Models
	a.MCPServers = raw.MCPServers
	a.Vaults = raw.Vaults
	a.DataPlaneCertificates = raw.DataPlaneCertificates

	return nil
}

// UnmarshalJSON decodes AI Gateways explicitly because YAML loading goes
// through JSON tags and the embedded SDK request type has a custom unmarshaler.
func (a *AIGatewayResource) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, ok := fields[aiGatewayLegacyProvidersField]; ok {
		return fmt.Errorf(
			"ai_gateways.%s is not supported; use ai_gateways.%s",
			aiGatewayLegacyProvidersField,
			aiGatewayModelProvidersField,
		)
	}

	var raw struct {
		Ref                   string                                  `json:"ref"`
		Kongctl               *KongctlMeta                            `json:"kongctl,omitempty"`
		External              *ExternalBlock                          `json:"_external,omitempty"`
		Name                  string                                  `json:"name,omitempty"`
		DisplayName           string                                  `json:"display_name"`
		Description           *string                                 `json:"description,omitempty"`
		ProxyURLs             []kkComps.AIGatewayProxyURL             `json:"proxy_urls,omitempty"`
		Labels                map[string]string                       `json:"labels,omitempty"`
		Providers             []AIGatewayProviderResource             `json:"model_providers,omitempty"`
		IdentityProviders     []AIGatewayIdentityProviderResource     `json:"identity_providers,omitempty"`
		Policies              []AIGatewayPolicyResource               `json:"policies,omitempty"`
		Agents                []AIGatewayAgentResource                `json:"agents,omitempty"`
		Consumers             []AIGatewayConsumerResource             `json:"consumers,omitempty"`
		ConsumerGroups        []AIGatewayConsumerGroupResource        `json:"consumer_groups,omitempty"`
		Models                []AIGatewayModelResource                `json:"models,omitempty"`
		MCPServers            []AIGatewayMCPServerResource            `json:"mcp_servers,omitempty"`
		Vaults                []AIGatewayVaultResource                `json:"vaults,omitempty"`
		DataPlaneCertificates []AIGatewayDataPlaneCertificateResource `json:"data_plane_certificates,omitempty"`
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
		Name:        raw.Name,
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		ProxyUrls:   raw.ProxyURLs,
		Labels:      raw.Labels,
	}
	a.Providers = raw.Providers
	a.IdentityProviders = raw.IdentityProviders
	a.Policies = raw.Policies
	a.Agents = raw.Agents
	a.Consumers = raw.Consumers
	a.ConsumerGroups = raw.ConsumerGroups
	a.Models = raw.Models
	a.MCPServers = raw.MCPServers
	a.Vaults = raw.Vaults
	a.DataPlaneCertificates = raw.DataPlaneCertificates

	return nil
}

// GetType returns the resource type.
func (a AIGatewayResource) GetType() ResourceType {
	return ResourceTypeAIGateway
}

// GetMoniker returns the resource moniker.
func (a AIGatewayResource) GetMoniker() string {
	return a.Name
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
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway %s", a.Ref)
	}
	return nil
}

// SetDefaults applies default values to AI Gateway resource.
func (a *AIGatewayResource) SetDefaults() {
	if a.Name == "" {
		a.Name = a.Ref
	}
	if a.DisplayName == "" {
		a.DisplayName = a.Name
	}
	for i := range a.Providers {
		a.Providers[i].SetDefaults()
	}
	for i := range a.Policies {
		a.Policies[i].SetDefaults()
	}
	for i := range a.Agents {
		a.Agents[i].SetDefaults()
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
	for i := range a.DataPlaneCertificates {
		a.DataPlaneCertificates[i].SetDefaults()
	}
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup.
func (a AIGatewayResource) GetKonnectMonikerFilter() string {
	if a.IsExternal() {
		return ""
	}
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (a *AIGatewayResource) TryMatchKonnectResource(konnectResource any) bool {
	if a.IsExternal() {
		if id, ok := tryMatchByNameWithExternal(a.Name, konnectResource, matchOptions{}, a.External); ok {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := tryMatchByField(konnectResource, "Name", a.Name); id != "" {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func aiGatewayExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainKongctlField(),
		explainField("name", explainStringNode("my-ai-gateway"), false, true),
		explainField("display_name", explainStringNode("My AI Gateway"), true, true),
		explainField("description", &ExplainNode{Kind: explainKindString, Nullable: true}, false, false),
		explainField("proxy_urls", explainArrayOf(explainObject(
			explainField("host", explainStringNode("proxy.example.com"), true, true),
			explainField("port", &ExplainNode{Kind: explainKindInteger, Literal: "443"}, true, true),
			explainField("protocol", explainStringNode("https"), true, true),
		)), false, false),
		explainField("labels", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("value"),
		}, false, false),
		explainField("model_providers", explainArrayOf(aiGatewayProviderInlineExplainNode()), false, false),
		explainField("identity_providers", explainArrayOf(aiGatewayIdentityProviderInlineExplainNode()), false, false),
		explainField("policies", explainArrayOf(aiGatewayPolicyInlineExplainNode()), false, false),
		explainField("agents", explainArrayOf(aiGatewayAgentInlineExplainNode()), false, false),
		explainField("consumers", explainArrayOf(aiGatewayConsumerInlineExplainNode()), false, false),
		explainField("consumer_groups", explainArrayOf(aiGatewayConsumerGroupInlineExplainNode()), false, false),
		explainField("models", explainArrayOf(&ExplainNode{Kind: explainKindObject}), false, false),
		explainField("mcp_servers", explainArrayOf(aiGatewayMCPServerInlineExplainNode()), false, false),
		explainField("vaults", explainArrayOf(aiGatewayVaultInlineExplainNode()), false, false),
		explainField(
			"data_plane_certificates",
			explainArrayOf(aiGatewayDataPlaneCertificateInlineExplainNode()),
			false,
			false,
		),
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

func aiGatewayAgentInlineExplainNode() *ExplainNode {
	node, err := aiGatewayAgentExplainNode(ExplainBuildContext{})
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

func aiGatewayConsumerCredentialInlineExplainNode() *ExplainNode {
	node, err := aiGatewayConsumerCredentialExplainNode(ExplainBuildContext{})
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

func aiGatewayDataPlaneCertificateInlineExplainNode() *ExplainNode {
	node, err := aiGatewayDataPlaneCertificateExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayIdentityProviderFieldName         = "name"
	aiGatewayIdentityProviderFieldType         = "type"
	aiGatewayIdentityProviderFieldDisplayName  = "display_name"
	aiGatewayIdentityProviderFieldLabels       = "labels"
	aiGatewayIdentityProviderFieldManagedBy    = "managed_by"
	aiGatewayIdentityProviderFieldConfig       = "config"
	aiGatewayIdentityProviderTypeOpenIDConnect = "openid-connect"
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
	if a.Type == aiGatewayIdentityProviderTypeOpenIDConnect {
		if issuer, ok := a.Config["issuer"].(string); !ok || strings.TrimSpace(issuer) == "" {
			return fmt.Errorf("config.issuer is required for OpenID Connect AI Gateway Identity Provider %s", a.Ref)
		}
		if salt, ok := a.Config["cache_tokens_salt"].(string); !ok || strings.TrimSpace(salt) == "" {
			return fmt.Errorf(
				"config.cache_tokens_salt is required for OpenID Connect AI Gateway Identity Provider %s",
				a.Ref,
			)
		}
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
	keyAuthSDK, err := autoExplainConcreteNode[kkComps.AIGatewayIdentityProviderKeyAuth](nil)
	if err != nil {
		return nil, err
	}
	keyAuth := aiGatewayIdentityProviderSDKExplainBranch(
		keyAuthSDK,
		"key-auth",
		"support-key-auth",
		"Support Key Auth",
	)
	openIDConnectSDK, err := autoExplainConcreteNode[kkComps.AIGatewayIdentityProviderOpenIDConnect](nil)
	if err != nil {
		return nil, err
	}
	openIDConnect := aiGatewayIdentityProviderSDKExplainBranch(
		openIDConnectSDK,
		aiGatewayIdentityProviderTypeOpenIDConnect,
		"support-oidc",
		"Support OIDC",
	)

	setExplainLiteral(keyAuth, []string{aiGatewayIdentityProviderFieldConfig, "key_names"}, "apikey")
	setExplainLiteral(openIDConnect, []string{aiGatewayIdentityProviderFieldConfig, "auth_methods"}, "bearer")
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayIdentityProviderFieldConfig, "cache_tokens_salt"},
		"support-cache-salt",
	)
	setExplainLiteral(openIDConnect, []string{aiGatewayIdentityProviderFieldConfig, "client_id"}, "support-client")
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayIdentityProviderFieldConfig, "client_secret"},
		"${OIDC_CLIENT_SECRET}",
	)
	setExplainLiteral(openIDConnect, []string{aiGatewayIdentityProviderFieldConfig, "consumer_claims"}, "sub")
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayIdentityProviderFieldConfig, "consumer_groups_claim"},
		"groups",
	)
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayIdentityProviderFieldConfig, "issuer"},
		"https://issuer.example.com",
	)
	setExplainLiteral(openIDConnect, []string{aiGatewayIdentityProviderFieldConfig, "scopes"}, "openid")

	config, ok := openIDConnect.lookup([]string{aiGatewayIdentityProviderFieldConfig})
	if ok {
		// These are supported OpenID Connect plugin fields that the API accepts
		// through the SDK config's additionalProperties contract.
		config.addField(explainField(
			"upstream_headers_claims",
			explainArrayOf(explainStringNode("sub")),
			false,
			false,
		))
		config.addField(explainField(
			"upstream_headers_names",
			explainArrayOf(explainStringNode("x-consumer-subject")),
			false,
			false,
		))
	}

	return explainUnionNode(keyAuth, openIDConnect), nil
}

func aiGatewayIdentityProviderSDKExplainBranch(
	branch *ExplainNode,
	providerType string,
	name string,
	displayName string,
) *ExplainNode {
	branch = explainWithCommonFields(
		branch,
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
	)
	explainSetConstStringField(branch, aiGatewayIdentityProviderFieldType, providerType)
	explainSetPathRequired(branch, []string{aiGatewayIdentityProviderFieldConfig})
	if providerType == aiGatewayIdentityProviderTypeOpenIDConnect {
		explainSetPathRequired(branch, []string{aiGatewayIdentityProviderFieldConfig, "issuer"})
	}
	setExplainLiteral(branch, []string{aiGatewayIdentityProviderFieldName}, name)
	setExplainLiteral(branch, []string{aiGatewayIdentityProviderFieldDisplayName}, displayName)
	return branch
}

func setExplainLiteral(node *ExplainNode, path []string, literal string) {
	target, ok := node.lookup(path)
	if !ok {
		return
	}
	for target.Kind == explainKindArray && target.Items != nil {
		target = target.Items
	}
	target.Literal = literal
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

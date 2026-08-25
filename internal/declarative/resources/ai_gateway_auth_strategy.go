package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayAuthStrategyFieldName         = "name"
	aiGatewayAuthStrategyFieldType         = "type"
	aiGatewayAuthStrategyFieldDisplayName  = "display_name"
	aiGatewayAuthStrategyFieldLabels       = "labels"
	aiGatewayAuthStrategyFieldManagedBy    = "managed_by"
	aiGatewayAuthStrategyFieldConfig       = "config"
	aiGatewayAuthStrategyTypeOpenIDConnect = "openid-connect"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayAuthStrategy,
		func(rs *ResourceSet) *[]AIGatewayAuthStrategyResource { return &rs.AIGatewayAuthStrategies },
		AutoExplain[AIGatewayAuthStrategyResource](
			WithExplainAliases(
				"ai_gateway_auth_strategies",
				"ai-gateway-auth-strategy",
				"ai-gateway-auth-strategies",
				"aigw-auth-strategy",
			),
			WithExplainRecommendedFields("ref", SchemaFieldAIGateway, "name", "type", "display_name", "config"),
			WithExplainSchemaBuilder(aiGatewayAuthStrategyExplainNode),
		),
		WithMaturity(aiGatewayMaturity),
	)
}

// AIGatewayAuthStrategyResource represents a Konnect AI Gateway Auth Strategy in declarative configuration.
type AIGatewayAuthStrategyResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level auth strategy declarations.
	AIGateway   string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name        string            `yaml:"name"                 json:"name"`
	Type        string            `yaml:"type"                 json:"type"`
	DisplayName string            `yaml:"display_name"         json:"display_name"`
	Labels      map[string]string `yaml:"labels,omitempty"     json:"labels,omitempty"`
	ManagedBy   map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
	Config      map[string]any    `yaml:"config"               json:"config"`
}

// GetType returns the resource type.
func (a AIGatewayAuthStrategyResource) GetType() ResourceType {
	return ResourceTypeAIGatewayAuthStrategy
}

// GetMoniker returns the provider name used for matching within the parent gateway.
func (a AIGatewayAuthStrategyResource) GetMoniker() string {
	return a.Name
}

// GetDependencies returns references to other resources this provider depends on.
func (a AIGatewayAuthStrategyResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

// Validate ensures the AI Gateway Auth Strategy resource is valid.
func (a AIGatewayAuthStrategyResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Auth Strategy ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Auth Strategy %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Auth Strategy %s", a.Ref)
	}
	if a.Type == "" {
		return fmt.Errorf("type is required for AI Gateway Auth Strategy %s", a.Ref)
	}
	if a.DisplayName == "" {
		return fmt.Errorf("display_name is required for AI Gateway Auth Strategy %s", a.Ref)
	}
	if a.Config == nil {
		return fmt.Errorf("config is required for AI Gateway Auth Strategy %s", a.Ref)
	}
	if a.Type == aiGatewayAuthStrategyTypeOpenIDConnect {
		if issuer, ok := a.Config["issuer"].(string); !ok || strings.TrimSpace(issuer) == "" {
			return fmt.Errorf("config.issuer is required for OpenID Connect AI Gateway Auth Strategy %s", a.Ref)
		}
		if salt, ok := a.Config["cache_tokens_salt"].(string); !ok || strings.TrimSpace(salt) == "" {
			return fmt.Errorf(
				"config.cache_tokens_salt is required for OpenID Connect AI Gateway Auth Strategy %s",
				a.Ref,
			)
		}
	}
	return nil
}

// SetDefaults applies default values to AI Gateway Auth Strategy resources.
func (a *AIGatewayAuthStrategyResource) SetDefaults() {
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
func (a AIGatewayAuthStrategyResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

// TryMatchKonnectResource attempts to match this provider with a Konnect resource.
func (a *AIGatewayAuthStrategyResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", a.Name); id != "" {
		a.SetKonnectID(id)
		return true
	}
	return false
}

// GetParentRef returns the parent AI Gateway reference.
func (a AIGatewayAuthStrategyResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayAuthStrategyResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayAuthStrategyResource) PayloadMap() (map[string]any, error) {
	payload := map[string]any{
		aiGatewayAuthStrategyFieldName:        a.Name,
		aiGatewayAuthStrategyFieldType:        a.Type,
		aiGatewayAuthStrategyFieldDisplayName: a.DisplayName,
		aiGatewayAuthStrategyFieldConfig:      a.Config,
	}
	if a.Labels != nil {
		payload[aiGatewayAuthStrategyFieldLabels] = a.Labels
	}
	if a.ManagedBy != nil {
		payload[aiGatewayAuthStrategyFieldManagedBy] = a.ManagedBy
	}
	return payload, nil
}

func (a AIGatewayAuthStrategyResource) MutablePayloadMap() (map[string]any, error) {
	return a.PayloadMap()
}

func (a AIGatewayAuthStrategyResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayAuthStrategyResource) MarshalYAML() (any, error) {
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
func (a *AIGatewayAuthStrategyResource) UnmarshalJSON(data []byte) error {
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

func aiGatewayAuthStrategyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	keyAuthSDK, err := autoExplainConcreteNode[kkComps.AIGatewayAuthStrategyKeyAuth](nil)
	if err != nil {
		return nil, err
	}
	keyAuth := aiGatewayAuthStrategySDKExplainBranch(
		keyAuthSDK,
		"key-auth",
		"support-key-auth",
		"Support Key Auth",
	)
	openIDConnectSDK, err := autoExplainConcreteNode[kkComps.AIGatewayAuthStrategyOpenIDConnect](nil)
	if err != nil {
		return nil, err
	}
	openIDConnect := aiGatewayAuthStrategySDKExplainBranch(
		openIDConnectSDK,
		aiGatewayAuthStrategyTypeOpenIDConnect,
		"support-oidc",
		"Support OIDC",
	)

	setExplainLiteral(keyAuth, []string{aiGatewayAuthStrategyFieldConfig, "key_names"}, "apikey")
	setExplainLiteral(openIDConnect, []string{aiGatewayAuthStrategyFieldConfig, "auth_methods"}, "bearer")
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayAuthStrategyFieldConfig, "cache_tokens_salt"},
		"support-cache-salt",
	)
	setExplainLiteral(openIDConnect, []string{aiGatewayAuthStrategyFieldConfig, "client_id"}, "support-client")
	explainReplacePath(
		openIDConnect,
		[]string{aiGatewayAuthStrategyFieldConfig, "client_secret"},
		explainArrayOf(explainSecretEnvNode("OIDC_CLIENT_SECRET")),
	)
	setExplainLiteral(openIDConnect, []string{aiGatewayAuthStrategyFieldConfig, "consumer_claims"}, "sub")
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayAuthStrategyFieldConfig, "consumer_groups_claim"},
		"groups",
	)
	setExplainLiteral(
		openIDConnect,
		[]string{aiGatewayAuthStrategyFieldConfig, "issuer"},
		"https://issuer.example.com",
	)
	setExplainLiteral(openIDConnect, []string{aiGatewayAuthStrategyFieldConfig, "scopes"}, "openid")

	config, ok := openIDConnect.lookup([]string{aiGatewayAuthStrategyFieldConfig})
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

func aiGatewayAuthStrategySDKExplainBranch(
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
	explainSetConstStringField(branch, aiGatewayAuthStrategyFieldType, providerType)
	explainSetPathRequired(branch, []string{aiGatewayAuthStrategyFieldConfig})
	if providerType == aiGatewayAuthStrategyTypeOpenIDConnect {
		explainSetPathRequired(branch, []string{aiGatewayAuthStrategyFieldConfig, "issuer"})
	}
	setExplainLiteral(branch, []string{aiGatewayAuthStrategyFieldName}, name)
	setExplainLiteral(branch, []string{aiGatewayAuthStrategyFieldDisplayName}, displayName)
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

func aiGatewayAuthStrategyInlineExplainNode() *ExplainNode {
	node, err := aiGatewayAuthStrategyExplainNode(ExplainBuildContext{})
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

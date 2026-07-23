package resources

import (
	"encoding/json"
	"fmt"
)

const (
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
		WithMaturity(aiGatewayMaturity),
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
	if err := validateAIGatewayProviderAuthConfig(a.Config); err != nil {
		return fmt.Errorf("invalid config for AI Gateway Model Provider %s: %w", a.Ref, err)
	}
	return nil
}

func validateAIGatewayProviderAuthConfig(config map[string]any) error {
	auth, ok := config["auth"].(map[string]any)
	if !ok {
		return nil
	}

	_, hasHeaderName := auth["header_name"]
	_, hasHeaderValue := auth["header_value"]
	if hasHeaderName || hasHeaderValue {
		return fmt.Errorf(
			"config.auth.header_name and config.auth.header_value are not supported; " +
				"use config.auth.headers[].name and config.auth.headers[].value",
		)
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
		SchemaFieldName:                   a.Name,
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
	return explainUnionNode(
		aiGatewayProviderExplainBranch("openai", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("anthropic", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("cerebras", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("cohere", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("dashscope", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("databricks", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("deepseek", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("huggingface", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("kimi", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("llama2", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("mistral", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("ollama", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("vercel", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("vllm", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("xai", aiGatewayProviderBasicConfigExplainNode()),
		aiGatewayProviderExplainBranch("bedrock", explainObject(
			explainField(
				"auth",
				explainUnionNode(aiGatewayProviderBasicAuthExplainNode(), aiGatewayProviderAWSAuthExplainNode()),
				true,
				true,
			),
		)),
		aiGatewayProviderExplainBranch("azure", explainObject(
			explainField(
				"auth",
				explainUnionNode(aiGatewayProviderBasicAuthExplainNode(), aiGatewayProviderAzureAuthExplainNode()),
				true,
				true,
			),
			explainField("instance", explainStringNode("kong-az-east"), true, true),
		)),
		aiGatewayProviderExplainBranch("gemini", aiGatewayProviderGCPConfigExplainNode()),
		aiGatewayProviderExplainBranch("vertex", aiGatewayProviderGCPConfigExplainNode()),
	), nil
}

func aiGatewayProviderExplainBranch(providerType string, config *ExplainNode) *ExplainNode {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode(providerType+"-provider"), true, true),
		explainField("type", explainConstStringNode(providerType), true, true),
		explainField("display_name", explainStringNode("AI Model Provider"), true, true),
		explainField("config", config, true, true),
		explainField("labels", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("value"),
		}, false, false),
		explainField("managed_by", &ExplainNode{
			Kind:       explainKindObject,
			Additional: explainStringNode("kongctl"),
		}, false, false),
	)
}

func aiGatewayProviderBasicConfigExplainNode() *ExplainNode {
	return explainObject(explainField("auth", aiGatewayProviderBasicAuthExplainNode(), true, true))
}

func aiGatewayProviderGCPConfigExplainNode() *ExplainNode {
	return explainObject(explainField(
		"auth",
		explainUnionNode(aiGatewayProviderBasicAuthExplainNode(), aiGatewayProviderGCPAuthExplainNode()),
		true,
		true,
	))
}

func aiGatewayProviderBasicAuthExplainNode() *ExplainNode {
	return explainObject(
		explainField("type", explainConstStringNode("basic"), true, true),
		explainField("headers", explainArrayOf(explainObject(
			explainField("name", explainStringNode("Authorization"), true, true),
			explainField("value", explainStringNode("Bearer ${MODEL_PROVIDER_API_KEY}"), false, true),
		)), false, true),
		explainField("params", explainArrayOf(explainObject(
			explainField("name", explainStringNode("api-version"), true, true),
			explainField("value", explainStringNode("2024-06-01"), false, true),
			explainField("location", &ExplainNode{
				Kind:    explainKindString,
				Enum:    []any{"body", "query"},
				Literal: "query",
			}, false, true),
		)), false, false),
	)
}

func aiGatewayProviderAWSAuthExplainNode() *ExplainNode {
	return explainObject(
		explainField("type", explainConstStringNode("aws"), true, true),
		explainField("access_key_id", explainStringNode("${AWS_ACCESS_KEY_ID}"), false, true),
		explainField("secret_access_key", explainStringNode("${AWS_SECRET_ACCESS_KEY}"), false, false),
		explainField("assume_role_arn", explainStringNode("arn:aws:iam::123456789012:role/model-provider"), false, false),
		explainField("role_session_name", explainStringNode("kong-ai-gateway"), false, false),
		explainField("sts_endpoint_url", explainStringNode("https://sts.amazonaws.com"), false, false),
		explainField("batch_role_arn", explainStringNode("arn:aws:iam::123456789012:role/batch"), false, false),
	)
}

func aiGatewayProviderAzureAuthExplainNode() *ExplainNode {
	return explainObject(
		explainField("type", explainConstStringNode("azure"), true, true),
		explainField("client_id", explainStringNode("${AZURE_CLIENT_ID}"), false, true),
		explainField("client_secret", explainStringNode("${AZURE_CLIENT_SECRET}"), false, false),
		explainField("tenant_id", explainStringNode("${AZURE_TENANT_ID}"), false, true),
		explainField("use_managed_identity", explainBoolNode("true"), false, true),
	)
}

func aiGatewayProviderGCPAuthExplainNode() *ExplainNode {
	return explainObject(
		explainField("type", explainConstStringNode("gcp"), true, true),
		explainField("service_account_json", explainStringNode("${GCP_SERVICE_ACCOUNT_JSON}"), false, false),
		explainField("metadata_url", explainStringNode("https://metadata.google.internal"), false, false),
		explainField("oauth_token_url", explainStringNode("https://oauth2.googleapis.com/token"), false, false),
		explainField("use_gcp_service_account", explainBoolNode("true"), false, true),
	)
}

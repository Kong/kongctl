package resources

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayProviderFieldType        = "type"
	aiGatewayProviderFieldDisplayName = "display_name"
	aiGatewayProviderFieldLabels      = "labels"
	aiGatewayProviderFieldManagedBy   = "managed_by"
	aiGatewayProviderFieldConfig      = "config"
)

func init() {
	registerAIGatewayChildResource(
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
		aiGatewayChildLoad[AIGatewayProviderResource]{
			extractOrder:  10,
			validateOrder: 10,
			nested:        func(gateway *AIGatewayResource) *[]AIGatewayProviderResource { return &gateway.Providers },
			setParent:     func(child *AIGatewayProviderResource, ref string) { child.AIGateway = ref },
		},
		WithNamespace(44, providerNamespaceParticipant),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]AIGatewayProviderResource {
			var groups [][]AIGatewayProviderResource
			for i := range rs.AIGateways {
				groups = append(groups, rs.AIGateways[i].Providers)
			}
			return groups
		}, providerNamespaceParticipant),
		WithIndependentChildOwnership(captureProviderNamespaceScopes,
			func(r *AIGatewayProviderResource) *KongctlMeta { return r.Kongctl }),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5}),
		WithChildSyncScope(
			ResourceTypeAIGateway,
			WithEmptyRootCollectionError(10, "each Model Provider must declare an ai_gateway parent"),
		),
	)
}

// AIGatewayProviderResource represents a Konnect AI Gateway Model Provider in declarative configuration.
type AIGatewayProviderResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level model provider declarations.
	AIGateway   string                            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name        string                            `yaml:"name"                 json:"name"`
	Type        string                            `yaml:"type"                 json:"type"`
	DisplayName string                            `yaml:"display_name"         json:"display_name"`
	Labels      map[string]string                 `yaml:"labels,omitempty"     json:"labels,omitempty"`
	ManagedBy   map[string]string                 `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
	Config      map[string]any                    `yaml:"config"               json:"config"`
	Datastores  []kkComps.AIGatewayDatastoreRef   `yaml:"datastores,omitempty" json:"datastores,omitempty"`
	TokenVault  *kkComps.AIGatewayTokenVaultInput `yaml:"token_vault,omitempty" json:"token_vault,omitempty"`
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
	if err := ValidateChildManagedBy(a.ManagedBy, true); err != nil {
		return err
	}
	if err := ValidateChildManagedBy(BuildChildManagedBy(
		GetNamespace(a.Kongctl), a.Protected(), "",
	), false); err != nil {
		return err
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
	if a.TokenVault != nil && a.TokenVault.Provider == "" {
		return fmt.Errorf("token_vault.provider is required")
	}
	if err := validateAIGatewayTokenVault(a.Datastores, a.TokenVault != nil); err != nil {
		return err
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
	if headers, ok := auth["headers"].([]any); ok && len(headers) > 1 {
		return fmt.Errorf("config.auth.headers supports at most one item")
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
	if a.DisplayName == "" {
		a.DisplayName = a.Name
	}
	if a.Type == "azure" && a.Config != nil {
		if service, ok := a.Config["service"]; !ok || service == nil {
			a.Config["service"] = "azure-openai"
		}
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
	vaultFields, err := AIGatewayTokenVaultFields(a.Datastores, a.TokenVault)
	if err != nil {
		return nil, err
	}
	maps.Copy(payload, vaultFields)
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
	if a.Kongctl != nil {
		payload["kongctl"] = a.Kongctl
	}
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
	if a.Kongctl != nil {
		payload["kongctl"] = a.Kongctl
	}
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return payload, nil
}

// UnmarshalJSON decodes the handwritten provider declaration separately from its SDK payload.
func (a *AIGatewayProviderResource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref         string                            `json:"ref"`
		AIGateway   string                            `json:"ai_gateway,omitempty"`
		Name        string                            `json:"name"`
		Type        string                            `json:"type"`
		DisplayName string                            `json:"display_name"`
		Labels      map[string]string                 `json:"labels,omitempty"`
		ManagedBy   map[string]string                 `json:"managed_by,omitempty"`
		Config      map[string]any                    `json:"config"`
		Datastores  []kkComps.AIGatewayDatastoreRef   `json:"datastores,omitempty"`
		TokenVault  *kkComps.AIGatewayTokenVaultInput `json:"token_vault,omitempty"`
		Kongctl *KongctlMeta `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: raw.Ref, Kongctl: raw.Kongctl}
	a.AIGateway = raw.AIGateway
	a.Name = raw.Name
	a.Type = raw.Type
	a.DisplayName = raw.DisplayName
	a.Labels = raw.Labels
	a.ManagedBy = raw.ManagedBy
	a.Config = raw.Config
	a.Datastores = raw.Datastores
	a.TokenVault = raw.TokenVault
	return nil
}

func aiGatewayProviderExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	sdkUnion, ok, err := autoExplainSDKUnionNode(
		reflect.TypeFor[kkComps.CreateAIGatewayModelProviderRequest](),
		nil,
		defaultExplainHints(""),
		nil,
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("SDK AI Gateway Model Provider request is no longer a union")
	}

	providerTypes := aiGatewayModelProviderSDKUnionTypes()
	if len(sdkUnion.OneOf) != len(providerTypes) {
		return nil, fmt.Errorf(
			"SDK AI Gateway Model Provider branch count changed: got %d schemas and %d union members",
			len(sdkUnion.OneOf),
			len(providerTypes),
		)
	}

	branches := make([]*ExplainNode, 0, len(sdkUnion.OneOf))
	for i, branch := range sdkUnion.OneOf {
		providerType := providerTypes[i]
		configured, err := aiGatewayProviderSDKExplainBranch(
			branch,
			providerType,
			aiGatewayModelProviderAuthTypes(providerType)...,
		)
		if err != nil {
			return nil, err
		}
		branches = append(branches, configured)
	}

	// Keep OpenAI as the primary scaffold while retaining SDK order for every
	// other branch.
	for i, branch := range branches {
		typeField, ok := branch.property(aiGatewayProviderFieldType)
		if ok && typeField.Node.Const == "openai" {
			branches[0], branches[i] = branches[i], branches[0]
			break
		}
	}
	return explainUnionNode(branches...), nil
}

func aiGatewayModelProviderSDKUnionTypes() []string {
	typ := reflect.TypeFor[kkComps.CreateAIGatewayModelProviderRequest]()
	providerTypes := make([]string, 0, typ.NumField())
	for field := range typ.Fields() {
		if field.Tag.Get("union") != "member" {
			continue
		}
		providerTypes = append(
			providerTypes,
			strings.ToLower(strings.TrimPrefix(field.Name, "AIGatewayModelProvider")),
		)
	}
	return providerTypes
}

func aiGatewayModelProviderAuthTypes(providerType string) []string {
	switch providerType {
	case "azure":
		return []string{authenticationTypeBasic, providerType}
	case "bedrock":
		return []string{authenticationTypeBasic, "aws"}
	case "gemini":
		return []string{authenticationTypeBasic, "gcp"}
	case "sagemaker":
		return []string{authenticationTypeBasic, "sagemaker"}
	default:
		return []string{authenticationTypeBasic}
	}
}

func aiGatewayProviderSDKExplainBranch(
	branch *ExplainNode,
	providerType string,
	authTypes ...string,
) (*ExplainNode, error) {
	branch = explainWithCommonFields(
		branch,
		explainResourceRefField(),
		explainKongctlField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
	)
	configureAIGatewayTokenVaultExplain(branch)
	explainSetConstStringField(branch, aiGatewayProviderFieldType, providerType)
	setExplainLiteral(branch, []string{SchemaFieldName}, providerType+"-provider")
	setExplainLiteral(branch, []string{aiGatewayProviderFieldDisplayName}, "AI Model Provider")

	auth, ok := branch.lookup([]string{aiGatewayProviderFieldConfig, "auth"})
	if !ok {
		return nil, fmt.Errorf("SDK AI Gateway Model Provider %s schema has no config.auth field", providerType)
	}
	if len(auth.OneOf) == 0 {
		if len(authTypes) != 1 {
			return nil, fmt.Errorf("SDK AI Gateway Model Provider %s auth schema is not a union", providerType)
		}
		explainSetConstStringField(auth, "type", authTypes[0])
		configureAIGatewayProviderAuthExplain(auth)
		return branch, nil
	}
	if len(auth.OneOf) != len(authTypes) {
		return nil, fmt.Errorf(
			"SDK AI Gateway Model Provider %s auth branch count changed: got %d, configured %d",
			providerType,
			len(auth.OneOf),
			len(authTypes),
		)
	}
	for i, authType := range authTypes {
		explainSetConstStringField(auth.OneOf[i], "type", authType)
		configureAIGatewayProviderAuthExplain(auth.OneOf[i])
	}
	explainRefreshUnionAggregate(auth)
	return branch, nil
}

func configureAIGatewayProviderAuthExplain(auth *ExplainNode) {
	if headers, ok := auth.property("headers"); ok {
		headers.Recommended = true
		if value, exists := headers.Node.Items.property("value"); exists {
			value.Recommended = true
		}
		setExplainLiteral(auth, []string{"headers", SchemaFieldName}, "Authorization")
		explainReplacePath(
			auth,
			[]string{"headers", "value"},
			explainSecretEnvCompositionNode("Bearer ", "MODEL_PROVIDER_API_KEY"),
		)
	}
	explainReplacePath(auth, []string{"params", "value"}, explainSecretEnvNode("MODEL_PROVIDER_PARAMETER"))
	explainReplacePath(auth, []string{"client_secret"}, explainSecretEnvNode("AZURE_CLIENT_SECRET"))
	explainReplacePath(auth, []string{"secret_access_key"}, explainSecretEnvNode("AWS_SECRET_ACCESS_KEY"))
	explainReplacePath(auth, []string{"aws", "secret_access_key"}, explainSecretEnvNode("AWS_SECRET_ACCESS_KEY"))
	explainReplacePath(auth, []string{"session_token"}, explainSecretEnvNode("AWS_SESSION_TOKEN"))
	explainReplacePath(auth, []string{"aws", "session_token"}, explainSecretEnvNode("AWS_SESSION_TOKEN"))
	explainReplacePath(auth, []string{"service_account_json"}, explainSecretEnvNode("GCP_SERVICE_ACCOUNT_JSON"))
}

func providerNamespaceParticipant(r *AIGatewayProviderResource) NamespaceParticipant {
	return NamespaceParticipant{
		Ref: r.Ref, Label: string(ResourceTypeAIGatewayProvider),
		SupportsProtected: true, Meta: &r.Kongctl,
	}
}

func (a AIGatewayProviderResource) Protected() bool {
	return a.Kongctl.IsProtected()
}

func captureProviderNamespaceScopes(rs *ResourceSet, defaultNamespace string) {
	scope := rs.EnsureSyncScope()
	for _, gateway := range rs.AIGateways {
		var namespaces []string
		for _, provider := range gateway.Providers {
			namespaces = append(namespaces, GetNamespace(provider.Kongctl))
		}
		CaptureOwnedChildCollection(scope, ResourceTypeAIGatewayProvider, gateway.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeAIGateway, gateway.Ref, ResourceTypeAIGatewayProvider), namespaces)
	}
	for _, provider := range rs.AIGatewayProviders {
		scope.AddChildNamespace(ResourceTypeAIGatewayProvider, provider.AIGateway, GetNamespace(provider.Kongctl))
	}
}

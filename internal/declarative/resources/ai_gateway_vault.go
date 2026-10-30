package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayVaultFieldID          = "id"
	aiGatewayVaultFieldName        = "name"
	aiGatewayVaultFieldType        = "type"
	aiGatewayVaultFieldDescription = "description"
	aiGatewayVaultFieldConfig      = "config"
	aiGatewayVaultFieldLabels      = "labels"
	aiGatewayVaultFieldUpdatedAt   = "updated_at"
)

func init() {
	registerAIGatewayChildResource(
		ResourceTypeAIGatewayVault,
		func(rs *ResourceSet) *[]AIGatewayVaultResource { return &rs.AIGatewayVaults },
		AutoExplain[AIGatewayVaultResource](
			WithExplainAliases(
				"ai_gateway_vaults",
				"ai-gateway-vault",
				"ai-gateway-vaults",
				"ai_gateway.vaults",
				"aigw-vault",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"type",
				"name",
				aiGatewayVaultFieldConfig,
			),
			WithExplainSchemaBuilder(aiGatewayVaultExplainNode),
		),
		aiGatewayChildLoad[AIGatewayVaultResource]{
			extractOrder:  100,
			validateOrder: 120,
			nested:        func(gateway *AIGatewayResource) *[]AIGatewayVaultResource { return &gateway.Vaults },
			setParent:     func(child *AIGatewayVaultResource, ref string) { child.AIGateway = ref },
		},
		WithAIGatewayOwnedChildNamespace(48, ResourceTypeAIGatewayVault,
			func(g *AIGatewayResource) *[]AIGatewayVaultResource { return &g.Vaults },
			func(r *AIGatewayVaultResource) *BaseResource { return &r.BaseResource },
			func(r *AIGatewayVaultResource) string { return r.AIGateway }),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5}),
		WithChildSyncScope(
			ResourceTypeAIGateway,
			WithEmptyRootCollectionError(120, "each Vault must declare an ai_gateway parent"),
		),
	)
}

// AIGatewayVaultResource represents a Vault nested under a Konnect AI Gateway.
type AIGatewayVaultResource struct {
	BaseResource `       yaml:",inline"              json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayVaultRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayVaultResource) GetType() ResourceType {
	return ResourceTypeAIGatewayVault
}

func (a AIGatewayVaultResource) GetMoniker() string {
	return a.Name()
}

func (a AIGatewayVaultResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayVaultResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayVaultResource) GetReferenceFieldMappings() map[string]string {
	mappings := map[string]string{
		SchemaFieldConfig + "." + SchemaFieldConfigStoreID: string(ResourceTypeAIGatewayConfigStore),
	}
	if a.AIGateway != "" {
		mappings[SchemaFieldAIGateway] = string(ResourceTypeAIGateway)
	}
	return mappings
}

func (a AIGatewayVaultResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Vault ref: %w", err)
	}
	if err := validateChildDeclarationManagedBy(
		aiGatewayPolicyStringMapField(a.CreateRequest(), "managed_by"), a.Kongctl,
	); err != nil {
		return err
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Vault %s", a.Ref)
	}
	if a.Name() == "" {
		return fmt.Errorf("name is required for AI Gateway Vault %s", a.Ref)
	}
	if a.VaultType() == "" {
		return fmt.Errorf("type is required for AI Gateway Vault %s", a.Ref)
	}
	if !a.hasPayload() {
		return fmt.Errorf("AI Gateway Vault %s must specify a valid Vault payload", a.Ref)
	}
	payload, err := a.PayloadMap()
	if err != nil {
		return err
	}
	if _, ok := payload[aiGatewayVaultFieldConfig]; !ok {
		return fmt.Errorf("config is required for AI Gateway Vault %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayVaultResource) SetDefaults() {
	if a == nil || !a.hasPayload() {
		return
	}

	// Defaults are best-effort; validation and planning surface malformed payloads through PayloadMap.
	payload, err := a.PayloadMap()
	if err != nil {
		return
	}
	if a.Ref == "" {
		if name, _ := payload[aiGatewayVaultFieldName].(string); name != "" {
			a.Ref = name
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	var req kkComps.CreateAIGatewayVaultRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	a.CreateAIGatewayVaultRequest = req
}

func (a AIGatewayVaultResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name())
}

func (a *AIGatewayVaultResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name()
	if name == "" {
		return false
	}
	id := AIGatewayVaultID(konnectResource)
	if id == "" {
		return false
	}
	if AIGatewayVaultName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayVaultResource) Name() string {
	return aiGatewayStringField(a.CreateAIGatewayVaultRequest, aiGatewayVaultFieldName)
}

func (a AIGatewayVaultResource) VaultType() string {
	if a.Type != "" {
		return string(a.Type)
	}
	return aiGatewayStringField(a.CreateAIGatewayVaultRequest, aiGatewayVaultFieldType)
}

func (a AIGatewayVaultResource) CreateRequest() kkComps.CreateAIGatewayVaultRequest {
	return a.CreateAIGatewayVaultRequest
}

func (a AIGatewayVaultResource) UpdateRequest() kkComps.UpdateAIGatewayVaultRequest {
	// UpdateRequest is best-effort for legacy callers; MutablePayloadMap surfaces payload errors to planners.
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayVaultRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayVaultRequest{}
	}
	var req kkComps.UpdateAIGatewayVaultRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayVaultRequest{}
	}
	return req
}

func (a AIGatewayVaultResource) PayloadMap() (map[string]any, error) {
	if !a.hasPayload() {
		return map[string]any{}, nil
	}
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway Vault payload")
}

func (a AIGatewayVaultResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayServerFields(payload)
	return payload, nil
}

func (a AIGatewayVaultResource) hasPayload() bool {
	return a.KonnectConfigStoreVault != nil ||
		a.EnvironmentVariableVault != nil ||
		a.AwsSecretsManagerVault != nil ||
		a.GoogleSecretManagerVault != nil ||
		a.AzureKeyVault != nil ||
		a.ConjurVault != nil ||
		a.HashiCorpVault != nil
}

func (a AIGatewayVaultResource) MarshalJSON() ([]byte, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	payload[SchemaFieldRef] = a.Ref
	if a.Kongctl != nil {
		payload[SchemaFieldKongctl] = a.Kongctl
	}
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return json.Marshal(payload)
}

func (a AIGatewayVaultResource) MarshalYAML() (any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	payload[SchemaFieldRef] = a.Ref
	if a.Kongctl != nil {
		payload[SchemaFieldKongctl] = a.Kongctl
	}
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return payload, nil
}

func (a *AIGatewayVaultResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var meta struct {
		Ref       string       `json:"ref"`
		AIGateway string       `json:"ai_gateway,omitempty"`
		Kongctl   *KongctlMeta `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	delete(raw, SchemaFieldRef)
	delete(raw, SchemaFieldAIGateway)
	delete(raw, SchemaFieldKongctl)

	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var req kkComps.CreateAIGatewayVaultRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref, Kongctl: meta.Kongctl}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayVaultRequest = req
	return nil
}

func AIGatewayVaultID(vault any) string {
	return aiGatewayStringField(vault, aiGatewayVaultFieldID)
}

func AIGatewayVaultName(vault any) string {
	return aiGatewayStringField(vault, aiGatewayVaultFieldName)
}

func AIGatewayVaultType(vault any) string {
	return aiGatewayStringField(vault, aiGatewayVaultFieldType)
}

func AIGatewayVaultDescription(vault any) string {
	return aiGatewayStringField(vault, aiGatewayVaultFieldDescription)
}

func AIGatewayVaultLabels(vault any) map[string]string {
	payload, err := marshalObjectToMap(vault, "AI Gateway Vault")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayVaultFieldLabels].(map[string]any)
	if !ok {
		return nil
	}
	labels := make(map[string]string, len(raw))
	for key, value := range raw {
		if stringValue, ok := value.(string); ok {
			labels[key] = stringValue
		}
	}
	return labels
}

func AIGatewayVaultUpdatedAt(vault any) time.Time {
	payload, err := marshalObjectToMap(vault, "AI Gateway Vault")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayVaultFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayVaultMutablePayloadMap(vault kkComps.AIGatewayVault) (map[string]any, error) {
	payload, err := marshalObjectToMap(vault, "AI Gateway Vault response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayServerFields(payload)
	return payload, nil
}

func AIGatewayVaultResourceFromResponse(
	gatewayRef string,
	vault kkComps.AIGatewayVault,
) (AIGatewayVaultResource, error) {
	payload, err := AIGatewayVaultMutablePayloadMap(vault)
	if err != nil {
		return AIGatewayVaultResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayVaultResource{}, err
	}
	var req kkComps.CreateAIGatewayVaultRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayVaultResource{}, err
	}

	ref := AIGatewayVaultID(vault)
	if ref == "" {
		ref = AIGatewayVaultName(vault)
	}
	return AIGatewayVaultResource{
		BaseResource:                BaseResource{Ref: ref},
		AIGateway:                   gatewayRef,
		CreateAIGatewayVaultRequest: req,
	}, nil
}

func aiGatewayVaultExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	hints := defaultExplainHints("")
	for _, field := range []string{"api_key", "token", "key", "client_secret", "secret_access_key", "secret_id"} {
		hints["config."+field] = ExplainFieldHint{
			Literal:      "!secret {source: !env VAULT_" + strings.ToUpper(field) + "}",
			PreferredTag: "!secret",
			Notes:        []string{"write-only secret; use a deferred source"},
		}
	}
	hints["config.kv"] = ExplainFieldHint{Enum: []any{"v1", "v2"}}
	hints["config.protocol"] = ExplainFieldHint{Enum: []any{"http", "https"}}
	hints["config.type"] = ExplainFieldHint{Enum: []any{"secrets"}}
	hints["config.host"] = ExplainFieldHint{Literal: "vault.example.net"}
	hints["config.port"] = ExplainFieldHint{Literal: "8200"}
	hints["config.prefix"] = ExplainFieldHint{Literal: "SUPPORT_", Recommended: new(true)}
	hints["config.region"] = ExplainFieldHint{Literal: "us-east-1", Recommended: new(true)}
	hints["config.assume_role_arn"] = ExplainFieldHint{Literal: "arn:aws:iam::123456789012:role/example-role"}
	for _, field := range []string{"endpoint_url", "sts_endpoint_url", "token_endpoint"} {
		hints["config."+field] = ExplainFieldHint{Literal: "https://vault.example.net"}
	}
	node, _, err := autoExplainSDKUnionNode(
		reflect.TypeFor[kkComps.CreateAIGatewayVaultRequest](), nil, hints, nil,
	)
	if err != nil {
		return nil, err
	}
	if err := aiGatewayVaultExplainDefaults(node, reflect.TypeFor[kkComps.CreateAIGatewayVaultRequest]()); err != nil {
		return nil, err
	}
	for _, branch := range node.OneOf {
		branch.addField(explainResourceRefField())
		branch.addField(explainKongctlField())
		branch.addField(explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true))
		setExplainLiteral(branch, []string{"name"}, "support-vault")
		setExplainLiteral(branch, []string{"config", "project_id"}, "my-project")
		setExplainLiteral(branch, []string{"config", "vault_uri"}, "https://vault.example.net")
		setExplainLiteral(branch, []string{"config", "location"}, "eastus")
		explainReplacePath(branch, []string{"config", "config_store_id"}, aiGatewayConfigStoreIDExplainNode())
	}
	// Keep the environment vault as the active scaffold example.
	for i, branch := range node.OneOf {
		if field, ok := branch.property("type"); ok && field.Node.Const == "env" {
			node.OneOf[0], node.OneOf[i] = node.OneOf[i], node.OneOf[0]
			break
		}
	}
	return node, nil
}

// The SDK represents optional API fields with pointers, and applies literal
// defaults during marshaling. Reflect those defaults in discovery without
// treating optional fields as nullable or changing manifest defaulting.
func aiGatewayVaultExplainDefaults(node *ExplainNode, typ reflect.Type) error {
	typ = derefExplainType(typ)
	node.Nullable = false
	if typ.Kind() != reflect.Struct {
		return nil
	}
	branchIndex := 0
	for field := range typ.Fields() {
		if field.Tag.Get("union") == "member" {
			if err := aiGatewayVaultExplainDefaults(node.OneOf[branchIndex], field.Type); err != nil {
				return err
			}
			branchIndex++
			continue
		}
		name, _, _, skip := explainFieldName(field, "json")
		if skip || name == "" {
			continue
		}
		property, ok := node.property(name)
		if !ok {
			continue
		}
		if err := aiGatewayVaultExplainDefaults(property.Node, field.Type); err != nil {
			return err
		}
		if literal, ok := field.Tag.Lookup("default"); ok {
			var value any = literal
			if derefExplainType(field.Type).Kind() != reflect.String {
				if err := json.Unmarshal([]byte(literal), &value); err != nil {
					return fmt.Errorf("decode SDK default for AI Gateway Vault %s: %w", name, err)
				}
			}
			property.Node.Default = value
			property.Node.Literal = literal
			if literal == "" {
				property.Node.Literal = `""`
			}
			property.Required = false
		}
	}
	return nil
}

func aiGatewayConfigStoreIDExplainNode() *ExplainNode {
	node := explainStringNode("config-store-id")
	node.RefKind = string(ResourceTypeAIGatewayConfigStore)
	node.PreferredTag = yamlTagRef
	node.Relationship = &ExplainRelationship{
		Target:       ResourceTypeAIGatewayConfigStore,
		Kind:         RelationshipKindAPIForeignKey,
		AcceptedTags: []string{yamlTagRef},
	}
	return node
}

func (a AIGatewayVaultResource) Protected() bool {
	return a.Kongctl.IsProtected()
}

func (a AIGatewayVaultResource) PublicLabels() map[string]string {
	if a.KonnectConfigStoreVault != nil {
		return a.KonnectConfigStoreVault.Labels
	}
	if a.EnvironmentVariableVault != nil {
		return a.EnvironmentVariableVault.Labels
	}
	if a.AwsSecretsManagerVault != nil {
		return a.AwsSecretsManagerVault.Labels
	}
	if a.GoogleSecretManagerVault != nil {
		return a.GoogleSecretManagerVault.Labels
	}
	if a.AzureKeyVault != nil {
		return a.AzureKeyVault.Labels
	}
	if a.ConjurVault != nil {
		return a.ConjurVault.Labels
	}
	if a.HashiCorpVault != nil {
		return a.HashiCorpVault.Labels
	}
	return nil
}

func AIGatewayVaultManagedBy(value any) map[string]string {
	return aiGatewayPolicyStringMapField(value, "managed_by")
}

func (a *AIGatewayVaultResource) SetChildManagedBy(metadata map[string]string) {
	if a.KonnectConfigStoreVault != nil {
		a.KonnectConfigStoreVault.ManagedBy = metadata
		return
	}
	if a.EnvironmentVariableVault != nil {
		a.EnvironmentVariableVault.ManagedBy = metadata
		return
	}
	if a.AwsSecretsManagerVault != nil {
		a.AwsSecretsManagerVault.ManagedBy = metadata
		return
	}
	if a.GoogleSecretManagerVault != nil {
		a.GoogleSecretManagerVault.ManagedBy = metadata
		return
	}
	if a.AzureKeyVault != nil {
		a.AzureKeyVault.ManagedBy = metadata
		return
	}
	if a.ConjurVault != nil {
		a.ConjurVault.ManagedBy = metadata
		return
	}
	if a.HashiCorpVault != nil {
		a.HashiCorpVault.ManagedBy = metadata
		return
	}
}

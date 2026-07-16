package resources

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
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
	registerResourceType(
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
		WithMaturity(aiGatewayMaturity),
	)
}

// AIGatewayVaultResource represents a Vault nested under a Konnect AI Gateway.
type AIGatewayVaultResource struct {
	BaseResource `yaml:",inline" json:",inline"`
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
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayVaultResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Vault ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Vault %s", a.Ref)
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
	if name, _ := payload[aiGatewayVaultFieldName].(string); name == "" && a.Ref != "" {
		payload[aiGatewayVaultFieldName] = a.Ref
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
	if id := AIGatewayVaultID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayVaultID(konnectResource); id != "" && AIGatewayVaultName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayVaultResource) Name() string {
	return aiGatewayVaultStringField(a.CreateAIGatewayVaultRequest, aiGatewayVaultFieldName)
}

func (a AIGatewayVaultResource) VaultType() string {
	if a.Type != "" {
		return string(a.Type)
	}
	return aiGatewayVaultStringField(a.CreateAIGatewayVaultRequest, aiGatewayVaultFieldType)
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
	stripAIGatewayVaultServerFields(payload)
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
		Ref       string          `json:"ref"`
		AIGateway string          `json:"ai_gateway,omitempty"`
		Kongctl   json.RawMessage `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}
	if len(meta.Kongctl) > 0 && string(meta.Kongctl) != jsonNullLiteral {
		return fmt.Errorf("kongctl metadata not supported on child resources")
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

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayVaultRequest = req
	return nil
}

func AIGatewayVaultID(vault any) string {
	return aiGatewayVaultStringField(vault, aiGatewayVaultFieldID)
}

func AIGatewayVaultName(vault any) string {
	return aiGatewayVaultStringField(vault, aiGatewayVaultFieldName)
}

func AIGatewayVaultType(vault any) string {
	return aiGatewayVaultStringField(vault, aiGatewayVaultFieldType)
}

func AIGatewayVaultDescription(vault any) string {
	return aiGatewayVaultStringField(vault, aiGatewayVaultFieldDescription)
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
	stripAIGatewayVaultServerFields(payload)
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

func stripAIGatewayVaultServerFields(payload map[string]any) {
	delete(payload, aiGatewayVaultFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, aiGatewayVaultFieldUpdatedAt)
}

func aiGatewayVaultStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Vault")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayVaultExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	commonFields := []*ExplainField{
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("support-env"), true, true),
		explainField("description", &ExplainNode{Kind: explainKindString, Nullable: true}, false, false),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	}

	return explainUnionNode(
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("env"), true, true),
			explainField("config", explainObject(
				explainField("prefix", explainStringNode("SUPPORT_"), false, true),
				explainField("base64_decode", explainBoolNode("false"), false, false),
			), true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("konnect"), true, true),
			explainField("config", explainObject(
				explainField("config_store_id", explainStringNode("config-store-id"), true, true),
			), true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("aws"), true, true),
			explainField("config", explainObject(
				explainField("region", explainStringNode("us-east-1"), false, true),
			), true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("gcp"), true, true),
			explainField("config", explainObject(
				explainField("project_id", explainStringNode("my-project"), true, true),
			), true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("azure"), true, true),
			explainField("config", explainObject(
				explainField("vault_uri", explainStringNode("https://vault.example.net"), true, true),
			), true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("conjur"), true, true),
			explainField("config", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, true, true),
		)...),
		explainObject(append(
			slices.Clone(commonFields),
			explainField("type", explainConstStringNode("hcv"), true, true),
			explainField("config", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, true, true),
		)...),
	), nil
}

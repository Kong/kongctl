package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	aiGatewayDatastoreFieldID          = "id"
	aiGatewayDatastoreFieldName        = "name"
	aiGatewayDatastoreFieldType        = "type"
	aiGatewayDatastoreFieldDescription = "description"
	aiGatewayDatastoreFieldConfig      = "config"
	aiGatewayDatastoreFieldLabels      = "labels"
	aiGatewayDatastoreFieldUpdatedAt   = "updated_at"
)

func init() {
	registerAIGatewayChildResource(
		ResourceTypeAIGatewayDatastore,
		func(rs *ResourceSet) *[]AIGatewayDatastoreResource { return &rs.AIGatewayDatastores },
		AutoExplain[AIGatewayDatastoreResource](
			WithExplainAliases(
				"ai_gateway_datastores",
				"ai-gateway-datastore",
				"ai-gateway-datastores",
				"ai_gateway.datastores",
				"aigw-datastore",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"type",
				"name",
				aiGatewayDatastoreFieldConfig,
			),
			WithExplainSchemaBuilder(aiGatewayDatastoreExplainNode),
		),
		aiGatewayChildLoad[AIGatewayDatastoreResource]{
			extractOrder:  105,
			validateOrder: 125,
			nested:        func(gateway *AIGatewayResource) *[]AIGatewayDatastoreResource { return &gateway.Datastores },
			setParent:     func(child *AIGatewayDatastoreResource, ref string) { child.AIGateway = ref },
		},
		withReferenceValidation(80),
		WithExternalUnsupportedReason(
			"AI Gateway datastore lookup requires gateway-scoped name materialization",
		),
		WithChildSyncScope(
			ResourceTypeAIGateway,
			WithEmptyRootCollectionError(125, "each Datastore must declare an ai_gateway parent"),
		),
	)
}

// AIGatewayDatastoreResource represents a Datastore nested under a Konnect AI Gateway.
type AIGatewayDatastoreResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayDatastoreRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayDatastoreResource) GetType() ResourceType {
	return ResourceTypeAIGatewayDatastore
}

func (a AIGatewayDatastoreResource) GetMoniker() string {
	return a.Name()
}

func (a AIGatewayDatastoreResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayDatastoreResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayDatastoreResource) GetReferenceFieldMappings() map[string]string {
	mappings := map[string]string{}
	if a.AIGateway != "" {
		mappings[SchemaFieldAIGateway] = string(ResourceTypeAIGateway)
	}
	return mappings
}

func (a AIGatewayDatastoreResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Datastore ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Datastore %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Datastore %s", a.Ref)
	}
	if a.Name() == "" {
		return fmt.Errorf("name is required for AI Gateway Datastore %s", a.Ref)
	}
	if a.DatastoreType() == "" {
		return fmt.Errorf("type is required for AI Gateway Datastore %s", a.Ref)
	}
	if !a.hasPayload() {
		return fmt.Errorf("AI Gateway Datastore %s must specify a valid Datastore payload", a.Ref)
	}
	payload, err := a.PayloadMap()
	if err != nil {
		return err
	}
	if _, ok := payload[aiGatewayDatastoreFieldConfig]; !ok {
		return fmt.Errorf("config is required for AI Gateway Datastore %s", a.Ref)
	}
	if aiGatewayDatastoreStringField(a.CreateRequest(), "display_name") == "" {
		return fmt.Errorf("display_name is required for AI Gateway Datastore %s", a.Ref)
	}
	if vault := a.TokenVaultDatastore; vault != nil {
		if vault.Config.Directory == "" || vault.Config.CurrentVersion == "" || len(vault.Config.Keys) == 0 {
			return fmt.Errorf("token-vault config requires directory, current_version, and keys")
		}
		versions := map[string]bool{}
		for _, key := range vault.Config.Keys {
			if key.Version == "" || versions[key.Version] {
				return fmt.Errorf("token-vault key versions must be nonempty and unique")
			}
			versions[key.Version] = true
		}
		if !versions[vault.Config.CurrentVersion] {
			return fmt.Errorf("token-vault current_version must match a configured key version")
		}
	}

	return nil
}

func (a *AIGatewayDatastoreResource) SetDefaults() {}

func (a AIGatewayDatastoreResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name())
}

func (a *AIGatewayDatastoreResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name()
	if name == "" {
		return false
	}
	id := AIGatewayDatastoreID(konnectResource)
	if id == "" {
		return false
	}
	if AIGatewayDatastoreName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayDatastoreResource) Name() string {
	return aiGatewayDatastoreStringField(
		a.CreateAIGatewayDatastoreRequest,
		aiGatewayDatastoreFieldName,
	)
}

func (a AIGatewayDatastoreResource) DatastoreType() string {
	if a.Type != "" {
		return string(a.Type)
	}
	return aiGatewayDatastoreStringField(
		a.CreateAIGatewayDatastoreRequest,
		aiGatewayDatastoreFieldType,
	)
}

func (a AIGatewayDatastoreResource) CreateRequest() kkComps.CreateAIGatewayDatastoreRequest {
	return a.CreateAIGatewayDatastoreRequest
}

func (a AIGatewayDatastoreResource) UpdateRequest() kkComps.UpdateAIGatewayDatastoreRequest {
	// UpdateRequest is best-effort for legacy callers; MutablePayloadMap surfaces payload errors to planners.
	payload, err := a.PayloadMap()
	if err != nil || len(payload) == 0 {
		return kkComps.UpdateAIGatewayDatastoreRequest{}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return kkComps.UpdateAIGatewayDatastoreRequest{}
	}
	var req kkComps.UpdateAIGatewayDatastoreRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return kkComps.UpdateAIGatewayDatastoreRequest{}
	}
	return req
}

func (a AIGatewayDatastoreResource) PayloadMap() (map[string]any, error) {
	if !a.hasPayload() {
		return map[string]any{}, nil
	}
	return marshalObjectToMap(a.CreateRequest(), "AI Gateway Datastore payload")
}

func (a AIGatewayDatastoreResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayDatastoreServerFields(payload)
	return payload, nil
}

func (a AIGatewayDatastoreResource) hasPayload() bool {
	return a.RedisCEDatastore != nil || a.RedisEEDatastore != nil || a.VectorDBDatastore != nil ||
		a.TokenVaultDatastore != nil
}

func (a AIGatewayDatastoreResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayDatastoreResource) MarshalYAML() (any, error) {
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

func (a *AIGatewayDatastoreResource) UnmarshalJSON(data []byte) error {
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
	var req kkComps.CreateAIGatewayDatastoreRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayDatastoreRequest = req
	return nil
}

func AIGatewayDatastoreID(datastore any) string {
	return aiGatewayDatastoreStringField(datastore, aiGatewayDatastoreFieldID)
}

func AIGatewayDatastoreName(datastore any) string {
	return aiGatewayDatastoreStringField(datastore, aiGatewayDatastoreFieldName)
}

func AIGatewayDatastoreType(datastore any) string {
	return aiGatewayDatastoreStringField(datastore, aiGatewayDatastoreFieldType)
}

func AIGatewayDatastoreDescription(datastore any) string {
	return aiGatewayDatastoreStringField(datastore, aiGatewayDatastoreFieldDescription)
}

func AIGatewayDatastoreLabels(datastore any) map[string]string {
	payload, err := marshalObjectToMap(datastore, "AI Gateway Datastore")
	if err != nil {
		return nil
	}
	raw, ok := payload[aiGatewayDatastoreFieldLabels].(map[string]any)
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

func AIGatewayDatastoreUpdatedAt(datastore any) time.Time {
	payload, err := marshalObjectToMap(datastore, "AI Gateway Datastore")
	if err != nil {
		return time.Time{}
	}
	if value, ok := payload[aiGatewayDatastoreFieldUpdatedAt].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func AIGatewayDatastoreMutablePayloadMap(
	datastore kkComps.AIGatewayDatastore,
) (map[string]any, error) {
	payload, err := marshalObjectToMap(datastore, "AI Gateway Datastore response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayDatastoreServerFields(payload)
	return payload, nil
}

func AIGatewayDatastoreResourceFromResponse(
	gatewayRef string,
	datastore kkComps.AIGatewayDatastore,
) (AIGatewayDatastoreResource, error) {
	payload, err := AIGatewayDatastoreMutablePayloadMap(datastore)
	if err != nil {
		return AIGatewayDatastoreResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayDatastoreResource{}, err
	}
	var req kkComps.CreateAIGatewayDatastoreRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayDatastoreResource{}, err
	}

	ref := AIGatewayDatastoreID(datastore)
	if ref == "" {
		ref = AIGatewayDatastoreName(datastore)
	}
	return AIGatewayDatastoreResource{
		BaseResource:                    BaseResource{Ref: ref},
		AIGateway:                       gatewayRef,
		CreateAIGatewayDatastoreRequest: req,
	}, nil
}

func stripAIGatewayDatastoreServerFields(payload map[string]any) {
	delete(payload, "konnect_managed")
	delete(payload, aiGatewayDatastoreFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, aiGatewayDatastoreFieldUpdatedAt)
}

func aiGatewayDatastoreStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway Datastore")
	if err != nil {
		return ""
	}
	if field, ok := payload[key].(string); ok {
		return field
	}
	return ""
}

func aiGatewayDatastoreExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	hints := defaultExplainHints("")
	for _, field := range []string{"password", "ssl_cert_key"} {
		hints["config."+field] = ExplainFieldHint{
			Literal:      "!secret {source: !env DATASTORE_SECRET}",
			PreferredTag: "!secret",
		}
	}
	hints["config.keys.key"] = ExplainFieldHint{
		Literal:      "!secret {source: !env TOKEN_VAULT_KEY}",
		PreferredTag: "!secret",
	}
	node, _, err := autoExplainSDKUnionNode(
		reflect.TypeFor[kkComps.CreateAIGatewayDatastoreRequest](),
		nil,
		hints,
		nil,
	)
	if err != nil {
		return nil, err
	}
	if err := aiGatewayVaultExplainDefaults(node, reflect.TypeFor[kkComps.CreateAIGatewayDatastoreRequest]()); err != nil {
		return nil, err
	}
	for _, branch := range node.OneOf {
		branch.addField(explainResourceRefField())
		branch.addField(explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true))
	}
	explainRefreshUnionAggregate(node)
	return node, nil
}

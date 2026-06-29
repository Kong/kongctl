package resources

import (
	"encoding/json"
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayModel,
		func(rs *ResourceSet) *[]AIGatewayModelResource { return &rs.AIGatewayModels },
		AutoExplain[AIGatewayModelResource](
			WithExplainAliases(
				"ai_gateway_models",
				"ai-gateway-model",
				"ai-gateway-models",
				"ai_gateway.models",
			),
			WithExplainRecommendedFields("ref", SchemaFieldAIGateway, "type", "name", "display_name"),
			WithExplainSchemaBuilder(aiGatewayModelExplainNode),
		),
	)
}

// AIGatewayModelResource represents a model nested under a Konnect AI Gateway.
type AIGatewayModelResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayModelRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayModelResource) GetType() ResourceType {
	return ResourceTypeAIGatewayModel
}

func (a AIGatewayModelResource) GetMoniker() string {
	return a.Name()
}

func (a AIGatewayModelResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayModelResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayModelResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway model ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway model %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway model %s", a.Ref)
	}
	if a.Name() == "" {
		return fmt.Errorf("name is required for AI Gateway model %s", a.Ref)
	}
	if a.DisplayName() == "" {
		return fmt.Errorf("display_name is required for AI Gateway model %s", a.Ref)
	}
	if a.ModelType() == "" {
		return fmt.Errorf("type is required for AI Gateway model %s", a.Ref)
	}
	if !a.hasPayload() {
		return fmt.Errorf("AI Gateway model %s must specify a valid api or model payload", a.Ref)
	}
	return nil
}

func (a *AIGatewayModelResource) SetDefaults() {
	if a == nil {
		return
	}
	if a.Ref == "" {
		a.Ref = a.Name()
	}

	enabled := true
	if a.AIGatewayModelAPI != nil {
		if a.AIGatewayModelAPI.Name == "" {
			a.AIGatewayModelAPI.Name = a.Ref
		}
		if a.AIGatewayModelAPI.DisplayName == "" {
			a.AIGatewayModelAPI.DisplayName = a.AIGatewayModelAPI.Name
		}
		if a.AIGatewayModelAPI.Enabled == nil {
			a.AIGatewayModelAPI.Enabled = &enabled
		}
		a.Type = kkComps.CreateAIGatewayModelRequestTypeAPI
		a.AIGatewayModelAPI.Type = kkComps.AIGatewayModelAPITypeAPI
	}
	if a.AIGatewayModelModel != nil {
		if a.AIGatewayModelModel.Name == "" {
			a.AIGatewayModelModel.Name = a.Ref
		}
		if a.AIGatewayModelModel.DisplayName == "" {
			a.AIGatewayModelModel.DisplayName = a.AIGatewayModelModel.Name
		}
		if a.AIGatewayModelModel.Enabled == nil {
			a.AIGatewayModelModel.Enabled = &enabled
		}
		a.Type = kkComps.CreateAIGatewayModelRequestTypeModel
		a.AIGatewayModelModel.Type = kkComps.AIGatewayModelModelTypeModel
	}
}

func (a AIGatewayModelResource) GetKonnectMonikerFilter() string {
	return a.Name()
}

func (a *AIGatewayModelResource) TryMatchKonnectResource(konnectResource any) bool {
	name := a.Name()
	if name == "" {
		return false
	}
	if id := AIGatewayModelID(konnectResource); id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id := AIGatewayModelID(konnectResource); id != "" && AIGatewayModelName(konnectResource) == name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayModelResource) Name() string {
	if a.AIGatewayModelAPI != nil {
		return a.AIGatewayModelAPI.Name
	}
	if a.AIGatewayModelModel != nil {
		return a.AIGatewayModelModel.Name
	}
	return ""
}

func (a AIGatewayModelResource) DisplayName() string {
	if a.AIGatewayModelAPI != nil {
		return a.AIGatewayModelAPI.DisplayName
	}
	if a.AIGatewayModelModel != nil {
		return a.AIGatewayModelModel.DisplayName
	}
	return ""
}

func (a AIGatewayModelResource) ModelType() string {
	if a.AIGatewayModelAPI != nil || a.Type == kkComps.CreateAIGatewayModelRequestTypeAPI {
		return string(kkComps.CreateAIGatewayModelRequestTypeAPI)
	}
	if a.AIGatewayModelModel != nil || a.Type == kkComps.CreateAIGatewayModelRequestTypeModel {
		return string(kkComps.CreateAIGatewayModelRequestTypeModel)
	}
	return ""
}

func (a AIGatewayModelResource) CreateRequest() kkComps.CreateAIGatewayModelRequest {
	if a.AIGatewayModelAPI != nil {
		return kkComps.CreateCreateAIGatewayModelRequestAPI(*a.AIGatewayModelAPI)
	}
	if a.AIGatewayModelModel != nil {
		return kkComps.CreateCreateAIGatewayModelRequestModel(*a.AIGatewayModelModel)
	}
	return kkComps.CreateAIGatewayModelRequest{}
}

func (a AIGatewayModelResource) UpdateRequest() kkComps.UpdateAIGatewayModelRequest {
	if a.AIGatewayModelAPI != nil {
		return kkComps.CreateUpdateAIGatewayModelRequestAPI(*a.AIGatewayModelAPI)
	}
	if a.AIGatewayModelModel != nil {
		return kkComps.CreateUpdateAIGatewayModelRequestModel(*a.AIGatewayModelModel)
	}
	return kkComps.UpdateAIGatewayModelRequest{}
}

func (a AIGatewayModelResource) PayloadMap() (map[string]any, error) {
	req := a.CreateRequest()
	if req.AIGatewayModelAPI == nil && req.AIGatewayModelModel == nil {
		return map[string]any{}, nil
	}
	return marshalObjectToMap(req, "AI Gateway model payload")
}

func (a AIGatewayModelResource) MutablePayloadMap() (map[string]any, error) {
	payload, err := a.PayloadMap()
	if err != nil {
		return nil, err
	}
	stripAIGatewayModelServerFields(payload)
	return payload, nil
}

func (a AIGatewayModelResource) hasPayload() bool {
	return a.AIGatewayModelAPI != nil || a.AIGatewayModelModel != nil
}

func (a AIGatewayModelResource) MarshalJSON() ([]byte, error) {
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

func (a AIGatewayModelResource) MarshalYAML() (any, error) {
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

func (a *AIGatewayModelResource) UnmarshalJSON(data []byte) error {
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
	delete(raw, "kongctl")
	if err := normalizeAIGatewayModelPayloadAliases(raw); err != nil {
		return err
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var req kkComps.CreateAIGatewayModelRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayModelRequest = req
	return nil
}

func normalizeAIGatewayModelPayloadAliases(raw map[string]json.RawMessage) error {
	if _, ok := raw["targets"]; ok {
		delete(raw, "target_models")
	} else if targets, ok := raw["target_models"]; ok {
		raw["targets"] = targets
		delete(raw, "target_models")
	}

	if err := normalizeAIGatewayModelAlias(raw); err != nil {
		return err
	}
	return nil
}

func normalizeAIGatewayModelAlias(raw map[string]json.RawMessage) error {
	name, ok, err := rawStringField(raw, "name")
	if err != nil || !ok || name == "" {
		return err
	}

	configRaw, ok := raw["config"]
	if !ok || isJSONNull(configRaw) {
		return nil
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config: %w", err)
	}

	modelRaw, ok := config["model"]
	if !ok || isJSONNull(modelRaw) {
		modelRaw = []byte(`{}`)
	}
	var model map[string]json.RawMessage
	if err := json.Unmarshal(modelRaw, &model); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config.model: %w", err)
	}
	if _, ok := model["alias"]; ok {
		return nil
	}

	alias, err := json.Marshal(name)
	if err != nil {
		return err
	}
	model["alias"] = alias

	encodedModel, err := json.Marshal(model)
	if err != nil {
		return err
	}
	config["model"] = encodedModel

	encodedConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}
	raw["config"] = encodedConfig
	return nil
}

func rawStringField(raw map[string]json.RawMessage, field string) (string, bool, error) {
	valueRaw, ok := raw[field]
	if !ok || isJSONNull(valueRaw) {
		return "", ok, nil
	}
	var value string
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return "", true, fmt.Errorf("failed to decode AI Gateway model %s: %w", field, err)
	}
	return value, true, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(raw) == jsonNullLiteral
}

func AIGatewayModelID(model any) string {
	switch m := model.(type) {
	case kkComps.AIGatewayModel:
		if m.AIGatewayModelAIGatewayModelAPI != nil {
			return m.AIGatewayModelAIGatewayModelAPI.ID
		}
		if m.AIGatewayModelAIGatewayModelModel != nil {
			return m.AIGatewayModelAIGatewayModelModel.ID
		}
	case *kkComps.AIGatewayModel:
		if m == nil {
			return ""
		}
		return AIGatewayModelID(*m)
	case kkComps.AIGatewayModelAIGatewayModelAPI:
		return m.ID
	case *kkComps.AIGatewayModelAIGatewayModelAPI:
		if m != nil {
			return m.ID
		}
	case kkComps.AIGatewayModelAIGatewayModelModel:
		return m.ID
	case *kkComps.AIGatewayModelAIGatewayModelModel:
		if m != nil {
			return m.ID
		}
	}
	return ""
}

func AIGatewayModelName(model any) string {
	switch m := model.(type) {
	case kkComps.AIGatewayModel:
		if m.AIGatewayModelAIGatewayModelAPI != nil {
			return m.AIGatewayModelAIGatewayModelAPI.Name
		}
		if m.AIGatewayModelAIGatewayModelModel != nil {
			return m.AIGatewayModelAIGatewayModelModel.Name
		}
	case *kkComps.AIGatewayModel:
		if m == nil {
			return ""
		}
		return AIGatewayModelName(*m)
	}
	return ""
}

func AIGatewayModelLabels(model any) map[string]string {
	switch m := model.(type) {
	case kkComps.AIGatewayModel:
		if m.AIGatewayModelAIGatewayModelAPI != nil {
			return m.AIGatewayModelAIGatewayModelAPI.Labels
		}
		if m.AIGatewayModelAIGatewayModelModel != nil {
			return m.AIGatewayModelAIGatewayModelModel.Labels
		}
	case *kkComps.AIGatewayModel:
		if m == nil {
			return nil
		}
		return AIGatewayModelLabels(*m)
	}
	return nil
}

func AIGatewayModelDisplayName(model kkComps.AIGatewayModel) string {
	if model.AIGatewayModelAIGatewayModelAPI != nil {
		return model.AIGatewayModelAIGatewayModelAPI.DisplayName
	}
	if model.AIGatewayModelAIGatewayModelModel != nil {
		return model.AIGatewayModelAIGatewayModelModel.DisplayName
	}
	return ""
}

func AIGatewayModelType(model kkComps.AIGatewayModel) string {
	return string(model.Type)
}

func AIGatewayModelEnabled(model kkComps.AIGatewayModel) *bool {
	if model.AIGatewayModelAIGatewayModelAPI != nil {
		return model.AIGatewayModelAIGatewayModelAPI.Enabled
	}
	if model.AIGatewayModelAIGatewayModelModel != nil {
		return model.AIGatewayModelAIGatewayModelModel.Enabled
	}
	return nil
}

func AIGatewayModelUpdatedAt(model kkComps.AIGatewayModel) time.Time {
	if model.AIGatewayModelAIGatewayModelAPI != nil {
		return model.AIGatewayModelAIGatewayModelAPI.UpdatedAt
	}
	if model.AIGatewayModelAIGatewayModelModel != nil {
		return model.AIGatewayModelAIGatewayModelModel.UpdatedAt
	}
	return time.Time{}
}

func AIGatewayModelMutablePayloadMap(model kkComps.AIGatewayModel) (map[string]any, error) {
	payload, err := marshalObjectToMap(model, "AI Gateway model response")
	if err != nil {
		return nil, err
	}
	stripAIGatewayModelServerFields(payload)
	return payload, nil
}

func AIGatewayModelResourceFromResponse(
	gatewayRef string,
	model kkComps.AIGatewayModel,
) (AIGatewayModelResource, error) {
	payload, err := AIGatewayModelMutablePayloadMap(model)
	if err != nil {
		return AIGatewayModelResource{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return AIGatewayModelResource{}, err
	}
	var req kkComps.CreateAIGatewayModelRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return AIGatewayModelResource{}, err
	}

	ref := AIGatewayModelID(model)
	if ref == "" {
		ref = AIGatewayModelName(model)
	}
	return AIGatewayModelResource{
		BaseResource:                BaseResource{Ref: ref},
		AIGateway:                   gatewayRef,
		CreateAIGatewayModelRequest: req,
	}, nil
}

func marshalObjectToMap(value any, label string) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s: %w", label, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", label, err)
	}
	return result, nil
}

func stripAIGatewayModelServerFields(payload map[string]any) {
	delete(payload, "id")
	delete(payload, "created_at")
	delete(payload, "updated_at")
}

func aiGatewayModelExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	commonFields := []*ExplainField{
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("support-gpt"), true, true),
		explainField("display_name", explainStringNode("Support GPT"), true, true),
		explainField("enabled", explainBoolNode("true"), false, true),
		explainField("acls", &ExplainNode{Kind: explainKindObject}, false, false),
		explainField("config", explainObject(
			explainField("route", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, true, true),
			explainField("model", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, true, true),
		), true, true),
		explainField("formats", explainArrayOf(explainObject(
			explainField("type", explainStringNode("openai"), true, true),
		)), true, true),
		explainField("target_models", explainArrayOf(explainObject(
			explainField("name", explainStringNode("gpt-4o"), true, true),
			explainField("provider", &ExplainNode{
				Kind:        explainKindString,
				Literal:     "existing-provider-name",
				Description: "AI Gateway Provider name in the parent gateway",
			}, true, true),
			explainField("config", explainObject(
				explainField("type", explainStringNode("openai"), true, true),
			), true, true),
		)), true, true),
		explainField("policies", explainArrayOf(&ExplainNode{Kind: explainKindObject}), true, false),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	}

	modelFields := append(
		slicesCloneExplainFields(commonFields),
		explainField("type", explainConstStringNode("model"), true, true),
		explainField("capabilities", explainArrayOf(explainStringNode("generate")), true, true),
	)
	apiFields := append(
		slicesCloneExplainFields(commonFields),
		explainField("type", explainConstStringNode("api"), true, true),
		explainField("capabilities", explainArrayOf(explainStringNode("files")), true, true),
	)

	return explainUnionNode(explainObject(modelFields...), explainObject(apiFields...)), nil
}

func slicesCloneExplainFields(fields []*ExplainField) []*ExplainField {
	cloned := make([]*ExplainField, len(fields))
	copy(cloned, fields)
	return cloned
}

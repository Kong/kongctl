package resources

import (
	"encoding/json"
	"fmt"
	"slices"
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
		WithMaturity(aiGatewayMaturity),
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

func (a AIGatewayModelResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
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
	return a.BaseResource.GetKonnectMonikerFilter(a.Name())
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
	delete(raw, SchemaFieldKongctl)
	if err := normalizeAIGatewayRouteModel(raw); err != nil {
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
	if err := setAIGatewayRouteModel(&req, raw); err != nil {
		return err
	}

	a.BaseResource = BaseResource{Ref: meta.Ref}
	a.AIGateway = meta.AIGateway
	a.CreateAIGatewayModelRequest = req
	return nil
}

func normalizeAIGatewayRouteModel(raw map[string]json.RawMessage) error {
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

	alias := name
	hasLegacyAlias := false
	if modelRaw, ok := config["model"]; ok && !isJSONNull(modelRaw) {
		var model map[string]json.RawMessage
		if err := json.Unmarshal(modelRaw, &model); err != nil {
			return fmt.Errorf("failed to decode AI Gateway model config.model: %w", err)
		}
		if aliasRaw, ok := model["alias"]; ok {
			hasLegacyAlias = true
			if err := json.Unmarshal(aliasRaw, &alias); err != nil {
				return fmt.Errorf("failed to decode AI Gateway model config.model.alias: %w", err)
			}
			if alias == "" {
				return fmt.Errorf("AI Gateway model config.model.alias must not be empty")
			}
			delete(model, "alias")
			encodedModel, err := json.Marshal(model)
			if err != nil {
				return err
			}
			config["model"] = encodedModel
		}
	}

	routeRaw, ok := config["route"]
	if !ok || isJSONNull(routeRaw) {
		routeRaw = []byte(`{}`)
	}
	var route map[string]json.RawMessage
	if err := json.Unmarshal(routeRaw, &route); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config.route: %w", err)
	}
	if routeModel, ok := route["model"]; ok && !isJSONNull(routeModel) {
		if hasLegacyAlias {
			return fmt.Errorf("AI Gateway model cannot specify both config.model.alias and config.route.model")
		}
		return nil
	}

	routeModel, err := json.Marshal(map[string]any{
		"body": map[string]any{
			"model": []string{alias},
		},
	})
	if err != nil {
		return err
	}
	route["model"] = routeModel

	encodedRoute, err := json.Marshal(route)
	if err != nil {
		return err
	}
	config["route"] = encodedRoute

	encodedConfig, err := json.Marshal(config)
	if err != nil {
		return err
	}
	raw["config"] = encodedConfig
	return nil
}

func setAIGatewayRouteModel(
	req *kkComps.CreateAIGatewayModelRequest,
	raw map[string]json.RawMessage,
) error {
	configRaw, ok := raw["config"]
	if !ok || isJSONNull(configRaw) {
		return nil
	}
	var config struct {
		Route struct {
			Model json.RawMessage `json:"model"`
		} `json:"route"`
	}
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config: %w", err)
	}
	if len(config.Route.Model) == 0 || isJSONNull(config.Route.Model) {
		return nil
	}

	routeModel, err := decodeAIGatewayRouteModel(config.Route.Model)
	if err != nil {
		return err
	}
	if req.AIGatewayModelAPI != nil {
		req.AIGatewayModelAPI.Config.Route.Model = &routeModel
	}
	if req.AIGatewayModelModel != nil {
		req.AIGatewayModelModel.Config.Route.Model = &routeModel
	}
	return nil
}

func decodeAIGatewayRouteModel(raw json.RawMessage) (kkComps.AIGatewayModelAliasConfig, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return kkComps.AIGatewayModelAliasConfig{},
			fmt.Errorf("failed to decode AI Gateway config.route.model: %w", err)
	}
	if len(fields) != 1 {
		return kkComps.AIGatewayModelAliasConfig{},
			fmt.Errorf("AI Gateway config.route.model must specify exactly one routing method")
	}

	if value, ok := fields["body"]; ok {
		var body map[string]any
		if err := json.Unmarshal(value, &body); err != nil {
			return kkComps.AIGatewayModelAliasConfig{},
				fmt.Errorf("failed to decode AI Gateway config.route.model.body: %w", err)
		}
		return kkComps.CreateAIGatewayModelAliasConfigAIGatewayModelAliasConfigBody(
			kkComps.AIGatewayModelAliasConfigBody{Body: body},
		), nil
	}
	if value, ok := fields["headers"]; ok {
		var headers map[string]any
		if err := json.Unmarshal(value, &headers); err != nil {
			return kkComps.AIGatewayModelAliasConfig{},
				fmt.Errorf("failed to decode AI Gateway config.route.model.headers: %w", err)
		}
		return kkComps.CreateAIGatewayModelAliasConfigAIGatewayModelAliasConfigHeaders(
			kkComps.AIGatewayModelAliasConfigHeaders{Headers: headers},
		), nil
	}
	if value, ok := fields["path_aliases"]; ok {
		var pathAliases []string
		if err := json.Unmarshal(value, &pathAliases); err != nil {
			return kkComps.AIGatewayModelAliasConfig{},
				fmt.Errorf("failed to decode AI Gateway config.route.model.path_aliases: %w", err)
		}
		return kkComps.CreateAIGatewayModelAliasConfigAIGatewayModelAliasConfigPath(
			kkComps.AIGatewayModelAliasConfigPath{PathAliases: pathAliases},
		), nil
	}
	return kkComps.AIGatewayModelAliasConfig{},
		fmt.Errorf("AI Gateway config.route.model has an unsupported routing method")
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
	delete(payload, SchemaFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, SchemaFieldUpdatedAt)
}

func aiGatewayModelExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	targetConfig, err := aiGatewayTargetConfigExplainNode()
	if err != nil {
		return nil, err
	}

	commonFields := []*ExplainField{
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("support-gpt"), true, true),
		explainField("display_name", explainStringNode("Support GPT"), true, true),
		explainField("enabled", explainBoolNode("true"), false, true),
		explainField("access", aiGatewayAccessExplainNode(true), false, false),
		explainField("formats", explainArrayOf(explainObject(
			explainField("type", explainStringNode("openai"), true, true),
		)), true, true),
		explainField("targets", explainArrayOf(explainObject(
			explainField("name", explainStringNode("gpt-4o"), true, true),
			explainField("provider", &ExplainNode{
				Kind:        explainKindString,
				Literal:     "existing-provider-name",
				Description: "AI Gateway Model Provider name in the parent gateway",
			}, true, true),
			explainField("config", targetConfig, true, true),
			explainField("weight", &ExplainNode{Kind: explainKindInteger, Literal: "100"}, false, false),
			explainField("semantic_description", explainStringNode("Primary target"), false, false),
			explainField("allow_auth_override", explainBoolNode("false"), false, false),
		)), true, true),
		explainField("policies", explainArrayOf(explainStringNode("policy-name")), false, false),
		explainField("labels", &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}, false, false),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	}

	modelFields := append(
		slices.Clone(commonFields),
		explainField("config", aiGatewayModelConfigExplainNode(true), true, true),
		explainField("type", explainConstStringNode("model"), true, true),
		explainField("capabilities", explainArrayOf(explainStringNode("generate")), true, true),
	)
	apiFields := append(
		slices.Clone(commonFields),
		explainField("config", aiGatewayModelConfigExplainNode(false), true, true),
		explainField("type", explainConstStringNode("api"), true, true),
		explainField("capabilities", explainArrayOf(explainStringNode("files")), true, true),
	)

	return explainUnionNode(explainObject(modelFields...), explainObject(apiFields...)), nil
}

func aiGatewayModelConfigExplainNode(includeModelConfig bool) *ExplainNode {
	headerModel := explainObject(explainField(
		"X-Model",
		explainArrayOf(explainStringNode("support-gpt")),
		false,
		true,
	))
	headerModel.Additional = explainArrayOf(explainStringNode("support-gpt"))

	routeModel := explainUnionNode(
		explainObject(explainField(
			"body",
			explainObject(explainField(
				"model",
				explainArrayOf(explainStringNode("support-gpt")),
				true,
				true,
			)),
			true,
			true,
		)),
		explainObject(explainField(
			"headers",
			headerModel,
			true,
			true,
		)),
		explainObject(explainField(
			"path_aliases",
			explainArrayOf(explainStringNode("support-gpt")),
			true,
			true,
		)),
	)
	route := explainObject(
		explainField("headers", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
		explainField("hosts", explainArrayOf(explainStringNode("api.example.com")), false, false),
		explainField(
			"https_redirect_status_code",
			&ExplainNode{Kind: explainKindInteger, Literal: "426"},
			false,
			false,
		),
		explainField("methods", explainArrayOf(explainStringNode("POST")), false, false),
		explainField("paths", explainArrayOf(explainStringNode("/v1/chat/completions")), false, false),
		explainField("preserve_host", explainBoolNode("false"), false, false),
		explainField("protocols", explainArrayOf(explainStringNode("https")), false, false),
		explainField("regex_priority", &ExplainNode{Kind: explainKindInteger, Literal: "0"}, false, false),
		explainField("request_buffering", explainBoolNode("true"), false, false),
		explainField("response_buffering", explainBoolNode("true"), false, false),
		explainField("strip_path", explainBoolNode("true"), false, false),
		explainField("tags", explainArrayOf(explainStringNode("ai-gateway")), false, false),
		explainField("model", routeModel, false, true),
	)

	fields := []*ExplainField{
		explainField("route", route, true, true),
		explainField("logging", explainObject(
			explainField("payloads", explainBoolNode("false"), false, false),
		), false, false),
		explainField("response_streaming", explainStringNode("allow"), false, false),
		explainField("max_request_body_size", &ExplainNode{Kind: explainKindInteger, Literal: "8388608"}, false, false),
		explainField("balancer", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
		explainField("proxy", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
	}
	if includeModelConfig {
		fields = append(fields, explainField(
			"model",
			explainObject(explainField("name_header", explainBoolNode("true"), false, false)),
			false,
			false,
		))
	}
	return explainObject(fields...)
}

func aiGatewayTargetConfigExplainNode() (*ExplainNode, error) {
	variants := []struct {
		name string
		node func() (*ExplainNode, error)
	}{
		{"anthropic", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetAnthropicConfig]("type", "anthropic")
		}},
		{"azure", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetAzureConfig]("type", "azure")
		}},
		{"bedrock", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetBedrockConfig]("type", "bedrock")
		}},
		{"cerebras", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetCerebrasConfig]("type", "cerebras")
		}},
		{"cohere", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetCohereConfig]("type", "cohere")
		}},
		{"dashscope", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetDashscopeConfig]("type", "dashscope")
		}},
		{"databricks", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetDatabricksConfig]("type", "databricks")
		}},
		{"deepseek", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetDeepseekConfig]("type", "deepseek")
		}},
		{"gemini", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetGeminiConfig]("type", "gemini")
		}},
		{"huggingface", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetHuggingfaceConfig]("type", "huggingface")
		}},
		{"kimi", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetKimiConfig]("type", "kimi")
		}},
		{"llama2", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetLlama2Config]("type", "llama2")
		}},
		{"mistral", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetMistralConfig]("type", "mistral")
		}},
		{"ollama", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetOllamaConfig]("type", "ollama")
		}},
		{"openai", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetOpenaiConfig]("type", "openai")
		}},
		{"vercel", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetVercelConfig]("type", "vercel")
		}},
		{"vertex", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetVertexConfig]("type", "vertex")
		}},
		{"vllm", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetVllmConfig]("type", "vllm")
		}},
		{"xai", func() (*ExplainNode, error) {
			return explainVariantNode[kkComps.AIGatewayTargetXaiConfig]("type", "xai")
		}},
	}

	branches := make([]*ExplainNode, 0, len(variants))
	for _, variant := range variants {
		node, err := variant.node()
		if err != nil {
			return nil, fmt.Errorf("failed to build %s AI Gateway target config schema: %w", variant.name, err)
		}
		branches = append(branches, node)
	}
	return explainUnionNode(branches...), nil
}

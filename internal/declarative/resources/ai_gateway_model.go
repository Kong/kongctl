package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerAIGatewayChildResource(
		ResourceTypeAIGatewayModel,
		func(rs *ResourceSet) *[]AIGatewayModelResource { return &rs.AIGatewayModels },
		AutoExplain[AIGatewayModelResource](
			WithExplainAliases(
				"ai_gateway_models",
				"ai-gateway-model",
				"ai-gateway-models",
				"ai_gateway.models",
			),
			WithExplainRecommendedFields(
				"ref",
				SchemaFieldAIGateway,
				"type",
				"name",
				"display_name",
			),
			WithExplainSchemaBuilder(aiGatewayModelExplainNode),
		),
		aiGatewayChildLoad[AIGatewayModelResource]{
			extractOrder:  70,
			validateOrder: 80,
			nested:        func(gateway *AIGatewayResource) *[]AIGatewayModelResource { return &gateway.Models },
			setParent:     func(child *AIGatewayModelResource, ref string) { child.AIGateway = ref },
		},
		WithNamespace(45, func(r *AIGatewayModelResource) NamespaceParticipant {
			return NamespaceParticipant{
				Ref:               r.Ref,
				Label:             "ai_gateway_model",
				SupportsProtected: true,
				Meta:              &r.Kongctl,
			}
		}),
		WithNestedNamespaceGroups(func(rs *ResourceSet) [][]AIGatewayModelResource {
			var models [][]AIGatewayModelResource
			for i := range rs.AIGateways {
				models = append(models, rs.AIGateways[i].Models)
			}
			return models
		}, func(r *AIGatewayModelResource) NamespaceParticipant {
			return NamespaceParticipant{
				Ref:               r.Ref,
				Label:             "ai_gateway_model",
				SupportsProtected: true,
				Meta:              &r.Kongctl,
			}
		}),
		WithIndependentChildOwnership(captureModelNamespaceScopes,
			func(model *AIGatewayModelResource) *KongctlMeta { return model.Kongctl }),

		WithManagedBy(ManagedByPolicy{MaxEntries: 5}),
		WithChildSyncScope(
			ResourceTypeAIGateway,
			WithEmptyRootCollectionError(80, "each model must declare an ai_gateway parent"),
		),
		// The intelligent balancer algorithm ships as a private tech preview with no public
		// GA date; kongctl does not model it. Exclude its exclusive fields from dump-default
		// omission so they stay visible rather than being treated as stable API defaults.
		WithDumpDefaultExclusion(
			"config.balancer.strategy",
			"intelligent balancer only; also marked unstable upstream and removed once a "+
				"balancing strategy is chosen",
		),
		WithDumpDefaultExclusion(
			"config.balancer.version",
			"intelligent balancer only; not a public GA surface",
		),
		WithDumpDefaultExclusion(
			"config.balancer.cost_quality_tradeoff",
			"intelligent balancer only; not a public GA surface",
		),
		WithDumpDefaultExclusion(
			"config.balancer.classifier.timeout",
			"intelligent balancer only; not a public GA surface",
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
	if err := ValidateChildManagedBy(a.ManagedBy(), true); err != nil {
		return fmt.Errorf("AI Gateway model %s: %w", a.Ref, err)
	}
	if err := ValidateChildManagedBy(BuildChildManagedBy(
		GetNamespace(a.Kongctl),
		a.Protected(), "",
	),
		false); err != nil {
		return err
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
		if a.AIGatewayModelAPI.DisplayName == "" {
			a.AIGatewayModelAPI.DisplayName = a.AIGatewayModelAPI.Name
		}
		if a.AIGatewayModelAPI.Enabled == nil {
			a.AIGatewayModelAPI.Enabled = &enabled
		}
		a.Type = kkComps.CreateAIGatewayModelRequestTypeAPI
	}
	if a.AIGatewayModelModel != nil {
		if a.AIGatewayModelModel.DisplayName == "" {
			a.AIGatewayModelModel.DisplayName = a.AIGatewayModelModel.Name
		}
		if a.AIGatewayModelModel.Enabled == nil {
			a.AIGatewayModelModel.Enabled = &enabled
		}
		a.Type = kkComps.CreateAIGatewayModelRequestTypeModel
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
	stripAIGatewayServerFields(payload)
	normalizeAIGatewayAuthStrategyAccess(payload)
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
	if a.Kongctl != nil {
		payload[SchemaFieldKongctl] = a.Kongctl
	}
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
	if a.Kongctl != nil {
		payload[SchemaFieldKongctl] = a.Kongctl
	}
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
	if err := normalizeAIGatewayRouteModel(raw); err != nil {
		return err
	}
	if err := rejectAIGatewayConsistentHashingSessionAffinity(raw); err != nil {
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

	a.BaseResource = BaseResource{Ref: meta.Ref, Kongctl: meta.Kongctl}
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
			return fmt.Errorf(
				"AI Gateway model cannot specify both config.model.alias and config.route.model",
			)
		}
		return nil
	}
	if !hasLegacyAlias {
		return nil
	}

	routeModel, err := json.Marshal(map[string]any{
		"body_param": "model",
		"values":     []string{alias},
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

// rejectAIGatewayConsistentHashingSessionAffinity rejects config.balancer.session_affinity
// under algorithm "consistent-hashing" before the SDK's union unmarshaller silently drops the
// unrecognized key: AIGatewayModelBalancerConsistentHashingConfig has no session_affinity field
// and no additionalProperties catch-all, so the SDK accepts and discards it rather than erroring.
func rejectAIGatewayConsistentHashingSessionAffinity(raw map[string]json.RawMessage) error {
	configRaw, ok := raw["config"]
	if !ok || isJSONNull(configRaw) {
		return nil
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config: %w", err)
	}
	balancerRaw, ok := config["balancer"]
	if !ok || isJSONNull(balancerRaw) {
		return nil
	}
	var balancer map[string]json.RawMessage
	if err := json.Unmarshal(balancerRaw, &balancer); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config.balancer: %w", err)
	}
	if _, hasAffinity := balancer["session_affinity"]; !hasAffinity {
		return nil
	}
	algorithmRaw, ok := balancer["algorithm"]
	if !ok || isJSONNull(algorithmRaw) {
		return nil
	}
	var algorithm string
	if err := json.Unmarshal(algorithmRaw, &algorithm); err != nil {
		return fmt.Errorf("failed to decode AI Gateway model config.balancer.algorithm: %w", err)
	}
	if algorithm == string(kkComps.AIGatewayModelBalancerConfigTypeConsistentHashing) {
		return fmt.Errorf(
			"AI Gateway model config.balancer.session_affinity is not supported with algorithm %q",
			algorithm,
		)
	}
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

func decodeAIGatewayRouteModel(raw json.RawMessage) (kkComps.AIGatewayModelSelectorConfig, error) {
	var selector kkComps.AIGatewayModelSelectorConfig
	if err := json.Unmarshal(raw, &selector); err != nil {
		return kkComps.AIGatewayModelSelectorConfig{},
			fmt.Errorf("failed to decode AI Gateway config.route.model: %w", err)
	}
	return selector, nil
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
	stripAIGatewayServerFields(payload)
	normalizeAIGatewayAuthStrategyAccess(payload)
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
	metadata := AIGatewayModelManagedBy(model)
	var kongctl *KongctlMeta
	if namespace := metadata[ModelNamespaceKey]; namespace != "" {
		if err := ValidateModelOwnership(metadata, namespace); err != nil {
			return AIGatewayModelResource{}, err
		}
		kongctl = &KongctlMeta{Namespace: new(namespace), Protected: new(metadata[ModelProtectedKey] == "true")}
	}
	delete(payload, "managed_by")
	(AIGatewayModelResource{}).OmitInheritedFields(payload)
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
		BaseResource:                BaseResource{Ref: ref, Kongctl: kongctl},
		AIGateway:                   gatewayRef,
		CreateAIGatewayModelRequest: req,
	}, nil
}

// OmitInheritedFields excludes datastore connections from model exports.
func (AIGatewayModelResource) OmitInheritedFields(payload map[string]any) {
	OmitAIGatewayModelDatastoreConnections(payload)
}

// OmitAIGatewayModelDatastoreConnections removes connection settings resolved
// from a referenced datastore: pgvector semantic-balancer embeddings, or redis
// session-affinity storage. The API expands these fields and overrides inline
// connection values; exporting them would pin inherited settings in manifests.
func OmitAIGatewayModelDatastoreConnections(payload map[string]any) {
	stores, _ := payload[SchemaFieldDatastores].([]any)
	if len(stores) == 0 {
		return
	}
	config, _ := payload[SchemaFieldConfig].(map[string]any)
	balancer, _ := config["balancer"].(map[string]any)

	if vector, ok := balancer["vectordb"].(map[string]any); ok && vector["type"] == "pgvector" {
		for _, key := range []string{"host", "database", "port", "user", "password", "timeout", "ssl"} {
			delete(vector, key)
		}
	}

	affinity, _ := balancer["session_affinity"].(map[string]any)
	if storage, ok := affinity["storage"].(map[string]any); ok && storage["type"] == "redis" {
		for _, key := range []string{
			"host", "port", "database", "username", "password", "ssl", "ssl_verify", "server_name",
			"connect_timeout", "read_timeout", "send_timeout", "connection_is_proxied",
			"cluster", "sentinel", "cloud_authentication", "keepalive",
		} {
			delete(storage, key)
		}
	}
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

func aiGatewayModelExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	targetConfig, err := aiGatewayTargetConfigExplainNode()
	if err != nil {
		return nil, err
	}
	apiConfig, err := aiGatewayModelConfigExplainNode(false)
	if err != nil {
		return nil, err
	}
	apiConfig.rejectLoadField(
		"model",
		`AI Gateway model field "config.model" is only supported when type is "model"`,
	)

	commonFields := []*ExplainField{
		explainResourceRefField(),
		explainKongctlField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField("name", explainStringNode("support-gpt"), true, true),
		explainField("display_name", explainStringNode("Support GPT"), true, true),
		explainField("enabled", explainBoolNode("true"), false, true),
		explainField("access", aiGatewayAccessExplainNode(true), false, false),
		explainField(
			"datastores",
			explainArrayOf(
				explainObject(
					explainField("name", explainStringNode("datastore-name"), true, true),
					explainField("purpose", &ExplainNode{
						Kind:    explainKindString,
						Literal: string(kkComps.PurposeSemanticBalancer),
						Enum:    []any{string(kkComps.PurposeSemanticBalancer), string(kkComps.PurposeSessionAffinity)},
					}, false, false),
				),
			),
			false,
			false,
		),
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
			explainField(
				"weight",
				&ExplainNode{Kind: explainKindInteger, Literal: "100"},
				false,
				false,
			),
			explainField("semantic_description", explainStringNode("Primary target"), false, false),
			explainField("allow_auth_override", explainBoolNode("false"), false, false),
		)), true, true),
		explainField("policies", explainArrayOf(explainStringNode("policy-name")), false, false),
		explainField(
			"labels",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")},
			false,
			false,
		),
		explainField(
			"managed_by",
			&ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")},
			false,
			false,
		),
	}

	modelConfig, err := aiGatewayModelConfigExplainNode(true)
	if err != nil {
		return nil, err
	}
	modelFields := append(
		slices.Clone(commonFields),
		explainField("config", modelConfig, true, true),
		explainField("type", explainConstStringNode("model"), true, true),
		explainField("capabilities", explainArrayOf(explainStringNode("generate")), true, true),
	)
	apiFields := append(
		slices.Clone(commonFields),
		explainField("config", apiConfig, true, true),
		explainField("type", explainConstStringNode("api"), true, true),
		explainField("capabilities", explainArrayOf(explainStringNode("files")), true, true),
	)

	return explainUnionNode(explainObject(modelFields...), explainObject(apiFields...)), nil
}

func aiGatewayModelConfigExplainNode(includeModelConfig bool) (*ExplainNode, error) {
	sessionAffinity, err := aiGatewayBalancerSessionAffinityExplainNode()
	if err != nil {
		return nil, err
	}
	routeModel := explainObject(
		explainField("body_param", explainStringNode("model"), false, true),
		explainField("header_param", explainStringNode("X-Model"), false, false),
		explainField("path_param", explainStringNode("model"), false, false),
		explainField("values", explainArrayOf(explainStringNode("support-gpt")), false, true),
	)
	route := explainObject(
		explainField(
			"headers",
			&ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}},
			false,
			false,
		),
		explainField("hosts", explainArrayOf(explainStringNode("api.example.com")), false, false),
		explainField(
			"https_redirect_status_code",
			&ExplainNode{Kind: explainKindInteger, Literal: "426"},
			false,
			false,
		),
		explainField("methods", explainArrayOf(explainStringNode("POST")), false, false),
		explainField(
			"paths",
			explainArrayOf(explainStringNode("/v1/chat/completions")),
			false,
			false,
		),
		explainField("preserve_host", explainBoolNode("false"), false, false),
		explainField("protocols", explainArrayOf(explainStringNode("https")), false, false),
		explainField(
			"regex_priority",
			&ExplainNode{Kind: explainKindInteger, Literal: "0"},
			false,
			false,
		),
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
		explainField(
			"max_request_body_size",
			&ExplainNode{Kind: explainKindInteger, Literal: "8388608"},
			false,
			false,
		),
		explainField("balancer", aiGatewayModelBalancerExplainNode(sessionAffinity), false, false),
		explainField(
			"proxy",
			&ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}},
			false,
			false,
		),
	}
	if includeModelConfig {
		fields = append(fields, explainField(
			"model",
			explainObject(explainField("name_header", explainBoolNode("true"), false, false)),
			false,
			false,
		))
	}
	return explainObject(fields...), nil
}

// aiGatewayModelBalancerExplainNode leaves most balancer fields unconstrained (the full
// seven-algorithm oneOf is not modeled here), but gives session_affinity a concrete schema
// since it is this config's reason to exist. session_affinity is omitted entirely for the
// consistent-hashing algorithm, which rejectAIGatewayConsistentHashingSessionAffinity enforces.
func aiGatewayModelBalancerExplainNode(sessionAffinity *ExplainNode) *ExplainNode {
	node := &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}
	node.addField(explainField("session_affinity", sessionAffinity, false, false))
	return node
}

// aiGatewayBalancerSessionAffinityExplainNode derives the session_affinity shape from the SDK
// type via reflection, so the local/redis storage union stays in sync with the SDK automatically.
func aiGatewayBalancerSessionAffinityExplainNode() (*ExplainNode, error) {
	hints := map[string]ExplainFieldHint{
		"storage.password": {
			Literal:      "!secret {source: !env SESSION_AFFINITY_REDIS_PASSWORD}",
			PreferredTag: "!secret",
		},
	}
	return autoExplainConcreteNode[kkComps.AIGatewayModelBalancerSessionAffinity](hints)
}

// aiGatewayTargetConfigExplainNode derives the target config variant list
// from the SDK's AIGatewayTargetConfig union via reflection, so a provider
// added to the SDK automatically gets explain/scaffold coverage without a
// hand-maintained list to keep in sync.
func aiGatewayTargetConfigExplainNode() (*ExplainNode, error) {
	node, ok, err := autoExplainSDKUnionNode(
		reflect.TypeFor[kkComps.AIGatewayTargetConfig](),
		nil,
		defaultExplainHints(""),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build AI Gateway target config schema: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("SDK AI Gateway target config is no longer a union")
	}

	// Keep Anthropic as the primary scaffold target (matching prior,
	// hand-written behavior) regardless of the SDK union's field order,
	// mirroring the equivalent pin in aiGatewayProviderExplainNode.
	for i, branch := range node.OneOf {
		typeField, ok := branch.property("type")
		if ok && typeField.Node.Const == "anthropic" {
			node.OneOf[0], node.OneOf[i] = node.OneOf[i], node.OneOf[0]
			break
		}
	}
	explainRefreshUnionAggregate(node)
	return node, nil
}

func captureModelNamespaceScopes(rs *ResourceSet, defaultNamespace string) {
	scope := rs.EnsureSyncScope()
	for _, gateway := range rs.AIGateways {
		namespaces := make([]string, 0, len(gateway.Models))
		for _, model := range gateway.Models {
			namespaces = append(namespaces, GetNamespace(model.Kongctl))
		}
		CaptureOwnedChildCollection(scope, ResourceTypeAIGatewayModel, gateway.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeAIGateway, gateway.Ref, ResourceTypeAIGatewayModel), namespaces)
	}
	for _, model := range rs.AIGatewayModels {
		scope.AddModelNamespace(model.AIGateway, GetNamespace(model.Kongctl))
	}
}

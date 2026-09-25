package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/maturity"
)

func init() {
	registerAIGatewayChildResource(
		ResourceTypeAIGatewayCustomPolicy,
		func(rs *ResourceSet) *[]AIGatewayCustomPolicyResource { return &rs.AIGatewayCustomPolicies },
		AutoExplain[AIGatewayCustomPolicyResource](
			WithExplainAliases("ai_gateway_custom_policies", "ai-gateway-custom-policy",
				"ai-gateway-custom-policies", "ai_gateway.custom_policies", "aigw-custom-policy"),
			WithExplainRecommendedFields("ref", SchemaFieldAIGateway, "name", "type", "display_name", "schema"),
			WithExplainSchemaBuilder(aiGatewayCustomPolicyExplainNode),
		),
		aiGatewayChildLoad[AIGatewayCustomPolicyResource]{
			extractOrder: 15, validateOrder: 25,
			nested:    func(gateway *AIGatewayResource) *[]AIGatewayCustomPolicyResource { return &gateway.CustomPolicies },
			setParent: func(child *AIGatewayCustomPolicyResource, ref string) { child.AIGateway = ref },
		},
		WithChildSyncScope(ResourceTypeAIGateway,
			WithEmptyRootCollectionError(25, "each custom policy must declare an ai_gateway parent")),
		WithMaturity(maturity.Metadata{
			Level:   maturity.LevelBeta,
			Message: "Requires an AI Gateway backend with custom-policy support enabled.",
		}),
	)
}

// AIGatewayCustomPolicyResource defines an installed or streaming plugin.
type AIGatewayCustomPolicyResource struct {
	BaseResource `        yaml:",inline"              json:",inline"`
	AIGateway    string  `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name         string  `yaml:"name"                 json:"name"`
	Type         string  `yaml:"type"                 json:"type"`
	DisplayName  string  `yaml:"display_name"         json:"display_name"`
	Schema       string  `yaml:"schema"               json:"schema"`
	Handler      *string `yaml:"handler,omitempty"    json:"handler,omitempty"`
}

func (a AIGatewayCustomPolicyResource) GetType() ResourceType {
	return ResourceTypeAIGatewayCustomPolicy
}
func (a AIGatewayCustomPolicyResource) GetMoniker() string { return a.Name }
func (a AIGatewayCustomPolicyResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayCustomPolicyResource) GetDependencies() []ResourceRef {
	if parent := a.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (a AIGatewayCustomPolicyResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayCustomPolicyResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

func (a *AIGatewayCustomPolicyResource) TryMatchKonnectResource(remote any) bool {
	if a.Name != "" && AIGatewayCustomPolicyName(remote) == a.Name {
		id := AIGatewayCustomPolicyID(remote)
		if id != "" {
			a.SetKonnectID(id)
			return true
		}
	}
	return false
}

func (a *AIGatewayCustomPolicyResource) SetDefaults() {
	if a.Ref == "" {
		a.Ref = a.Name
	}
}

func (a AIGatewayCustomPolicyResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid custom policy ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway custom policies")
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required")
	}
	if a.Name == "" || a.DisplayName == "" || a.Schema == "" {
		return fmt.Errorf("name, display_name, and schema are required")
	}
	switch a.Type {
	case "installed":
		if a.Handler != nil {
			return fmt.Errorf("handler is only supported for streaming custom policies")
		}
	case "streaming":
		if a.Handler == nil || *a.Handler == "" {
			return fmt.Errorf("handler is required for streaming custom policies")
		}
	default:
		return fmt.Errorf("custom policy type must be installed or streaming")
	}
	return nil
}

func (a AIGatewayCustomPolicyResource) MutablePayloadMap() (map[string]any, error) {
	fields := map[string]any{"name": a.Name, "type": a.Type, "display_name": a.DisplayName, "schema": a.Schema}
	if a.Handler != nil {
		fields["handler"] = *a.Handler
	}
	return fields, nil
}

func AIGatewayCustomPolicyID(policy any) string {
	return aiGatewayPolicyStringField(policy, aiGatewayPolicyFieldID)
}

func AIGatewayCustomPolicyName(policy any) string {
	return aiGatewayPolicyStringField(policy, aiGatewayPolicyFieldName)
}

func AIGatewayCustomPolicyMutablePayloadMap(policy kkComps.AIGatewayCustomPolicy) (map[string]any, error) {
	payload, err := marshalObjectToMap(policy, "AI Gateway custom policy")
	if err != nil {
		return nil, err
	}
	stripAIGatewayModelServerFields(payload)
	return payload, nil
}

func AIGatewayCustomPolicyResourceFromResponse(
	gatewayRef string, policy kkComps.AIGatewayCustomPolicy,
) (AIGatewayCustomPolicyResource, error) {
	data, err := json.Marshal(policy)
	if err != nil {
		return AIGatewayCustomPolicyResource{}, fmt.Errorf("encode custom policy: %w", err)
	}
	var result AIGatewayCustomPolicyResource
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("decode custom policy: %w", err)
	}
	result.Ref, result.AIGateway = result.Name, gatewayRef
	return result, nil
}

func aiGatewayCustomPolicyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	installed, err := explainVariantNode[kkComps.CreateAIGatewayCustomPolicyInstalledRequest]("type", "installed")
	if err != nil {
		return nil, err
	}
	streaming, err := explainVariantNode[kkComps.CreateAIGatewayCustomPolicyStreamingRequest]("type", "streaming")
	if err != nil {
		return nil, err
	}
	branches := []*ExplainNode{installed, streaming}
	for i, branch := range branches {
		branch = explainWithCommonFields(branch, explainResourceRefField(),
			explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true))
		branches[i] = branch
		setExplainLiteral(branch, []string{SchemaFieldName}, "custom-plugin")
		setExplainLiteral(branch, []string{"schema"}, "return { name = \"custom-plugin\", fields = {} }")
	}
	return explainUnionNode(branches...), nil
}

package resources

import (
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

var aiGatewayConfigStoreDisplayNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._~-]*$`)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayConfigStore,
		func(rs *ResourceSet) *[]AIGatewayConfigStoreResource { return &rs.AIGatewayConfigStores },
		AutoExplain[AIGatewayConfigStoreResource](
			WithExplainAliases(
				"ai_gateway_config_stores",
				"ai-gateway-config-store",
				"ai-gateway-config-stores",
				"ai_gateway.config_stores",
				"aigw-config-store",
			),
			WithExplainRecommendedFields("ref", SchemaFieldAIGateway, SchemaFieldName, SchemaFieldDisplayName),
			WithExplainSchemaBuilder(aiGatewayConfigStoreExplainNode),
		),
	)
}

// AIGatewayConfigStoreResource represents a Config Store nested under a Konnect AI Gateway.
type AIGatewayConfigStoreResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	AIGateway    string  `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name         string  `yaml:"name"                 json:"name"`
	DisplayName  *string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
}

func (a AIGatewayConfigStoreResource) GetType() ResourceType {
	return ResourceTypeAIGatewayConfigStore
}

func (a AIGatewayConfigStoreResource) GetMoniker() string {
	return a.Name
}

func (a AIGatewayConfigStoreResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayConfigStoreResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayConfigStoreResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayConfigStoreResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway Config Store ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway Config Store %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway Config Store %s", a.Ref)
	}
	if a.Name == "" {
		return fmt.Errorf("name is required for AI Gateway Config Store %s", a.Ref)
	}
	if a.DisplayName != nil && !aiGatewayConfigStoreDisplayNamePattern.MatchString(*a.DisplayName) {
		return fmt.Errorf(
			"display_name for AI Gateway Config Store %s may only contain letters, numbers, periods, hyphens, "+
				"underscores, and tildes",
			a.Ref,
		)
	}
	return nil
}

func (a *AIGatewayConfigStoreResource) SetDefaults() {
	if a == nil {
		return
	}
	if a.Ref == "" {
		a.Ref = a.Name
	}
	if a.Name == "" {
		a.Name = a.Ref
	}
}

func (a AIGatewayConfigStoreResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.Name)
}

func (a *AIGatewayConfigStoreResource) TryMatchKonnectResource(konnectResource any) bool {
	id := AIGatewayConfigStoreID(konnectResource)
	if id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id != "" && AIGatewayConfigStoreName(konnectResource) == a.Name {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayConfigStoreResource) CreateRequest() kkComps.CreateAIGatewayConfigStoreRequest {
	return kkComps.CreateAIGatewayConfigStoreRequest{
		Name:        a.Name,
		DisplayName: a.DisplayName,
	}
}

func (a AIGatewayConfigStoreResource) PayloadMap() (map[string]any, error) {
	payload := map[string]any{SchemaFieldName: a.Name}
	if a.DisplayName != nil {
		payload[SchemaFieldDisplayName] = *a.DisplayName
	}
	return payload, nil
}

func (a AIGatewayConfigStoreResource) MutablePayloadMap() (map[string]any, error) {
	return a.PayloadMap()
}

func AIGatewayConfigStoreID(resource any) string {
	switch typed := resource.(type) {
	case kkComps.AIGatewayConfigStore:
		return typed.ID
	case *kkComps.AIGatewayConfigStore:
		if typed != nil {
			return typed.ID
		}
	}
	return ""
}

func AIGatewayConfigStoreName(resource any) string {
	switch typed := resource.(type) {
	case kkComps.AIGatewayConfigStore:
		return typed.Name
	case *kkComps.AIGatewayConfigStore:
		if typed != nil {
			return typed.Name
		}
	}
	return ""
}

// AIGatewayConfigStoreResourceFromResponse maps a Config Store API response to declarative form.
func AIGatewayConfigStoreResourceFromResponse(
	gatewayRef string,
	store kkComps.AIGatewayConfigStore,
) AIGatewayConfigStoreResource {
	ref := store.ID
	if ref == "" {
		ref = store.Name
	}
	return AIGatewayConfigStoreResource{
		BaseResource: BaseResource{Ref: ref},
		AIGateway:    gatewayRef,
		Name:         store.Name,
		DisplayName:  store.DisplayName,
	}
}

func aiGatewayConfigStoreExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField(SchemaFieldName, explainStringNode("my-config-store"), true, true),
		explainField(SchemaFieldDisplayName, explainStringNode("My-Config-Store"), false, true),
	), nil
}

func aiGatewayConfigStoreInlineExplainNode() *ExplainNode {
	node, err := aiGatewayConfigStoreExplainNode(ExplainBuildContext{})
	if err != nil {
		return explainObject()
	}
	return node
}

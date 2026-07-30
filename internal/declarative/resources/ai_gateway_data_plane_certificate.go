package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

const (
	aiGatewayDataPlaneCertificateFieldID          = "id"
	aiGatewayDataPlaneCertificateFieldCert        = "cert"
	aiGatewayDataPlaneCertificateFieldTitle       = "title"
	aiGatewayDataPlaneCertificateFieldDescription = "description"
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayDataPlaneCertificate,
		func(rs *ResourceSet) *[]AIGatewayDataPlaneCertificateResource {
			return &rs.AIGatewayDataPlaneCertificates
		},
		AutoExplain[AIGatewayDataPlaneCertificateResource](
			WithExplainAliases(
				"ai_gateway_data_plane_certificates",
				"ai-gateway-data-plane-certificate",
				"ai-gateway-data-plane-certificates",
				"ai_gateway.data_plane_certificates",
				"aigw-dpc",
				"aigw-dpcs",
			),
			WithExplainFieldHint(aiGatewayDataPlaneCertificateFieldCert, ExplainFieldHint{
				FileSample:   "./certs/data-plane.pem",
				PreferredTag: "!file",
				Notes: []string{
					"Use !file or !env to avoid inlining PEM data in configuration.",
				},
			}),
			WithExplainRecommendedFields(
				SchemaFieldRef,
				SchemaFieldAIGateway,
				aiGatewayDataPlaneCertificateFieldTitle,
				aiGatewayDataPlaneCertificateFieldCert,
			),
			WithExplainSchemaBuilder(aiGatewayDataPlaneCertificateExplainNode),
		),
	)
}

// AIGatewayDataPlaneCertificateResource represents a data plane certificate nested
// under a Konnect AI Gateway.
type AIGatewayDataPlaneCertificateResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	// Parent AI Gateway reference for root-level declarations.
	AIGateway string `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`

	kkComps.CreateAIGatewayDataPlaneCertificateRequest `yaml:",inline" json:",inline"`
}

func (a AIGatewayDataPlaneCertificateResource) GetType() ResourceType {
	return ResourceTypeAIGatewayDataPlaneCertificate
}

func (a AIGatewayDataPlaneCertificateResource) GetMoniker() string {
	return a.Title
}

func (a AIGatewayDataPlaneCertificateResource) GetDependencies() []ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}}
}

func (a AIGatewayDataPlaneCertificateResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayDataPlaneCertificateResource) GetReferenceFieldMappings() map[string]string {
	if a.AIGateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func (a AIGatewayDataPlaneCertificateResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway data plane certificate ref: %w", err)
	}
	if a.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway data plane certificate %s", a.Ref)
	}
	if a.AIGateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway data plane certificate %s", a.Ref)
	}
	if a.Title == "" {
		return fmt.Errorf("title is required for AI Gateway data plane certificate %s", a.Ref)
	}
	if a.Cert == "" {
		return fmt.Errorf("cert is required for AI Gateway data plane certificate %s", a.Ref)
	}
	return nil
}

func (a *AIGatewayDataPlaneCertificateResource) SetDefaults() {
	if a == nil {
		return
	}
	if a.Ref == "" {
		a.Ref = a.Title
	}
}

func aiGatewayDataPlaneCertificateExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	certNode := explainStringNode("!file ./certs/data-plane.pem")
	certNode.PreferredTag = "!file"
	certNode.Notes = []string{
		"Use !file or !env to avoid inlining PEM data in configuration.",
	}

	return explainObject(
		explainResourceRefField(),
		explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField(aiGatewayDataPlaneCertificateFieldTitle, explainStringNode("support-data-plane-cert"), true, true),
		explainField(
			aiGatewayDataPlaneCertificateFieldDescription,
			&ExplainNode{Kind: explainKindString, Nullable: true},
			false,
			false,
		),
		explainField(aiGatewayDataPlaneCertificateFieldCert, certNode, true, true),
	), nil
}

func (a AIGatewayDataPlaneCertificateResource) GetKonnectMonikerFilter() string {
	return ""
}

func (a *AIGatewayDataPlaneCertificateResource) TryMatchKonnectResource(konnectResource any) bool {
	title := a.Title
	if title == "" {
		return false
	}

	id := AIGatewayDataPlaneCertificateID(konnectResource)
	if id != "" && (util.IsValidUUID(a.Ref) || a.GetKonnectID() != "") {
		if a.Ref == id || a.GetKonnectID() == id {
			a.SetKonnectID(id)
			return true
		}
	}
	if id != "" && AIGatewayDataPlaneCertificateTitle(konnectResource) == title {
		a.SetKonnectID(id)
		return true
	}
	return false
}

func (a AIGatewayDataPlaneCertificateResource) CreateRequest() kkComps.CreateAIGatewayDataPlaneCertificateRequest {
	return a.CreateAIGatewayDataPlaneCertificateRequest
}

func (a AIGatewayDataPlaneCertificateResource) PayloadMap() map[string]any {
	payload := map[string]any{
		aiGatewayDataPlaneCertificateFieldCert:  a.Cert,
		aiGatewayDataPlaneCertificateFieldTitle: a.Title,
	}
	if a.Description != nil {
		payload[aiGatewayDataPlaneCertificateFieldDescription] = *a.Description
	}
	return payload
}

func (a AIGatewayDataPlaneCertificateResource) MarshalJSON() ([]byte, error) {
	payload := a.PayloadMap()
	payload[SchemaFieldRef] = a.Ref
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return json.Marshal(payload)
}

func (a AIGatewayDataPlaneCertificateResource) MarshalYAML() (any, error) {
	payload := a.PayloadMap()
	payload[SchemaFieldRef] = a.Ref
	if a.AIGateway != "" {
		payload[SchemaFieldAIGateway] = a.AIGateway
	}
	return payload, nil
}

func (a *AIGatewayDataPlaneCertificateResource) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref         string          `json:"ref"`
		AIGateway   string          `json:"ai_gateway,omitempty"`
		Kongctl     json.RawMessage `json:"kongctl,omitempty"`
		Cert        string          `json:"cert"`
		Title       string          `json:"title"`
		Description *string         `json:"description,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Kongctl) > 0 && string(raw.Kongctl) != jsonNullLiteral {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	a.BaseResource = BaseResource{Ref: raw.Ref}
	a.AIGateway = raw.AIGateway
	a.CreateAIGatewayDataPlaneCertificateRequest = kkComps.CreateAIGatewayDataPlaneCertificateRequest{
		Cert:        raw.Cert,
		Title:       raw.Title,
		Description: raw.Description,
	}
	return nil
}

func (a *AIGatewayDataPlaneCertificateResource) UnmarshalYAML(unmarshal func(any) error) error {
	var raw struct {
		Ref         string       `yaml:"ref"`
		AIGateway   string       `yaml:"ai_gateway,omitempty"`
		Kongctl     *KongctlMeta `yaml:"kongctl,omitempty"`
		Cert        string       `yaml:"cert"`
		Title       string       `yaml:"title"`
		Description *string      `yaml:"description,omitempty"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}
	if raw.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	a.BaseResource = BaseResource{Ref: raw.Ref}
	a.AIGateway = raw.AIGateway
	a.CreateAIGatewayDataPlaneCertificateRequest = kkComps.CreateAIGatewayDataPlaneCertificateRequest{
		Cert:        raw.Cert,
		Title:       raw.Title,
		Description: raw.Description,
	}
	return nil
}

func AIGatewayDataPlaneCertificateID(cert any) string {
	switch typed := cert.(type) {
	case kkComps.AIGatewayDataPlaneClientCertificate:
		return typed.ID
	case *kkComps.AIGatewayDataPlaneClientCertificate:
		if typed == nil {
			return ""
		}
		return typed.ID
	default:
		return ""
	}
}

func AIGatewayDataPlaneCertificateTitle(cert any) string {
	switch typed := cert.(type) {
	case kkComps.AIGatewayDataPlaneClientCertificate:
		return typed.Title
	case *kkComps.AIGatewayDataPlaneClientCertificate:
		if typed == nil {
			return ""
		}
		return typed.Title
	default:
		return ""
	}
}

func AIGatewayDataPlaneCertificateResourceFromResponse(
	gatewayRef string,
	cert kkComps.AIGatewayDataPlaneClientCertificate,
) AIGatewayDataPlaneCertificateResource {
	ref := AIGatewayDataPlaneCertificateID(cert)
	if ref == "" {
		ref = AIGatewayDataPlaneCertificateTitle(cert)
	}
	return AIGatewayDataPlaneCertificateResource{
		BaseResource: BaseResource{Ref: ref},
		AIGateway:    gatewayRef,
		CreateAIGatewayDataPlaneCertificateRequest: kkComps.CreateAIGatewayDataPlaneCertificateRequest{
			Cert:        cert.Cert,
			Title:       cert.Title,
			Description: cert.Description,
		},
	}
}

package resources

import (
	"fmt"
	"regexp"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
)

var (
	aiGatewaySNINamePattern  = regexp.MustCompile(`^[0-9a-z-]+$`)
	aiGatewayHostnamePattern = regexp.MustCompile(
		`^(?:(?:\*\.)?[a-zA-Z0-9](?:[-a-zA-Z0-9]*[a-zA-Z0-9])?` +
			`(?:\.[a-zA-Z0-9](?:[-a-zA-Z0-9]*[a-zA-Z0-9])?)*(?:\.)?` +
			`|[a-zA-Z0-9](?:[-a-zA-Z0-9]*[a-zA-Z0-9])?` +
			`(?:\.[a-zA-Z0-9](?:[-a-zA-Z0-9]*[a-zA-Z0-9])?)*\.\*)$`,
	)
)

func init() {
	registerResourceType(
		ResourceTypeAIGatewayCertificate,
		func(rs *ResourceSet) *[]AIGatewayCertificateResource { return &rs.AIGatewayCertificates },
		AutoExplain[AIGatewayCertificateResource](
			WithExplainAliases(
				"ai_gateway_certificates", "ai-gateway-certificate", "ai-gateway-certificates",
				"ai_gateway.certificates", "aigw-certificate", "aigw-certificates",
			),
			WithExplainRecommendedFields(SchemaFieldRef, SchemaFieldAIGateway, SchemaFieldName, SchemaFieldCert, SchemaFieldKey),
			WithExplainSchemaBuilder(aiGatewayCertificateExplainNode),
		),
		WithExternalUnsupportedReason("AI Gateway certificate lookup requires gateway-scoped name materialization"),
	)
	registerResourceType(
		ResourceTypeAIGatewayCACertificate,
		func(rs *ResourceSet) *[]AIGatewayCACertificateResource { return &rs.AIGatewayCACertificates },
		AutoExplain[AIGatewayCACertificateResource](
			WithExplainAliases(
				"ai_gateway_ca_certificates", "ai-gateway-ca-certificate", "ai-gateway-ca-certificates",
				"ai_gateway.ca_certificates", "aigw-ca-certificate", "aigw-ca-certificates",
			),
			WithExplainRecommendedFields(SchemaFieldRef, SchemaFieldAIGateway, SchemaFieldName, SchemaFieldCert),
			WithExplainSchemaBuilder(aiGatewayCACertificateExplainNode),
		),
	)
	registerResourceType(
		ResourceTypeAIGatewaySNI,
		func(rs *ResourceSet) *[]AIGatewaySNIResource { return &rs.AIGatewaySNIs },
		AutoExplain[AIGatewaySNIResource](
			WithExplainAliases(
				"ai_gateway_snis", "ai-gateway-sni", "ai-gateway-snis",
				"ai_gateway.snis", "aigw-sni", "aigw-snis",
			),
			WithExplainRecommendedFields(
				SchemaFieldRef, SchemaFieldAIGateway, SchemaFieldName, SchemaFieldDisplayName, "hostname", SchemaFieldCertificate,
			),
			WithExplainSchemaBuilder(aiGatewaySNIExplainNode),
		),
	)
}

type AIGatewayCertificateResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	AIGateway    string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name         string            `yaml:"name" json:"name"`
	Cert         string            `yaml:"cert" json:"cert"`
	Key          string            `yaml:"key,omitempty" json:"key,omitempty"`
	CertAlt      *string           `yaml:"cert_alt,omitempty" json:"cert_alt,omitempty"`
	KeyAlt       *string           `yaml:"key_alt,omitempty" json:"key_alt,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	ManagedBy    map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
}

func (a AIGatewayCertificateResource) GetType() ResourceType { return ResourceTypeAIGatewayCertificate }
func (a AIGatewayCertificateResource) GetMoniker() string    { return a.Name }
func (a AIGatewayCertificateResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayCertificateResource) GetDependencies() []ResourceRef {
	if parent := a.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (a AIGatewayCertificateResource) GetReferenceFieldMappings() map[string]string {
	return aiGatewayParentReferenceMapping(a.AIGateway)
}

func (a AIGatewayCertificateResource) Validate() error {
	if err := validateAIGatewayTLSChild(a.BaseResource, a.AIGateway, a.Name, "certificate"); err != nil {
		return err
	}
	if a.Cert == "" {
		return fmt.Errorf("cert is required for AI Gateway certificate %s", a.Ref)
	}
	if a.KeyAlt != nil && a.CertAlt == nil {
		return fmt.Errorf("key_alt requires cert_alt for AI Gateway certificate %s", a.Ref)
	}
	return nil
}
func (a *AIGatewayCertificateResource) SetDefaults()                   {}
func (a AIGatewayCertificateResource) GetKonnectMonikerFilter() string { return "" }
func (a *AIGatewayCertificateResource) TryMatchKonnectResource(value any) bool {
	return matchAIGatewayTLSResource(
		&a.BaseResource, a.Name, AIGatewayCertificateID(value), AIGatewayCertificateName(value),
	)
}
func (a AIGatewayCertificateResource) GetLabels() map[string]string        { return a.Labels }
func (a *AIGatewayCertificateResource) SetLabels(labels map[string]string) { a.Labels = labels }
func (a AIGatewayCertificateResource) PayloadMap() map[string]any {
	payload := map[string]any{SchemaFieldName: a.Name, SchemaFieldCert: a.Cert}
	addAIGatewayTLSCommonFields(payload, a.CertAlt, a.Labels, a.ManagedBy)
	return payload
}

type AIGatewayCACertificateResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	AIGateway    string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name         string            `yaml:"name" json:"name"`
	Cert         string            `yaml:"cert" json:"cert"`
	Labels       map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	ManagedBy    map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
}

func (a AIGatewayCACertificateResource) GetType() ResourceType {
	return ResourceTypeAIGatewayCACertificate
}
func (a AIGatewayCACertificateResource) GetMoniker() string { return a.Name }
func (a AIGatewayCACertificateResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewayCACertificateResource) GetDependencies() []ResourceRef {
	if parent := a.GetParentRef(); parent != nil {
		return []ResourceRef{*parent}
	}
	return nil
}

func (a AIGatewayCACertificateResource) GetReferenceFieldMappings() map[string]string {
	return aiGatewayParentReferenceMapping(a.AIGateway)
}

func (a AIGatewayCACertificateResource) Validate() error {
	if err := validateAIGatewayTLSChild(a.BaseResource, a.AIGateway, a.Name, "CA certificate"); err != nil {
		return err
	}
	if a.Cert == "" {
		return fmt.Errorf("cert is required for AI Gateway CA certificate %s", a.Ref)
	}
	return nil
}
func (a *AIGatewayCACertificateResource) SetDefaults()                   {}
func (a AIGatewayCACertificateResource) GetKonnectMonikerFilter() string { return "" }
func (a *AIGatewayCACertificateResource) TryMatchKonnectResource(value any) bool {
	return matchAIGatewayTLSResource(
		&a.BaseResource, a.Name, AIGatewayCACertificateID(value), AIGatewayCACertificateName(value),
	)
}
func (a AIGatewayCACertificateResource) GetLabels() map[string]string        { return a.Labels }
func (a *AIGatewayCACertificateResource) SetLabels(labels map[string]string) { a.Labels = labels }
func (a AIGatewayCACertificateResource) PayloadMap() map[string]any {
	payload := map[string]any{SchemaFieldName: a.Name, SchemaFieldCert: a.Cert}
	addAIGatewayTLSCommonFields(payload, nil, a.Labels, a.ManagedBy)
	return payload
}

type AIGatewaySNIResource struct {
	BaseResource `yaml:",inline" json:",inline"`
	AIGateway    string            `yaml:"ai_gateway,omitempty" json:"ai_gateway,omitempty"`
	Name         string            `yaml:"name" json:"name"`
	DisplayName  string            `yaml:"display_name" json:"display_name"`
	Hostname     string            `yaml:"hostname" json:"hostname"`
	Certificate  string            `yaml:"certificate" json:"certificate"`
	Labels       map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	ManagedBy    map[string]string `yaml:"managed_by,omitempty" json:"managed_by,omitempty"`
}

func (a AIGatewaySNIResource) GetType() ResourceType { return ResourceTypeAIGatewaySNI }
func (a AIGatewaySNIResource) GetMoniker() string    { return a.Name }
func (a AIGatewaySNIResource) GetParentRef() *ResourceRef {
	if a.AIGateway == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeAIGateway, Ref: NormalizeResourceRef(a.AIGateway)}
}

func (a AIGatewaySNIResource) GetDependencies() []ResourceRef {
	dependencies := make([]ResourceRef, 0, 2)
	if parent := a.GetParentRef(); parent != nil {
		dependencies = append(dependencies, *parent)
	}
	if a.Certificate != "" {
		dependencies = append(dependencies, ResourceRef{
			Kind: ResourceTypeAIGatewayCertificate, Ref: NormalizeResourceRef(a.Certificate),
		})
	}
	return dependencies
}

func (a AIGatewaySNIResource) GetReferenceFieldMappings() map[string]string {
	mappings := aiGatewayParentReferenceMapping(a.AIGateway)
	if mappings == nil {
		mappings = make(map[string]string)
	}
	mappings[SchemaFieldCertificate] = string(ResourceTypeAIGatewayCertificate)
	return mappings
}

func (a AIGatewaySNIResource) Validate() error {
	if err := validateAIGatewayTLSChild(a.BaseResource, a.AIGateway, a.Name, "SNI"); err != nil {
		return err
	}
	if len(a.Name) > 256 || !aiGatewaySNINamePattern.MatchString(a.Name) {
		return fmt.Errorf("name for AI Gateway SNI %s must contain only lowercase letters, numbers, and hyphens", a.Ref)
	}
	if a.DisplayName == "" || len(a.DisplayName) > 256 {
		return fmt.Errorf("display_name is required and must not exceed 256 characters for AI Gateway SNI %s", a.Ref)
	}
	if a.Hostname == "" || !aiGatewayHostnamePattern.MatchString(a.Hostname) {
		return fmt.Errorf("hostname is invalid for AI Gateway SNI %s", a.Ref)
	}
	if a.Certificate == "" {
		return fmt.Errorf("certificate is required for AI Gateway SNI %s", a.Ref)
	}
	return nil
}
func (a *AIGatewaySNIResource) SetDefaults()                   {}
func (a AIGatewaySNIResource) GetKonnectMonikerFilter() string { return "" }
func (a *AIGatewaySNIResource) TryMatchKonnectResource(value any) bool {
	return matchAIGatewayTLSResource(&a.BaseResource, a.Name, AIGatewaySNIID(value), AIGatewaySNIName(value))
}
func (a AIGatewaySNIResource) GetLabels() map[string]string        { return a.Labels }
func (a *AIGatewaySNIResource) SetLabels(labels map[string]string) { a.Labels = labels }
func (a AIGatewaySNIResource) PayloadMap() map[string]any {
	payload := map[string]any{
		SchemaFieldName: a.Name, SchemaFieldDisplayName: a.DisplayName,
		"hostname": a.Hostname, SchemaFieldCertificate: a.Certificate,
	}
	addAIGatewayTLSCommonFields(payload, nil, a.Labels, a.ManagedBy)
	return payload
}

func validateAIGatewayTLSChild(base BaseResource, gateway, name, label string) error {
	if err := ValidateRef(base.Ref); err != nil {
		return fmt.Errorf("invalid AI Gateway %s ref: %w", label, err)
	}
	if base.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on AI Gateway %s %s", label, base.Ref)
	}
	if gateway == "" {
		return fmt.Errorf("ai_gateway is required for AI Gateway %s %s", label, base.Ref)
	}
	if name == "" {
		return fmt.Errorf("name is required for AI Gateway %s %s", label, base.Ref)
	}
	return nil
}

func aiGatewayParentReferenceMapping(gateway string) map[string]string {
	if gateway == "" {
		return nil
	}
	return map[string]string{SchemaFieldAIGateway: string(ResourceTypeAIGateway)}
}

func matchAIGatewayTLSResource(base *BaseResource, desiredName, remoteID, remoteName string) bool {
	if remoteID == "" {
		return false
	}
	if (util.IsValidUUID(base.Ref) || base.GetKonnectID() != "") &&
		(base.Ref == remoteID || base.GetKonnectID() == remoteID) {
		base.SetKonnectID(remoteID)
		return true
	}
	if desiredName != "" && desiredName == remoteName {
		base.SetKonnectID(remoteID)
		return true
	}
	return false
}

func addAIGatewayTLSCommonFields(payload map[string]any, certAlt *string, labels, managedBy map[string]string) {
	if certAlt != nil {
		payload[SchemaFieldCertAlt] = *certAlt
	}
	if labels != nil {
		payload["labels"] = labels
	}
	if managedBy != nil {
		payload["managed_by"] = managedBy
	}
}

func AIGatewayCertificateID(value any) string     { return aiGatewayTLSStringField(value, "ID") }
func AIGatewayCertificateName(value any) string   { return aiGatewayTLSStringField(value, "Name") }
func AIGatewayCACertificateID(value any) string   { return aiGatewayTLSStringField(value, "ID") }
func AIGatewayCACertificateName(value any) string { return aiGatewayTLSStringField(value, "Name") }
func AIGatewaySNIID(value any) string             { return aiGatewayTLSStringField(value, "ID") }
func AIGatewaySNIName(value any) string           { return aiGatewayTLSStringField(value, "Name") }

func aiGatewayTLSStringField(value any, field string) string {
	name, id := extractNameAndID(value, "")
	if field == "ID" {
		return id
	}
	return name
}

func AIGatewayCertificateUpdatedAt(value kkComps.AIGatewayCertificate) time.Time {
	return value.UpdatedAt
}

func AIGatewayCACertificateUpdatedAt(value kkComps.AIGatewayCACertificate) time.Time {
	return value.UpdatedAt
}
func AIGatewaySNIUpdatedAt(value kkComps.AIGatewaySNI) time.Time { return value.UpdatedAt }

func AIGatewayCertificateMutablePayloadMap(value kkComps.AIGatewayCertificate) map[string]any {
	return AIGatewayCertificateResourceFromResponse("", value).PayloadMap()
}

func AIGatewayCACertificateMutablePayloadMap(value kkComps.AIGatewayCACertificate) map[string]any {
	return AIGatewayCACertificateResourceFromResponse("", value).PayloadMap()
}

func AIGatewaySNIMutablePayloadMap(value kkComps.AIGatewaySNI) (map[string]any, error) {
	resource, err := AIGatewaySNIResourceFromResponse("", value)
	if err != nil {
		return nil, err
	}
	return resource.PayloadMap(), nil
}

func AIGatewayCertificateResourceFromResponse(
	gatewayRef string,
	value kkComps.AIGatewayCertificate,
) AIGatewayCertificateResource {
	return AIGatewayCertificateResource{
		BaseResource: BaseResource{Ref: value.Name}, AIGateway: gatewayRef, Name: value.Name,
		Cert: value.Cert, CertAlt: value.CertAlt, Labels: value.Labels, ManagedBy: value.ManagedBy,
	}
}

func AIGatewayCACertificateResourceFromResponse(
	gatewayRef string,
	value kkComps.AIGatewayCACertificate,
) AIGatewayCACertificateResource {
	return AIGatewayCACertificateResource{
		BaseResource: BaseResource{Ref: value.Name}, AIGateway: gatewayRef, Name: value.Name,
		Cert: value.Cert, Labels: value.Labels, ManagedBy: value.ManagedBy,
	}
}

func AIGatewaySNIResourceFromResponse(gatewayRef string, value kkComps.AIGatewaySNI) (AIGatewaySNIResource, error) {
	hostname, ok := value.Hostname.(string)
	if !ok {
		return AIGatewaySNIResource{}, fmt.Errorf("AI Gateway SNI %s hostname is not a string", value.Name)
	}
	return AIGatewaySNIResource{
		BaseResource: BaseResource{Ref: value.Name}, AIGateway: gatewayRef, Name: value.Name,
		DisplayName: value.DisplayName, Hostname: hostname, Certificate: value.Certificate,
		Labels: value.Labels, ManagedBy: value.ManagedBy,
	}, nil
}

func aiGatewayCertificateExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	cert := explainStringNode("!file ./certs/runtime.pem")
	cert.PreferredTag = "!file"
	key := explainSecretEnvNode("AI_GATEWAY_RUNTIME_PRIVATE_KEY")
	return explainObject(
		explainResourceRefField(), explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField(SchemaFieldName, explainStringNode("runtime-cert"), true, true),
		explainField(SchemaFieldCert, cert, true, true), explainField(SchemaFieldKey, key, false, false),
		explainField(SchemaFieldCertAlt, cert, false, true), explainField(SchemaFieldKeyAlt, key, false, false),
		explainAIGatewayTLSLabels(), explainAIGatewayTLSManagedBy(),
	), nil
}

func aiGatewayCACertificateExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	cert := explainStringNode("!file ./certs/root-ca.pem")
	cert.PreferredTag = "!file"
	return explainObject(
		explainResourceRefField(), explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField(SchemaFieldName, explainStringNode("root-ca"), true, true),
		explainField(SchemaFieldCert, cert, true, true), explainAIGatewayTLSLabels(), explainAIGatewayTLSManagedBy(),
	), nil
}

func aiGatewaySNIExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainResourceRefField(), explainRefField(SchemaFieldAIGateway, ResourceTypeAIGateway, true),
		explainField(SchemaFieldName, explainStringNode("api-sni"), true, true),
		explainField(SchemaFieldDisplayName, explainStringNode("API SNI"), true, true),
		explainField("hostname", explainStringNode("*.example.test"), true, true),
		explainField(SchemaFieldCertificate, explainStringNode("!ref runtime-cert"), true, true),
		explainAIGatewayTLSLabels(), explainAIGatewayTLSManagedBy(),
	), nil
}

func explainAIGatewayTLSLabels() *ExplainField {
	node := &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("value")}
	return explainField("labels", node, false, false)
}

func explainAIGatewayTLSManagedBy() *ExplainField {
	node := &ExplainNode{Kind: explainKindObject, Additional: explainStringNode("kongctl")}
	return explainField("managed_by", node, false, false)
}

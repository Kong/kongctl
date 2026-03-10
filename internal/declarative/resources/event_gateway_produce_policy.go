package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayProducePolicy,
		func(rs *ResourceSet) *[]EventGatewayProducePolicyResource { return &rs.EventGatewayProducePolicies },
	)
}

// EventGatewayProducePolicyResource represents a produce policy in declarative configuration.
// The SDK represents produce policies as a union type (EventGatewayProducePolicyCreate)
// with variants: modify_headers, schema_validation, and encrypt.
type EventGatewayProducePolicyResource struct {
	kkComps.EventGatewayProducePolicyCreate `yaml:",inline" json:",inline"`
	Ref                                     string `yaml:"ref"                            json:"ref"`
	// Parent Event Gateway Virtual Cluster reference (for root-level definitions)
	VirtualCluster string `yaml:"virtual_cluster,omitempty" json:"virtual_cluster,omitempty"`
	EventGateway   string `yaml:"event_gateway,omitempty"   json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayProducePolicyResource) GetType() ResourceType {
	return ResourceTypeEventGatewayProducePolicy
}

func (e EventGatewayProducePolicyResource) GetRef() string {
	return e.Ref
}

// GetMoniker returns the name of the policy from whichever union variant is set.
func (e EventGatewayProducePolicyResource) GetMoniker() string {
	if e.EventGatewayModifyHeadersPolicyCreate != nil && e.EventGatewayModifyHeadersPolicyCreate.Name != nil {
		return *e.EventGatewayModifyHeadersPolicyCreate.Name
	}
	if e.EventGatewayProduceSchemaValidationPolicy != nil && e.EventGatewayProduceSchemaValidationPolicy.Name != nil {
		return *e.EventGatewayProduceSchemaValidationPolicy.Name
	}
	if e.EventGatewayEncryptPolicy != nil && e.EventGatewayEncryptPolicy.Name != nil {
		return *e.EventGatewayEncryptPolicy.Name
	}
	return e.Ref
}

func (e EventGatewayProducePolicyResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.VirtualCluster != "" {
		// Dependency on parent Event Gateway Virtual Cluster when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway_virtual_cluster", Ref: e.VirtualCluster})
	}
	return deps
}

func (e EventGatewayProducePolicyResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayProducePolicyResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid produce policy ref: %w", err)
	}

	// Ensure exactly one union variant is set
	variantCount := 0
	if e.EventGatewayModifyHeadersPolicyCreate != nil {
		variantCount++
	}
	if e.EventGatewayProduceSchemaValidationPolicy != nil {
		variantCount++
	}
	if e.EventGatewayEncryptPolicy != nil {
		variantCount++
	}

	if variantCount == 0 {
		return fmt.Errorf(
			"produce policy must specify one of: '%s', '%s', or '%s' type",
			kkComps.EventGatewayProducePolicyCreateTypeModifyHeaders,
			kkComps.EventGatewayProducePolicyCreateTypeSchemaValidation,
			kkComps.EventGatewayProducePolicyCreateTypeEncrypt,
		)
	}
	if variantCount > 1 {
		return fmt.Errorf("produce policy must specify exactly one type")
	}

	return nil
}

func (e *EventGatewayProducePolicyResource) SetDefaults() {
	// No name field at the top level for union types — defaults are managed
	// inside the union variants. Nothing to set beyond what the SDK provides.
}

func (e EventGatewayProducePolicyResource) GetKonnectMonikerFilter() string {
	return "" // TODO: the API does not support filtering by name for produce policies.
}

func (e *EventGatewayProducePolicyResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if nameField.IsValid() && idField.IsValid() {
		var konnectName string
		// Name may be *string in the response type
		if nameField.Kind() == reflect.Pointer && !nameField.IsNil() {
			konnectName = nameField.Elem().String()
		} else if nameField.Kind() == reflect.String {
			konnectName = nameField.String()
		}

		if idField.Kind() == reflect.String && konnectName != "" && konnectName == e.GetMoniker() {
			e.konnectID = idField.String()
			return true
		}
	}

	return false
}

// REQUIRED: Implement ResourceWithParent
func (e EventGatewayProducePolicyResource) GetParentRef() *ResourceRef {
	if e.VirtualCluster != "" {
		return &ResourceRef{Kind: "event_gateway_virtual_cluster", Ref: e.VirtualCluster}
	}
	return nil
}

// MarshalJSON ensures produce policy metadata (ref, virtual_cluster)
// are included. Without this, the embedded union type's MarshalJSON is promoted
// and drops our metadata fields.
func (e EventGatewayProducePolicyResource) MarshalJSON() ([]byte, error) {
	// Marshal the embedded union type first to get the policy body
	policyBytes, err := json.Marshal(e.EventGatewayProducePolicyCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal produce policy: %w", err)
	}

	// Unmarshal into a generic map so we can add metadata fields
	var result map[string]any
	if err := json.Unmarshal(policyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal produce policy body: %w", err)
	}

	result["ref"] = e.Ref
	if e.VirtualCluster != "" {
		result["virtual_cluster"] = e.VirtualCluster
	}
	if e.EventGateway != "" {
		result["event_gateway"] = e.EventGateway
	}

	return json.Marshal(result)
}

// UnmarshalJSON handles the union type correctly.
// It rejects kongctl metadata and delegates to the SDK union type for the policy body.
// The SDK union type requires a "type" discriminator field to determine which variant
// to use. Supported types: modify_headers, schema_validation, encrypt.
func (e *EventGatewayProducePolicyResource) UnmarshalJSON(data []byte) error {
	// First extract our metadata fields
	var meta struct {
		Ref            string `json:"ref"`
		VirtualCluster string `json:"virtual_cluster,omitempty"`
		EventGateway   string `json:"event_gateway,omitempty"`
		Kongctl        any    `json:"kongctl,omitempty"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	if meta.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	e.Ref = meta.Ref
	e.VirtualCluster = meta.VirtualCluster
	e.EventGateway = meta.EventGateway

	// Validate required type field
	if err := validateProducePolicyTypeField(data); err != nil {
		return err
	}

	// Delegate the policy-specific fields to the SDK union type's UnmarshalJSON.
	if err := json.Unmarshal(data, &e.EventGatewayProducePolicyCreate); err != nil {
		return fmt.Errorf("failed to unmarshal produce policy: %w", err)
	}

	return nil
}

// validateProducePolicyTypeField ensures the required type discriminator is present.
func validateProducePolicyTypeField(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Define allowed types from SDK constants
	modifyHeadersType := string(kkComps.EventGatewayProducePolicyCreateTypeModifyHeaders)
	schemaValidationType := string(kkComps.EventGatewayProducePolicyCreateTypeSchemaValidation)
	encryptType := string(kkComps.EventGatewayProducePolicyCreateTypeEncrypt)

	// Validate policy type
	policyType, hasType := raw["type"]
	if !hasType {
		return fmt.Errorf(
			"produce policy requires 'type' field (one of: '%s', '%s', '%s')",
			modifyHeadersType,
			schemaValidationType,
			encryptType,
		)
	}

	policyTypeStr, ok := policyType.(string)
	if !ok {
		return fmt.Errorf("produce policy 'type' must be a string")
	}

	validTypes := []string{modifyHeadersType, schemaValidationType, encryptType}
	isValid := false
	for _, t := range validTypes {
		if policyTypeStr == t {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf(
			"produce policy 'type' must be one of '%s', '%s', or '%s', got '%s'",
			modifyHeadersType,
			schemaValidationType,
			encryptType,
			policyTypeStr,
		)
	}

	return nil
}

package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayConsumePolicy,
		func(rs *ResourceSet) *[]EventGatewayConsumePolicyResource {
			return &rs.EventGatewayConsumePolicies
		},
		AutoExplain[EventGatewayConsumePolicyResource](),
	)
}

// EventGatewayConsumePolicyResource represents a consume policy in declarative configuration.
// Consume policies are grandchildren: Event Gateway → Virtual Cluster → Consume Policy.
// The SDK represents consume policies as a discriminated union type (EventGatewayConsumePolicyCreate)
// with four variants: modify_headers, schema_validation, decrypt, skip_record.
// The "type" discriminator field is required.
type EventGatewayConsumePolicyResource struct {
	kkComps.EventGatewayConsumePolicyCreate `yaml:",inline" json:",inline"`
	Ref                                     string `yaml:"ref" json:"ref"`
	// Parent Virtual Cluster reference (for root-level definitions)
	VirtualCluster string `yaml:"virtual_cluster,omitempty" json:"virtual_cluster,omitempty"`
	EventGateway   string `yaml:"event_gateway,omitempty"   json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayConsumePolicyResource) GetType() ResourceType {
	return ResourceTypeEventGatewayConsumePolicy
}

func (e EventGatewayConsumePolicyResource) GetRef() string {
	return e.Ref
}

// GetMoniker returns the name of the policy from whichever union variant is set.
func (e EventGatewayConsumePolicyResource) GetMoniker() string {
	if e.EventGatewayModifyHeadersPolicyCreate != nil && e.EventGatewayModifyHeadersPolicyCreate.Name != nil {
		return *e.EventGatewayModifyHeadersPolicyCreate.Name
	}
	if e.EventGatewayConsumeSchemaValidationPolicy != nil &&
		e.EventGatewayConsumeSchemaValidationPolicy.Name != nil {
		return *e.EventGatewayConsumeSchemaValidationPolicy.Name
	}
	if e.EventGatewayDecryptPolicy != nil && e.EventGatewayDecryptPolicy.Name != nil {
		return *e.EventGatewayDecryptPolicy.Name
	}
	if e.EventGatewaySkipRecordPolicyCreate != nil && e.EventGatewaySkipRecordPolicyCreate.Name != nil {
		return *e.EventGatewaySkipRecordPolicyCreate.Name
	}
	return e.Ref
}

func (e EventGatewayConsumePolicyResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.VirtualCluster != "" {
		deps = append(deps, ResourceRef{Kind: ResourceTypeEventGatewayVirtualCluster, Ref: e.VirtualCluster})
	}
	return deps
}

func (e EventGatewayConsumePolicyResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayConsumePolicyResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid consume policy ref: %w", err)
	}

	// Exactly one union variant must be set — the SDK's UnmarshalJSON enforces the discriminator,
	// so here we just verify one variant is present after unmarshaling.
	hasVariant := e.EventGatewayModifyHeadersPolicyCreate != nil ||
		e.EventGatewayConsumeSchemaValidationPolicy != nil ||
		e.EventGatewayDecryptPolicy != nil ||
		e.EventGatewaySkipRecordPolicyCreate != nil
	if !hasVariant {
		return fmt.Errorf(
			"consume policy must specify 'type' field (one of: modify_headers, schema_validation, decrypt, skip_record)",
		)
	}

	return nil
}

func (e *EventGatewayConsumePolicyResource) SetDefaults() {
	// Names for union types are managed inside the variants; nothing to propagate.
}

func (e EventGatewayConsumePolicyResource) GetKonnectMonikerFilter() string {
	return "" // TODO: the API does not support filtering by name for consume policies.
}

func (e *EventGatewayConsumePolicyResource) TryMatchKonnectResource(konnectResource any) bool {
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
func (e EventGatewayConsumePolicyResource) GetParentRef() *ResourceRef {
	if e.VirtualCluster != "" {
		return &ResourceRef{Kind: ResourceTypeEventGatewayVirtualCluster, Ref: e.VirtualCluster}
	}
	return nil
}

// MarshalJSON ensures consume policy metadata (ref, virtual_cluster, event_gateway)
// are included in the serialized output.
func (e EventGatewayConsumePolicyResource) MarshalJSON() ([]byte, error) {
	// Marshal the embedded union type first to get the policy body
	policyBytes, err := json.Marshal(e.EventGatewayConsumePolicyCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal consume policy: %w", err)
	}

	// Unmarshal into a generic map so we can add metadata fields
	var result map[string]any
	if err := json.Unmarshal(policyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal consume policy body: %w", err)
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
// The SDK union type requires a "type" discriminator field to determine which variant to use:
//   - "modify_headers" → EventGatewayModifyHeadersPolicyCreate
//   - "schema_validation" → EventGatewayConsumeSchemaValidationPolicy
//   - "decrypt" → EventGatewayDecryptPolicy
//   - "skip_record" → EventGatewaySkipRecordPolicyCreate
func (e *EventGatewayConsumePolicyResource) UnmarshalJSON(data []byte) error {
	// Extract our metadata fields
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

	// Validate required type discriminator
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := validateConsumePolicyType(raw); err != nil {
		return err
	}

	// The SDK union type's UnmarshalJSON handles discriminating based on "type".
	if err := json.Unmarshal(data, &e.EventGatewayConsumePolicyCreate); err != nil {
		return fmt.Errorf("failed to unmarshal consume policy: %w", err)
	}

	return nil
}

// validateConsumePolicyType ensures the required type discriminator is present and valid.
func validateConsumePolicyType(raw map[string]any) error {
	validTypes := []string{
		string(kkComps.EventGatewayConsumePolicyCreateTypeModifyHeaders),
		string(kkComps.EventGatewayConsumePolicyCreateTypeSchemaValidation),
		string(kkComps.EventGatewayConsumePolicyCreateTypeDecrypt),
		string(kkComps.EventGatewayConsumePolicyCreateTypeSkipRecord),
	}

	policyType, hasType := raw["type"]
	if !hasType {
		return fmt.Errorf(
			"consume policy requires 'type' field (one of: modify_headers, schema_validation, decrypt, skip_record)",
		)
	}

	policyTypeStr, ok := policyType.(string)
	if !ok {
		return fmt.Errorf("consume policy 'type' must be a string")
	}

	if slices.Contains(validTypes, policyTypeStr) {
		return nil
	}

	return fmt.Errorf(
		"consume policy 'type' must be one of [modify_headers, schema_validation, decrypt, skip_record], got %q",
		policyTypeStr,
	)
}

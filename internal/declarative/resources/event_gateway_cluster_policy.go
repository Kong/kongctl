package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayClusterPolicy,
		func(rs *ResourceSet) *[]EventGatewayClusterPolicyResource { return &rs.EventGatewayClusterPolicies },
		AutoExplain[EventGatewayClusterPolicyResource](),
	)
}

// EventGatewayClusterPolicyResource represents a cluster-level policy in declarative configuration.
// The SDK represents cluster policies as a union type (EventGatewayClusterPolicyModify)
// with the "acls" variant (EventGatewayACLsPolicy).
type EventGatewayClusterPolicyResource struct {
	kkComps.EventGatewayClusterPolicyModify `yaml:",inline" json:",inline"`
	Ref                                     string `yaml:"ref"                            json:"ref"`
	// Parent Event Gateway Virtual Cluster reference (for root-level definitions)
	VirtualCluster string `yaml:"virtual_cluster,omitempty" json:"virtual_cluster,omitempty"`
	EventGateway   string `yaml:"event_gateway,omitempty"   json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayClusterPolicyResource) GetType() ResourceType {
	return ResourceTypeEventGatewayClusterPolicy
}

func (e EventGatewayClusterPolicyResource) GetRef() string {
	return e.Ref
}

// GetMoniker returns the name of the policy from whichever union variant is set.
func (e EventGatewayClusterPolicyResource) GetMoniker() string {
	if e.EventGatewayACLsPolicy != nil && e.EventGatewayACLsPolicy.Name != nil {
		return *e.EventGatewayACLsPolicy.Name
	}
	return e.Ref
}

func (e EventGatewayClusterPolicyResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.VirtualCluster != "" {
		// Dependency on parent Event Gateway Virtual Cluster when defined at root level
		deps = append(deps, ResourceRef{Kind: ResourceTypeEventGatewayVirtualCluster, Ref: e.VirtualCluster})
	}
	return deps
}

func (e EventGatewayClusterPolicyResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayClusterPolicyResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid cluster policy ref: %w", err)
	}

	// Ensure exactly one union variant is set
	hasACLs := e.EventGatewayACLsPolicy != nil
	if !hasACLs {
		return fmt.Errorf("cluster policy must specify the 'acls' type")
	}

	return nil
}

func (e *EventGatewayClusterPolicyResource) SetDefaults() {
	// No name field at the top level for union types — defaults are managed
	// inside the union variants. Nothing to set beyond what the SDK provides.
}

func (e EventGatewayClusterPolicyResource) GetKonnectMonikerFilter() string {
	return "" // TODO: the API does not support filtering by name for cluster policies.
}

func (e *EventGatewayClusterPolicyResource) TryMatchKonnectResource(konnectResource any) bool {
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
func (e EventGatewayClusterPolicyResource) GetParentRef() *ResourceRef {
	if e.VirtualCluster != "" {
		return &ResourceRef{Kind: ResourceTypeEventGatewayVirtualCluster, Ref: e.VirtualCluster}
	}
	return nil
}

// MarshalJSON ensures cluster policy metadata (ref, virtual_cluster)
// are included. Without this, the embedded union type's MarshalJSON is promoted
// and drops our metadata fields.
func (e EventGatewayClusterPolicyResource) MarshalJSON() ([]byte, error) {
	// Marshal the embedded union type first to get the policy body
	policyBytes, err := json.Marshal(e.EventGatewayClusterPolicyModify)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cluster policy: %w", err)
	}

	// Unmarshal into a generic map so we can add metadata fields
	var result map[string]any
	if err := json.Unmarshal(policyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster policy body: %w", err)
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
// to use. Currently only "acls" type is supported (EventGatewayACLsPolicy).
func (e *EventGatewayClusterPolicyResource) UnmarshalJSON(data []byte) error {
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

	// Validate required type field and config in one pass
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := validateClusterPolicyType(raw); err != nil {
		return err
	}
	if err := validateClusterPolicyConfigMap(raw); err != nil {
		return err
	}

	// Delegate the policy-specific fields to the SDK union type's UnmarshalJSON.
	if err := json.Unmarshal(data, &e.EventGatewayClusterPolicyModify); err != nil {
		return fmt.Errorf("failed to unmarshal cluster policy: %w", err)
	}

	return nil
}

// validateClusterPolicyType ensures the required type discriminator is present.
// - Policy level: "type" is required (currently only "acls" is supported)
func validateClusterPolicyType(raw map[string]any) error {
	// Define allowed types from SDK constants
	aclsType := string(kkComps.EventGatewayClusterPolicyModifyTypeAcls)

	// Validate policy type
	policyType, hasType := raw["type"]
	if !hasType {
		return fmt.Errorf("cluster policy requires 'type' field ('%s')", aclsType)
	}

	policyTypeStr, ok := policyType.(string)
	if !ok {
		return fmt.Errorf("cluster policy 'type' must be a string")
	}

	if policyTypeStr != aclsType {
		return fmt.Errorf("cluster policy 'type' must be '%s', got '%s'", aclsType, policyTypeStr)
	}

	return nil
}

// validateClusterPolicyConfigMap ensures config is present and config.rules has at least one element.
func validateClusterPolicyConfigMap(raw map[string]any) error {
	config, hasConfig := raw["config"]
	if !hasConfig {
		return fmt.Errorf("cluster policy requires 'config' field")
	}

	configMap, ok := config.(map[string]any)
	if !ok {
		return fmt.Errorf("cluster policy 'config' must be an object")
	}

	rules, hasRules := configMap["rules"]
	if !hasRules {
		return fmt.Errorf("cluster policy config requires 'rules' field")
	}

	rulesSlice, ok := rules.([]any)
	if !ok {
		return fmt.Errorf("cluster policy config 'rules' must be an array")
	}

	if len(rulesSlice) < 1 {
		return fmt.Errorf("cluster policy config 'rules' must have at least one element")
	}

	return nil
}

package resources

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayClusterPolicy,
		func(rs *ResourceSet) *[]EventGatewayClusterPolicyResource { return &rs.EventGatewayClusterPolicies },
		AutoExplain[EventGatewayClusterPolicyResource](
			WithExplainSchemaBuilder(eventGatewayClusterPolicyExplainNode),
		),
		WithChildSyncScope(
			ResourceTypeEventGatewayVirtualCluster,
			WithNestedScopeWithin(ResourceTypeEventGatewayControlPlane),
		),
	)
}

// EventGatewayClusterPolicyResource represents a cluster-level policy in declarative configuration.
// The SDK represents cluster policies as a union type (EventGatewayClusterPolicyModify)
// with acls and request_rule_validator variants.
type EventGatewayClusterPolicyResource struct {
	kkComps.EventGatewayClusterPolicyModify `yaml:",inline" json:",inline"`
	Ref                                     string `yaml:"ref"                       json:"ref"`
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
	if e.EventGatewayRequestRuleValidatorPolicy != nil && e.EventGatewayRequestRuleValidatorPolicy.Name != nil {
		return *e.EventGatewayRequestRuleValidatorPolicy.Name
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
	hasValidator := e.EventGatewayRequestRuleValidatorPolicy != nil
	if !hasACLs && !hasValidator {
		return fmt.Errorf("cluster policy must specify the 'acls' or 'request_rule_validator' type")
	}
	if hasACLs && hasValidator {
		return fmt.Errorf("cluster policy must specify exactly one type")
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
	return matchEventGatewayPolicy(&e.konnectID, e.GetMoniker(), konnectResource)
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
	return marshalEventGatewayPolicy(e.EventGatewayClusterPolicyModify, "cluster", e.Ref, e.VirtualCluster, e.EventGateway)
}

// UnmarshalJSON handles the union type correctly.
// It rejects kongctl metadata and delegates to the SDK union type for the policy body.
// The SDK union type requires a "type" discriminator field to determine which variant
// to use: acls or request_rule_validator.
func (e *EventGatewayClusterPolicyResource) UnmarshalJSON(data []byte) error {
	meta, err := unmarshalEventGatewayPolicyMetadata(data)
	if err != nil {
		return err
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
// - Policy level: "type" is required.
func validateClusterPolicyType(raw map[string]any) error {
	// Define allowed types from SDK constants
	validTypes := []string{
		string(kkComps.EventGatewayClusterPolicyModifyTypeAcls),
		string(kkComps.EventGatewayClusterPolicyModifyTypeRequestRuleValidator),
	}
	validTypesFmt := strings.Join(validTypes, ", ")

	// Validate policy type
	policyType, hasType := raw["type"]
	if !hasType {
		return fmt.Errorf("cluster policy requires 'type' field (one of: %s)", validTypesFmt)
	}

	policyTypeStr, ok := policyType.(string)
	if !ok {
		return fmt.Errorf("cluster policy 'type' must be a string")
	}

	if !slices.Contains(validTypes, policyTypeStr) {
		return fmt.Errorf("cluster policy 'type' must be one of [%s], got %q", validTypesFmt, policyTypeStr)
	}

	return nil
}

// validateClusterPolicyConfigMap checks the collection required by each policy type.
func validateClusterPolicyConfigMap(raw map[string]any) error {
	config, hasConfig := raw["config"]
	if !hasConfig {
		return fmt.Errorf("cluster policy requires 'config' field")
	}

	configMap, ok := config.(map[string]any)
	if !ok {
		return fmt.Errorf("cluster policy 'config' must be an object")
	}

	if raw["type"] == string(kkComps.EventGatewayClusterPolicyModifyTypeRequestRuleValidator) {
		requests, ok := configMap["requests"].([]any)
		if !ok || len(requests) == 0 {
			return fmt.Errorf("request rule validator config requires a non-empty 'requests' array")
		}
		return nil
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

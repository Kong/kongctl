package resources

import (
	"encoding/json"
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// EventGatewayListenerPolicyResource represents a listener policy in declarative configuration.
// The SDK represents listener policies as a union type (EventGatewayListenerPolicyCreate)
// with two variants: TLSServer and ForwardToVirtualCluster.
type EventGatewayListenerPolicyResource struct {
	kkComps.EventGatewayListenerPolicyCreate `yaml:",inline" json:",inline"`
	Ref                                      string `yaml:"ref"                        json:"ref"`
	// Parent Event Gateway Listener reference (for root-level definitions)
	EventGatewayListener string `yaml:"listener,omitempty" json:"listener,omitempty"`
	EventGateway         string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayListenerPolicyResource) GetType() ResourceType {
	return ResourceTypeEventGatewayListenerPolicy
}

func (e EventGatewayListenerPolicyResource) GetRef() string {
	return e.Ref
}

// GetMoniker returns the name of the policy from whichever union variant is set.
func (e EventGatewayListenerPolicyResource) GetMoniker() string {
	if e.EventGatewayTLSListenerPolicy != nil && e.EventGatewayTLSListenerPolicy.Name != nil {
		return *e.EventGatewayTLSListenerPolicy.Name
	}
	if e.ForwardToVirtualClusterPolicy != nil && e.ForwardToVirtualClusterPolicy.Name != nil {
		return *e.ForwardToVirtualClusterPolicy.Name
	}
	return e.Ref
}

func (e EventGatewayListenerPolicyResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGatewayListener != "" {
		// Dependency on parent Event Gateway Listener when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway_listener", Ref: e.EventGatewayListener})
	}
	return deps
}

func (e EventGatewayListenerPolicyResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayListenerPolicyResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid listener policy ref: %w", err)
	}

	// Ensure exactly one union variant is set
	hasTLS := e.EventGatewayTLSListenerPolicy != nil
	hasForward := e.ForwardToVirtualClusterPolicy != nil
	if !hasTLS && !hasForward {
		return fmt.Errorf("listener policy must specify either tls_server or forward_to_virtual_cluster")
	}

	return nil
}

func (e *EventGatewayListenerPolicyResource) SetDefaults() {
	// No name field at the top level for union types — defaults are managed
	// inside the union variants. Nothing to set beyond what the SDK provides.
}

func (e EventGatewayListenerPolicyResource) GetKonnectMonikerFilter() string {
	return "" // TODO: the API does not support filtering by name for listener policies.
}

func (e *EventGatewayListenerPolicyResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Ptr {
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
		if nameField.Kind() == reflect.Ptr && !nameField.IsNil() {
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
func (e EventGatewayListenerPolicyResource) GetParentRef() *ResourceRef {
	if e.EventGatewayListener != "" {
		return &ResourceRef{Kind: "event_gateway_listener", Ref: e.EventGatewayListener}
	}
	return nil
}

// MarshalJSON ensures listener policy metadata (ref, event_gateway_listener)
// are included. Without this, the embedded union type's MarshalJSON is promoted
// and drops our metadata fields.
func (e EventGatewayListenerPolicyResource) MarshalJSON() ([]byte, error) {
	// Marshal the embedded union type first to get the policy body
	policyBytes, err := json.Marshal(e.EventGatewayListenerPolicyCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal listener policy: %w", err)
	}

	// Unmarshal into a generic map so we can add metadata fields
	var result map[string]any
	if err := json.Unmarshal(policyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal listener policy body: %w", err)
	}

	result["ref"] = e.Ref
	if e.EventGatewayListener != "" {
		result["listener"] = e.EventGatewayListener
	}

	return json.Marshal(result)
}

// UnmarshalJSON handles the union type correctly.
// It rejects kongctl metadata and delegates to the SDK union type for the policy body.
// The SDK union type requires a "type" discriminator field to determine which variant
// (tls_server or forward_to_virtual_cluster) to use.
// For forward_to_virtual_cluster policies, the config must also have a "type" field
// ("sni" or "port_mapping") since ForwardToVirtualClusterPolicyConfig is also a union type.
func (e *EventGatewayListenerPolicyResource) UnmarshalJSON(data []byte) error {
	// First extract our metadata fields
	var meta struct {
		Ref                  string `json:"ref"`
		EventGatewayListener string `json:"listener,omitempty"`
		Kongctl              any    `json:"kongctl,omitempty"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	if meta.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	e.Ref = meta.Ref
	e.EventGatewayListener = meta.EventGatewayListener

	// Validate required type fields
	if err := validatePolicyTypeFields(data); err != nil {
		return err
	}

	// Delegate the policy-specific fields to the SDK union type's UnmarshalJSON.
	if err := json.Unmarshal(data, &e.EventGatewayListenerPolicyCreate); err != nil {
		return fmt.Errorf("failed to unmarshal listener policy: %w", err)
	}

	return nil
}

// validatePolicyTypeFields ensures required type discriminators are present.
// - Policy level: "type" is always required (uses SDK's EventGatewayListenerPolicyCreateType)
// - Config level: "config.type" is required only for forward_to_virtual_cluster
func validatePolicyTypeFields(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Define allowed types from SDK constants
	tlsServerType := string(kkComps.EventGatewayListenerPolicyCreateTypeTLSServer)
	forwardToVCType := string(kkComps.EventGatewayListenerPolicyCreateTypeForwardToVirtualCluster)
	sniConfigType := string(kkComps.ForwardToVirtualClusterPolicyConfigTypeSni)
	portMappingConfigType := string(kkComps.ForwardToVirtualClusterPolicyConfigTypePortMapping)

	// Validate policy type
	policyType, hasType := raw["type"]
	if !hasType {
		return fmt.Errorf("listener policy requires 'type' field ('%s' or '%s')", tlsServerType, forwardToVCType)
	}

	policyTypeStr, ok := policyType.(string)
	if !ok {
		return fmt.Errorf("listener policy 'type' must be a string")
	}

	if policyTypeStr != tlsServerType && policyTypeStr != forwardToVCType {
		return fmt.Errorf(
			"listener policy 'type' must be '%s' or '%s', got '%s'", tlsServerType, forwardToVCType, policyTypeStr)
	}

	// For forward_to_virtual_cluster, validate config.type
	if policyTypeStr == forwardToVCType {
		config, hasConfig := raw["config"]
		if !hasConfig {
			return fmt.Errorf("%s policy requires 'config' field", forwardToVCType)
		}

		configMap, ok := config.(map[string]any)
		if !ok {
			return fmt.Errorf("listener policy 'config' must be an object")
		}

		configType, hasConfigType := configMap["type"]
		if !hasConfigType {
			return fmt.Errorf("%s policy config requires 'type' field ('%s' or '%s')",
				forwardToVCType, sniConfigType, portMappingConfigType)
		}

		configTypeStr, ok := configType.(string)
		if !ok {
			return fmt.Errorf("listener policy config 'type' must be a string")
		}

		if configTypeStr != sniConfigType && configTypeStr != portMappingConfigType {
			return fmt.Errorf("%s config 'type' must be '%s' or '%s', got '%s'",
				forwardToVCType, sniConfigType, portMappingConfigType, configTypeStr)
		}
	}

	return nil
}

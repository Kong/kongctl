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
	EventGatewayListener string `yaml:"listener,omitempty" json:"listener,omitempty"` //nolint:lll

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
		result["event_gateway_listener"] = e.EventGatewayListener
	}

	return json.Marshal(result)
}

// UnmarshalJSON handles the union type correctly.
// It rejects kongctl metadata and delegates to the SDK union type for the policy body.
// The SDK union type requires a "type" discriminator field to determine which variant
// (tls_server or forward_to_virtual_cluster) to use. If not present, we detect the
// policy type based on the config fields and inject the discriminator.
func (e *EventGatewayListenerPolicyResource) UnmarshalJSON(data []byte) error {
	// First extract our metadata fields
	var meta struct {
		Ref                  string `json:"ref"`
		EventGatewayListener string `json:"event_gateway_listener,omitempty"`
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

	// Check if the "type" discriminator is present. If not, detect and inject it.
	// The SDK requires "type" to be "tls_server" or "forward_to_virtual_cluster".
	dataWithType, err := ensurePolicyTypeDiscriminator(data)
	if err != nil {
		return fmt.Errorf("failed to determine listener policy type: %w", err)
	}

	// Delegate the policy-specific fields to the SDK union type's UnmarshalJSON.
	if err := json.Unmarshal(dataWithType, &e.EventGatewayListenerPolicyCreate); err != nil {
		return fmt.Errorf("failed to unmarshal listener policy: %w", err)
	}

	return nil
}

// ensurePolicyTypeDiscriminator checks if the JSON data has a "type" field.
// If not present, it detects the policy type based on the config structure
// and injects the appropriate discriminator ("tls_server" or "forward_to_virtual_cluster").
// For forward_to_virtual_cluster policies, it also ensures the config has a "type" field
// ("sni" or "port_mapping") since ForwardToVirtualClusterPolicyConfig is also a union type.
func ensurePolicyTypeDiscriminator(data []byte) ([]byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	modified := false

	// If "type" is not present, detect and inject it
	if _, hasType := raw["type"]; !hasType {
		policyType := detectListenerPolicyType(raw)
		if policyType == "" {
			return nil, fmt.Errorf("unable to determine policy type: must have 'type' field " +
				"or config with 'certificates' (for tls_server) or SNI/port_mapping fields (for forward_to_virtual_cluster)")
		}
		raw["type"] = policyType
		modified = true
	}

	// For forward_to_virtual_cluster policies, ensure config has inner "type" discriminator
	if raw["type"] == "forward_to_virtual_cluster" {
		if config, hasConfig := raw["config"]; hasConfig {
			if configMap, ok := config.(map[string]any); ok {
				if _, hasConfigType := configMap["type"]; !hasConfigType {
					// Detect inner config type based on fields
					configType := detectForwardPolicyConfigType(configMap)
					if configType != "" {
						configMap["type"] = configType
						raw["config"] = configMap
						modified = true
					}
				}
			}
		}
	}

	if modified {
		return json.Marshal(raw)
	}
	return data, nil
}

// detectForwardPolicyConfigType determines the config type for forward_to_virtual_cluster policies.
// Returns "sni" if sni_suffix is present, "port_mapping" if destination/port_mappings is present.
func detectForwardPolicyConfigType(configMap map[string]any) string {
	// SNI config indicators
	if _, hasSNISuffix := configMap["sni_suffix"]; hasSNISuffix {
		return "sni"
	}
	if _, hasAdvertisedPort := configMap["advertised_port"]; hasAdvertisedPort {
		return "sni"
	}
	if _, hasBrokerHostFormat := configMap["broker_host_format"]; hasBrokerHostFormat {
		return "sni"
	}

	// Port mapping config indicators: destination, advertised_host, port_mappings
	if _, hasDestination := configMap["destination"]; hasDestination {
		return "port_mapping"
	}
	if _, hasAdvertisedHost := configMap["advertised_host"]; hasAdvertisedHost {
		return "port_mapping"
	}
	if _, hasPortMappings := configMap["port_mappings"]; hasPortMappings {
		return "port_mapping"
	}
	if _, hasBootstrapPort := configMap["bootstrap_port"]; hasBootstrapPort {
		return "port_mapping"
	}
	if _, hasMinBrokerID := configMap["min_broker_id"]; hasMinBrokerID {
		return "port_mapping"
	}

	return ""
}

// detectListenerPolicyType determines the policy type based on the config structure.
// TLS policies have config with "certificates", "versions", "allow_plaintext".
// Forward policies have config with "destination", "type" (sni/port_mapping), "virtual_cluster", etc.
func detectListenerPolicyType(raw map[string]any) string {
	config, hasConfig := raw["config"]
	if !hasConfig {
		return ""
	}

	configMap, ok := config.(map[string]any)
	if !ok {
		return ""
	}

	// TLS policy indicators: certificates, versions, allow_plaintext
	if _, hasCerts := configMap["certificates"]; hasCerts {
		return "tls_server"
	}
	if _, hasVersions := configMap["versions"]; hasVersions {
		return "tls_server"
	}
	if _, hasAllowPlaintext := configMap["allow_plaintext"]; hasAllowPlaintext {
		return "tls_server"
	}

	// Forward policy indicators: destination, type (sni/port_mapping), virtual_cluster, port_mappings, advertised_host
	if _, hasDestination := configMap["destination"]; hasDestination {
		return "forward_to_virtual_cluster"
	}
	if _, hasAdvertisedHost := configMap["advertised_host"]; hasAdvertisedHost {
		return "forward_to_virtual_cluster"
	}
	if configType, hasConfigType := configMap["type"]; hasConfigType {
		if t, ok := configType.(string); ok && (t == "sni" || t == "port_mapping") {
			return "forward_to_virtual_cluster"
		}
	}
	if _, hasVC := configMap["virtual_cluster"]; hasVC {
		return "forward_to_virtual_cluster"
	}
	if _, hasPortMappings := configMap["port_mappings"]; hasPortMappings {
		return "forward_to_virtual_cluster"
	}
	if _, hasSNISuffix := configMap["sni_suffix"]; hasSNISuffix {
		return "forward_to_virtual_cluster"
	}

	return ""
}

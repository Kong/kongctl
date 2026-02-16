package resources

import (
	"fmt"
	"reflect"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/util"
	"sigs.k8s.io/yaml"
)

// GatewayServiceResource represents a gateway service within a control plane.
type GatewayServiceResource struct {
	// Service captures inline gateway service properties. It is excluded from direct
	// YAML/JSON serialization because external services are declared without payloads;
	// the custom UnmarshalYAML implementation below materializes this struct only when
	// inline fields are present in the configuration.
	Service      *kkComps.Service `yaml:"-"                       json:"-"`
	Ref          string           `yaml:"ref"                     json:"ref"`
	ControlPlane string           `yaml:"control_plane,omitempty" json:"control_plane,omitempty"`
	External     *ExternalBlock   `yaml:"_external,omitempty"     json:"_external,omitempty"`

	// Resolved Konnect identifiers (not serialized)
	konnectID             string `yaml:"-" json:"-"`
	controlPlaneKonnectID string `yaml:"-" json:"-"`
	// deckBaseDir removed (deck config is now control-plane scoped).
}

// GetType returns the resource type.
func (s GatewayServiceResource) GetType() ResourceType {
	return ResourceTypeGatewayService
}

// GetRef returns the declarative reference.
func (s GatewayServiceResource) GetRef() string {
	return s.Ref
}

// GetMoniker returns a human-friendly identifier (the service name when available).
func (s GatewayServiceResource) GetMoniker() string {
	if s.Service != nil && s.Service.Name != nil {
		return *s.Service.Name
	}
	return ""
}

// GetDependencies declares the resource dependencies.
func (s GatewayServiceResource) GetDependencies() []ResourceRef {
	deps := make([]ResourceRef, 0, 1)
	if s.ControlPlane != "" {
		deps = append(deps, ResourceRef{Kind: string(ResourceTypeControlPlane), Ref: s.ControlPlane})
	}
	return deps
}

// GetReferenceFieldMappings returns reference validation mappings.
func (s GatewayServiceResource) GetReferenceFieldMappings() map[string]string {
	mappings := make(map[string]string)
	if s.ControlPlane != "" && !util.IsValidUUID(s.ControlPlane) {
		mappings["control_plane"] = string(ResourceTypeControlPlane)
	}
	return mappings
}

// Validate ensures the resource is well-formed.
func (s GatewayServiceResource) Validate() error {
	if err := ValidateRef(s.Ref); err != nil {
		return fmt.Errorf("invalid gateway_service ref: %w", err)
	}

	if s.ControlPlane == "" {
		return fmt.Errorf("gateway_service control_plane is required")
	}

	if !s.IsExternal() {
		if s.Service == nil {
			return fmt.Errorf("gateway_service %s: inline configuration is required for managed services", s.Ref)
		}
		if strings.TrimSpace(s.Service.Host) == "" {
			return fmt.Errorf("gateway_service %s: host is required", s.Ref)
		}
	}

	if s.External != nil {
		if err := s.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
		if s.External.Selector != nil {
			if len(s.External.Selector.MatchFields) != 1 {
				return fmt.Errorf("gateway_service %s: selector supports matchFields.name only", s.Ref)
			}
			if _, ok := s.External.Selector.MatchFields["name"]; !ok {
				return fmt.Errorf("gateway_service %s: selector supports matchFields.name only", s.Ref)
			}
		}
	}

	return nil
}

// UnmarshalYAML customizes decoding so external services without inline fields bypass SDK validation.
// Inline gateway service attributes populate the embedded SDK struct, while purely external
// resources leave it nil so serialization continues to emit only the external metadata.
func (s *GatewayServiceResource) UnmarshalYAML(unmarshal func(any) error) error {
	type alias struct {
		Ref          string         `yaml:"ref"`
		ControlPlane string         `yaml:"control_plane"`
		External     *ExternalBlock `yaml:"_external"`
		Kongctl      *KongctlMeta   `yaml:"kongctl"`
		Inline       map[string]any `yaml:",inline"`
	}

	var raw alias
	if err := unmarshal(&raw); err != nil {
		return err
	}

	s.Ref = raw.Ref
	s.ControlPlane = raw.ControlPlane
	s.External = raw.External
	// Kongctl metadata is stored via dedicated field on resource set when needed; ignore here.

	// Remove known non-service fields that might appear in Inline map if not declared explicitly.
	delete(raw.Inline, "ref")
	delete(raw.Inline, "control_plane")
	delete(raw.Inline, "_external")
	delete(raw.Inline, "kongctl")

	if len(raw.Inline) == 0 {
		s.Service = nil
		return nil
	}

	// Marshal inline fields back to YAML and decode using SDK struct for consistency.
	data, err := yaml.Marshal(raw.Inline)
	if err != nil {
		return fmt.Errorf("marshal gateway service fields: %w", err)
	}

	var svc kkComps.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return err
	}
	s.Service = &svc
	return nil
}

// SetDefaults applies default values where applicable.
func (s *GatewayServiceResource) SetDefaults() {
	// For now there are no defaults to apply.
}

// GetKonnectID returns the resolved Konnect service ID if available.
func (s GatewayServiceResource) GetKonnectID() string {
	return s.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect lookups.
func (s GatewayServiceResource) GetKonnectMonikerFilter() string {
	// Gateway services currently rely on explicit resolution.
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect service object.
func (s *GatewayServiceResource) TryMatchKonnectResource(konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false
	}

	idField := v.FieldByName("ID")
	if !idField.IsValid() || idField.Kind() != reflect.String {
		// The service type promoted through the SDK exposes ID as string
		return false
	}

	controlPlaneIDField := v.FieldByName("ControlPlaneID")
	if !controlPlaneIDField.IsValid() || controlPlaneIDField.Kind() != reflect.String {
		return false
	}

	match := false
	if s.IsExternal() && s.External != nil {
		if s.External.ID != "" {
			match = (idField.String() == s.External.ID)
		} else if s.External.Selector != nil {
			match = s.External.Selector.Match(konnectResource)
		}
	} else if s.Service != nil && s.Service.Name != nil {
		nameField := v.FieldByName("Name")
		if !nameField.IsValid() {
			return false
		}
		match = nameField.Kind() == reflect.String && nameField.String() == *s.Service.Name
	}

	if !match {
		return false
	}

	s.konnectID = idField.String()
	s.controlPlaneKonnectID = controlPlaneIDField.String()
	return true
}

// GetParentRef returns the parent control plane reference.
func (s GatewayServiceResource) GetParentRef() *ResourceRef {
	if s.ControlPlane == "" {
		return nil
	}
	return &ResourceRef{Kind: string(ResourceTypeControlPlane), Ref: s.ControlPlane}
}

// ResolvedControlPlaneID exposes the konnect control plane identifier.
func (s GatewayServiceResource) ResolvedControlPlaneID() string {
	return s.controlPlaneKonnectID
}

// SetResolvedControlPlaneID records the Konnect control plane ID for this service.
func (s *GatewayServiceResource) SetResolvedControlPlaneID(id string) {
	s.controlPlaneKonnectID = id
}

// IsExternal returns true when the service is marked as externally managed.
func (s *GatewayServiceResource) IsExternal() bool {
	return s.External != nil && s.External.IsExternal()
}

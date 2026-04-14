package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewaySchemaRegistry,
		func(rs *ResourceSet) *[]EventGatewaySchemaRegistryResource {
			return &rs.EventGatewaySchemaRegistries
		},
		AutoExplain[EventGatewaySchemaRegistryResource](),
	)
}

// EventGatewaySchemaRegistryResource represents a schema registry in declarative configuration.
// The SDK represents schema registries as a union type (SchemaRegistryCreate) with the
// "confluent" variant (SchemaRegistryConfluent).
type EventGatewaySchemaRegistryResource struct {
	kkComps.SchemaRegistryCreate `yaml:",inline" json:",inline"`
	Ref                          string `yaml:"ref"                            json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewaySchemaRegistryResource) GetType() ResourceType {
	return ResourceTypeEventGatewaySchemaRegistry
}

func (e EventGatewaySchemaRegistryResource) GetRef() string {
	return e.Ref
}

// GetMoniker returns the name of the schema registry from whichever union variant is set.
func (e EventGatewaySchemaRegistryResource) GetMoniker() string {
	if e.SchemaRegistryConfluent != nil {
		return e.SchemaRegistryConfluent.Name
	}
	return e.Ref
}

func (e EventGatewaySchemaRegistryResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGateway != "" {
		// Dependency on parent Event Gateway when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway", Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewaySchemaRegistryResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewaySchemaRegistryResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid schema registry ref: %w", err)
	}

	// Ensure exactly one union variant is set and that it's confluent
	if e.SchemaRegistryConfluent == nil {
		return fmt.Errorf("schema registry must specify the 'confluent' type")
	}

	return nil
}

func (e *EventGatewaySchemaRegistryResource) SetDefaults() {
	// Name defaults are managed inside the union variant.
	// If the confluent variant has no name, use ref.
	if e.SchemaRegistryConfluent != nil && e.SchemaRegistryConfluent.Name == "" {
		e.SchemaRegistryConfluent.Name = e.Ref
	}
}

func (e EventGatewaySchemaRegistryResource) GetKonnectMonikerFilter() string {
	return "" // TODO: The API does not support server-side filtering by name for schema registries.
}

func (e *EventGatewaySchemaRegistryResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", e.GetMoniker()); id != "" {
		e.konnectID = id
		return true
	}
	return false
}

// REQUIRED: Implement ResourceWithParent
func (e EventGatewaySchemaRegistryResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: "event_gateway", Ref: e.EventGateway}
	}
	return nil
}

// MarshalJSON ensures schema registry metadata (ref, event_gateway) are included along with
// the union body. Without this, the embedded SchemaRegistryCreate's MarshalJSON is promoted
// and drops our metadata fields.
func (e EventGatewaySchemaRegistryResource) MarshalJSON() ([]byte, error) {
	// Marshal the embedded union type first to get the registry body
	registryBytes, err := json.Marshal(e.SchemaRegistryCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema registry: %w", err)
	}

	// Unmarshal into a generic map so we can add metadata fields
	var result map[string]any
	if err := json.Unmarshal(registryBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema registry body: %w", err)
	}

	result["ref"] = e.Ref
	if e.EventGateway != "" {
		result["event_gateway"] = e.EventGateway
	}

	return json.Marshal(result)
}

// UnmarshalJSON handles the union type correctly.
// It rejects kongctl metadata and delegates to the SDK union type for the registry body.
// The SDK union type requires a "type" discriminator field to determine which variant to use.
// Currently only "confluent" type is supported.
func (e *EventGatewaySchemaRegistryResource) UnmarshalJSON(data []byte) error {
	// First extract our metadata fields
	var meta struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`
		Kongctl      any    `json:"kongctl,omitempty"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	if meta.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	e.Ref = meta.Ref
	e.EventGateway = meta.EventGateway

	// Validate required type discriminator field
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := validateSchemaRegistryType(raw); err != nil {
		return err
	}

	// Delegate the registry-specific fields to the SDK union type's UnmarshalJSON.
	if err := json.Unmarshal(data, &e.SchemaRegistryCreate); err != nil {
		return fmt.Errorf("failed to unmarshal schema registry: %w", err)
	}

	return nil
}

// validateSchemaRegistryType ensures the required type discriminator is present.
func validateSchemaRegistryType(raw map[string]any) error {
	typeVal, ok := raw["type"]
	if !ok {
		return fmt.Errorf("schema registry must specify a 'type' field (currently only 'confluent' is supported)")
	}

	allowedType := string(kkComps.SchemaRegistryCreateTypeConfluent)
	typStr, _ := typeVal.(string)
	if typStr != allowedType {
		return fmt.Errorf("unsupported schema registry type %q (currently only %q is supported)", typStr, allowedType)
	}

	return nil
}

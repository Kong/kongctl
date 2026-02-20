package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

type EventGatewayVirtualClusterResource struct {
	kkComps.CreateVirtualClusterRequest `       yaml:",inline"                 json:",inline"`
	Ref                                 string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayVirtualClusterResource) GetType() ResourceType {
	return ResourceTypeEventGatewayVirtualCluster
}

func (e EventGatewayVirtualClusterResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayVirtualClusterResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayVirtualClusterResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGateway != "" {
		// Dependency on parent Event Gateway when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway", Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewayVirtualClusterResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayVirtualClusterResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid child ref: %w", err)
	}

	return nil
}

func (e *EventGatewayVirtualClusterResource) SetDefaults() {
	// If Name is not set, use ref as default
	if e.Name == "" {
		e.Name = e.Ref
	}
}

func (e EventGatewayVirtualClusterResource) GetKonnectMonikerFilter() string {
	return fmt.Sprintf("name[eq]=%s", e.Name) // TODO: the API does not support filtering by name.
}

func (e *EventGatewayVirtualClusterResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", e.Name); id != "" {
		e.konnectID = id
		return true
	}
	return false
}

// REQUIRED: Implement ResourceWithParent
func (e EventGatewayVirtualClusterResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: "event_gateway", Ref: e.EventGateway}
	}
	return nil
}

// Custom JSON unmarshaling to reject kongctl metadata
func (e *EventGatewayVirtualClusterResource) UnmarshalJSON(data []byte) error {
	// Temporary structure for unmarshaling resource metadata together with
	// the CreateVirtualClusterRequest fields from the SDK.
	var temp struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`
		Kongctl      any    `json:"kongctl,omitempty"`

		// Fields from kkComps.CreateVirtualClusterRequest
		Name           string                                       `json:"name"`
		Description    *string                                      `json:"description,omitempty"`
		Destination    kkComps.BackendClusterReferenceModify        `json:"destination"`
		Authentication []kkComps.VirtualClusterAuthenticationScheme `json:"authentication"`
		Namespace      *kkComps.VirtualClusterNamespace             `json:"namespace,omitempty"`
		ACLMode        kkComps.VirtualClusterACLMode                `json:"acl_mode"`
		DNSLabel       string                                       `json:"dns_label"`
		Labels         map[string]string                            `json:"labels,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on child resources")
	}

	// Populate resource metadata
	e.Ref = temp.Ref
	e.EventGateway = temp.EventGateway

	// Populate embedded CreateVirtualClusterRequest fields
	e.Name = temp.Name
	e.Description = temp.Description
	e.Destination = temp.Destination
	e.Authentication = temp.Authentication
	e.Namespace = temp.Namespace
	e.ACLMode = temp.ACLMode
	e.DNSLabel = temp.DNSLabel
	e.Labels = temp.Labels

	return nil
}

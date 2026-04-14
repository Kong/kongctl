package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayControlPlane,
		func(rs *ResourceSet) *[]EventGatewayControlPlaneResource { return &rs.EventGatewayControlPlanes },
		AutoExplain[EventGatewayControlPlaneResource](),
	)
}

type EventGatewayControlPlaneResource struct {
	BaseResource
	kkComps.CreateGatewayRequest `yaml:",inline" json:",inline"`

	// Nested child resources
	BackendClusters       []EventGatewayBackendClusterResource       `yaml:"backend_clusters,omitempty"        json:"backend_clusters,omitempty"`        //nolint:lll
	VirtualClusters       []EventGatewayVirtualClusterResource       `yaml:"virtual_clusters,omitempty"        json:"virtual_clusters,omitempty"`        //nolint:lll
	Listeners             []EventGatewayListenerResource             `yaml:"listeners,omitempty"               json:"listeners,omitempty"`               //nolint:lll
	DataPlaneCertificates []EventGatewayDataPlaneCertificateResource `yaml:"data_plane_certificates,omitempty" json:"data_plane_certificates,omitempty"` //nolint:lll
	SchemaRegistries      []EventGatewaySchemaRegistryResource       `yaml:"schema_registries,omitempty"       json:"schema_registries,omitempty"`       //nolint:lll
}

func (e EventGatewayControlPlaneResource) GetType() ResourceType {
	return ResourceTypeEventGatewayControlPlane
}

func (e EventGatewayControlPlaneResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayControlPlaneResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

func (e EventGatewayControlPlaneResource) GetLabels() map[string]string {
	return e.Labels
}

func (e *EventGatewayControlPlaneResource) SetLabels(labels map[string]string) {
	// Convert map to SDK format
	e.Labels = labels
}

func (e EventGatewayControlPlaneResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid Event Gateway Control Plane ref: %w", err)
	}

	// Validate backend clusters
	backendClusterRefs := make(map[string]bool)
	for i, bc := range e.BackendClusters {
		if err := bc.Validate(); err != nil {
			return fmt.Errorf("invalid backend cluster %d: %w", i, err)
		}
		if backendClusterRefs[bc.GetRef()] {
			return fmt.Errorf("duplicate backend cluster ref: %s", bc.GetRef())
		}
		backendClusterRefs[bc.GetRef()] = true
	}

	// Validate virtual clusters
	virtualClusterRefs := make(map[string]bool)
	for i, vc := range e.VirtualClusters {
		if err := vc.Validate(); err != nil {
			return fmt.Errorf("invalid virtual cluster %d: %w", i, err)
		}
		if virtualClusterRefs[vc.GetRef()] {
			return fmt.Errorf("duplicate virtual cluster ref: %s", vc.GetRef())
		}
		virtualClusterRefs[vc.GetRef()] = true
	}

	// Validate listeners
	listenerRefs := make(map[string]bool)
	for i, l := range e.Listeners {
		if err := l.Validate(); err != nil {
			return fmt.Errorf("invalid listener %d: %w", i, err)
		}
		if listenerRefs[l.GetRef()] {
			return fmt.Errorf("duplicate listener ref: %s", l.GetRef())
		}
		listenerRefs[l.GetRef()] = true
	}

	// Validate data plane certificates
	dataPlaneCertRefs := make(map[string]bool)
	for i, dpc := range e.DataPlaneCertificates {
		if err := dpc.Validate(); err != nil {
			return fmt.Errorf("invalid data plane certificate %d: %w", i, err)
		}
		if dataPlaneCertRefs[dpc.GetRef()] {
			return fmt.Errorf("duplicate data plane certificate ref: %s", dpc.GetRef())
		}
		dataPlaneCertRefs[dpc.GetRef()] = true
	}

	// Validate schema registries
	schemaRegistryRefs := make(map[string]bool)
	for i, sr := range e.SchemaRegistries {
		if err := sr.Validate(); err != nil {
			return fmt.Errorf("invalid schema registry %d: %w", i, err)
		}
		if schemaRegistryRefs[sr.GetRef()] {
			return fmt.Errorf("duplicate schema registry ref: %s", sr.GetRef())
		}
		schemaRegistryRefs[sr.GetRef()] = true
	}

	return nil
}

func (e *EventGatewayControlPlaneResource) SetDefaults() {
	if e.Name == "" {
		e.Name = e.Ref
	}

	for i := range e.BackendClusters {
		e.BackendClusters[i].SetDefaults()
	}

	for i := range e.VirtualClusters {
		e.VirtualClusters[i].SetDefaults()
	}

	for i := range e.Listeners {
		e.Listeners[i].SetDefaults()
	}

	for i := range e.DataPlaneCertificates {
		e.DataPlaneCertificates[i].SetDefaults()
	}

	for i := range e.SchemaRegistries {
		e.SchemaRegistries[i].SetDefaults()
	}
}

func (e EventGatewayControlPlaneResource) GetKonnectMonikerFilter() string {
	return e.BaseResource.GetKonnectMonikerFilter(e.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (e *EventGatewayControlPlaneResource) TryMatchKonnectResource(konnectResource any) bool {
	return e.TryMatchByName(e.Name, konnectResource, matchOptions{sdkType: "EventGatewayInfo"})
}

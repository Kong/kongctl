package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerExternalResourceType(
		ResourceTypeEventGatewayControlPlane,
		func(rs *ResourceSet) *[]EventGatewayControlPlaneResource { return &rs.EventGatewayControlPlanes },
		AutoExplain[EventGatewayControlPlaneResource](
			WithExplainAliases("egw"),
		),
		ExternalResolutionRegistration{AllowAnyStringSelector: true},
		WithNamespace(60, func(r *EventGatewayControlPlaneResource) NamespaceParticipant {
			return NamespaceParticipant{
				Ref:               r.Ref,
				Label:             SchemaFieldEventGateway,
				SupportsProtected: true,
				Meta:              &r.Kongctl,
				External:          r.IsExternal(),
			}
		}),
		WithRootSyncScope(),
		withCollectionValidationOmitted("Event Gateway roots retain validation outside the loader collection pass"),
	)
}

func (e *EventGatewayControlPlaneResource) GetExternalBlock() *ExternalBlock { return e.External }

type EventGatewayControlPlaneResource struct {
	BaseResource
	kkComps.CreateGatewayRequest `yaml:",inline" json:",inline"`

	// Nested child resources
	BackendClusters       []EventGatewayBackendClusterResource       `yaml:"backend_clusters,omitempty"        json:"backend_clusters,omitempty"`        //nolint:lll
	VirtualClusters       []EventGatewayVirtualClusterResource       `yaml:"virtual_clusters,omitempty"        json:"virtual_clusters,omitempty"`        //nolint:lll
	Listeners             []EventGatewayListenerResource             `yaml:"listeners,omitempty"               json:"listeners,omitempty"`               //nolint:lll
	DataPlaneCertificates []EventGatewayDataPlaneCertificateResource `yaml:"data_plane_certificates,omitempty" json:"data_plane_certificates,omitempty"` //nolint:lll
	SchemaRegistries      []EventGatewaySchemaRegistryResource       `yaml:"schema_registries,omitempty"       json:"schema_registries,omitempty"`       //nolint:lll
	StaticKeys            []EventGatewayStaticKeyResource            `yaml:"static_keys,omitempty"             json:"static_keys,omitempty"`             //nolint:lll
	TrustBundles          []EventGatewayTLSTrustBundleResource       `yaml:"tls_trust_bundles,omitempty"       json:"tls_trust_bundles,omitempty"`       //nolint:lll

	// External resource marker
	External *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`
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

	if e.External != nil {
		if err := e.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
	}

	// Validate backend clusters
	if err := validateNestedSlice(e.BackendClusters, "backend cluster"); err != nil {
		return err
	}

	// Validate virtual clusters
	if err := validateNestedSlice(e.VirtualClusters, "virtual cluster"); err != nil {
		return err
	}

	// Validate listeners
	if err := validateNestedSlice(e.Listeners, "listener"); err != nil {
		return err
	}

	// Validate data plane certificates
	if err := validateNestedSlice(e.DataPlaneCertificates, "data plane certificate"); err != nil {
		return err
	}

	// Validate schema registries
	if err := validateNestedSlice(e.SchemaRegistries, "schema registry"); err != nil {
		return err
	}

	// Validate static keys
	if err := validateNestedSlice(e.StaticKeys, "static key"); err != nil {
		return err
	}

	// Validate TLS trust bundles
	if err := validateNestedSlice(e.TrustBundles, "TLS trust bundle"); err != nil {
		return err
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

	for i := range e.StaticKeys {
		e.StaticKeys[i].SetDefaults()
	}

	for i := range e.TrustBundles {
		e.TrustBundles[i].SetDefaults()
	}
}

func (e EventGatewayControlPlaneResource) GetKonnectMonikerFilter() string {
	return e.BaseResource.GetKonnectMonikerFilter(e.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource.
func (e *EventGatewayControlPlaneResource) TryMatchKonnectResource(konnectResource any) bool {
	id, ok := tryMatchByNameWithExternal(
		e.Name,
		konnectResource,
		matchOptions{sdkType: "EventGatewayInfo"},
		e.External,
	)
	if ok {
		e.SetKonnectID(id)
	}
	return ok
}

// IsExternal returns true if this Event Gateway is externally managed.
func (e *EventGatewayControlPlaneResource) IsExternal() bool {
	return e.External != nil && e.External.IsExternal()
}

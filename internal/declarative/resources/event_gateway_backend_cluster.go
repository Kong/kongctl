package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

type EventGatewayBackendClusterResource struct {
	kkComps.CreateBackendClusterRequest `       yaml:",inline"                 json:",inline"`
	Ref                                 string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

func (e EventGatewayBackendClusterResource) GetType() ResourceType {
	return ResourceTypeEventGatewayBackendCluster
}

func (e EventGatewayBackendClusterResource) GetRef() string {
	return e.Ref
}

func (e EventGatewayBackendClusterResource) GetMoniker() string {
	return e.Name
}

func (e EventGatewayBackendClusterResource) GetDependencies() []ResourceRef {
	deps := []ResourceRef{}
	if e.EventGateway != "" {
		// Dependency on parent Event Gateway when defined at root level
		deps = append(deps, ResourceRef{Kind: "event_gateway", Ref: e.EventGateway})
	}
	return deps
}

func (e EventGatewayBackendClusterResource) GetKonnectID() string {
	return e.konnectID
}

func (e EventGatewayBackendClusterResource) Validate() error {
	if err := ValidateRef(e.Ref); err != nil {
		return fmt.Errorf("invalid child ref: %w", err)
	}

	return nil
}

func (e *EventGatewayBackendClusterResource) SetDefaults() {
	// If Name is not set, use ref as default
	if e.Name == "" {
		e.Name = e.Ref
	}
}

func (e EventGatewayBackendClusterResource) GetKonnectMonikerFilter() string {
	return fmt.Sprintf("name[eq]=%s", e.Name) // TODO: the API does not support filtering by name.
}

func (e *EventGatewayBackendClusterResource) TryMatchKonnectResource(konnectResource any) bool {
	if id := tryMatchByField(konnectResource, "Name", e.Name); id != "" {
		e.konnectID = id
		return true
	}
	return false
}

// REQUIRED: Implement ResourceWithParent
func (e EventGatewayBackendClusterResource) GetParentRef() *ResourceRef {
	if e.EventGateway != "" {
		return &ResourceRef{Kind: "event_gateway", Ref: e.EventGateway}
	}
	return nil
}

// MarshalJSON ensures backend cluster metadata (ref, event_gateway) are included.
// Without this, the embedded CreateBackendClusterRequest's MarshalJSON is promoted and drops metadata fields.
func (e EventGatewayBackendClusterResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`

		// Fields from kkComps.CreateBackendClusterRequest
		Name                                     string                                     `json:"name"`
		Description                              *string                                    `json:"description,omitempty"`
		Authentication                           kkComps.BackendClusterAuthenticationScheme `json:"authentication"`
		InsecureAllowAnonymousVirtualClusterAuth *bool                                      `json:"insecure_allow_anonymous_virtual_cluster_auth,omitempty"` //nolint:lll
		BootstrapServers                         []string                                   `json:"bootstrap_servers"`
		TLS                                      kkComps.BackendClusterTLS                  `json:"tls"`
		MetadataUpdateIntervalSeconds            *int64                                     `json:"metadata_update_interval_seconds,omitempty"` //nolint:lll
		Labels                                   map[string]string                          `json:"labels,omitempty"`
	}

	payload := alias{
		Ref:                                      e.Ref,
		EventGateway:                             e.EventGateway,
		Name:                                     e.Name,
		Description:                              e.Description,
		Authentication:                           e.Authentication,
		InsecureAllowAnonymousVirtualClusterAuth: e.InsecureAllowAnonymousVirtualClusterAuth,
		BootstrapServers:                         e.BootstrapServers,
		TLS:                                      e.TLS,
		MetadataUpdateIntervalSeconds:            e.MetadataUpdateIntervalSeconds,
		Labels:                                   e.Labels,
	}

	return json.Marshal(payload)
}

// Custom JSON unmarshaling to reject kongctl metadata
func (e *EventGatewayBackendClusterResource) UnmarshalJSON(data []byte) error {
	// Temporary structure for unmarshaling resource metadata together with
	// the CreateBackendClusterRequest fields from the SDK.
	var temp struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`
		Kongctl      any    `json:"kongctl,omitempty"`

		// Fields from kkComps.CreateBackendClusterRequest
		Name                                     string                                     `json:"name"`
		Description                              *string                                    `json:"description,omitempty"`
		Authentication                           kkComps.BackendClusterAuthenticationScheme `json:"authentication"`
		InsecureAllowAnonymousVirtualClusterAuth *bool                                      `json:"insecure_allow_anonymous_virtual_cluster_auth,omitempty"` //nolint:lll
		BootstrapServers                         []string                                   `json:"bootstrap_servers"`
		TLS                                      kkComps.BackendClusterTLS                  `json:"tls"`
		MetadataUpdateIntervalSeconds            *int64                                     `json:"metadata_update_interval_seconds,omitempty"` //nolint:lll
		Labels                                   map[string]string                          `json:"labels,omitempty"`
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

	// Populate embedded CreateBackendClusterRequest fields
	e.Name = temp.Name
	e.Description = temp.Description
	e.Authentication = temp.Authentication
	e.InsecureAllowAnonymousVirtualClusterAuth = temp.InsecureAllowAnonymousVirtualClusterAuth
	e.BootstrapServers = temp.BootstrapServers
	e.TLS = temp.TLS
	e.MetadataUpdateIntervalSeconds = temp.MetadataUpdateIntervalSeconds
	e.Labels = temp.Labels

	return nil
}

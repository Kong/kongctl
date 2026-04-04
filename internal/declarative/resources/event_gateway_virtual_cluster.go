package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeEventGatewayVirtualCluster,
		func(rs *ResourceSet) *[]EventGatewayVirtualClusterResource { return &rs.EventGatewayVirtualClusters },
		AutoExplain[EventGatewayVirtualClusterResource](),
	)
}

type EventGatewayVirtualClusterResource struct {
	kkComps.CreateVirtualClusterRequest `       yaml:",inline"                 json:",inline"`
	Ref                                 string `yaml:"ref"                     json:"ref"`
	// Parent Event Gateway reference (for root-level definitions)
	EventGateway string `yaml:"event_gateway,omitempty" json:"event_gateway,omitempty"`

	// Nested child resources
	ClusterPolicies []EventGatewayClusterPolicyResource `yaml:"cluster_policies,omitempty" json:"cluster_policies,omitempty"` //nolint:lll
	ProducePolicies []EventGatewayProducePolicyResource `yaml:"produce_policies,omitempty" json:"produce_policies,omitempty"` //nolint:lll
	ConsumePolicies []EventGatewayConsumePolicyResource `yaml:"consume_policies,omitempty"  json:"consume_policies,omitempty"` //nolint:lll

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

	// Validate cluster policies
	clusterPolicyRefs := make(map[string]bool)
	for i, cp := range e.ClusterPolicies {
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("invalid cluster policy %d: %w", i, err)
		}
		if clusterPolicyRefs[cp.GetRef()] {
			return fmt.Errorf("duplicate cluster policy ref: %s", cp.GetRef())
		}
		clusterPolicyRefs[cp.GetRef()] = true
	}

	// Validate produce policies
	producePolicyRefs := make(map[string]bool)
	for i, pp := range e.ProducePolicies {
		if err := pp.Validate(); err != nil {
			return fmt.Errorf("invalid produce policy %d: %w", i, err)
		}
		if producePolicyRefs[pp.GetRef()] {
			return fmt.Errorf("duplicate produce policy ref: %s", pp.GetRef())
		}
		producePolicyRefs[pp.GetRef()] = true
	// Validate consume policies
	consumePolicyRefs := make(map[string]bool)
	for i, cp := range e.ConsumePolicies {
		if err := cp.Validate(); err != nil {
			return fmt.Errorf("invalid consume policy %d: %w", i, err)
		}
		if consumePolicyRefs[cp.GetRef()] {
			return fmt.Errorf("duplicate consume policy ref: %s", cp.GetRef())
		}
		consumePolicyRefs[cp.GetRef()] = true
	}

	return nil
}

func (e *EventGatewayVirtualClusterResource) SetDefaults() {
	// If Name is not set, use ref as default
	if e.Name == "" {
		e.Name = e.Ref
	}

	// Apply defaults to cluster policies
	for i := range e.ClusterPolicies {
		e.ClusterPolicies[i].SetDefaults()
	}

	// Apply defaults to produce policies
	for i := range e.ProducePolicies {
		e.ProducePolicies[i].SetDefaults()
	}
	// Apply defaults to consume policies
	for i := range e.ConsumePolicies {
		e.ConsumePolicies[i].SetDefaults()
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

// MarshalJSON ensures virtual cluster metadata (ref, event_gateway) are included.
// Without this, the embedded CreateVirtualClusterRequest's MarshalJSON is promoted and drops metadata fields.
func (e EventGatewayVirtualClusterResource) MarshalJSON() ([]byte, error) {
	type alias struct {
		Ref          string `json:"ref"`
		EventGateway string `json:"event_gateway,omitempty"`

		// Fields from kkComps.CreateVirtualClusterRequest
		Name           string                                       `json:"name"`
		Description    *string                                      `json:"description,omitempty"`
		Destination    kkComps.BackendClusterReferenceModify        `json:"destination"`
		Authentication []kkComps.VirtualClusterAuthenticationScheme `json:"authentication"`
		Namespace      *kkComps.VirtualClusterNamespace             `json:"namespace,omitempty"`
		ACLMode        kkComps.VirtualClusterACLMode                `json:"acl_mode"`
		DNSLabel       string                                       `json:"dns_label"`
		Labels         map[string]string                            `json:"labels,omitempty"`

		// Nested child resources
		ClusterPolicies []EventGatewayClusterPolicyResource `json:"cluster_policies,omitempty"`
		ProducePolicies []EventGatewayProducePolicyResource `json:"produce_policies,omitempty"`
		ConsumePolicies []EventGatewayConsumePolicyResource `json:"consume_policies,omitempty"`
	}

	payload := alias{
		Ref:             e.Ref,
		EventGateway:    e.EventGateway,
		Name:            e.Name,
		Description:     e.Description,
		Destination:     e.Destination,
		Authentication:  e.Authentication,
		Namespace:       e.Namespace,
		ACLMode:         e.ACLMode,
		DNSLabel:        e.DNSLabel,
		Labels:          e.Labels,
		ClusterPolicies: e.ClusterPolicies,
		ProducePolicies: e.ProducePolicies,
		ConsumePolicies: e.ConsumePolicies,
	}

	return json.Marshal(payload)
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

		// Nested child resources
		ClusterPolicies []EventGatewayClusterPolicyResource `json:"cluster_policies,omitempty"`
		ProducePolicies []EventGatewayProducePolicyResource `json:"produce_policies,omitempty"`
		ConsumePolicies []EventGatewayConsumePolicyResource `json:"consume_policies,omitempty"`
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

	// Populate nested child resources
	e.ClusterPolicies = temp.ClusterPolicies
	e.ProducePolicies = temp.ProducePolicies
	e.ConsumePolicies = temp.ConsumePolicies

	return nil
}

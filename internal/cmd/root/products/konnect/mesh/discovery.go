package mesh

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
)

// discoveryPath is the control plane endpoint that describes every resource
// type it serves. Driving the command surface from it means new Kong Mesh
// policy and resource types, including enterprise ones, need no kongctl
// release.
const discoveryPath = "/_resources"

// Resource scopes reported by the control plane.
const (
	ScopeMesh   = "Mesh"
	ScopeGlobal = "Global"
)

// ResourceDescriptor describes one resource type served by a Kong Mesh control
// plane. Optional fields are inconsistently populated across control plane
// versions and resource types, so read them through the accessors below rather
// than directly.
type ResourceDescriptor struct {
	Name                string            `json:"name"`
	Path                string            `json:"path"`
	Scope               string            `json:"scope"`
	ShortName           string            `json:"shortName"`
	ReadOnly            bool              `json:"readOnly"`
	SingularDisplayName string            `json:"singularDisplayName"`
	PluralDisplayName   string            `json:"pluralDisplayName"`
	IncludeInFederation bool              `json:"includeInFederation"`
	Policy              *PolicyDescriptor `json:"policy,omitempty"`
}

// PolicyDescriptor carries the policy specific metadata the control plane
// reports for resource types that are policies.
type PolicyDescriptor struct {
	IsTargetRef       bool `json:"isTargetRef"`
	HasToTargetRef    bool `json:"hasToTargetRef"`
	HasFromTargetRef  bool `json:"hasFromTargetRef"`
	HasRulesTargetRef bool `json:"hasRulesTargetRef"`
	IsFromAsRules     bool `json:"isFromAsRules"`
}

// discoveryResponse matches the control plane envelope, which wraps the
// descriptors rather than returning a bare array.
type discoveryResponse struct {
	Resources []ResourceDescriptor `json:"resources"`
}

// IsMeshScoped reports whether the resource type lives inside a mesh, and so
// whether requests for it carry a mesh name.
func (d ResourceDescriptor) IsMeshScoped() bool {
	return d.Scope == ScopeMesh
}

// IsPolicy reports whether the control plane classifies this type as a policy.
func (d ResourceDescriptor) IsPolicy() bool {
	return d.Policy != nil
}

// Singular returns a display name for one instance of the resource type,
// falling back to the type name when the control plane leaves it empty.
func (d ResourceDescriptor) Singular() string {
	if name := strings.TrimSpace(d.SingularDisplayName); name != "" {
		return name
	}
	return d.Name
}

// Plural returns a display name for a collection of the resource type, falling
// back to the URL path when the control plane leaves it empty.
func (d ResourceDescriptor) Plural() string {
	if name := strings.TrimSpace(d.PluralDisplayName); name != "" {
		return name
	}
	if path := strings.TrimSpace(d.Path); path != "" {
		return path
	}
	return d.Name
}

// Alias returns the short command alias for the resource type, or an empty
// string when the control plane reports none.
func (d ResourceDescriptor) Alias() string {
	return strings.TrimSpace(d.ShortName)
}

// CollectionPath returns the control plane API path listing every instance of
// the resource type. Mesh scoped types are addressed within a mesh; global
// types sit at the root.
//
// The path is derived entirely from the descriptor, which is what allows one
// implementation to serve every resource type.
func (d ResourceDescriptor) CollectionPath(mesh string) string {
	if !d.IsMeshScoped() {
		return "/" + d.Path
	}
	if mesh == "" {
		mesh = meshcommon.DefaultMesh
	}
	return fmt.Sprintf("/meshes/%s/%s", mesh, d.Path)
}

// ItemPath returns the control plane API path addressing a single named
// instance of the resource type.
func (d ResourceDescriptor) ItemPath(mesh, name string) string {
	return d.CollectionPath(mesh) + "/" + name
}

// Discover fetches the resource types served by the selected control plane.
//
// Results are sorted by the control plane, which returns them alphabetically by
// type name; callers relying on a specific order should sort explicitly.
func Discover(helper cmd.Helper) ([]ResourceDescriptor, error) {
	body, err := fetch(helper, discoveryPath)
	if err != nil {
		return nil, err
	}
	return decodeDiscoveryResponse(body)
}

// decodeDiscoveryResponse parses a control plane discovery payload, dropping
// descriptors that carry no path since nothing can be addressed without one.
func decodeDiscoveryResponse(body []byte) ([]ResourceDescriptor, error) {
	var payload discoveryResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode mesh resource discovery response: %w", err)
	}

	descriptors := make([]ResourceDescriptor, 0, len(payload.Resources))
	for _, descriptor := range payload.Resources {
		if strings.TrimSpace(descriptor.Path) == "" {
			continue
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

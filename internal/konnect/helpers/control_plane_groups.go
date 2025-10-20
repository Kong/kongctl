package helpers

import (
	"context"

	kkCOM "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
)

// ControlPlaneGroupsAPI defines the subset of SDK operations required for managing control plane group memberships.
type ControlPlaneGroupsAPI interface {
	GetControlPlanesIDGroupMemberships(
		ctx context.Context,
		request kkOPS.GetControlPlanesIDGroupMembershipsRequest,
		opts ...kkOPS.Option,
	) (*kkOPS.GetControlPlanesIDGroupMembershipsResponse, error)

	PutControlPlanesIDGroupMemberships(
		ctx context.Context,
		id string,
		groupMembership *kkCOM.GroupMembership,
		opts ...kkOPS.Option,
	) (*kkOPS.PutControlPlanesIDGroupMembershipsResponse, error)
}

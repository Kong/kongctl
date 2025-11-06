package helpers

import (
	"context"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// MeAPI interface for the Me API operations
type MeAPI interface {
	GetUsersMe(ctx context.Context, opts ...kkOps.Option) (*kkOps.GetUsersMeResponse, error)
	GetOrganizationsMe(ctx context.Context, opts ...kkOps.Option) (*kkOps.GetOrganizationsMeResponse, error)
}

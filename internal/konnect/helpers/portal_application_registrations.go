package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalApplicationRegistrationAPI defines the interface for operations on Portal Application Registrations
type PortalApplicationRegistrationAPI interface {
	ListRegistrations(
		ctx context.Context,
		request kkOps.ListRegistrationsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListRegistrationsResponse, error)
	GetApplicationRegistration(
		ctx context.Context,
		request kkOps.GetApplicationRegistrationRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetApplicationRegistrationResponse, error)
	DeleteApplicationRegistration(
		ctx context.Context,
		request kkOps.DeleteApplicationRegistrationRequest,
		opts ...kkOps.Option,
	) (*kkOps.DeleteApplicationRegistrationResponse, error)
}

// PortalApplicationRegistrationAPIImpl provides an implementation of the PortalApplicationRegistrationAPI interface
type PortalApplicationRegistrationAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListRegistrations lists application registrations scoped to a portal
func (p *PortalApplicationRegistrationAPIImpl) ListRegistrations(
	ctx context.Context, request kkOps.ListRegistrationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListRegistrationsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.ApplicationRegistrations.ListRegistrations(ctx, request, opts...)
}

// GetApplicationRegistration retrieves a specific application registration
func (p *PortalApplicationRegistrationAPIImpl) GetApplicationRegistration(
	ctx context.Context,
	request kkOps.GetApplicationRegistrationRequest,
	opts ...kkOps.Option,
) (*kkOps.GetApplicationRegistrationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.ApplicationRegistrations.GetApplicationRegistration(ctx, request, opts...)
}

// DeleteApplicationRegistration removes a specific application registration
func (p *PortalApplicationRegistrationAPIImpl) DeleteApplicationRegistration(
	ctx context.Context,
	request kkOps.DeleteApplicationRegistrationRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteApplicationRegistrationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.ApplicationRegistrations.DeleteApplicationRegistration(ctx, request, opts...)
}

var _ PortalApplicationRegistrationAPI = (*PortalApplicationRegistrationAPIImpl)(nil)

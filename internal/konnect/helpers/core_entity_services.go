package helpers

import (
	"context"

	kkCOM "github.com/Kong/sdk-konnect-go/models/components"
	kkOPS "github.com/Kong/sdk-konnect-go/models/operations"
)

// CoreEntityServicesAPI is the interface for Kong Gateway core entity services (not Service Catalog)
// These are the services that exist within a control plane context
type CoreEntityServicesAPI interface {
	ListService(ctx context.Context, request kkOPS.ListServiceRequest,
		opts ...kkOPS.Option) (*kkOPS.ListServiceResponse, error)
	CreateService(ctx context.Context, controlPlaneID string, service kkCOM.Service,
		opts ...kkOPS.Option) (*kkOPS.CreateServiceResponse, error)
	GetService(ctx context.Context, controlPlaneID string, serviceIDOrName string,
		opts ...kkOPS.Option) (*kkOPS.GetServiceResponse, error)
	UpsertService(ctx context.Context, controlPlaneID string, serviceIDOrName string, service kkCOM.Service,
		opts ...kkOPS.Option) (*kkOPS.UpsertServiceResponse, error)
	DeleteService(ctx context.Context, controlPlaneID string, serviceIDOrName string,
		opts ...kkOPS.Option) (*kkOPS.DeleteServiceResponse, error)
}
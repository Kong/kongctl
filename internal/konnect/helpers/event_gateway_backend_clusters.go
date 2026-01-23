package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type EventGatewayBackendClusterAPI interface {
	// Event Gateway Backend Cluster operations
	ListEventGatewayBackendClusters(ctx context.Context, request kkOps.ListEventGatewayBackendClustersRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayBackendClustersResponse, error)
	FetchEventGatewayBackendCluster(ctx context.Context, gatewayID string, clusterID string,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayBackendClusterResponse, error)
	CreateEventGatewayBackendCluster(ctx context.Context, gatewayID string, request kkComps.CreateBackendClusterRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayBackendClusterResponse, error)
	UpdateEventGatewayBackendCluster(ctx context.Context, gatewayID string, clusterID string, request kkComps.UpdateBackendClusterRequest,
		opts ...kkOps.Option) (*kkOps.UpdateEventGatewayBackendClusterResponse, error)
	DeleteEventGatewayBackendCluster(ctx context.Context, gatewayID string, clusterID string,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayBackendClusterResponse, error)
}

// EventGatewayBackendClusterAPIImpl provides an implementation of the EventGatewayBackendClusterAPI interface.
// It implements all Event Gateway Backend Cluster operations defined by EventGatewayBackendClusterAPI.
type EventGatewayBackendClusterAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayBackendClusterAPIImpl) ListEventGatewayBackendClusters(ctx context.Context, request kkOps.ListEventGatewayBackendClustersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayBackendClustersResponse, error) {
	return a.SDK.EventGatewayBackendClusters.ListEventGatewayBackendClusters(ctx, request, opts...)
}

func (a *EventGatewayBackendClusterAPIImpl) FetchEventGatewayBackendCluster(ctx context.Context, gatewayID string, clusterID string,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayBackendClusterResponse, error) {
	return a.SDK.EventGatewayBackendClusters.GetEventGatewayBackendCluster(ctx, gatewayID, clusterID, opts...)
}

func (a *EventGatewayBackendClusterAPIImpl) CreateEventGatewayBackendCluster(ctx context.Context, gatewayID string, request kkComps.CreateBackendClusterRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayBackendClusterResponse, error) {
	return a.SDK.EventGatewayBackendClusters.CreateEventGatewayBackendCluster(ctx, gatewayID, &request, opts...)
}

func (a *EventGatewayBackendClusterAPIImpl) UpdateEventGatewayBackendCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
	request kkComps.UpdateBackendClusterRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayBackendClusterResponse, error) {
	putRequest := kkOps.UpdateEventGatewayBackendClusterRequest{
		BackendClusterID:            clusterID,
		GatewayID:                   gatewayID,
		UpdateBackendClusterRequest: &request,
	}

	return a.SDK.EventGatewayBackendClusters.UpdateEventGatewayBackendCluster(ctx, putRequest, opts...)
}

func (a *EventGatewayBackendClusterAPIImpl) DeleteEventGatewayBackendCluster(ctx context.Context, gatewayID string, clusterID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayBackendClusterResponse, error) {
	return a.SDK.EventGatewayBackendClusters.DeleteEventGatewayBackendCluster(ctx, gatewayID, clusterID, opts...)
}

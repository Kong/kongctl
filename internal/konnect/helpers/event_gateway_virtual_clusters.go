package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type EventGatewayVirtualClusterAPI interface {
	// Event Gateway Virtual Cluster operations
	ListEventGatewayVirtualClusters(ctx context.Context, request kkOps.ListEventGatewayVirtualClustersRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayVirtualClustersResponse, error)
	FetchEventGatewayVirtualCluster(ctx context.Context, gatewayID string, clusterID string,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayVirtualClusterResponse, error)
	CreateEventGatewayVirtualCluster(ctx context.Context, gatewayID string, request kkComps.CreateVirtualClusterRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayVirtualClusterResponse, error)
	UpdateEventGatewayVirtualCluster(
		ctx context.Context,
		gatewayID string,
		clusterID string,
		request kkComps.UpdateVirtualClusterRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateEventGatewayVirtualClusterResponse, error)
	DeleteEventGatewayVirtualCluster(ctx context.Context, gatewayID string, clusterID string,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayVirtualClusterResponse, error)
}

// EventGatewayVirtualClusterAPIImpl provides an implementation of the EventGatewayVirtualClusterAPI interface.
// It implements all Event Gateway Virtual Cluster operations defined by EventGatewayVirtualClusterAPI.
type EventGatewayVirtualClusterAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayVirtualClusterAPIImpl) ListEventGatewayVirtualClusters(
	ctx context.Context,
	request kkOps.ListEventGatewayVirtualClustersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClustersResponse, error) {
	return a.SDK.EventGatewayVirtualClusters.ListEventGatewayVirtualClusters(ctx, request, opts...)
}

func (a *EventGatewayVirtualClusterAPIImpl) FetchEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterResponse, error) {
	return a.SDK.EventGatewayVirtualClusters.GetEventGatewayVirtualCluster(ctx, gatewayID, clusterID, opts...)
}

func (a *EventGatewayVirtualClusterAPIImpl) CreateEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateVirtualClusterRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterResponse, error) {
	return a.SDK.EventGatewayVirtualClusters.CreateEventGatewayVirtualCluster(ctx, gatewayID, &request, opts...)
}

func (a *EventGatewayVirtualClusterAPIImpl) UpdateEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
	request kkComps.UpdateVirtualClusterRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayVirtualClusterResponse, error) {
	putRequest := kkOps.UpdateEventGatewayVirtualClusterRequest{
		VirtualClusterID:            clusterID,
		GatewayID:                   gatewayID,
		UpdateVirtualClusterRequest: &request,
	}

	return a.SDK.EventGatewayVirtualClusters.UpdateEventGatewayVirtualCluster(ctx, putRequest, opts...)
}

func (a *EventGatewayVirtualClusterAPIImpl) DeleteEventGatewayVirtualCluster(
	ctx context.Context,
	gatewayID string,
	clusterID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayVirtualClusterResponse, error) {
	return a.SDK.EventGatewayVirtualClusters.DeleteEventGatewayVirtualCluster(ctx, gatewayID, clusterID, opts...)
}

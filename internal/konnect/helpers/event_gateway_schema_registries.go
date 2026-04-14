package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// EventGatewaySchemaRegistryAPI defines operations for Event Gateway Schema Registries.
type EventGatewaySchemaRegistryAPI interface {
	ListEventGatewaySchemaRegistries(
		ctx context.Context,
		request kkOps.ListEventGatewaySchemaRegistriesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListEventGatewaySchemaRegistriesResponse, error)

	GetEventGatewaySchemaRegistry(
		ctx context.Context,
		gatewayID string,
		schemaRegistryID string,
		opts ...kkOps.Option,
	) (*kkOps.GetEventGatewaySchemaRegistryResponse, error)

	CreateEventGatewaySchemaRegistry(
		ctx context.Context,
		gatewayID string,
		request kkComps.SchemaRegistryCreate,
		opts ...kkOps.Option,
	) (*kkOps.CreateEventGatewaySchemaRegistryResponse, error)

	UpdateEventGatewaySchemaRegistry(
		ctx context.Context,
		gatewayID string,
		schemaRegistryID string,
		request kkComps.SchemaRegistryUpdate,
		opts ...kkOps.Option,
	) (*kkOps.UpdateEventGatewaySchemaRegistryResponse, error)

	DeleteEventGatewaySchemaRegistry(
		ctx context.Context,
		gatewayID string,
		schemaRegistryID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteEventGatewaySchemaRegistryResponse, error)
}

// EventGatewaySchemaRegistryAPIImpl implements EventGatewaySchemaRegistryAPI using the Konnect SDK.
type EventGatewaySchemaRegistryAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewaySchemaRegistryAPIImpl) ListEventGatewaySchemaRegistries(
	ctx context.Context,
	request kkOps.ListEventGatewaySchemaRegistriesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewaySchemaRegistriesResponse, error) {
	return a.SDK.EventGatewaySchemaRegistries.ListEventGatewaySchemaRegistries(ctx, request, opts...)
}

func (a *EventGatewaySchemaRegistryAPIImpl) GetEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	schemaRegistryID string,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewaySchemaRegistryResponse, error) {
	return a.SDK.EventGatewaySchemaRegistries.GetEventGatewaySchemaRegistry(
		ctx, gatewayID, schemaRegistryID, opts...)
}

func (a *EventGatewaySchemaRegistryAPIImpl) CreateEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	request kkComps.SchemaRegistryCreate,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewaySchemaRegistryResponse, error) {
	return a.SDK.EventGatewaySchemaRegistries.CreateEventGatewaySchemaRegistry(
		ctx, gatewayID, &request, opts...)
}

func (a *EventGatewaySchemaRegistryAPIImpl) UpdateEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	schemaRegistryID string,
	request kkComps.SchemaRegistryUpdate,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewaySchemaRegistryResponse, error) {
	req := kkOps.UpdateEventGatewaySchemaRegistryRequest{
		GatewayID:            gatewayID,
		SchemaRegistryID:     schemaRegistryID,
		SchemaRegistryUpdate: &request,
	}
	return a.SDK.EventGatewaySchemaRegistries.UpdateEventGatewaySchemaRegistry(ctx, req, opts...)
}

func (a *EventGatewaySchemaRegistryAPIImpl) DeleteEventGatewaySchemaRegistry(
	ctx context.Context,
	gatewayID string,
	schemaRegistryID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewaySchemaRegistryResponse, error) {
	return a.SDK.EventGatewaySchemaRegistries.DeleteEventGatewaySchemaRegistry(
		ctx, gatewayID, schemaRegistryID, opts...)
}

package helpers

import (
	"context"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

func GetAllGatewayPlugins(ctx context.Context, requestPageSize int64, cpID string, kkClient *kk.SDK,
) ([]kkComps.Plugin, error) {
	var allData []kkComps.Plugin

	offset := ""
	for {
		req := kkOps.ListPluginRequest{
			Size:           new(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         new(offset),
		}

		res, err := kkClient.Plugins.ListPlugin(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		nextOffset, ok := nextOffsetToken(res.Object.Offset)
		if !ok {
			break
		}
		offset = nextOffset
	}

	return allData, nil
}

func GetAllGatewayServicePlugins(
	ctx context.Context,
	requestPageSize int64,
	cpID string,
	serviceID string,
	kkClient *kk.SDK,
) ([]kkComps.Plugin, error) {
	var allData []kkComps.Plugin

	offset := ""
	for {
		req := kkOps.ListPluginWithServiceRequest{
			ControlPlaneID: cpID,
			ServiceID:      serviceID,
			Size:           new(requestPageSize),
		}
		if offset != "" {
			req.Offset = new(offset)
		}

		res, err := kkClient.Plugins.ListPluginWithService(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		nextOffset, ok := nextOffsetToken(res.Object.Offset)
		if !ok {
			break
		}
		offset = nextOffset
	}

	return allData, nil
}

func GetAllGatewayRoutePlugins(
	ctx context.Context,
	requestPageSize int64,
	cpID string,
	routeID string,
	kkClient *kk.SDK,
) ([]kkComps.Plugin, error) {
	var allData []kkComps.Plugin

	offset := ""
	for {
		req := kkOps.ListPluginWithRouteRequest{
			ControlPlaneID: cpID,
			RouteID:        routeID,
			Size:           new(requestPageSize),
		}
		if offset != "" {
			req.Offset = new(offset)
		}

		res, err := kkClient.Plugins.ListPluginWithRoute(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		nextOffset, ok := nextOffsetToken(res.Object.Offset)
		if !ok {
			break
		}
		offset = nextOffset
	}

	return allData, nil
}

func GetAllGatewayConsumerPlugins(
	ctx context.Context,
	requestPageSize int64,
	cpID string,
	consumerID string,
	kkClient *kk.SDK,
) ([]kkComps.Plugin, error) {
	var allData []kkComps.Plugin

	offset := ""
	for {
		req := kkOps.ListPluginWithConsumerRequest{
			ControlPlaneID:              cpID,
			ConsumerIDForNestedEntities: consumerID,
			Size:                        new(requestPageSize),
		}
		if offset != "" {
			req.Offset = new(offset)
		}

		res, err := kkClient.Plugins.ListPluginWithConsumer(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		nextOffset, ok := nextOffsetToken(res.Object.Offset)
		if !ok {
			break
		}
		offset = nextOffset
	}

	return allData, nil
}

func GetAllGatewayConsumerGroupPlugins(
	ctx context.Context,
	requestPageSize int64,
	cpID string,
	groupID string,
	kkClient *kk.SDK,
) ([]kkComps.Plugin, error) {
	var allData []kkComps.Plugin

	offset := ""
	for {
		req := kkOps.ListPluginWithConsumerGroupRequest{
			ControlPlaneID:  cpID,
			ConsumerGroupID: groupID,
			Size:            new(requestPageSize),
		}
		if offset != "" {
			req.Offset = new(offset)
		}

		res, err := kkClient.Plugins.ListPluginWithConsumerGroup(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		nextOffset, ok := nextOffsetToken(res.Object.Offset)
		if !ok {
			break
		}
		offset = nextOffset
	}

	return allData, nil
}

package helpers

import (
	"context"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

func GetAllGatewayConsumerGroups(ctx context.Context, requestPageSize int64, cpID string, kkClient *kk.SDK,
) ([]kkComps.ConsumerGroup, error) {
	var allData []kkComps.ConsumerGroup

	offset := ""
	for {
		req := kkOps.ListConsumerGroupRequest{
			Size:           kk.Int64(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         kk.String(offset),
		}

		res, err := kkClient.ConsumerGroups.ListConsumerGroup(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		if res.Object.Offset != nil {
			offset = *res.Object.Offset
		} else {
			break
		}
	}

	return allData, nil
}

func GetAllGatewayConsumerGroupConsumers(
	ctx context.Context,
	requestPageSize int64,
	cpID string,
	groupID string,
	kkClient *kk.SDK,
) ([]kkComps.Consumer, error) {
	var allData []kkComps.Consumer

	offset := ""
	for {
		req := kkOps.ListConsumersForConsumerGroupRequest{
			ControlPlaneID:  cpID,
			ConsumerGroupID: groupID,
			Size:            kk.Int64(requestPageSize),
		}
		if offset != "" {
			req.Offset = kk.String(offset)
		}

		res, err := kkClient.ConsumerGroups.ListConsumersForConsumerGroup(ctx, req)
		if err != nil {
			return nil, err
		}

		if res.Object == nil {
			break
		}

		allData = append(allData, res.Object.Data...)

		if res.Object.Offset != nil {
			offset = *res.Object.Offset
		} else {
			break
		}
	}

	return allData, nil
}

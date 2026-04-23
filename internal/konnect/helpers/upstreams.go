package helpers

import (
	"context"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

func GetAllGatewayUpstreams(ctx context.Context, requestPageSize int64, cpID string, kkClient *kk.SDK,
) ([]kkComps.Upstream, error) {
	var allData []kkComps.Upstream

	offset := ""
	for {
		req := kkOps.ListUpstreamRequest{
			Size:           new(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         new(offset),
		}

		res, err := kkClient.Upstreams.ListUpstream(ctx, req)
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

func GetAllGatewayTargetsForUpstream(
	ctx context.Context,
	requestPageSize int64,
	cpID string,
	upstreamID string,
	kkClient *kk.SDK,
) ([]kkComps.Target, error) {
	var allData []kkComps.Target

	offset := ""
	for {
		req := kkOps.ListTargetWithUpstreamRequest{
			ControlPlaneID:      cpID,
			UpstreamIDForTarget: upstreamID,
			Size:                new(requestPageSize),
		}
		if offset != "" {
			req.Offset = new(offset)
		}

		res, err := kkClient.Targets.ListTargetWithUpstream(ctx, req)
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

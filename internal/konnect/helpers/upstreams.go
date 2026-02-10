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
			Size:           kk.Int64(requestPageSize),
			ControlPlaneID: cpID,
			Offset:         kk.String(offset),
		}

		res, err := kkClient.Upstreams.ListUpstream(ctx, req)
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
			Size:                kk.Int64(requestPageSize),
		}
		if offset != "" {
			req.Offset = kk.String(offset)
		}

		res, err := kkClient.Targets.ListTargetWithUpstream(ctx, req)
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

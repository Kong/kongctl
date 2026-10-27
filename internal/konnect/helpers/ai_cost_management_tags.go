package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AICostManagementTagsAPI manages organization-wide capture in the selected region.
type AICostManagementTagsAPI interface {
	GetCostManagementTags(
		context.Context,
		...kkOps.Option,
	) (*kkOps.GetCostManagementTagsResponse, error)
	UpdateCostManagementTags(
		context.Context,
		kkComps.UpdateCostManagementTags,
		...kkOps.Option,
	) (*kkOps.UpdateCostManagementTagsResponse, error)
}

type AICostManagementTagsAPIImpl struct{ SDK *kkSDK.SDK }

func (a *AICostManagementTagsAPIImpl) GetCostManagementTags(
	ctx context.Context,
	opts ...kkOps.Option,
) (*kkOps.GetCostManagementTagsResponse, error) {
	return a.SDK.AIGatewayCostManagementTags.GetCostManagementTags(ctx, opts...)
}

func (a *AICostManagementTagsAPIImpl) UpdateCostManagementTags(
	ctx context.Context,
	req kkComps.UpdateCostManagementTags,
	opts ...kkOps.Option,
) (*kkOps.UpdateCostManagementTagsResponse, error) {
	return a.SDK.AIGatewayCostManagementTags.UpdateCostManagementTags(ctx, req, opts...)
}

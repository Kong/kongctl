package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
)

type aiCostManagementTagsContract struct{}

func (aiCostManagementTagsContract) ResourceType() string {
	return planner.ResourceTypeAICostManagementTags
}

func (aiCostManagementTagsContract) ValidatePayload(
	_ context.Context,
	change planner.PlannedChange,
) error {
	if change.Action != planner.ActionUpdate {
		return fmt.Errorf("AI cost management tags support update only")
	}
	var req kkComps.UpdateCostManagementTags
	if err := mapAIGatewaySDKRequest("AI cost management tags", change.Fields, &req); err != nil {
		return err
	}
	r := resources.AICostManagementTagsResource{
		BaseResource: resources.BaseResource{Ref: change.ResourceRef},
		Headers:      req.Headers,
		Prefixes:     req.Prefixes,
	}
	if err := r.Validate(); err != nil {
		return err
	}
	return validateMappedPayload(
		planner.ResourceTypeAICostManagementTags,
		change.Action,
		change.Fields,
		req,
	)
}

func (e *Executor) registerAICostManagementTagsExecutor() {
	e.registerResourceExecutor(
		resourceExecutor{
			contract: aiCostManagementTagsContract{},
			update: func(ctx context.Context, change *planner.PlannedChange) (string, error) {
				if e.dryRun {
					return change.ResourceRef, nil
				}
				var req kkComps.UpdateCostManagementTags
				if err := mapAIGatewaySDKRequest("AI cost management tags", change.Fields, &req); err != nil {
					return "", err
				}
				if err := e.client.UpdateAICostManagementTags(ctx, req); err != nil {
					return "", err
				}
				return change.ResourceRef, nil
			},
		},
	)
}

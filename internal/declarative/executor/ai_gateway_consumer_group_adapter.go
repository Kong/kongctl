package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayConsumerGroupAdapter implements ResourceOperations for AI Gateway Consumer Groups.
type AIGatewayConsumerGroupAdapter struct {
	client *state.Client
}

// NewAIGatewayConsumerGroupAdapter creates a new AI Gateway Consumer Group adapter.
func NewAIGatewayConsumerGroupAdapter(client *state.Client) *AIGatewayConsumerGroupAdapter {
	return &AIGatewayConsumerGroupAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayConsumerGroupRequest.
func (a *AIGatewayConsumerGroupAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayConsumerGroupRequest,
) error {
	fields = cloneFieldsWithoutAIGatewayConsumerGroupMembership(fields)
	if err := mapAIGatewaySDKRequest("AI Gateway Consumer Group create", fields, create); err != nil {
		return err
	}
	if create.Name == "" || create.DisplayName == "" {
		return fmt.Errorf("name and display_name are required")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayConsumerGroupRequest.
func (a *AIGatewayConsumerGroupAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayConsumerGroupRequest,
	_ map[string]string,
) error {
	fields = cloneFieldsWithoutAIGatewayConsumerGroupMembership(fields)
	return mapAIGatewaySDKRequest("AI Gateway Consumer Group update", fields, update)
}

// Create creates an AI Gateway ConsumerGroup.
func (a *AIGatewayConsumerGroupAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayConsumerGroupRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	payload, err := ownedChildPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateChildWriteContext("AI Gateway consumer group", execCtx, payload, namespace); err != nil {
		return "", err
	}
	consumerGroups, err := a.client.ListAIGatewayConsumerGroups(ctx, gatewayID)
	if err != nil {
		return "", err
	}
	for _, consumerGroup := range consumerGroups {
		if consumerGroup.Name == payload[planner.FieldName] {
			return "", childCreationCollision("AI Gateway consumer group", consumerGroup.Name)
		}
	}
	return a.client.CreateAIGatewayConsumerGroup(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway ConsumerGroup.
func (a *AIGatewayConsumerGroupAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayConsumerGroupRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}
	payload, err := ownedChildPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateChildWriteContext("AI Gateway consumer group", execCtx, payload, namespace); err != nil {
		return "", err
	}
	current, err := a.client.GetAIGatewayConsumerGroup(ctx, gatewayID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("AI Gateway consumer group no longer exists")
	}
	currentPayload, err := resources.AIGatewayConsumerGroupMutablePayloadMap(
		current.AIGatewayConsumerGroup,
	)
	if err != nil {
		return "", err
	}
	currentPayload = cloneFieldsWithoutAIGatewayConsumerGroupMembership(currentPayload)
	payload, err = preserveOwnedMappedChildMetadataUpdate[kkComps.UpdateAIGatewayConsumerGroupRequest](
		"AI Gateway consumer group",
		currentPayload,
		payload,
		execCtx,
		planner.FieldConsumers,
	)
	if err != nil {
		return "", err
	}
	payload, err = prepareOwnedMappedChildUpdate(
		"AI Gateway consumer group",
		currentPayload,
		payload,
		execCtx,
	)
	if err != nil {
		return "", err
	}
	if err := mapAIGatewaySDKRequest("AI Gateway consumer group update", payload, &req); err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayConsumerGroup(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway ConsumerGroup.
func (a *AIGatewayConsumerGroupAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetAIGatewayConsumerGroup(ctx, gatewayID, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if err := resources.ValidateChildOwnership(
		"AI Gateway consumer group", current.ManagedBy, execCtx.Namespace,
	); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("AI Gateway consumer group is protected and cannot be deleted")
	}
	return a.client.DeleteAIGatewayConsumerGroup(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Consumer Group by ID.
func (a *AIGatewayConsumerGroupAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getAIGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}
	group, err := a.client.GetAIGatewayConsumerGroup(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, nil
	}
	return &aiGatewayConsumerGroupResourceInfo{group: group}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayConsumerGroupAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Consumer Groups")
}

// ResourceType returns the resource type.
func (a *AIGatewayConsumerGroupAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayConsumerGroup
}

// RequiredFields returns required fields for create.
func (a *AIGatewayConsumerGroupAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayConsumerGroupAdapter) SupportsUpdate() bool {
	return true
}

func cloneFieldsWithoutAIGatewayConsumerGroupMembership(fields map[string]any) map[string]any {
	clone := make(map[string]any, len(fields))
	for key, value := range fields {
		if key == planner.FieldConsumers {
			continue
		}
		clone[key] = value
	}
	return clone
}

func (a *AIGatewayConsumerGroupAdapter) getAIGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	if gatewayID := aiGatewayIDFromChange(execCtx.PlannedChange); gatewayID != "" {
		return gatewayID, nil
	}

	return "", fmt.Errorf("AI Gateway ID required for Consumer Group operations")
}

type aiGatewayConsumerGroupResourceInfo struct {
	group *state.AIGatewayConsumerGroup
}

func (a *aiGatewayConsumerGroupResourceInfo) GetID() string {
	return resources.AIGatewayConsumerGroupID(a.group.AIGatewayConsumerGroup)
}

func (a *aiGatewayConsumerGroupResourceInfo) GetName() string {
	return resources.AIGatewayConsumerGroupName(a.group.AIGatewayConsumerGroup)
}

func (a *aiGatewayConsumerGroupResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayConsumerGroupLabels(a.group.AIGatewayConsumerGroup)
}

func (a *aiGatewayConsumerGroupResourceInfo) GetNormalizedLabels() map[string]string {
	return a.group.NormalizedLabels
}

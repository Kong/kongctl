package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayPolicyAdapter implements ResourceOperations for AI Gateway Policies.
type AIGatewayPolicyAdapter struct {
	client *state.Client
}

// NewAIGatewayPolicyAdapter creates a new AI Gateway Policy adapter.
func NewAIGatewayPolicyAdapter(client *state.Client) *AIGatewayPolicyAdapter {
	return &AIGatewayPolicyAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayPolicyRequest.
func (a *AIGatewayPolicyAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayPolicyRequest,
) error {
	if err := mapAIGatewaySDKRequest("AI Gateway Policy create", fields, create); err != nil {
		return err
	}
	if create.Name == "" || create.Type == "" || create.DisplayName == "" || create.Config == nil {
		return fmt.Errorf("name, type, display_name, and config are required")
	}
	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayPolicyRequest.
func (a *AIGatewayPolicyAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayPolicyRequest,
	_ map[string]string,
) error {
	return mapAIGatewaySDKRequest("AI Gateway Policy update", fields, update)
}

// Create creates an AI Gateway Policy.
func (a *AIGatewayPolicyAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayPolicyRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "Policy", true)
	if err != nil {
		return "", err
	}
	payload, err := ownedChildPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateChildWriteContext("AI Gateway policy", execCtx, payload, namespace); err != nil {
		return "", err
	}
	policies, err := a.client.ListAIGatewayPolicies(ctx, gatewayID)
	if err != nil {
		return "", err
	}
	for _, policy := range policies {
		if policy.Name == payload[planner.FieldName] {
			return "", childCreationCollision("AI Gateway policy", policy.Name)
		}
	}
	return a.client.CreateAIGatewayPolicy(ctx, gatewayID, req, namespace)
}

// Update updates an AI Gateway Policy.
func (a *AIGatewayPolicyAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayPolicyRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "Policy", true)
	if err != nil {
		return "", err
	}
	payload, err := ownedChildPayload(req)
	if err != nil {
		return "", err
	}
	if err := validateChildWriteContext("AI Gateway policy", execCtx, payload, namespace); err != nil {
		return "", err
	}
	current, err := a.client.GetAIGatewayPolicy(ctx, gatewayID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("AI Gateway policy no longer exists")
	}
	currentPayload, err := resources.AIGatewayPolicyMutablePayloadMap(current.AIGatewayPolicy)
	if err != nil {
		return "", err
	}
	payload, err = preservePolicyMetadataUpdatePayload(currentPayload, payload, execCtx)
	if err != nil {
		return "", err
	}
	payload, err = prepareOwnedMappedChildUpdate("AI Gateway policy", currentPayload, payload, execCtx)
	if err != nil {
		return "", err
	}
	if err := mapAIGatewaySDKRequest("AI Gateway policy update", payload, &req); err != nil {
		return "", err
	}
	return a.client.UpdateAIGatewayPolicy(ctx, gatewayID, id, req, namespace)
}

// Delete deletes an AI Gateway Policy.
func (a *AIGatewayPolicyAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "Policy", true)
	if err != nil {
		return err
	}
	current, err := a.client.GetAIGatewayPolicy(ctx, gatewayID, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if err := resources.ValidateChildOwnership(
		"AI Gateway policy", current.ManagedBy, execCtx.Namespace,
	); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("AI Gateway policy is protected and cannot be deleted")
	}
	return a.client.DeleteAIGatewayPolicy(ctx, gatewayID, id)
}

// GetByID gets an AI Gateway Policy by ID.
func (a *AIGatewayPolicyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := aiGatewayIDFromExecutionContext(execCtx, "Policy", true)
	if err != nil {
		return nil, err
	}
	policy, err := a.client.GetAIGatewayPolicy(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}
	return &aiGatewayPolicyResourceInfo{policy: policy}, nil
}

// GetByName is not supported without a parent gateway context.
func (a *AIGatewayPolicyAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for AI Gateway Policies")
}

// ResourceType returns the resource type.
func (a *AIGatewayPolicyAdapter) ResourceType() string {
	return planner.ResourceTypeAIGatewayPolicy
}

// RequiredFields returns required fields for create.
func (a *AIGatewayPolicyAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldType, planner.FieldDisplayName, planner.FieldConfig}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayPolicyAdapter) SupportsUpdate() bool {
	return true
}

type aiGatewayPolicyResourceInfo struct {
	policy *state.AIGatewayPolicy
}

func (a *aiGatewayPolicyResourceInfo) GetID() string {
	return resources.AIGatewayPolicyID(a.policy.AIGatewayPolicy)
}

func (a *aiGatewayPolicyResourceInfo) GetName() string {
	return resources.AIGatewayPolicyName(a.policy.AIGatewayPolicy)
}

func (a *aiGatewayPolicyResourceInfo) GetLabels() map[string]string {
	return resources.AIGatewayPolicyLabels(a.policy.AIGatewayPolicy)
}

func (a *aiGatewayPolicyResourceInfo) GetNormalizedLabels() map[string]string {
	return a.policy.NormalizedLabels
}

// A metadata-only write preserves config and other public fields from the fresh
// read. Policy config is arbitrary JSON and can contain readable credentials;
// keep that observed config in memory rather than copying it into saved plans.
func preservePolicyMetadataUpdatePayload(current, requested map[string]any,
	execCtx *ExecutionContext,
) (map[string]any, error) {
	return preserveOwnedMappedChildMetadataUpdate[kkComps.UpdateAIGatewayPolicyRequest](
		"AI Gateway policy",
		current,
		requested,
		execCtx,
	)
}

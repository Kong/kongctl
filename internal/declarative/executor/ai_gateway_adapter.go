package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// AIGatewayAdapter implements ResourceOperations for AI Gateways.
type AIGatewayAdapter struct {
	client *state.Client
}

// NewAIGatewayAdapter creates a new AI Gateway adapter.
func NewAIGatewayAdapter(client *state.Client) *AIGatewayAdapter {
	return &AIGatewayAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateAIGatewayRequest.
func (a *AIGatewayAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateAIGatewayRequest,
) error {
	create.DisplayName, _ = fields[planner.FieldDisplayName].(string)
	create.Name, _ = fields[planner.FieldName].(string)
	if create.Name == "" {
		return fmt.Errorf("AI Gateway name is required")
	}
	common.MapOptionalStringFieldToPtr(&create.Description, fields, planner.FieldDescription)

	if value, ok := fields[planner.FieldProxyURLs]; ok {
		proxyURLs, err := aiGatewayProxyURLsFromField(value)
		if err != nil {
			return err
		}
		create.ProxyUrls = proxyURLs
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

// MapUpdateFields maps planner fields to UpdateAIGatewayRequest.
func (a *AIGatewayAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateAIGatewayRequest,
	currentLabels map[string]string,
) error {
	update.DisplayName, _ = fields[planner.FieldDisplayName].(string)
	update.Name, _ = fields[planner.FieldName].(string)
	if update.Name == "" {
		return fmt.Errorf("AI Gateway name is required")
	}

	if value, ok := fields[planner.FieldDescription]; ok {
		if desc, ok := value.(string); ok {
			update.Description = &desc
		}
	}

	if value, ok := fields[planner.FieldProxyURLs]; ok {
		proxyURLs, err := aiGatewayProxyURLsFromField(value)
		if err != nil {
			return err
		}
		update.ProxyUrls = proxyURLs
	}

	desiredLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	if desiredLabels != nil {
		plannerCurrentLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels])
		if plannerCurrentLabels != nil {
			currentLabels = plannerCurrentLabels
		}
		update.Labels = labels.BuildUpdateStringLabels(desiredLabels, currentLabels, execCtx.Namespace, execCtx.Protection)
	} else if currentLabels != nil {
		update.Labels = labels.BuildUpdateStringLabels(currentLabels, currentLabels, execCtx.Namespace, execCtx.Protection)
	}

	return nil
}

// Create creates an AI Gateway.
func (a *AIGatewayAdapter) Create(
	ctx context.Context,
	req kkComps.CreateAIGatewayRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateAIGateway(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update updates an AI Gateway.
func (a *AIGatewayAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateAIGatewayRequest,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateAIGateway(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete deletes an AI Gateway.
func (a *AIGatewayAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteAIGateway(ctx, id)
}

// GetByName fetches an AI Gateway by display name.
func (a *AIGatewayAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	gateway, err := a.client.GetAIGatewayByDisplayName(ctx, name)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, nil
	}
	return &aiGatewayResourceInfo{gateway: gateway}, nil
}

// GetByID fetches an AI Gateway by ID.
func (a *AIGatewayAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	gateway, err := a.client.GetAIGatewayByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, nil
	}
	return &aiGatewayResourceInfo{gateway: gateway}, nil
}

// ResourceType returns the resource type.
func (a *AIGatewayAdapter) ResourceType() string {
	return planner.ResourceTypeAIGateway
}

// RequiredFields returns required fields for create.
func (a *AIGatewayAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldDisplayName}
}

// SupportsUpdate indicates update support.
func (a *AIGatewayAdapter) SupportsUpdate() bool {
	return true
}

func aiGatewayProxyURLsFromField(value any) ([]kkComps.AIGatewayProxyURL, error) {
	if proxyURLs, ok := value.([]kkComps.AIGatewayProxyURL); ok {
		return proxyURLs, nil
	}
	if value == nil {
		return nil, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode AI Gateway proxy_urls: %w", err)
	}

	var proxyURLs []kkComps.AIGatewayProxyURL
	if err := json.Unmarshal(data, &proxyURLs); err != nil {
		return nil, fmt.Errorf("failed to decode AI Gateway proxy_urls: %w", err)
	}
	return proxyURLs, nil
}

type aiGatewayResourceInfo struct {
	gateway *state.AIGateway
}

func (a *aiGatewayResourceInfo) GetID() string {
	return a.gateway.ID
}

func (a *aiGatewayResourceInfo) GetName() string {
	return a.gateway.DisplayName
}

func (a *aiGatewayResourceInfo) GetLabels() map[string]string {
	return a.gateway.Labels
}

func (a *aiGatewayResourceInfo) GetNormalizedLabels() map[string]string {
	return a.gateway.NormalizedLabels
}

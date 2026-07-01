package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayPolicies lists all policies for an AI Gateway.
func (c *Client) ListAIGatewayPolicies(ctx context.Context, gatewayID string) ([]AIGatewayPolicy, error) {
	if err := ValidateAPIClient(c.aiGatewayPoliciesAPI, "AI Gateway Policies API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayPolicy
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayPoliciesRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayPoliciesAPI.ListAiGatewayPolicies(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Policies", nil)
		}

		if resp == nil || resp.ListAIGatewayPoliciesResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayPoliciesResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayPoliciesResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	policies := make([]AIGatewayPolicy, 0, len(allData))
	for _, policy := range allData {
		policies = append(policies, AIGatewayPolicy{
			AIGatewayPolicy:  policy,
			NormalizedLabels: normalizedAIGatewayPolicyLabels(policy),
		})
	}
	return policies, nil
}

// GetAIGatewayPolicy fetches an AI Gateway Policy by ID or name.
func (c *Client) GetAIGatewayPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
) (*AIGatewayPolicy, error) {
	if err := ValidateAPIClient(c.aiGatewayPoliciesAPI, "AI Gateway Policies API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayPoliciesAPI.GetAiGatewayPolicy(ctx, gatewayID, policyID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Policy by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayPolicy),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayPolicy == nil {
		return nil, nil
	}

	return &AIGatewayPolicy{
		AIGatewayPolicy:  *resp.AIGatewayPolicy,
		NormalizedLabels: normalizedAIGatewayPolicyLabels(*resp.AIGatewayPolicy),
	}, nil
}

// GetAIGatewayPolicyByName finds an AI Gateway Policy by name within a gateway.
func (c *Client) GetAIGatewayPolicyByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayPolicy, error) {
	policies, err := c.ListAIGatewayPolicies(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Policies to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayPolicy),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range policies {
		if resources.AIGatewayPolicyName(policies[i].AIGatewayPolicy) == name {
			return &policies[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayPolicy creates a new policy under an AI Gateway.
func (c *Client) CreateAIGatewayPolicy(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayPolicyRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayPoliciesAPI, "AI Gateway Policies API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayPoliciesAPI.CreateAiGatewayPolicy(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Policy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayPolicy),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayPolicy == nil {
		return "", fmt.Errorf("create AI Gateway Policy response missing data")
	}

	return resources.AIGatewayPolicyID(*resp.AIGatewayPolicy), nil
}

// UpdateAIGatewayPolicy updates an existing policy under an AI Gateway.
func (c *Client) UpdateAIGatewayPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
	req kkComps.UpdateAIGatewayPolicyRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayPoliciesAPI, "AI Gateway Policies API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayPoliciesAPI.UpdateAiGatewayPolicy(ctx, kkOps.UpdateAiGatewayPolicyRequest{
		GatewayID:                    gatewayID,
		PolicyIDOrName:               policyID,
		UpdateAIGatewayPolicyRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Policy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayPolicy),
			ResourceName: req.Name,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayPolicy == nil {
		return "", fmt.Errorf("update AI Gateway Policy response missing data")
	}

	return resources.AIGatewayPolicyID(*resp.AIGatewayPolicy), nil
}

// DeleteAIGatewayPolicy deletes an AI Gateway Policy by ID.
func (c *Client) DeleteAIGatewayPolicy(ctx context.Context, gatewayID string, policyID string) error {
	if err := ValidateAPIClient(c.aiGatewayPoliciesAPI, "AI Gateway Policies API"); err != nil {
		return err
	}

	_, err := c.aiGatewayPoliciesAPI.DeleteAiGatewayPolicy(ctx, gatewayID, policyID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Policy", nil)
	}

	return nil
}

func normalizedAIGatewayPolicyLabels(policy kkComps.AIGatewayPolicy) map[string]string {
	normalized := resources.AIGatewayPolicyLabels(policy)
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}

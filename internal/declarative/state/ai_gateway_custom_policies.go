package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayCustomPolicies lists all policies for an AI Gateway.
func (c *Client) ListAIGatewayCustomPolicies(ctx context.Context, gatewayID string) ([]AIGatewayCustomPolicy, error) {
	if err := ValidateAPIClient(c.aiGatewayCustomPoliciesAPI, "AI Gateway Custom Policies API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayCustomPolicy
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayCustomPoliciesRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayCustomPoliciesAPI.ListAiGatewayCustomPolicies(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Custom Policies", nil)
		}

		if resp == nil || resp.ListAIGatewayCustomPoliciesResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayCustomPoliciesResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(resp.ListAIGatewayCustomPoliciesResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	policies := make([]AIGatewayCustomPolicy, 0, len(allData))
	for _, policy := range allData {
		policies = append(policies, AIGatewayCustomPolicy{
			AIGatewayCustomPolicy: policy,
			NormalizedLabels:      normalizedAIGatewayCustomPolicyLabels(policy),
		})
	}
	return policies, nil
}

// GetAIGatewayCustomPolicy fetches an AI Gateway Custom Policy by ID or name.
func (c *Client) GetAIGatewayCustomPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
) (*AIGatewayCustomPolicy, error) {
	if err := ValidateAPIClient(c.aiGatewayCustomPoliciesAPI, "AI Gateway Custom Policies API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayCustomPoliciesAPI.GetAiGatewayCustomPolicy(ctx, gatewayID, policyID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Custom Policy by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayCustomPolicy),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayCustomPolicy == nil {
		return nil, nil
	}

	return &AIGatewayCustomPolicy{
		AIGatewayCustomPolicy: *resp.AIGatewayCustomPolicy,
		NormalizedLabels:      normalizedAIGatewayCustomPolicyLabels(*resp.AIGatewayCustomPolicy),
	}, nil
}

// GetAIGatewayCustomPolicyByName finds an AI Gateway Custom Policy by name within a gateway.
func (c *Client) GetAIGatewayCustomPolicyByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayCustomPolicy, error) {
	policies, err := c.ListAIGatewayCustomPolicies(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(err, "list AI Gateway Custom Policies to find by name", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayCustomPolicy),
			ResourceName: name,
			UseEnhanced:  true,
		})
	}

	for i := range policies {
		if resources.AIGatewayCustomPolicyName(policies[i].AIGatewayCustomPolicy) == name {
			return &policies[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayCustomPolicy creates a new policy under an AI Gateway.
func (c *Client) CreateAIGatewayCustomPolicy(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayCustomPolicyRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayCustomPoliciesAPI, "AI Gateway Custom Policies API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayCustomPoliciesAPI.CreateAiGatewayCustomPolicy(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Custom Policy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayCustomPolicy),
			ResourceName: resources.AIGatewayCustomPolicyName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayCustomPolicy == nil {
		return "", fmt.Errorf("create AI Gateway Custom Policy response missing data")
	}

	return resources.AIGatewayCustomPolicyID(*resp.AIGatewayCustomPolicy), nil
}

// UpdateAIGatewayCustomPolicy updates an existing policy under an AI Gateway.
func (c *Client) UpdateAIGatewayCustomPolicy(
	ctx context.Context,
	gatewayID string,
	policyID string,
	req kkComps.UpdateAIGatewayCustomPolicyRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayCustomPoliciesAPI, "AI Gateway Custom Policies API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayCustomPoliciesAPI.UpdateAiGatewayCustomPolicy(ctx, kkOps.UpdateAiGatewayCustomPolicyRequest{
		GatewayID:                          gatewayID,
		CustomPolicyIDOrName:               policyID,
		UpdateAIGatewayCustomPolicyRequest: req,
	})
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Custom Policy", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayCustomPolicy),
			ResourceName: resources.AIGatewayCustomPolicyName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayCustomPolicy == nil {
		return "", fmt.Errorf("update AI Gateway Custom Policy response missing data")
	}

	return resources.AIGatewayCustomPolicyID(*resp.AIGatewayCustomPolicy), nil
}

// DeleteAIGatewayCustomPolicy deletes an AI Gateway Custom Policy by ID.
func (c *Client) DeleteAIGatewayCustomPolicy(ctx context.Context, gatewayID string, policyID string) error {
	if err := ValidateAPIClient(c.aiGatewayCustomPoliciesAPI, "AI Gateway Custom Policies API"); err != nil {
		return err
	}

	_, err := c.aiGatewayCustomPoliciesAPI.DeleteAiGatewayCustomPolicy(ctx, gatewayID, policyID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Custom Policy", nil)
	}

	return nil
}

func normalizedAIGatewayCustomPolicyLabels(_ kkComps.AIGatewayCustomPolicy) map[string]string {
	return map[string]string{}
}

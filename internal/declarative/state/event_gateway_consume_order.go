package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// OrderEventGatewayConsumePolicies observes the execution chain, rather than
// relying on the resource list's unspecified ordering. Children are included in
// the flattened chain and retain their order when filtered by parent.
func (c *Client) OrderEventGatewayConsumePolicies(
	ctx context.Context, gatewayID, virtualClusterID string, policies []EventGatewayConsumePolicyInfo,
) ([]EventGatewayConsumePolicyInfo, error) {
	if err := ValidateAPIClient(c.eventGatewayConsumePolicyAPI, "event gateway consume policy API"); err != nil {
		return nil, err
	}
	res, err := c.eventGatewayConsumePolicyAPI.GetEventGatewayVirtualClusterConsumePolicyChain(ctx,
		kkOps.GetEventGatewayVirtualClusterConsumePolicyChainRequest{
			GatewayID: gatewayID, VirtualClusterID: virtualClusterID,
		})
	if err != nil {
		return nil, fmt.Errorf("get consume policy chain: %w", err)
	}
	if res == nil || res.EventGatewayPolicyChainResponse == nil {
		return nil, fmt.Errorf("missing consume policy chain response")
	}
	byID := make(map[string]EventGatewayConsumePolicyInfo, len(policies))
	for _, policy := range policies {
		byID[policy.ID] = policy
	}
	ordered := make([]EventGatewayConsumePolicyInfo, 0, len(policies))
	for _, id := range res.EventGatewayPolicyChainResponse.Policies {
		policy, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("consume policy chain contains unknown or duplicate ID %q; retry planning", id)
		}
		ordered = append(ordered, policy)
		delete(byID, id)
	}
	if len(byID) != 0 {
		return nil, fmt.Errorf("consume policy chain is missing policies; retry planning")
	}
	return ordered, nil
}

func (c *Client) MoveEventGatewayConsumePolicy(
	ctx context.Context, gatewayID, virtualClusterID, policyID string, index int,
) error {
	if err := ValidateAPIClient(c.eventGatewayConsumePolicyAPI, "event gateway consume policy API"); err != nil {
		return err
	}
	_, err := c.eventGatewayConsumePolicyAPI.MoveEventGatewayVirtualClusterConsumePolicy(ctx,
		kkOps.MoveEventGatewayVirtualClusterConsumePolicyRequest{
			GatewayID: gatewayID, VirtualClusterID: virtualClusterID, PolicyID: policyID,
			MoveEventGatewayPolicy: &kkComps.MoveEventGatewayPolicy{Index: int64(index)},
		})
	if err != nil {
		return fmt.Errorf("move consume policy %s to index %d: %w", policyID, index, err)
	}
	return nil
}

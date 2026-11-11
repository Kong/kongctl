package executor

import (
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/planner"
)

type consumeOrderContract struct{ payloadContract }

func (c consumeOrderContract) ValidatePayload(ctx context.Context, change planner.PlannedChange) error {
	if change.ConsumePolicyOrder == nil {
		return c.payloadContract.ValidatePayload(ctx, change)
	}
	order := change.ConsumePolicyOrder
	if change.Action != planner.ActionUpdate || len(change.Fields) != 0 || len(order.Names) == 0 ||
		(order.ParentName != "" && order.ParentID != "") {
		return fmt.Errorf("invalid consume policy ordering operation")
	}
	seen := make(map[string]bool)
	for _, name := range order.Names {
		if name == "" || seen[name] {
			return fmt.Errorf("empty or duplicate consume policy name %q", name)
		}
		seen[name] = true
	}
	for _, field := range []string{planner.FieldEventGatewayID, planner.FieldEventGatewayVirtualClusterID} {
		ref := change.References[field]
		if ref.ID == "" && ref.Ref == "" {
			return fmt.Errorf("consume policy order requires %s", field)
		}
	}
	return nil
}

func (e *Executor) executeConsumePolicyOrder(ctx context.Context, change *planner.PlannedChange) (string, error) {
	if err := e.prepareEventGatewayClusterPolicy(ctx, change); err != nil {
		return "", err
	}
	gw := change.References[planner.FieldEventGatewayID].ID
	vc := change.References[planner.FieldEventGatewayVirtualClusterID].ID
	policies, err := e.client.ListEventGatewayConsumePolicies(ctx, gw, vc)
	if err != nil {
		return "", err
	}
	policies, err = e.client.OrderEventGatewayConsumePolicies(ctx, gw, vc, policies)
	if err != nil {
		return "", err
	}
	order := change.ConsumePolicyOrder
	parent := order.ParentID
	if order.ParentName != "" {
		for _, policy := range policies {
			if policy.Name != nil && *policy.Name == order.ParentName {
				if parent != "" {
					return "", fmt.Errorf("ambiguous consume policy parent %q", order.ParentName)
				}
				parent = policy.ID
			}
		}
		if parent == "" {
			return "", fmt.Errorf("consume policy parent %q no longer exists", order.ParentName)
		}
	}
	byName := make(map[string]string)
	var siblings []string
	for _, policy := range policies {
		policyParent := ""
		if policy.ParentPolicyID != nil {
			policyParent = *policy.ParentPolicyID
		}
		if policyParent != parent {
			continue
		}
		siblings = append(siblings, policy.ID)
		if policy.Name != nil {
			if _, ok := byName[*policy.Name]; ok {
				return "", fmt.Errorf("ambiguous consume policy name %q", *policy.Name)
			}
			byName[*policy.Name] = policy.ID
		}
	}
	// Validate the entire intent before sending the first move. Undeclared
	// siblings are retained, in their original relative order, after the prefix.
	ids := make([]string, len(order.Names))
	for i, name := range order.Names {
		id := byName[name]
		if id == "" {
			return "", fmt.Errorf("consume policy %q is missing from the intended parent", name)
		}
		ids[i] = id
	}
	for i, id := range ids {
		pos := slices.Index(siblings, id)
		if pos == i {
			continue
		}
		if err := e.client.MoveEventGatewayConsumePolicy(ctx, gw, vc, id, i); err != nil {
			return "", err
		}
		siblings = slices.Delete(siblings, pos, pos+1)
		siblings = slices.Insert(siblings, i, id)
	}
	return vc, nil
}

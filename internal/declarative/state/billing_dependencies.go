package state

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// BillingDependentKinds is shared by plan-time and fresh execution-time guards.
func BillingDependentKinds(kind resources.ResourceType) []resources.ResourceType {
	// Scoped billing lifecycle classification; other kinds follow the default path.
	//exhaustive:ignore
	switch kind {
	case resources.ResourceTypeFeature:
		return []resources.ResourceType{resources.ResourceTypeBillingPlan}
	case resources.ResourceTypeTaxCode:
		return []resources.ResourceType{resources.ResourceTypeBillingPlan, resources.ResourceTypeBillingProfile}
	case resources.ResourceTypeBillingApp:
		return []resources.ResourceType{resources.ResourceTypeBillingProfile}
	default:
		return nil
	}
}

func (c *Client) ValidateBillingDeletion(ctx context.Context, kind resources.ResourceType, id string) error {
	for _, parent := range BillingDependentKinds(kind) {
		values, err := c.ListBilling(ctx, resources.BillingKindForResourceType(parent))
		if err != nil {
			return fmt.Errorf("inspect %s before deleting %s: %w", parent, kind, err)
		}
		for _, v := range values {
			for path, target := range resources.BillingReferenceTargets(parent, v) {
				if target != kind {
					continue
				}
				value, _, err := resources.BillingPointer(v, path)
				if err != nil {
					return err
				}
				if value == id {
					return fmt.Errorf(
						"cannot delete %s %s: %s %s still references it at %s",
						kind,
						id,
						parent,
						v.ID(),
						path,
					)
				}
			}
		}
	}
	return nil
}

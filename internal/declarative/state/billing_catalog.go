package state

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/konnect/helpers"
)

func (c *Client) BillingCatalog() (helpers.BillingCatalogAPI, error) {
	if c.billingCatalogAPI == nil {
		return nil, fmt.Errorf("billing catalog API is not configured")
	}
	return c.billingCatalogAPI, nil
}

func (c *Client) ListBilling(ctx context.Context, kind helpers.BillingKind) ([]helpers.BillingObject, error) {
	api, err := c.BillingCatalog()
	if err != nil {
		return nil, err
	}
	options := helpers.BillingListOptions{IncludeDeleted: kind == helpers.BillingTaxCode}
	return api.List(ctx, kind, 100, options)
}

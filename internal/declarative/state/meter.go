package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type Meter struct {
	kkComps.Meter
	NormalizedLabels map[string]string
}

// ListMeters includes unmanaged meters so global key collisions cannot cause
// implicit adoption or cross-namespace mutations during planning.
func (c *Client) ListMeters(ctx context.Context) ([]Meter, error) {
	values, err := helpers.ListAllMeters(ctx, c.metersAPI, 100)
	if err != nil {
		return nil, WrapAPIError(err, "list meters", nil)
	}
	result := make([]Meter, 0, len(values))
	for _, v := range values {
		result = append(result, Meter{Meter: v, NormalizedLabels: v.Labels})
	}
	return result, nil
}

func (c *Client) ListManagedMeters(ctx context.Context, namespaces []string) ([]Meter, error) {
	if len(namespaces) == 0 {
		return nil, nil
	}
	values, err := c.ListMeters(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Meter, 0)
	for _, m := range values {
		if labels.IsManagedResource(m.Labels) && shouldIncludeNamespace(m.Labels[labels.NamespaceKey], namespaces) {
			result = append(result, m)
		}
	}
	return result, nil
}

func (c *Client) GetMeterByID(ctx context.Context, id string) (*Meter, error) {
	if c.metersAPI == nil {
		return nil, fmt.Errorf("meters API not configured")
	}
	resp, err := c.metersAPI.GetMeter(ctx, id)
	if err != nil {
		return nil, WrapAPIError(err, "get meter", nil)
	}
	if resp == nil || resp.Meter == nil {
		return nil, fmt.Errorf("get meter response missing data")
	}
	return &Meter{Meter: *resp.Meter, NormalizedLabels: resp.Meter.Labels}, nil
}

func (c *Client) CreateMeter(ctx context.Context, req kkComps.CreateMeterRequest, _ string) (*kkComps.Meter, error) {
	if c.metersAPI == nil {
		return nil, fmt.Errorf("meters API not configured")
	}
	if err := helpers.ValidateMeterCreate(req); err != nil {
		return nil, err
	}
	resp, err := c.metersAPI.CreateMeter(ctx, req)
	if err != nil {
		return nil, WrapAPIError(err, "create meter", nil)
	}
	if resp == nil || resp.Meter == nil {
		return nil, fmt.Errorf("create meter response missing data")
	}
	return resp.Meter, nil
}

func (c *Client) UpdateMeter(
	ctx context.Context,
	id string,
	req kkComps.UpdateMeterRequest,
	_ string,
) (*kkComps.Meter, error) {
	if c.metersAPI == nil {
		return nil, fmt.Errorf("meters API not configured")
	}
	if err := helpers.ValidateMeterUpdate(req); err != nil {
		return nil, err
	}
	resp, err := c.metersAPI.UpdateMeter(ctx, id, req)
	if err != nil {
		return nil, WrapAPIError(err, "update meter", nil)
	}
	if resp == nil || resp.Meter == nil {
		return nil, fmt.Errorf("update meter response missing data")
	}
	return resp.Meter, nil
}

func (c *Client) DeleteMeter(ctx context.Context, id string) error {
	if c.metersAPI == nil {
		return fmt.Errorf("meters API not configured")
	}
	_, err := c.metersAPI.DeleteMeter(ctx, id)
	if err != nil {
		return WrapAPIError(err, "delete meter", nil)
	}
	return nil
}

package state

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

// ListAIGatewayDatastores lists all Datastores for an AI Gateway.
func (c *Client) ListAIGatewayDatastores(
	ctx context.Context,
	gatewayID string,
) ([]AIGatewayDatastore, error) {
	if err := ValidateAPIClient(c.aiGatewayDatastoresAPI, "AI Gateway Datastores API"); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayDatastoreSummary
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayDatastoresRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayDatastoresAPI.ListAiGatewayDatastores(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Datastores", nil)
		}

		if resp == nil || resp.ListAIGatewayDatastoresResponse == nil {
			break
		}

		allData = append(allData, resp.ListAIGatewayDatastoresResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(
			resp.ListAIGatewayDatastoresResponse.Meta.Page.Next,
		)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	datastores := make([]AIGatewayDatastore, 0, len(allData))
	for _, summary := range allData {
		datastore, err := c.GetAIGatewayDatastore(ctx, gatewayID, summary.ID)
		if err != nil {
			return nil, err
		}
		if datastore != nil {
			datastores = append(datastores, *datastore)
		}
	}

	return datastores, nil
}

// GetAIGatewayDatastore fetches an AI Gateway Datastore by ID or name.
func (c *Client) GetAIGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	datastoreID string,
) (*AIGatewayDatastore, error) {
	if err := ValidateAPIClient(c.aiGatewayDatastoresAPI, "AI Gateway Datastores API"); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayDatastoresAPI.GetAiGatewayDatastore(ctx, gatewayID, datastoreID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway Datastore by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayDatastore),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayDatastore == nil {
		return nil, nil
	}

	return &AIGatewayDatastore{
		AIGatewayDatastore: *resp.AIGatewayDatastore,
		KonnectManaged:     aiGatewayDatastoreIsKonnectManaged(*resp.AIGatewayDatastore),
		NormalizedLabels:   normalizedAIGatewayDatastoreLabels(*resp.AIGatewayDatastore),
	}, nil
}

// GetAIGatewayDatastoreByName finds an AI Gateway Datastore by name within a gateway.
func (c *Client) GetAIGatewayDatastoreByName(
	ctx context.Context,
	gatewayID string,
	name string,
) (*AIGatewayDatastore, error) {
	datastores, err := c.ListAIGatewayDatastores(ctx, gatewayID)
	if err != nil {
		return nil, WrapAPIError(
			err,
			"list AI Gateway Datastores to find by name",
			&ErrorWrapperOptions{
				ResourceType: string(resources.ResourceTypeAIGatewayDatastore),
				ResourceName: name,
				UseEnhanced:  true,
			},
		)
	}

	for i := range datastores {
		if resources.AIGatewayDatastoreName(datastores[i].AIGatewayDatastore) == name {
			return &datastores[i], nil
		}
	}

	return nil, nil
}

// CreateAIGatewayDatastore creates a new Datastore under an AI Gateway.
func (c *Client) CreateAIGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayDatastoreRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayDatastoresAPI, "AI Gateway Datastores API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayDatastoresAPI.CreateAiGatewayDatastore(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway Datastore", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayDatastore),
			ResourceName: aiGatewayDatastoreCreateRequestName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayDatastore == nil {
		return "", fmt.Errorf("create AI Gateway Datastore response missing data")
	}

	return resources.AIGatewayDatastoreID(*resp.AIGatewayDatastore), nil
}

// UpdateAIGatewayDatastore updates an existing Datastore under an AI Gateway.
func (c *Client) UpdateAIGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	datastoreID string,
	req kkComps.UpdateAIGatewayDatastoreRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayDatastoresAPI, "AI Gateway Datastores API"); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayDatastoresAPI.UpdateAiGatewayDatastore(
		ctx,
		kkOps.UpdateAiGatewayDatastoreRequest{
			GatewayID:                       gatewayID,
			DatastoreIDOrName:               datastoreID,
			UpdateAIGatewayDatastoreRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(err, "update AI Gateway Datastore", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayDatastore),
			ResourceName: aiGatewayDatastoreUpdateRequestName(req),
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayDatastore == nil {
		return "", fmt.Errorf("update AI Gateway Datastore response missing data")
	}

	return resources.AIGatewayDatastoreID(*resp.AIGatewayDatastore), nil
}

// DeleteAIGatewayDatastore deletes an AI Gateway Datastore by ID.
func (c *Client) DeleteAIGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	datastoreID string,
) error {
	if err := ValidateAPIClient(c.aiGatewayDatastoresAPI, "AI Gateway Datastores API"); err != nil {
		return err
	}

	_, err := c.aiGatewayDatastoresAPI.DeleteAiGatewayDatastore(ctx, gatewayID, datastoreID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway Datastore", nil)
	}

	return nil
}

func normalizedAIGatewayDatastoreLabels(datastore kkComps.AIGatewayDatastore) map[string]string {
	normalized := resources.AIGatewayDatastoreLabels(datastore)
	if normalized == nil {
		normalized = make(map[string]string)
	}
	return normalized
}

func aiGatewayDatastoreCreateRequestName(req kkComps.CreateAIGatewayDatastoreRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return stringFromRaw(raw["name"])
}

func aiGatewayDatastoreUpdateRequestName(req kkComps.UpdateAIGatewayDatastoreRequest) string {
	data, err := json.Marshal(req)
	if err != nil {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return stringFromRaw(raw["name"])
}

func aiGatewayDatastoreIsKonnectManaged(value kkComps.AIGatewayDatastore) bool {
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	var fields struct {
		KonnectManaged bool `json:"konnect_managed"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		return false
	}
	return fields.KonnectManaged
}

func (c *Client) ListAIGatewayDatastoreUsage(
	ctx context.Context,
	gatewayID, datastoreID string,
) ([]kkComps.AIGatewayDatastoreUsageEntry, error) {
	if err := ValidateAPIClient(c.aiGatewayDatastoresAPI, "AI Gateway Datastores API"); err != nil {
		return nil, err
	}
	var data []kkComps.AIGatewayDatastoreUsageEntry
	var after *string
	pageSize := int64(100)
	for {
		resp, err := c.aiGatewayDatastoresAPI.ListAiGatewayDatastoreUsage(
			ctx,
			kkOps.ListAiGatewayDatastoreUsageRequest{
				GatewayID: gatewayID, DatastoreIDOrName: datastoreID, PageSize: &pageSize, PageAfter: after,
			},
		)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway Datastore usage", nil)
		}
		if resp == nil || resp.ListAIGatewayDatastoreUsageResponse == nil {
			break
		}
		data = append(data, resp.ListAIGatewayDatastoreUsageResponse.Data...)
		next := pagination.ExtractPageAfterCursor(
			resp.ListAIGatewayDatastoreUsageResponse.Meta.Page.Next,
		)
		if next == "" {
			break
		}
		after = &next
	}
	return data, nil
}

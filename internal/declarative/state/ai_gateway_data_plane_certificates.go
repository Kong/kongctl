package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

const aiGatewayDataPlaneCertificatesAPIName = "AI Gateway data plane certificates API"

// ListAIGatewayDataPlaneCertificates lists all data plane certificates for an AI Gateway.
func (c *Client) ListAIGatewayDataPlaneCertificates(
	ctx context.Context,
	gatewayID string,
) ([]AIGatewayDataPlaneCertificate, error) {
	if err := ValidateAPIClient(c.aiGatewayDataPlaneCertificatesAPI, aiGatewayDataPlaneCertificatesAPIName); err != nil {
		return nil, err
	}

	var allData []kkComps.AIGatewayDataPlaneClientCertificate
	var pageAfter *string
	pageSize := int64(100)

	for {
		req := kkOps.ListAiGatewayDataPlaneCertificatesRequest{
			GatewayID: gatewayID,
			PageSize:  &pageSize,
			PageAfter: pageAfter,
		}

		resp, err := c.aiGatewayDataPlaneCertificatesAPI.ListAiGatewayDataPlaneCertificates(ctx, req)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway data plane certificates", nil)
		}

		if resp == nil || resp.ListAIGatewayDataPlaneCertificatesResponse == nil {
			return []AIGatewayDataPlaneCertificate{}, nil
		}

		allData = append(allData, resp.ListAIGatewayDataPlaneCertificatesResponse.Data...)

		nextCursor := pagination.ExtractPageAfterCursor(
			resp.ListAIGatewayDataPlaneCertificatesResponse.Meta.Page.Next,
		)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	certs := make([]AIGatewayDataPlaneCertificate, 0, len(allData))
	for _, cert := range allData {
		certs = append(certs, AIGatewayDataPlaneCertificate{
			AIGatewayDataPlaneClientCertificate: cert,
		})
	}
	return certs, nil
}

// GetAIGatewayDataPlaneCertificate fetches an AI Gateway data plane certificate by ID.
func (c *Client) GetAIGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
) (*AIGatewayDataPlaneCertificate, error) {
	if err := ValidateAPIClient(c.aiGatewayDataPlaneCertificatesAPI, aiGatewayDataPlaneCertificatesAPIName); err != nil {
		return nil, err
	}

	resp, err := c.aiGatewayDataPlaneCertificatesAPI.GetAiGatewayDataPlaneCertificate(ctx, gatewayID, certificateID)
	if err != nil {
		return nil, WrapAPIError(err, "get AI Gateway data plane certificate by ID", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayDataPlaneCertificate),
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayDataPlaneClientCertificate == nil {
		return nil, nil
	}

	return &AIGatewayDataPlaneCertificate{
		AIGatewayDataPlaneClientCertificate: *resp.AIGatewayDataPlaneClientCertificate,
	}, nil
}

// CreateAIGatewayDataPlaneCertificate creates a new data plane certificate under an AI Gateway.
func (c *Client) CreateAIGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayDataPlaneCertificateRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayDataPlaneCertificatesAPI, aiGatewayDataPlaneCertificatesAPIName); err != nil {
		return "", err
	}

	resp, err := c.aiGatewayDataPlaneCertificatesAPI.CreateAiGatewayDataPlaneCertificate(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(err, "create AI Gateway data plane certificate", &ErrorWrapperOptions{
			ResourceType: string(resources.ResourceTypeAIGatewayDataPlaneCertificate),
			ResourceName: req.Title,
			Namespace:    namespace,
			UseEnhanced:  true,
		})
	}

	if resp == nil || resp.AIGatewayDataPlaneClientCertificate == nil {
		return "", fmt.Errorf("create AI Gateway data plane certificate response missing data")
	}

	return resp.AIGatewayDataPlaneClientCertificate.ID, nil
}

// DeleteAIGatewayDataPlaneCertificate deletes an AI Gateway data plane certificate by ID.
func (c *Client) DeleteAIGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
) error {
	if err := ValidateAPIClient(c.aiGatewayDataPlaneCertificatesAPI, aiGatewayDataPlaneCertificatesAPIName); err != nil {
		return err
	}

	_, err := c.aiGatewayDataPlaneCertificatesAPI.DeleteAiGatewayDataPlaneCertificate(ctx, gatewayID, certificateID)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway data plane certificate", nil)
	}

	return nil
}

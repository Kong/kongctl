package state

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/util/pagination"
)

const (
	aiGatewayCertificatesAPIName   = "AI Gateway certificates API"
	aiGatewayCACertificatesAPIName = "AI Gateway CA certificates API"
	aiGatewaySNIsAPIName           = "AI Gateway SNIs API"
)

func (c *Client) ListAIGatewayCertificates(ctx context.Context, gatewayID string) ([]AIGatewayCertificate, error) {
	if err := ValidateAPIClient(c.aiGatewayCertificatesAPI, aiGatewayCertificatesAPIName); err != nil {
		return nil, err
	}
	var result []AIGatewayCertificate
	var after *string
	size := int64(100)
	for {
		resp, err := c.aiGatewayCertificatesAPI.ListAiGatewayCertificates(ctx, kkOps.ListAiGatewayCertificatesRequest{
			GatewayID: gatewayID, PageSize: &size, PageAfter: after,
		})
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway certificates", nil)
		}
		if resp == nil || resp.ListAIGatewayCertificatesResponse == nil {
			break
		}
		for _, item := range resp.ListAIGatewayCertificatesResponse.Data {
			result = append(result, AIGatewayCertificate{AIGatewayCertificate: item, NormalizedLabels: item.Labels})
		}
		next := pagination.ExtractPageAfterCursor(resp.ListAIGatewayCertificatesResponse.Meta.Page.Next)
		if next == "" {
			break
		}
		after = &next
	}
	return result, nil
}

func (c *Client) GetAIGatewayCertificate(ctx context.Context, gatewayID, id string) (*AIGatewayCertificate, error) {
	if err := ValidateAPIClient(c.aiGatewayCertificatesAPI, aiGatewayCertificatesAPIName); err != nil {
		return nil, err
	}
	resp, err := c.aiGatewayCertificatesAPI.GetAiGatewayCertificate(ctx, gatewayID, id)
	if err != nil {
		return nil, WrapAPIError(
			err, "get AI Gateway certificate",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewayCertificate, "", ""),
		)
	}
	if resp == nil || resp.AIGatewayCertificate == nil {
		return nil, nil
	}
	return &AIGatewayCertificate{
		AIGatewayCertificate: *resp.AIGatewayCertificate,
		NormalizedLabels:     resp.AIGatewayCertificate.Labels,
	}, nil
}

func (c *Client) CreateAIGatewayCertificate(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayCertificateRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayCertificatesAPI, aiGatewayCertificatesAPIName); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayCertificatesAPI.CreateAiGatewayCertificate(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(
			err, "create AI Gateway certificate",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewayCertificate, req.Name, namespace),
		)
	}
	if resp == nil || resp.AIGatewayCertificate == nil {
		return "", fmt.Errorf("create AI Gateway certificate response missing data")
	}
	return resp.AIGatewayCertificate.ID, nil
}

func (c *Client) UpdateAIGatewayCertificate(
	ctx context.Context,
	gatewayID, id string,
	req kkComps.UpdateAIGatewayCertificateRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayCertificatesAPI, aiGatewayCertificatesAPIName); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayCertificatesAPI.UpdateAiGatewayCertificate(
		ctx,
		kkOps.UpdateAiGatewayCertificateRequest{
			GatewayID: gatewayID, CertificateIDOrName: id, UpdateAIGatewayCertificateRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(
			err, "update AI Gateway certificate",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewayCertificate, req.Name, namespace),
		)
	}
	if resp == nil || resp.AIGatewayCertificate == nil {
		return id, nil
	}
	return resp.AIGatewayCertificate.ID, nil
}

func (c *Client) DeleteAIGatewayCertificate(ctx context.Context, gatewayID, id string) error {
	if err := ValidateAPIClient(c.aiGatewayCertificatesAPI, aiGatewayCertificatesAPIName); err != nil {
		return err
	}
	_, err := c.aiGatewayCertificatesAPI.DeleteAiGatewayCertificate(ctx, gatewayID, id)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway certificate", nil)
	}
	return nil
}

func (c *Client) ListAIGatewayCACertificates(ctx context.Context, gatewayID string) ([]AIGatewayCACertificate, error) {
	if err := ValidateAPIClient(c.aiGatewayCACertificatesAPI, aiGatewayCACertificatesAPIName); err != nil {
		return nil, err
	}
	var result []AIGatewayCACertificate
	var after *string
	size := int64(100)
	for {
		resp, err := c.aiGatewayCACertificatesAPI.ListAiGatewayCaCertificates(
			ctx,
			kkOps.ListAiGatewayCaCertificatesRequest{GatewayID: gatewayID, PageSize: &size, PageAfter: after},
		)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway CA certificates", nil)
		}
		if resp == nil || resp.ListAIGatewayCACertificatesResponse == nil {
			break
		}
		for _, item := range resp.ListAIGatewayCACertificatesResponse.Data {
			result = append(result, AIGatewayCACertificate{AIGatewayCACertificate: item, NormalizedLabels: item.Labels})
		}
		next := pagination.ExtractPageAfterCursor(resp.ListAIGatewayCACertificatesResponse.Meta.Page.Next)
		if next == "" {
			break
		}
		after = &next
	}
	return result, nil
}

func (c *Client) GetAIGatewayCACertificate(ctx context.Context, gatewayID, id string) (*AIGatewayCACertificate, error) {
	if err := ValidateAPIClient(c.aiGatewayCACertificatesAPI, aiGatewayCACertificatesAPIName); err != nil {
		return nil, err
	}
	resp, err := c.aiGatewayCACertificatesAPI.GetAiGatewayCaCertificate(ctx, gatewayID, id)
	if err != nil {
		return nil, WrapAPIError(
			err, "get AI Gateway CA certificate",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewayCACertificate, "", ""),
		)
	}
	if resp == nil || resp.AIGatewayCACertificate == nil {
		return nil, nil
	}
	return &AIGatewayCACertificate{
		AIGatewayCACertificate: *resp.AIGatewayCACertificate,
		NormalizedLabels:       resp.AIGatewayCACertificate.Labels,
	}, nil
}

func (c *Client) CreateAIGatewayCACertificate(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewayCACertificateRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayCACertificatesAPI, aiGatewayCACertificatesAPIName); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayCACertificatesAPI.CreateAiGatewayCaCertificate(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(
			err, "create AI Gateway CA certificate",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewayCACertificate, req.Name, namespace),
		)
	}
	if resp == nil || resp.AIGatewayCACertificate == nil {
		return "", fmt.Errorf("create AI Gateway CA certificate response missing data")
	}
	return resp.AIGatewayCACertificate.ID, nil
}

func (c *Client) UpdateAIGatewayCACertificate(
	ctx context.Context,
	gatewayID, id string,
	req kkComps.UpdateAIGatewayCACertificateRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewayCACertificatesAPI, aiGatewayCACertificatesAPIName); err != nil {
		return "", err
	}
	resp, err := c.aiGatewayCACertificatesAPI.UpdateAiGatewayCaCertificate(
		ctx,
		kkOps.UpdateAiGatewayCaCertificateRequest{
			GatewayID: gatewayID, CaCertificateIDOrName: id, UpdateAIGatewayCACertificateRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(
			err, "update AI Gateway CA certificate",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewayCACertificate, req.Name, namespace),
		)
	}
	if resp == nil || resp.AIGatewayCACertificate == nil {
		return id, nil
	}
	return resp.AIGatewayCACertificate.ID, nil
}

func (c *Client) DeleteAIGatewayCACertificate(ctx context.Context, gatewayID, id string) error {
	if err := ValidateAPIClient(c.aiGatewayCACertificatesAPI, aiGatewayCACertificatesAPIName); err != nil {
		return err
	}
	_, err := c.aiGatewayCACertificatesAPI.DeleteAiGatewayCaCertificate(ctx, gatewayID, id)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway CA certificate", nil)
	}
	return nil
}

func (c *Client) ListAIGatewaySNIs(ctx context.Context, gatewayID string) ([]AIGatewaySNI, error) {
	if err := ValidateAPIClient(c.aiGatewaySNIsAPI, aiGatewaySNIsAPIName); err != nil {
		return nil, err
	}
	var result []AIGatewaySNI
	var after *string
	size := int64(100)
	for {
		resp, err := c.aiGatewaySNIsAPI.ListAiGatewaySnis(
			ctx,
			kkOps.ListAiGatewaySnisRequest{GatewayID: gatewayID, PageSize: &size, PageAfter: after},
		)
		if err != nil {
			return nil, WrapAPIError(err, "list AI Gateway SNIs", nil)
		}
		if resp == nil || resp.ListAIGatewaySNIsResponse == nil {
			break
		}
		for _, item := range resp.ListAIGatewaySNIsResponse.Data {
			result = append(result, AIGatewaySNI{AIGatewaySNI: item, NormalizedLabels: item.Labels})
		}
		next := pagination.ExtractPageAfterCursor(resp.ListAIGatewaySNIsResponse.Meta.Page.Next)
		if next == "" {
			break
		}
		after = &next
	}
	return result, nil
}

func (c *Client) GetAIGatewaySNI(ctx context.Context, gatewayID, id string) (*AIGatewaySNI, error) {
	if err := ValidateAPIClient(c.aiGatewaySNIsAPI, aiGatewaySNIsAPIName); err != nil {
		return nil, err
	}
	resp, err := c.aiGatewaySNIsAPI.GetAiGatewaySni(ctx, gatewayID, id)
	if err != nil {
		return nil, WrapAPIError(
			err, "get AI Gateway SNI",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewaySNI, "", ""),
		)
	}
	if resp == nil || resp.AIGatewaySNI == nil {
		return nil, nil
	}
	return &AIGatewaySNI{AIGatewaySNI: *resp.AIGatewaySNI, NormalizedLabels: resp.AIGatewaySNI.Labels}, nil
}

func (c *Client) CreateAIGatewaySNI(
	ctx context.Context,
	gatewayID string,
	req kkComps.CreateAIGatewaySNIRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewaySNIsAPI, aiGatewaySNIsAPIName); err != nil {
		return "", err
	}
	resp, err := c.aiGatewaySNIsAPI.CreateAiGatewaySni(ctx, gatewayID, req)
	if err != nil {
		return "", WrapAPIError(
			err, "create AI Gateway SNI",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewaySNI, req.Name, namespace),
		)
	}
	if resp == nil || resp.AIGatewaySNI == nil {
		return "", fmt.Errorf("create AI Gateway SNI response missing data")
	}
	return resp.AIGatewaySNI.ID, nil
}

func (c *Client) UpdateAIGatewaySNI(
	ctx context.Context,
	gatewayID, id string,
	req kkComps.UpdateAIGatewaySNIRequest,
	namespace string,
) (string, error) {
	if err := ValidateAPIClient(c.aiGatewaySNIsAPI, aiGatewaySNIsAPIName); err != nil {
		return "", err
	}
	resp, err := c.aiGatewaySNIsAPI.UpdateAiGatewaySni(
		ctx,
		kkOps.UpdateAiGatewaySniRequest{
			GatewayID: gatewayID, SniIDOrName: id, UpdateAIGatewaySNIRequest: req,
		},
	)
	if err != nil {
		return "", WrapAPIError(
			err, "update AI Gateway SNI",
			aiGatewayTLSErrorOptions(resources.ResourceTypeAIGatewaySNI, req.Name, namespace),
		)
	}
	if resp == nil || resp.AIGatewaySNI == nil {
		return id, nil
	}
	return resp.AIGatewaySNI.ID, nil
}

func (c *Client) DeleteAIGatewaySNI(ctx context.Context, gatewayID, id string) error {
	if err := ValidateAPIClient(c.aiGatewaySNIsAPI, aiGatewaySNIsAPIName); err != nil {
		return err
	}
	_, err := c.aiGatewaySNIsAPI.DeleteAiGatewaySni(ctx, gatewayID, id)
	if err != nil {
		return WrapAPIError(err, "delete AI Gateway SNI", nil)
	}
	return nil
}

func aiGatewayTLSErrorOptions(
	resourceType resources.ResourceType,
	resourceName, namespace string,
) *ErrorWrapperOptions {
	return &ErrorWrapperOptions{
		ResourceType: string(resourceType), ResourceName: resourceName, Namespace: namespace, UseEnhanced: true,
	}
}

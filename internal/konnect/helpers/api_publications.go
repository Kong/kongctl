package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"

	"github.com/kong/kongctl/internal/konnect/apiutil"
)

// APIPublicationAPI defines the interface for operations on API Publications
type APIPublicationAPI interface {
	// API Publication operations
	PublishAPIToPortal(ctx context.Context, request kkOps.PublishAPIToPortalRequest,
		opts ...kkOps.Option) (*kkOps.PublishAPIToPortalResponse, error)
	DeletePublication(ctx context.Context, apiID string, portalID string,
		opts ...kkOps.Option) (*kkOps.DeletePublicationResponse, error)
	ListAPIPublications(ctx context.Context, request kkOps.ListAPIPublicationsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIPublicationsResponse, error)
}

// APIPublicationAPIImpl provides an implementation of the APIPublicationAPI interface
type APIPublicationAPIImpl struct {
	SDK        *kkSDK.SDK
	BaseURL    string
	Token      string
	HTTPClient kkSDK.HTTPClient
}

// PublishAPIToPortal implements the APIPublicationAPI interface
func (a *APIPublicationAPIImpl) PublishAPIToPortal(ctx context.Context, request kkOps.PublishAPIToPortalRequest,
	opts ...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIPublication == nil {
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}

	if requiresMergedPublicationPayload(request.APIPublication) {
		return a.publishAPIToPortalWithMergedPayload(ctx, request)
	}

	return a.SDK.APIPublication.PublishAPIToPortal(ctx, request, opts...)
}

// DeletePublication implements the APIPublicationAPI interface
func (a *APIPublicationAPIImpl) DeletePublication(ctx context.Context, apiID string, portalID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIPublication == nil {
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}
	return a.SDK.APIPublication.DeletePublication(ctx, apiID, portalID, opts...)
}

// ListAPIPublications implements the APIPublicationAPI interface
func (a *APIPublicationAPIImpl) ListAPIPublications(ctx context.Context,
	request kkOps.ListAPIPublicationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIPublication == nil {
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}
	return a.SDK.APIPublication.ListAPIPublications(ctx, request, opts...)
}

// GetPublicationsForAPI fetches all publication objects for a specific API
func GetPublicationsForAPI(ctx context.Context, kkClient APIPublicationAPI, apiID string) ([]any, error) {
	if kkClient == nil {
		return nil, fmt.Errorf("APIPublicationAPI client is nil")
	}

	apiIDFilter := &kkComponents.UUIDFieldFilter{
		Eq: &apiID,
	}

	publications, err := paginateAllPageNumber(func(pageSize, pageNumber int64) (
		[]kkComponents.APIPublicationListItem, float64, error,
	) {
		req := kkOps.ListAPIPublicationsRequest{
			PageSize:   Int64(pageSize),
			PageNumber: Int64(pageNumber),
			Filter: &kkComponents.APIPublicationFilterParameters{
				APIID: apiIDFilter,
			},
		}

		res, err := kkClient.ListAPIPublications(ctx, req)
		if err != nil {
			return nil, 0, err
		}

		if res == nil || res.ListAPIPublicationResponse == nil {
			return []kkComponents.APIPublicationListItem{}, 0, nil
		}

		return res.ListAPIPublicationResponse.Data, res.ListAPIPublicationResponse.Meta.Page.Total, nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]any, len(publications))
	for i, pub := range publications {
		result[i] = pub
	}

	return result, nil
}

func requiresExplicitAuthStrategyIDs(publication kkComponents.APIPublication) bool {
	return publication.AuthStrategyIds != nil && len(publication.AuthStrategyIds) == 0
}

func requiresMergedPublicationPayload(publication kkComponents.APIPublication) bool {
	return publication.Visibility == nil ||
		publication.AuthStrategyIds == nil ||
		requiresExplicitAuthStrategyIDs(publication)
}

func marshalMergedAPIPublicationPayload(publication kkComponents.APIPublication) ([]byte, error) {
	payload := map[string]any{}

	if publication.AutoApproveRegistrations != nil {
		payload["auto_approve_registrations"] = publication.AutoApproveRegistrations
	}

	if publication.AuthStrategyIds != nil {
		if len(publication.AuthStrategyIds) == 0 {
			payload["auth_strategy_ids"] = nil
		} else {
			authStrategyIDs := make([]string, len(publication.AuthStrategyIds))
			copy(authStrategyIDs, publication.AuthStrategyIds)
			payload["auth_strategy_ids"] = authStrategyIDs
		}
	}

	if publication.Visibility != nil {
		payload["visibility"] = publication.Visibility
	}

	return json.Marshal(payload)
}

func (a *APIPublicationAPIImpl) publishAPIToPortalWithMergedPayload(
	ctx context.Context,
	request kkOps.PublishAPIToPortalRequest,
) (*kkOps.PublishAPIToPortalResponse, error) {
	if strings.TrimSpace(a.BaseURL) == "" {
		return nil, fmt.Errorf("base URL is required for API publication requests")
	}

	publication := request.APIPublication
	if publication.Visibility == nil || publication.AuthStrategyIds == nil {
		current, err := a.fetchExistingPublication(ctx, request.APIID, request.PortalID)
		if err != nil {
			return nil, err
		}

		if current != nil {
			if publication.Visibility == nil {
				publication.Visibility = current.Visibility
			}
			if publication.AuthStrategyIds == nil {
				if current.AuthStrategyIds == nil {
					publication.AuthStrategyIds = []string{}
				} else {
					publication.AuthStrategyIds = append([]string(nil), current.AuthStrategyIds...)
				}
			}
			if publication.AutoApproveRegistrations == nil {
				publication.AutoApproveRegistrations = current.AutoApproveRegistrations
			}
		}
	}

	payload, err := marshalMergedAPIPublicationPayload(publication)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API publication request: %w", err)
	}

	path := fmt.Sprintf("/v3/apis/%s/publications/%s", url.PathEscape(request.APIID), url.PathEscape(request.PortalID))
	result, err := apiutil.Request(
		ctx,
		a.HTTPClient,
		http.MethodPut,
		a.BaseURL,
		path,
		a.Token,
		map[string]string{
			"Content-Type": "application/json",
		},
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}

	response := &kkOps.PublishAPIToPortalResponse{
		ContentType: result.Header.Get("Content-Type"),
		StatusCode:  result.StatusCode,
		RawResponse: &http.Response{
			StatusCode: result.StatusCode,
			Header:     result.Header.Clone(),
			Body:       io.NopCloser(bytes.NewReader(result.Body)),
		},
	}

	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		body := strings.TrimSpace(string(result.Body))
		if body == "" {
			return nil, fmt.Errorf("publish API to portal failed with status %d", result.StatusCode)
		}
		return nil, fmt.Errorf("publish API to portal failed with status %d: %s", result.StatusCode, body)
	}

	if len(bytes.TrimSpace(result.Body)) == 0 {
		return response, nil
	}

	var publicationResponse kkComponents.APIPublicationResponse
	if err := json.Unmarshal(result.Body, &publicationResponse); err != nil {
		return nil, fmt.Errorf("failed to decode API publication response: %w", err)
	}
	response.APIPublicationResponse = &publicationResponse

	return response, nil
}

func (a *APIPublicationAPIImpl) fetchExistingPublication(
	ctx context.Context,
	apiID string,
	portalID string,
) (*kkComponents.APIPublicationResponse, error) {
	if a.SDK == nil || a.SDK.APIPublication == nil {
		return nil, nil
	}

	resp, err := a.SDK.APIPublication.FetchPublication(ctx, apiID, portalID)
	if err != nil {
		var notFound *kkErrors.NotFoundError
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch current API publication: %w", err)
	}
	if resp == nil || resp.APIPublicationResponse == nil {
		return nil, nil
	}

	return resp.APIPublicationResponse, nil
}

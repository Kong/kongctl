//go:build integration
// +build integration

package declarative_test

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/mock"
)

// MockPortalAPI implements helpers.PortalAPI for testing
type MockPortalAPI struct {
	mock.Mock
	t *testing.T
}

func NewMockPortalAPI(t *testing.T) *MockPortalAPI {
	return &MockPortalAPI{t: t}
}

func (m *MockPortalAPI) ListPortals(
	ctx context.Context,
	request kkOps.ListPortalsRequest,
) (*kkOps.ListPortalsResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListPortalsResponse), args.Error(1)
}

func (m *MockPortalAPI) GetPortal(ctx context.Context, id string) (*kkOps.GetPortalResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.GetPortalResponse), args.Error(1)
}

func (m *MockPortalAPI) CreatePortal(
	ctx context.Context,
	portal kkComps.CreatePortal,
) (*kkOps.CreatePortalResponse, error) {
	args := m.Called(ctx, portal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkComps.UpdatePortal,
) (*kkOps.UpdatePortalResponse, error) {
	req := kkOps.UpdatePortalRequest{
		PortalID:     id,
		UpdatePortal: portal,
	}
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) DeletePortal(
	ctx context.Context,
	id string,
	force bool,
) (*kkOps.DeletePortalResponse, error) {
	args := m.Called(ctx, id, force)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeletePortalResponse), args.Error(1)
}

// MockAPIAPI implements helpers.APIAPI for testing
type MockAPIAPI struct {
	mock.Mock
	t *testing.T
}

func NewMockAPIAPI(t *testing.T) *MockAPIAPI {
	return &MockAPIAPI{t: t}
}

func (m *MockAPIAPI) ListApis(
	ctx context.Context,
	request kkOps.ListApisRequest,
	_ ...kkOps.Option,
) (*kkOps.ListApisResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListApisResponse), args.Error(1)
}

func (m *MockAPIAPI) FetchAPI(ctx context.Context, apiID string, _ ...kkOps.Option) (*kkOps.FetchAPIResponse, error) {
	args := m.Called(ctx, apiID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.FetchAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) CreateAPI(
	ctx context.Context,
	api kkComps.CreateAPIRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateAPIResponse, error) {
	args := m.Called(ctx, api)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) UpdateAPI(
	ctx context.Context,
	id string,
	api kkComps.UpdateAPIRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAPIResponse, error) {
	args := m.Called(ctx, id, api)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) DeleteAPI(
	ctx context.Context,
	id string,
	_ ...kkOps.Option,
) (*kkOps.DeleteAPIResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteAPIResponse), args.Error(1)
}

// API Version operations
func (m *MockAPIAPI) CreateAPIVersion(
	ctx context.Context,
	apiID string,
	request kkComps.CreateAPIVersionRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateAPIVersionResponse, error) {
	args := m.Called(ctx, apiID, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateAPIVersionResponse), args.Error(1)
}

func (m *MockAPIAPI) ListAPIVersions(
	ctx context.Context,
	request kkOps.ListAPIVersionsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListAPIVersionsResponse), args.Error(1)
}

func (m *MockAPIAPI) UpdateAPIVersion(
	ctx context.Context,
	request kkOps.UpdateAPIVersionRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAPIVersionResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateAPIVersionResponse), args.Error(1)
}

func (m *MockAPIAPI) DeleteAPIVersion(
	ctx context.Context,
	request kkOps.DeleteAPIVersionRequest,
	_ ...kkOps.Option,
) (*kkOps.DeleteAPIVersionResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteAPIVersionResponse), args.Error(1)
}

// API Publication operations
func (m *MockAPIAPI) PublishAPIToPortal(
	ctx context.Context,
	request kkOps.PublishAPIToPortalRequest,
	_ ...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.PublishAPIToPortalResponse), args.Error(1)
}

func (m *MockAPIAPI) ListAPIPublications(
	ctx context.Context,
	request kkOps.ListAPIPublicationsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListAPIPublicationsResponse), args.Error(1)
}

func (m *MockAPIAPI) DeletePublication(
	ctx context.Context,
	apiID string,
	portalID string,
	_ ...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	args := m.Called(ctx, apiID, portalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeletePublicationResponse), args.Error(1)
}

// API Implementation operations
func (m *MockAPIAPI) ListAPIImplementations(
	ctx context.Context,
	request kkOps.ListAPIImplementationsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListAPIImplementationsResponse), args.Error(1)
}

// API Document operations
func (m *MockAPIAPI) CreateAPIDocument(
	ctx context.Context,
	apiID string,
	request kkComps.CreateAPIDocumentRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateAPIDocumentResponse, error) {
	args := m.Called(ctx, apiID, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateAPIDocumentResponse), args.Error(1)
}

func (m *MockAPIAPI) ListAPIDocuments(
	ctx context.Context,
	request kkOps.ListAPIDocumentsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAPIDocumentsResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListAPIDocumentsResponse), args.Error(1)
}

func (m *MockAPIAPI) FetchAPIDocument(
	ctx context.Context,
	apiID string,
	documentID string,
	_ ...kkOps.Option,
) (*kkOps.FetchAPIDocumentResponse, error) {
	args := m.Called(ctx, apiID, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.FetchAPIDocumentResponse), args.Error(1)
}

func (m *MockAPIAPI) UpdateAPIDocument(
	ctx context.Context,
	apiID string,
	documentID string,
	request kkComps.APIDocument,
	_ ...kkOps.Option,
) (*kkOps.UpdateAPIDocumentResponse, error) {
	args := m.Called(ctx, apiID, documentID, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateAPIDocumentResponse), args.Error(1)
}

func (m *MockAPIAPI) DeleteAPIDocument(
	ctx context.Context,
	apiID string,
	documentID string,
	_ ...kkOps.Option,
) (*kkOps.DeleteAPIDocumentResponse, error) {
	args := m.Called(ctx, apiID, documentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteAPIDocumentResponse), args.Error(1)
}

// MockAppAuthStrategiesAPI implements helpers.AppAuthStrategiesAPI for testing
type MockAppAuthStrategiesAPI struct {
	mock.Mock
	t *testing.T
}

func NewMockAppAuthStrategiesAPI(t *testing.T) *MockAppAuthStrategiesAPI {
	return &MockAppAuthStrategiesAPI{t: t}
}

func (m *MockAppAuthStrategiesAPI) ListAppAuthStrategies(
	ctx context.Context,
	request kkOps.ListAppAuthStrategiesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAppAuthStrategiesResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListAppAuthStrategiesResponse), args.Error(1)
}

func (m *MockAppAuthStrategiesAPI) GetAppAuthStrategy(
	ctx context.Context,
	id string,
) (*kkOps.GetAppAuthStrategyResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.GetAppAuthStrategyResponse), args.Error(1)
}

func (m *MockAppAuthStrategiesAPI) CreateAppAuthStrategy(
	_ context.Context,
	_ kkComps.CreateAppAuthStrategyRequest,
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	// Return nil to simulate not implemented
	return nil, fmt.Errorf("CreateAppAuthStrategy not implemented in mock")
}

func (m *MockAppAuthStrategiesAPI) UpdateAppAuthStrategy(
	_ context.Context,
	_ string,
	_ kkComps.UpdateAppAuthStrategyRequest,
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	// Return nil to simulate not implemented
	return nil, fmt.Errorf("UpdateAppAuthStrategy not implemented in mock")
}

func (m *MockAppAuthStrategiesAPI) DeleteAppAuthStrategy(
	ctx context.Context,
	id string,
) (*kkOps.DeleteAppAuthStrategyResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteAppAuthStrategyResponse), args.Error(1)
}

// Helper method for MockAPIAPI to check if method has expectations
func (m *MockAPIAPI) HasExpectations() bool {
	return len(m.ExpectedCalls) > 0
}

// CreateManagedPortal creates a portal with KONGCTL labels
func CreateManagedPortal(name, id, description string) kkComps.Portal {
	descPtr := &description
	return kkComps.Portal{
		ID:          id,
		Name:        name,
		Description: descPtr,
		Labels: map[string]string{
			labels.NamespaceKey: "default",
		},
	}
}

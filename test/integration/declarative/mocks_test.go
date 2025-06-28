//go:build integration
// +build integration

package declarative_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
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
	request kkInternalOps.ListPortalsRequest,
) (*kkInternalOps.ListPortalsResponse, error) {
	// Default behavior: return empty list
	if !m.IsMethodCallRegistered("ListPortals") {
		return &kkInternalOps.ListPortalsResponse{
			ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
				Data: []kkInternalComps.Portal{},
				Meta: kkInternalComps.PaginatedMeta{
					Page: kkInternalComps.PageMeta{
						Total: 0,
					},
				},
			},
		}, nil
	}
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkInternalOps.ListPortalsResponse), args.Error(1)
}

func (m *MockPortalAPI) GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkInternalOps.GetPortalResponse), args.Error(1)
}

func (m *MockPortalAPI) CreatePortal(
	ctx context.Context,
	portal kkInternalComps.CreatePortal,
) (*kkInternalOps.CreatePortalResponse, error) {
	args := m.Called(ctx, portal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkInternalOps.CreatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkInternalComps.UpdatePortal,
) (*kkInternalOps.UpdatePortalResponse, error) {
	req := kkInternalOps.UpdatePortalRequest{
		PortalID:     id,
		UpdatePortal: portal,
	}
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkInternalOps.UpdatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) DeletePortal(
	ctx context.Context,
	id string,
	force bool,
) (*kkInternalOps.DeletePortalResponse, error) {
	args := m.Called(ctx, id, force)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkInternalOps.DeletePortalResponse), args.Error(1)
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
	opts ...kkOps.Option,
) (*kkOps.ListAppAuthStrategiesResponse, error) {
	// Default behavior: return empty list
	if !m.IsMethodCallRegistered("ListAppAuthStrategies") {
		// Return nil response to simulate no auth strategies
		return nil, nil
	}
	args := m.Called(ctx, request, opts)
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

// Helper to check if a method was registered with mock expectations
func (m *MockPortalAPI) IsMethodCallRegistered(method string) bool {
	for _, call := range m.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func (m *MockAppAuthStrategiesAPI) IsMethodCallRegistered(method string) bool {
	for _, call := range m.ExpectedCalls {
		if call.Method == method {
			return true
		}
	}
	return false
}

// CreateManagedPortal creates a portal with KONGCTL labels
func CreateManagedPortal(name, id, description string, configHash string) kkInternalComps.Portal {
	descPtr := &description
	return kkInternalComps.Portal{
		ID:          id,
		Name:        name,
		Description: descPtr,
		Labels: map[string]string{
			labels.ManagedKey:    "true",
			labels.ConfigHashKey: configHash,
		},
	}
}
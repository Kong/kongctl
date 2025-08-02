package planner

import (
	"context"
	"errors"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/mock"
)

// MockPortalAPI is a mock implementation of PortalAPI
type MockPortalAPI struct {
	mock.Mock
}

func (m *MockPortalAPI) ListPortals(
	ctx context.Context,
	req kkOps.ListPortalsRequest,
) (*kkOps.ListPortalsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListPortalsResponse), args.Error(1)
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
	args := m.Called(ctx, id, portal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) GetPortal(ctx context.Context, id string) (*kkOps.GetPortalResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.GetPortalResponse), args.Error(1)
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

// MockAPIAPI is a mock implementation of APIAPI
type MockAPIAPI struct {
	mock.Mock
}

func (m *MockAPIAPI) ListApis(
	ctx context.Context,
	req kkOps.ListApisRequest,
	_ ...kkOps.Option,
) (*kkOps.ListApisResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListApisResponse), args.Error(1)
}

func (m *MockAPIAPI) FetchAPI(
	ctx context.Context,
	apiID string,
	_ ...kkOps.Option,
) (*kkOps.FetchAPIResponse, error) {
	args := m.Called(ctx, apiID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.FetchAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) CreateAPI(
	ctx context.Context,
	request kkComps.CreateAPIRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateAPIResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) UpdateAPI(
	ctx context.Context,
	apiID string,
	request kkComps.UpdateAPIRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAPIResponse, error) {
	args := m.Called(ctx, apiID, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) DeleteAPI(
	ctx context.Context,
	apiID string,
	_ ...kkOps.Option,
) (*kkOps.DeleteAPIResponse, error) {
	args := m.Called(ctx, apiID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteAPIResponse), args.Error(1)
}

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

// MockAppAuthStrategiesAPI is a mock implementation of AppAuthStrategiesAPI
type MockAppAuthStrategiesAPI struct {
	mock.Mock
}

func (m *MockAppAuthStrategiesAPI) ListAppAuthStrategies(
	ctx context.Context,
	req kkOps.ListAppAuthStrategiesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAppAuthStrategiesResponse, error) {
	args := m.Called(ctx, req)
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
	ctx context.Context,
	strategy kkComps.CreateAppAuthStrategyRequest,
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	args := m.Called(ctx, strategy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateAppAuthStrategyResponse), args.Error(1)
}

func (m *MockAppAuthStrategiesAPI) UpdateAppAuthStrategy(
	ctx context.Context,
	id string,
	strategy kkComps.UpdateAppAuthStrategyRequest,
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	args := m.Called(ctx, id, strategy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateAppAuthStrategyResponse), args.Error(1)
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

func TestResolveReferences_PortalReference(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	// No mock needed - the auth strategy is being created in the same plan

	changes := []PlannedChange{
		{
			ID:           "1-c-auth",
			ResourceType: "application_auth_strategy",
			ResourceRef:  "basic-auth",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				"name": "Basic Auth",
			},
		},
		{
			ID:           "2-c-portal",
			ResourceType: "portal",
			ResourceRef:  "dev-portal",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				"name":                                "Dev Portal",
				"default_application_auth_strategy_id": "basic-auth",
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Check that we have references for the portal change
	changeRefs, ok := result.ChangeReferences["2-c-portal"]
	if !ok {
		t.Fatal("Expected references for change 2-c-portal")
	}

	// Check the auth strategy reference
	authRef, ok := changeRefs["default_application_auth_strategy_id"]
	if !ok {
		t.Fatal("Expected reference for default_application_auth_strategy_id")
	}

	if authRef.Ref != "basic-auth" {
		t.Errorf("Expected ref 'basic-auth', got %s", authRef.Ref)
	}

	// Should be "[unknown]" since it's being created in this plan
	if authRef.ID != "[unknown]" {
		t.Errorf("Expected ID '<unknown>' for in-plan reference, got %s", authRef.ID)
	}

	// Should have one error for unimplemented auth strategy resolution
	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors for in-plan reference, got %d", len(result.Errors))
	}

	mockAPI.AssertExpectations(t)
}

func TestResolveReferences_ExistingPortal(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	// Mock the ListPortals call
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:   "portal-existing-123",
					Name: "existing-portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	changes := []PlannedChange{
		{
			ID:           "1-u-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionUpdate,
			Fields: map[string]interface{}{
				"portal_id": "existing-portal",
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Check that we have references for the api change
	changeRefs, ok := result.ChangeReferences["1-u-api"]
	if !ok {
		t.Fatal("Expected references for change 1-u-api")
	}

	// Check the portal reference
	portalRef, ok := changeRefs["portal_id"]
	if !ok {
		t.Fatal("Expected reference for portal_id")
	}

	if portalRef.Ref != "existing-portal" {
		t.Errorf("Expected ref 'existing-portal', got %s", portalRef.Ref)
	}

	if portalRef.ID != "portal-existing-123" {
		t.Errorf("Expected ID 'portal-existing-123', got %s", portalRef.ID)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}

	mockAPI.AssertExpectations(t)
}

func TestResolveReferences_MissingPortal(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	// Mock empty ListPortals response
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 0,
				},
			},
		},
	}, nil)

	changes := []PlannedChange{
		{
			ID:           "1-c-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				"portal_id": "non-existent",
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Should have one error for missing portal
	if len(result.Errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(result.Errors))
	}

	expectedErr := "change 1-c-api: failed to resolve portal reference \"non-existent\": portal not found"
	if result.Errors[0].Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, result.Errors[0].Error())
	}

	mockAPI.AssertExpectations(t)
}

func TestResolveReferences_UUIDSkipped(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	changes := []PlannedChange{
		{
			ID:           "1-c-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				// This is already a UUID, should not be treated as a reference
				"portal_id": "12345678-1234-5678-1234-567812345678",
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Should have no references since UUID is not a ref
	if len(result.ChangeReferences) != 0 {
		t.Errorf("Expected no references, got %d", len(result.ChangeReferences))
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}

	// Should not have called ListPortals
	mockAPI.AssertNotCalled(t, "ListPortals")
}

func TestResolveReferences_FieldChange(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	// Mock the ListPortals call
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:   "portal-456",
					Name: "new-portal",
					Labels: map[string]string{
						labels.NamespaceKey: "default",
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{
					Total: 1,
				},
			},
		},
	}, nil)

	changes := []PlannedChange{
		{
			ID:           "1-u-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionUpdate,
			Fields: map[string]interface{}{
				"portal_id": FieldChange{
					Old: "old-portal",
					New: "new-portal",
				},
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Check that we have references for the api change
	changeRefs, ok := result.ChangeReferences["1-u-api"]
	if !ok {
		t.Fatal("Expected references for change 1-u-api")
	}

	// Check the portal reference
	portalRef, ok := changeRefs["portal_id"]
	if !ok {
		t.Fatal("Expected reference for portal_id")
	}

	if portalRef.Ref != "new-portal" {
		t.Errorf("Expected ref 'new-portal', got %s", portalRef.Ref)
	}

	if portalRef.ID != "portal-456" {
		t.Errorf("Expected ID 'portal-456', got %s", portalRef.ID)
	}

	mockAPI.AssertExpectations(t)
}

func TestResolveReferences_UnimplementedTypes(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	changes := []PlannedChange{
		{
			ID:           "1-c-portal",
			ResourceType: "portal",
			ResourceRef:  "my-portal",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				"default_application_auth_strategy_id": "auth-ref",
			},
		},
		{
			ID:           "2-c-service",
			ResourceType: "service",
			ResourceRef:  "my-service",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				"control_plane_id": "cp-ref",
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Should have 2 errors for unimplemented types
	if len(result.Errors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(result.Errors))
	}

	// Check error messages
	expectedAuthErr := "change 1-c-portal: failed to resolve application_auth_strategy " +
		"reference \"auth-ref\": auth strategy resolution not yet implemented"
	if result.Errors[0].Error() != expectedAuthErr {
		t.Errorf("Expected error %q, got %q", expectedAuthErr, result.Errors[0].Error())
	}

	expectedCPErr := "change 2-c-service: failed to resolve control_plane " +
		"reference \"cp-ref\": control plane resolution not yet implemented"
	if result.Errors[1].Error() != expectedCPErr {
		t.Errorf("Expected error %q, got %q", expectedCPErr, result.Errors[1].Error())
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"12345678-1234-5678-1234-567812345678", true},
		{"a0b1c2d3-e4f5-6789-abcd-ef0123456789", true},
		{"not-a-uuid", false},
		{"12345678-1234-5678-1234", false}, // Too short
		{"12345678-1234-5678-1234-567812345678-extra", false}, // Too long
		{"12345678_1234_5678_1234_567812345678", false}, // Wrong separator
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isUUID(tt.input)
			if result != tt.expected {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveReferences_NetworkError(t *testing.T) {
	ctx := context.Background()
	mockAPI := new(MockPortalAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	resolver := NewReferenceResolver(client)

	// Mock network error
	mockAPI.On("ListPortals", ctx, mock.Anything).Return(nil, errors.New("network error"))

	changes := []PlannedChange{
		{
			ID:           "1-c-api",
			ResourceType: "api",
			ResourceRef:  "my-api",
			Action:       ActionCreate,
			Fields: map[string]interface{}{
				"portal_id": "some-portal",
			},
		},
	}

	result, err := resolver.ResolveReferences(ctx, changes)
	if err != nil {
		t.Fatalf("ResolveReferences failed: %v", err)
	}

	// Should have one error
	if len(result.Errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(result.Errors))
	}

	// Error should mention network error
	if !containsSubstring(result.Errors[0].Error(), "network error") {
		t.Errorf("Expected error to contain 'network error', got %q", result.Errors[0].Error())
	}

	mockAPI.AssertExpectations(t)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

// Ensure interfaces are implemented
var (
	_ helpers.PortalAPI = (*MockPortalAPI)(nil)
)